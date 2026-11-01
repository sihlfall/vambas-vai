package parser

import (
	"strings"

	"sihlfall.ch/vambas-vai/ast"
)

// Member and argument names admit statement/marker keywords as well as the
// contextual identifiers permitted in ordinary expressions by the VBA grammar.
func isUnrestrictedIdentifierToken(kind ast.Kind) bool {
	return kind == ast.KindIdentifier || kind == ast.KindPROPERTY || kind >= ast.KindABS && kind <= ast.KindUBOUND ||
		kind >= ast.KindACCESS && kind <= ast.KindXOR
}

func (p *Parser) parseBracketedIdentifier() *ast.Node {
	pos, start := p.nodePos, p.scanner.TokenEnd()
	p.nextToken()
	depth := 1
	for p.token != ast.KindEndOfFile && p.token != ast.KindNEWLINE {
		if p.token == ast.KindR_SQUARE_BRACKET {
			depth--
			if depth == 0 {
				break
			}
		} else if p.token == ast.KindL_SQUARE_BRACKET {
			depth++
		}
		// Preserve an enclosing procedure boundary in a damaged foreign name.
		if p.token == ast.KindEND {
			next := p.lookAheadToken()
			if next == ast.KindSUB || next == ast.KindFUNCTION {
				break
			}
		}
		p.nextToken()
	}
	text := strings.TrimSpace(p.scanner.Input[start:p.scanner.TokenStart()])
	if text == "" {
		p.parseErrorAtCurrentToken(ast.DiagnosticIdentifierExpected, "Identifier expected inside brackets")
	}
	p.parseExpected(ast.KindR_SQUARE_BRACKET)
	return p.finishNode(p.factory.NewIdentifier(p.internIdentifier(text)), pos)
}

// These layers follow typescript-go's primary/member/call expression parsing.
// VBA applies parentheses for both calls and indexing, so there is no synthetic
// distinction between f(i) and a(i) before resolution.
func (p *Parser) parseLeftHandSideExpressionOrHigher(statement bool) *ast.Node {
	expression := p.parsePrimaryExpression()
	return p.parseCallExpressionRest(expression.Pos(), expression, statement)
}

func (p *Parser) parseMemberExpressionRest(pos int, expression *ast.Node, statement bool) *ast.Node {
	for p.token == ast.KindDOT || p.token == ast.KindEXCLAMATIONPOINT {
		// In an implicit statement call, whitespace starts an argument. A
		// leading dot/bang then belongs to the enclosing With, not the callee.
		if statement && expression != nil && p.hasCallArgumentWhitespace() {
			return expression
		}
		operator := p.parseTokenNode()
		name := p.parseIdentifierWorker(true)
		expression = p.finishNode(p.factory.NewPropertyAccessExpression(expression, operator, name), pos)
	}
	return expression
}

func (p *Parser) hasCallArgumentWhitespace() bool {
	trivia := p.scanner.Input[p.nodePos:p.scanner.TokenStart()]
	return len(trivia) != 0 && !strings.ContainsAny(trivia, "\r\n")
}

func (p *Parser) parseCallExpressionRest(pos int, expression *ast.Node, statement bool) *ast.Node {
	for {
		expression = p.parseMemberExpressionRest(pos, expression, statement)
		if p.token != ast.KindLPAREN {
			return expression
		}
		// In an implicit statement, F (x) passes the parenthesized expression
		// x. Keep that wrapper: in VBA it can force ByVal argument evaluation.
		// The grammar's whitespaceIndexExpr permits an explicit continuation
		// before an index suffix; ordinary whitespace starts statement arguments.
		if statement && p.hasCallArgumentWhitespace() {
			return expression
		}
		arguments := p.parseArgumentList(true)
		expression = p.finishNode(p.factory.NewCallExpression(expression, arguments, true), pos)
	}
}

func (p *Parser) isArgumentListEnd() bool {
	if p.isNamedArgument() {
		return false
	}
	return p.token == ast.KindRPAREN || p.token == ast.KindTHEN || p.isRecoveryBoundary()
}

func (p *Parser) isNamedArgument() bool {
	if isUnrestrictedIdentifierToken(p.token) {
		return p.lookAheadToken() == ast.KindASSIGN
	}
	if p.token != ast.KindL_SQUARE_BRACKET {
		return false
	}
	state := *p.scanner
	state.SetOnError(nil)
	depth := 1
	for depth > 0 {
		switch state.Scan(0) {
		case ast.KindL_SQUARE_BRACKET:
			depth++
		case ast.KindR_SQUARE_BRACKET:
			depth--
		case ast.KindNEWLINE, ast.KindEndOfFile:
			return false
		}
	}
	return state.Scan(0) == ast.KindASSIGN
}

func (p *Parser) parseArgumentExpression() *ast.Node {
	if p.token == ast.KindBYVAL || p.token == ast.KindADDRESSOF {
		pos, operator := p.nodePos, p.token
		p.nextToken()
		return p.finishNode(p.factory.NewPrefixUnaryExpression(operator, p.parseExpression()), pos)
	}
	return p.parseExpression()
}

func (p *Parser) parseArgument() *ast.Node {
	if p.token == ast.KindCOMMA || p.isArgumentListEnd() {
		return p.finishNodeWithEnd(p.factory.NewOmittedExpression(), p.nodePos, p.nodePos)
	}
	if p.isNamedArgument() {
		pos := p.nodePos
		name := p.parseIdentifierWorker(true)
		p.parseExpected(ast.KindASSIGN)
		value := p.parseArgumentExpression()
		return p.finishNode(p.factory.NewNamedArgument(name, value), pos)
	}
	return p.parseArgumentExpression()
}

func (p *Parser) parseArgumentList(parentheses bool) *ast.NodeList {
	if parentheses {
		p.parseExpected(ast.KindLPAREN)
	}
	pos := p.nodePos
	var arguments []*ast.Node
	seenNamed := false
	names := make(map[string]bool)
	for !p.isArgumentListEnd() {
		before := p.scanner.TokenStart()
		argument := p.parseArgument()
		arguments = append(arguments, argument)
		if argument.Kind == ast.KindNamedArgument {
			seenNamed = true
			key := ast.SymbolNameKey(argument.AsNamedArgument().Name.AsIdentifier().Text)
			if names[key] {
				p.parseErrorAtNode(argument, ast.DiagnosticInvalidArgument, "Duplicate named argument")
			}
			names[key] = true
		} else if seenNamed {
			p.parseErrorAtNode(argument, ast.DiagnosticInvalidArgument, "Positional arguments cannot follow named arguments")
		}
		if p.token == ast.KindCOMMA {
			p.nextToken()
			if p.isArgumentListEnd() {
				omitted := p.parseArgument()
				if seenNamed {
					p.parseErrorAtNode(omitted, ast.DiagnosticInvalidArgument, "An omitted argument cannot follow named arguments")
				}
				arguments = append(arguments, omitted)
				break
			}
			continue
		}
		if p.isArgumentListEnd() {
			break
		}
		p.parseExpected(ast.KindCOMMA)
		if p.scanner.TokenStart() == before {
			p.nextToken()
		}
		for p.token != ast.KindCOMMA && !p.isArgumentListEnd() {
			p.nextToken()
		}
		if p.token == ast.KindCOMMA {
			p.nextToken()
		}
	}
	list := p.newNodeList(arguments, pos, p.nodePos)
	if parentheses {
		p.parseExpected(ast.KindRPAREN)
	}
	return list
}

func (p *Parser) finishCallStatement(pos int, keyword, target *ast.Node) *ast.Node {
	if !p.isArgumentListEnd() {
		if keyword != nil {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidCall, "Arguments to an explicit Call must be parenthesized")
		} else if p.scanner.TokenStart() == p.nodePos {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidCall, "Implicit call arguments must be separated from the target by whitespace")
		}
		arguments := p.parseArgumentList(false)
		target = p.finishNode(p.factory.NewCallExpression(target, arguments, false), target.Pos())
	} else if target.Kind != ast.KindCallExpression {
		arguments := p.newNodeList(nil, p.nodePos, p.nodePos)
		target = p.finishNode(p.factory.NewCallExpression(target, arguments, false), target.Pos())
	}
	return p.finishNode(p.factory.NewCallStatement(keyword, target), pos)
}

func (p *Parser) parseCallStatement() *ast.Node {
	pos := p.nodePos
	keyword := p.parseTokenNode()
	if p.scanner.TokenStart() == p.nodePos {
		p.parseErrorAtCurrentToken(ast.DiagnosticInvalidCall, "Call must be followed by whitespace and a target")
	}
	target := p.parseLeftHandSideExpressionOrHigher(false)
	return p.finishCallStatement(pos, keyword, target)
}

func (p *Parser) parseAssignmentOrCallStatement() *ast.Node {
	pos := p.nodePos
	target := p.parseLeftHandSideExpressionOrHigher(true)
	if p.token == ast.KindEQ {
		p.nextToken()
		right := p.parseExpression()
		return p.finishNode(p.factory.NewLetStatement(target, right), pos)
	}
	return p.finishCallStatement(pos, nil, target)
}

func (p *Parser) parseNewExpression() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	// VBA New takes a type expression rather than a JavaScript constructor
	// value and argument list. Keep qualification in the existing type syntax.
	name := p.parseIdentifier()
	for p.token == ast.KindDOT {
		p.nextToken()
		right := p.parseIdentifier()
		name = p.finishNode(p.factory.NewQualifiedName(name, right), name.Pos())
	}
	return p.finishNode(p.factory.NewNewExpression(name), pos)
}
