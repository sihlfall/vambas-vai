package parser

import (
	"strings"

	"sihlfall.ch/vambas-vai/ast"
)

// Recognize the statement special form without changing expression/call parsing
// (including a user's ordinary procedure named Mid and implicit call parentheses).
func (p *Parser) isMidAssignment() bool {
	name := strings.ToLower(p.scanner.Input[p.scanner.TokenStart():p.scanner.TokenEnd()])
	if name != "mid" && name != "mid$" {
		return false
	}
	state := *p.scanner
	state.SetOnError(nil)
	kind := state.Scan(0)
	if kind == ast.KindDOLLAR {
		kind = state.Scan(0)
	}
	if kind != ast.KindLPAREN {
		return false
	}
	depth := 1
	for depth > 0 {
		switch state.Scan(0) {
		case ast.KindLPAREN:
			depth++
		case ast.KindRPAREN:
			depth--
		case ast.KindEndOfFile, ast.KindNEWLINE, ast.KindCOLON:
			return false
		}
	}
	return state.Scan(0) == ast.KindEQ
}

func (p *Parser) parseMidAssignment() *ast.Node {
	pos := p.nodePos
	mode := p.parseIdentifierWorker(true)
	p.parseExpected(ast.KindLPAREN)
	target := p.parseExpression()
	p.parseExpected(ast.KindCOMMA)
	start := p.parseExpression()
	var length *ast.Node
	if p.token == ast.KindCOMMA {
		p.nextToken()
		length = p.parseExpression()
	}
	p.parseExpected(ast.KindRPAREN)
	p.parseExpected(ast.KindEQ)
	expression := p.parseExpression()
	return p.finishNode(p.factory.NewMidAssignmentStatement(mode, target, start, length, expression), pos)
}
