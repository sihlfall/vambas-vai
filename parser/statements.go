package parser

import (
	"sihlfall.ch/vambas-vai/ast"
	"strings"
)

func (p *Parser) parseSetStatement() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	left := p.parseLExpression()
	p.parseExpected(ast.KindEQ)
	right := p.parseExpression()
	return p.finishNode(p.factory.NewSetStatement(left, right), pos)
}

func (p *Parser) parseWithStatement() *ast.Node {
	if !p.enterRecursion() {
		return nil
	}
	defer p.leaveRecursion()
	pos := p.nodePos
	p.nextToken()
	expression := p.parseExpression()
	p.parseEndOfStatement()
	body := p.parseBlock(pcWithStatements)
	if p.token == ast.KindEND && p.lookAheadToken() == ast.KindWITH {
		p.nextToken()
		p.nextToken()
	} else {
		p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "End With expected")
	}
	return p.finishNode(p.factory.NewWithStatement(expression, body), pos)
}

func (p *Parser) parseWhileStatement() *ast.Node {
	if !p.enterRecursion() {
		return nil
	}
	defer p.leaveRecursion()
	pos := p.nodePos
	p.nextToken()
	condition := p.parseExpression()
	p.parseEndOfStatement()
	body := p.parseBlock(pcWhileStatements)
	p.parseExpected(ast.KindWEND)
	return p.finishNode(p.factory.NewWhileStatement(condition, body), pos)
}

func (p *Parser) parseDoCondition() (ast.Kind, *ast.Node) {
	if p.token != ast.KindWHILE && p.token != ast.KindUNTIL {
		return ast.KindNONE, nil
	}
	kind := p.token
	p.nextToken()
	return kind, p.parseExpression()
}

func (p *Parser) parseDoStatement() *ast.Node {
	if !p.enterRecursion() {
		return nil
	}
	defer p.leaveRecursion()
	pos := p.nodePos
	p.nextToken()
	kind, condition := p.parseDoCondition()
	p.parseEndOfStatement()
	body := p.parseBlock(pcDoStatements)
	p.parseExpected(ast.KindLOOP)
	atEnd := false
	if p.token == ast.KindWHILE || p.token == ast.KindUNTIL {
		if condition != nil {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidControlFlow, "Do cannot have conditions at both ends")
			// Keep the first condition; consume the invalid second annotation.
			p.parseDoCondition()
		} else {
			kind, condition = p.parseDoCondition()
			atEnd = true
		}
	}
	return p.finishNode(p.factory.NewDoStatement(condition, body, kind, atEnd), pos)
}

func (p *Parser) parseNextVariable() *ast.Node {
	if p.pendingNext && p.token == ast.KindCOMMA {
		p.pendingNext = false
		p.nextToken()
	} else if !p.parseExpected(ast.KindNEXT) {
		return nil
	}
	var variable *ast.Node
	if !p.isRecoveryBoundary() && p.token != ast.KindCOMMA {
		variable = p.parseLExpression()
	}
	p.pendingNext = p.token == ast.KindCOMMA
	return variable
}

func (p *Parser) parseForStatement() *ast.Node {
	if !p.enterRecursion() {
		return nil
	}
	defer p.leaveRecursion()
	pos := p.nodePos
	p.nextToken()
	each := p.token == ast.KindEACH
	if each {
		p.nextToken()
	}
	variable := p.parseLExpression()
	var initializer, expression, step *ast.Node
	if each {
		initializer = variable
		p.parseExpected(ast.KindIN)
		expression = p.parseExpression()
	} else {
		p.parseExpected(ast.KindEQ)
		start := p.parseExpression()
		initializer = p.finishNode(p.factory.NewLetStatement(variable, start), variable.Pos())
		p.parseExpected(ast.KindTO)
		expression = p.parseExpression()
		if p.token == ast.KindSTEP {
			p.nextToken()
			step = p.parseExpression()
		}
	}
	p.parseEndOfStatement()
	body := p.parseBlock(pcForStatements)
	next := p.parseNextVariable()
	if each {
		return p.finishNode(p.factory.NewForEachStatement(initializer, expression, body, next), pos)
	}
	return p.finishNode(p.factory.NewForStatement(initializer, expression, step, body, next), pos)
}

func (p *Parser) parseExitStatement() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	kind := p.token
	switch kind {
	case ast.KindDO, ast.KindFOR, ast.KindFUNCTION, ast.KindSUB, ast.KindPROPERTY:
		p.nextToken()
	default:
		kind = ast.KindNONE
		p.parseErrorAtCurrentToken(ast.DiagnosticInvalidControlFlow, "Expected Do, For, Function, Sub, or Property after Exit")
		if !p.isRecoveryBoundary() {
			p.nextToken()
		}
	}
	return p.finishNode(p.factory.NewExitStatement(kind), pos)
}

func isCaseComparison(kind ast.Kind) bool {
	switch kind {
	case ast.KindEQ, ast.KindNEQ, ast.KindLT, ast.KindLEQ, ast.KindGT, ast.KindGEQ, ast.KindLIKE:
		return true
	}
	return false
}

func (p *Parser) parseCaseExpression() *ast.Node {
	pos := p.nodePos
	var is *ast.Node
	if p.token == ast.KindIS {
		is = p.parseTokenNode()
	}
	if is != nil || isCaseComparison(p.token) {
		var operator *ast.Node
		if isCaseComparison(p.token) {
			operator = p.parseTokenNode()
		} else {
			p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Case comparison operator expected")
		}
		expression := p.parseExpression()
		return p.finishNode(p.factory.NewCaseComparisonClause(is, operator, expression), pos)
	}
	lower := p.parseExpression()
	if p.token == ast.KindTO {
		p.nextToken()
		upper := p.parseExpression()
		return p.finishNode(p.factory.NewCaseRangeClause(lower, upper), pos)
	}
	return lower
}

func (p *Parser) parseOnGotoStatement() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	if p.token == ast.KindERROR {
		return p.parseErrorPolicy(pos)
	}
	expression := p.parseExpression()
	spaced := p.isSpacedGosub()
	gosub := p.token == ast.KindGOSUB || spaced
	if spaced {
		p.nextToken()
	}
	if gosub {
		p.nextToken()
	} else {
		p.parseExpected(ast.KindGOTO)
	}
	destinations := p.parseDelimitedList(ast.KindNEWLINE, false, func() *ast.Node {
		return p.parseGotoTarget(p.nodePos)
	})
	node := p.factory.NewOnGotoStatement(expression, destinations)
	if gosub {
		node.Kind = ast.KindOnGosubStatement
	}
	return p.finishNode(node, pos)
}

func (p *Parser) parseSelectStatement() *ast.Node {
	if !p.enterRecursion() {
		return nil
	}
	defer p.leaveRecursion()
	pos := p.nodePos
	p.nextToken()
	p.parseExpected(ast.KindCASE)
	expression := p.parseExpression()
	p.parseEndOfStatement()
	blockPos := p.nodePos
	var clauses []*ast.Node
	seenDefault := false
	for {
		if p.token == ast.KindNEWLINE || p.token == ast.KindCOLON {
			p.nextToken()
			continue
		}
		if p.token != ast.KindCASE {
			break
		}
		clausePos := p.nodePos
		p.nextToken()
		var expressions *ast.NodeList
		if seenDefault {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidControlFlow, "Case Else must be the last clause")
		}
		if p.token == ast.KindELSE {
			seenDefault = true
			p.nextToken()
		} else {
			expressions = p.parseDelimitedList(ast.KindNEWLINE, false, p.parseCaseExpression)
		}
		p.parseEndOfStatement()
		body := p.parseBlock(pcCaseStatements)
		clauses = append(clauses, p.finishNode(p.factory.NewCaseClause(expressions, body), clausePos))
	}
	list := p.newNodeList(clauses, blockPos, p.nodePos)
	block := p.finishNode(p.factory.NewCaseBlock(list), blockPos)
	if p.token == ast.KindEND && p.lookAheadToken() == ast.KindSELECT {
		p.nextToken()
		p.nextToken()
	} else {
		p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "End Select expected")
	}
	return p.finishNode(p.factory.NewSelectStatement(expression, block), pos)
}

func (p *Parser) parseErrorPolicy(pos int) *ast.Node {
	p.nextToken() // Error
	if p.token == ast.KindRESUME {
		p.nextToken()
		p.parseExpected(ast.KindNEXT)
		return p.finishNode(p.factory.NewOnErrorStatement(3, nil), pos)
	}
	p.parseExpected(ast.KindGOTO)
	if p.token == ast.KindMINUS {
		p.nextToken()
		if p.token == ast.KindNumericLiteral && p.scanner.TokenValue == "1" {
			p.nextToken()
			return p.finishNode(p.factory.NewOnErrorStatement(4, nil), pos)
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Expected -1")
	}
	if p.token == ast.KindNumericLiteral && p.scanner.TokenValue == "0" {
		p.nextToken()
		return p.finishNode(p.factory.NewOnErrorStatement(1, nil), pos)
	}
	target := p.parseGotoTarget(p.nodePos)
	return p.finishNode(p.factory.NewOnErrorStatement(2, target), pos)
}
func (p *Parser) parseResumeStatement() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	mode := 0
	var target *ast.Node
	if p.token == ast.KindNEXT {
		mode = 1
		p.nextToken()
	} else if p.token == ast.KindNumericLiteral && p.scanner.TokenValue == "0" {
		p.nextToken()
	} else if !p.isRecoveryBoundary() {
		mode = 2
		target = p.parseGotoTarget(p.nodePos)
	}
	return p.finishNode(p.factory.NewResumeStatement(mode, target), pos)
}

func (p *Parser) isSpacedGosub() bool {
	return p.token == ast.KindIdentifier && strings.EqualFold(p.tokenValue, "Go") && p.lookAheadToken() == ast.KindSUB
}
