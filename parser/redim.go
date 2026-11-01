package parser

import "sihlfall.ch/vambas-vai/ast"

func (p *Parser) parseReDimStatement() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	var preserve *ast.Node
	if p.token == ast.KindPRESERVE {
		preserve = p.parseTokenNode()
	}
	listPos := p.nodePos
	var declarations []*ast.Node
	for {
		before := p.scanner.TokenStart()
		declarations = append(declarations, p.parseReDimDeclaration())
		if p.token != ast.KindCOMMA {
			break
		}
		p.nextToken()
		if p.scanner.TokenStart() == before {
			break
		}
	}
	return p.finishNode(p.factory.NewReDimStatement(preserve, p.newNodeList(declarations, listPos, p.nodePos)), pos)
}

func (p *Parser) parseReDimDeclaration() *ast.Node {
	pos := p.nodePos
	var target *ast.Node
	if p.token == ast.KindIdentifier || p.token == ast.KindL_SQUARE_BRACKET {
		target = p.parseDeclarationName()
	} else {
		target = p.parsePrimaryExpression()
	}
	var dimensions *ast.NodeList
	for {
		target = p.parseMemberExpressionRest(target.Pos(), target, false)
		if target.Kind == ast.KindPropertyAccessExpression && target.End() == p.scanner.TokenStart() && typeHintText(p.token) != "" {
			name := target.AsPropertyAccessExpression().Name
			name.AsIdentifier().Text += typeHintText(p.token)
			name.AsIdentifier().TypeHint = p.token
			p.nextToken()
			p.finishNode(name, name.Pos())
			p.finishNode(target, target.Pos())
		}
		if p.token != ast.KindLPAREN {
			p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Array bounds in parentheses expected after ReDim target")
			dimensions = p.newNodeList(nil, p.nodePos, p.nodePos)
			break
		}
		dimensions = p.parseReDimParentheses()
		if p.token != ast.KindDOT && p.token != ast.KindEXCLAMATIONPOINT && p.token != ast.KindLPAREN {
			if len(dimensions.Nodes) == 0 {
				p.parseErrorAtCurrentToken(ast.DiagnosticInvalidArgument, "ReDim requires at least one bound")
			}
			for i, dimension := range dimensions.Nodes {
				if dimension.Kind != ast.KindArrayDimension {
					p.parseErrorAtNode(dimension, ast.DiagnosticInvalidArgument, "ReDim bounds cannot be named or omitted arguments")
					dimensions.Nodes[i] = p.finishNodeWithEnd(p.factory.NewArrayDimension(nil, dimension), dimension.Pos(), dimension.End())
				} else if upper := dimension.AsArrayDimension().UpperBound; upper.Kind == ast.KindPrefixUnaryExpression && upper.AsPrefixUnaryExpression().Operator == ast.KindBYVAL {
					p.parseErrorAtNode(upper, ast.DiagnosticInvalidArgument, "ByVal is not valid in ReDim bounds")
				}
			}
			break
		}
		// A receiver application precedes the final allocation bounds. Restore
		// its normal call/index shape without inventing a call for ReDim itself.
		args := make([]*ast.Node, 0, len(dimensions.Nodes))
		for _, dimension := range dimensions.Nodes {
			if dimension.Kind != ast.KindArrayDimension {
				args = append(args, dimension)
				continue
			}
			d := dimension.AsArrayDimension()
			if d.LowerBound != nil {
				p.parseErrorAtNode(dimension, ast.DiagnosticInvalidArgument, "To bounds are only valid in the final ReDim parentheses")
				args = append(args, dimension)
			} else {
				args = append(args, d.UpperBound)
			}
		}
		target = p.finishNode(p.factory.NewCallExpression(target, p.newNodeList(args, dimensions.Loc.Pos(), dimensions.Loc.End()), true), target.Pos())
	}
	typeNode := p.parseTypeAnnotation(false)
	return p.finishNode(p.factory.NewReDimDeclaration(target, dimensions, typeNode), pos)
}

// Until the following suffix is known, parentheses can be a receiver's normal
// argument list or the final allocation bounds. This keeps named/omitted receiver
// arguments in the existing expression nodes instead of losing their syntax.
func (p *Parser) parseReDimParentheses() *ast.NodeList {
	p.parseExpected(ast.KindLPAREN)
	list := p.parseDelimitedList(ast.KindRPAREN, true, func() *ast.Node {
		if p.isNamedArgument() || p.token == ast.KindCOMMA || p.isArgumentListEnd() {
			return p.parseArgument()
		}
		pos := p.nodePos
		upper := p.parseArgumentExpression()
		var lower *ast.Node
		if p.token == ast.KindTO {
			lower = upper
			p.nextToken()
			upper = p.parseExpression()
		}
		return p.finishNode(p.factory.NewArrayDimension(lower, upper), pos)
	})
	p.parseExpected(ast.KindRPAREN)
	return list
}
