package parser

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
)

// These list contexts are the VBA counterpart of typescript-go's ParsingContext.
// An enclosing procedure header or block terminator is retained for its caller.
type parsingContext uint8

const (
	pcFunctionStatements parsingContext = iota
	pcIfStatements
	pcSingleLineStatements
	pcForStatements
	pcDoStatements
	pcWhileStatements
	pcCaseStatements
	pcWithStatements
)

func (p *Parser) Diagnostics() []*ast.Diagnostic { return p.diagnostics }

func (p *Parser) scanError(code ast.DiagnosticCode, pos, length int, message string) {
	p.parseErrorAtRange(core.NewTextRange(pos, pos+length), code, message)
}

func (p *Parser) parseErrorAtCurrentToken(code ast.DiagnosticCode, message string) {
	p.parseErrorAtRange(p.scanner.TokenRange(), code, message)
}

func (p *Parser) parseErrorAtNode(node *ast.Node, code ast.DiagnosticCode, message string) {
	p.parseErrorAtRange(node.Loc, code, message)
	node.Flags |= ast.NodeFlagsThisNodeHasError | ast.NodeFlagsThisNodeOrAnySubNodesHasError
	p.hasParseError = false
}

func (p *Parser) parseErrorAtRange(loc core.TextRange, code ast.DiagnosticCode, message string) {
	// Avoid cascades at the same token, including a scanner and parser error.
	if len(p.diagnostics) == 0 || p.diagnostics[len(p.diagnostics)-1].Pos() != loc.Pos() {
		p.diagnostics = append(p.diagnostics, &ast.Diagnostic{Loc: loc, Code: code, Message: message})
	}
	p.hasParseError = true
}

func (p *Parser) parseExpected(kind ast.Kind) bool {
	if p.token == kind {
		p.nextToken()
		return true
	}
	text := map[ast.Kind]string{
		ast.KindFUNCTION: "Function", ast.KindSUB: "Sub", ast.KindTHEN: "Then", ast.KindIF: "If",
		ast.KindEQ: "=", ast.KindLPAREN: "(", ast.KindRPAREN: ")", ast.KindCOLON: ":",
		ast.KindCOMMA: ",", ast.KindR_SQUARE_BRACKET: "]",
		ast.KindTO: "To", ast.KindIN: "In", ast.KindNEXT: "Next", ast.KindLOOP: "Loop",
		ast.KindWEND: "Wend", ast.KindCASE: "Case", ast.KindSELECT: "Select",
		ast.KindGOTO: "GoTo",
	}[kind]
	p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, text+" expected")
	return false
}

func (p *Parser) createMissingIdentifier() *ast.Node {
	return p.finishNodeWithEnd(p.factory.NewIdentifier(""), p.nodePos, p.nodePos)
}

func (p *Parser) finishNode(node *ast.Node, pos int) *ast.Node {
	return p.finishNodeWithEnd(node, pos, p.nodePos)
}

func (p *Parser) finishNodeWithEnd(node *ast.Node, pos, end int) *ast.Node {
	node.Loc = core.NewTextRange(pos, end)
	if p.hasParseError {
		node.Flags |= ast.NodeFlagsThisNodeHasError
		p.hasParseError = false
	}
	if node.Flags&ast.NodeFlagsThisNodeHasError != 0 {
		node.Flags |= ast.NodeFlagsThisNodeOrAnySubNodesHasError
	}
	node.ForEachChild(func(child *ast.Node) bool {
		child.Parent = node
		if child.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
			node.Flags |= ast.NodeFlagsThisNodeOrAnySubNodesHasError
		}
		return false
	})
	return node
}

func (p *Parser) newNodeList(nodes []*ast.Node, pos, end int) *ast.NodeList {
	list := p.factory.NewNodeList(p.nodeSlicePool.Clone(nodes))
	list.Loc = core.NewTextRange(pos, end)
	return list
}

func (p *Parser) emptyBlock() *ast.Node {
	pos := p.nodePos
	return p.finishNodeWithEnd(p.factory.NewBlock(p.newNodeList(nil, pos, pos)), pos, pos)
}

func (p *Parser) lookAheadToken() ast.Kind {
	state := *p.scanner
	state.SetOnError(nil)
	return state.Scan(0)
}

func (p *Parser) isFunctionStart() bool {
	switch p.token {
	case ast.KindFUNCTION, ast.KindSUB:
		return true
	case ast.KindPROPERTY:
		return isAccessorKeyword(p.lookAheadToken())
	case ast.KindSTATIC, ast.KindPUBLIC, ast.KindPRIVATE, ast.KindFRIEND, ast.KindGLOBAL:
		state := *p.scanner
		state.SetOnError(nil)
		kind := state.Scan(0)
		if kind == ast.KindSTATIC {
			kind = state.Scan(0)
		}
		return kind == ast.KindFUNCTION || kind == ast.KindSUB || kind == ast.KindPROPERTY && isAccessorKeyword(state.Scan(0))
	}
	return false
}

func (p *Parser) isListTerminator(context parsingContext) bool {
	if p.token == ast.KindEndOfFile || p.isFunctionStart() {
		return true
	}
	if p.isEndBlock() {
		next := p.lookAheadToken()
		return context != pcFunctionStatements || next == ast.KindFUNCTION || next == ast.KindSUB || next == ast.KindPROPERTY
	}
	if context != pcFunctionStatements && (p.isStructuredTerminator() || p.pendingNext && p.token == ast.KindCOMMA) {
		return true
	}
	return context == pcSingleLineStatements && p.token == ast.KindNEWLINE
}

func (p *Parser) isRecoveryBoundary() bool {
	return p.token == ast.KindNEWLINE || p.token == ast.KindCOLON || p.token == ast.KindEndOfFile ||
		p.isStructuredTerminator() || p.isFunctionStart() || p.isTypeDeclarationStart()
}

func (p *Parser) isEndBlock() bool {
	if p.token != ast.KindEND {
		return false
	}
	switch p.lookAheadToken() {
	case ast.KindFUNCTION, ast.KindSUB, ast.KindPROPERTY, ast.KindIF, ast.KindSELECT, ast.KindWITH, ast.KindTYPE, ast.KindENUM:
		return true
	}
	return false
}

func (p *Parser) isStructuredTerminator() bool {
	switch p.token {
	case ast.KindELSE, ast.KindELSEIF, ast.KindLOOP, ast.KindNEXT, ast.KindWEND, ast.KindCASE:
		return true
	}
	return p.isEndBlock()
}

func (p *Parser) recoverStatement() {
	for !p.isRecoveryBoundary() {
		p.nextToken()
	}
}

// Attach skipped text's errors to the affected statement or containing block,
// rather than carrying the pending error flag into the next valid statement.
func (p *Parser) finishStatementRecovery(node *ast.Node, diagnosticsBefore int, hadError bool) bool {
	hasError := hadError || p.hasParseError || len(p.diagnostics) != diagnosticsBefore
	p.hasParseError = false
	if hasError && node != nil {
		node.Flags |= ast.NodeFlagsThisNodeHasError | ast.NodeFlagsThisNodeOrAnySubNodesHasError
		return false
	}
	return hasError
}

func (p *Parser) enterRecursion() bool {
	if p.depth >= 256 {
		p.parseErrorAtCurrentToken(ast.DiagnosticNestingTooDeep, "Syntax nesting is too deep")
		p.recoverStatement()
		return false
	}
	p.depth++
	return true
}
func (p *Parser) leaveRecursion() { p.depth-- }
