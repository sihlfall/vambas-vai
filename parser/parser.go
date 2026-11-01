package parser

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/scanner"
)

type Parser struct {
	scanner            *scanner.ScannerState
	factory            ast.NodeFactory
	token              ast.Kind
	tokenValue         string
	nodePos            int
	insideSingleLineIf bool
	depth              int
	diagnostics        []*ast.Diagnostic
	hasParseError      bool
	seenProcedure      bool
	seenOptionExplicit bool
	seenOptionPrivate  bool
	seenOptionBase     bool
	pendingNext        bool // A comma in Next inner, outer closes another For.
	nodeSlicePool      core.Pool[*ast.Node]
	identifiers        map[string]string
}

func (p *Parser) internIdentifier(text string) string {
	if value, ok := p.identifiers[text]; ok {
		return value
	}
	if p.identifiers == nil {
		p.identifiers = make(map[string]string)
	}
	p.identifiers[text] = text
	return text
}

func (p *Parser) nextTokenWithFlags(flags scanner.ScanFlags) {
	p.token = p.scanner.Scan(flags)
	p.tokenValue = p.scanner.TokenValue
	p.nodePos = p.scanner.TokenFullStart()
}

func (p *Parser) nextToken() { p.nextTokenWithFlags(scanner.ScanFlagNone) }

func (p *Parser) parseTokenNode() *ast.Node {
	pos, kind := p.nodePos, p.token
	p.nextToken()
	return p.finishNode(p.factory.NewToken(kind), pos)
}

func (p *Parser) parseIdentifier() *ast.Node {
	return p.parseIdentifierWorker(false)
}

func (p *Parser) parseIdentifierWorker(unrestricted bool) *ast.Node {
	if p.token == ast.KindL_SQUARE_BRACKET {
		return p.parseBracketedIdentifier()
	}
	if !isIdentifierToken(p.token) && !(unrestricted && isUnrestrictedIdentifierToken(p.token)) {
		p.parseErrorAtCurrentToken(ast.DiagnosticIdentifierExpected, "Identifier expected")
		return p.createMissingIdentifier()
	}
	pos := p.nodePos
	text := p.tokenValue
	if p.token != ast.KindIdentifier {
		text = p.scanner.Input[p.scanner.TokenStart():p.scanner.TokenEnd()]
	}
	node := p.factory.NewIdentifier(p.internIdentifier(text))
	if len(text) > 0 && text[len(text)-1] == '$' {
		node.AsIdentifier().TypeHint = ast.KindDOLLAR
	}
	p.nextToken()
	if name := node.AsIdentifier(); name.TypeHint == ast.KindNONE {
		// Equal full/token starts mean that the character immediately follows
		// the identifier, rather than being separated by trivia.
		if p.nodePos == p.scanner.TokenStart() && p.isReferenceTypeHint() {
			name.Text += typeHintText(p.token)
			name.TypeHint = p.token
			p.nextToken()
		}
	}
	return p.finishNode(node, pos)
}

func (p *Parser) parseMeExpression() *ast.Expression {
	pos := p.nodePos
	p.nextToken()
	return p.finishNode(p.factory.NewMeExpression(), pos)
}

func (p *Parser) parseLExpression() *ast.Expression {
	return p.parseLeftHandSideExpressionOrHigher(false)
}

func (p *Parser) parsePrimaryExpression() *ast.Expression {
	pos := p.nodePos
	switch p.token {
	case ast.KindLPAREN:
		p.nextToken()
		inner := p.parseExpression()
		p.parseExpected(ast.KindRPAREN)
		return p.finishNode(p.factory.NewParenthesizedExpression(inner), pos)
	case ast.KindIdentifier, ast.KindL_SQUARE_BRACKET:
		return p.parseIdentifier()
	case ast.KindNumericLiteral:
		node := p.factory.NewNumericLiteral(p.tokenValue)
		node.AsNumericLiteral().TokenFlags = p.scanner.TokenFlags()
		end := p.scanner.TokenEnd()
		p.nextToken()
		if suffix := typeHintText(p.token); suffix != "" && p.token != ast.KindDOLLAR && p.scanner.TokenStart() == end {
			node.AsNumericLiteral().Text += suffix
			p.nextToken()
		}
		return p.finishNode(node, pos)
	case ast.KindStringLiteral:
		node := p.factory.NewStringLiteral(p.tokenValue)
		node.AsStringLiteral().TokenFlags = p.scanner.TokenFlags()
		p.nextToken()
		return p.finishNode(node, pos)
	case ast.KindTRUE, ast.KindFALSE, ast.KindNOTHING, ast.KindEMPTY, ast.KindNULL:
		return p.parseTokenNode()
	case ast.KindME:
		return p.parseMeExpression()
	case ast.KindNEW:
		return p.parseNewExpression()
	case ast.KindDOT, ast.KindEXCLAMATIONPOINT:
		// Start an omitted-receiver access with one member. Subsequent suffixes
		// belong to the usual member/call loop, which knows statement context.
		operator := p.parseTokenNode()
		name := p.parseIdentifierWorker(true)
		return p.finishNode(p.factory.NewPropertyAccessExpression(nil, operator, name), pos)
	default:
		if isIdentifierToken(p.token) {
			return p.parseIdentifier()
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticExpressionExpected, "Expression expected")
		return p.createMissingIdentifier()
	}
}

// Precedence climbing follows typescript-go, with VBA's operator levels. In
// particular Not binds below comparisons, and exponentiation binds above signs.
func binaryPrecedence(kind ast.Kind) int {
	switch kind {
	case ast.KindIMP:
		return 1
	case ast.KindEQV:
		return 2
	case ast.KindXOR:
		return 3
	case ast.KindOR:
		return 4
	case ast.KindAND:
		return 5
	case ast.KindEQ, ast.KindNEQ, ast.KindLT, ast.KindLEQ, ast.KindGT, ast.KindGEQ, ast.KindIS:
		return 7
	case ast.KindAMPERSAND:
		return 8
	case ast.KindPLUS, ast.KindMINUS:
		return 9
	case ast.KindMOD:
		return 10
	case ast.KindINTDIV:
		return 11
	case ast.KindMULT, ast.KindDIV:
		return 12
	case ast.KindPOW:
		return 14
	default:
		return -1
	}
}

func (p *Parser) parseUnaryExpressionOrHigher() *ast.Expression {
	pos, operator := p.nodePos, p.token
	switch operator {
	case ast.KindPLUS, ast.KindMINUS, ast.KindNOT:
		p.nextToken()
		precedence := 13
		if operator == ast.KindNOT {
			precedence = 6
		}
		operand := p.parseBinaryExpressionOrHigher(precedence)
		return p.finishNode(p.factory.NewPrefixUnaryExpression(operator, operand), pos)
	default:
		return p.parseLeftHandSideExpressionOrHigher(false)
	}
}

func (p *Parser) parseBinaryExpressionOrHigher(minimum int) *ast.Expression {
	if !p.enterRecursion() {
		return p.createMissingIdentifier()
	}
	defer p.leaveRecursion()
	left := p.parseUnaryExpressionOrHigher()
	for {
		precedence := binaryPrecedence(p.token)
		if precedence < minimum {
			break
		}
		pos, kind := left.Pos(), p.token
		operator := p.parseTokenNode()
		nextMinimum := precedence + 1
		if kind == ast.KindPOW {
			nextMinimum = precedence
		}
		right := p.parseBinaryExpressionOrHigher(nextMinimum)
		left = p.finishNode(p.factory.NewBinaryExpression(left, operator, right), pos)
	}
	return left
}

func (p *Parser) parseExpression() *ast.Expression { return p.parseBinaryExpressionOrHigher(0) }

func (p *Parser) parseGotoStatement() *ast.Statement {
	pos := p.nodePos
	p.nextToken()
	return p.parseGotoTarget(pos)
}

func (p *Parser) parseGotoTarget(pos int) *ast.Statement {
	if p.token == ast.KindMINUS || p.token == ast.KindNumericLiteral {
		numberPos, minus := p.nodePos, p.token == ast.KindMINUS
		if minus {
			p.nextToken()
		}
		var number *ast.Node
		if p.token == ast.KindNumericLiteral {
			number = p.factory.NewLineNumber(p.tokenValue, minus)
			p.nextToken()
			p.finishNode(number, numberPos)
		} else {
			p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Line number expected")
			number = p.finishNodeWithEnd(p.factory.NewLineNumber("", minus), p.nodePos, p.nodePos)
		}
		return p.finishNode(p.factory.NewGotoStatement(number, nil), pos)
	}
	label := p.parseIdentifier()
	return p.finishNode(p.factory.NewGotoStatement(nil, label), pos)
}

func (p *Parser) parseIfStatementRemainingMultiline(condition *ast.Expression, pos int) *ast.Statement {
	if !p.enterRecursion() {
		return p.finishNode(p.factory.NewIfStatement(condition, p.emptyBlock(), nil), pos)
	}
	defer p.leaveRecursion()
	thenStatement := p.parseBlock(pcIfStatements)
	var elseStatement *ast.Node
	if p.token == ast.KindELSEIF {
		elsePos := p.nodePos
		p.nextToken()
		innerCondition := p.parseExpression()
		p.parseExpected(ast.KindTHEN)
		p.parseEndOfStatement()
		elseStatement = p.parseIfStatementRemainingMultiline(innerCondition, elsePos)
	} else {
		if p.token == ast.KindELSE {
			p.nextToken()
			p.parseEndOfStatement()
			elseStatement = p.parseBlock(pcIfStatements)
		}
		if p.token == ast.KindEND && p.lookAheadToken() == ast.KindIF {
			p.nextToken()
			p.nextToken()
		} else {
			p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "End If expected")
		}
	}
	return p.finishNode(p.factory.NewIfStatement(condition, thenStatement, elseStatement), pos)
}

func (p *Parser) parseIfStatementRemainingSingleline(condition *ast.Expression, pos int) *ast.Statement {
	saved := p.insideSingleLineIf
	p.insideSingleLineIf = true
	defer func() { p.insideSingleLineIf = saved }()
	thenStatement := p.parseBlock(pcSingleLineStatements)
	var elseStatement *ast.Node
	if p.token == ast.KindELSE {
		p.nextToken()
		elseStatement = p.parseBlock(pcSingleLineStatements)
	}
	return p.finishNode(p.factory.NewIfStatement(condition, thenStatement, elseStatement), pos)
}

func (p *Parser) parseIfStatement() *ast.Statement {
	if !p.enterRecursion() {
		return nil
	}
	defer p.leaveRecursion()
	pos := p.nodePos
	p.nextToken()
	condition := p.parseExpression()
	p.parseExpected(ast.KindTHEN)
	if p.token == ast.KindNEWLINE || p.token == ast.KindCOLON {
		p.nextToken()
		return p.parseIfStatementRemainingMultiline(condition, pos)
	}
	return p.parseIfStatementRemainingSingleline(condition, pos)
}

func (p *Parser) parseLetStatement() *ast.Statement {
	pos := p.nodePos
	if p.token == ast.KindLET {
		p.nextToken()
	}
	left := p.parseLExpression()
	p.parseExpected(ast.KindEQ)
	right := p.parseExpression()
	return p.finishNode(p.factory.NewLetStatement(left, right), pos)
}

func (p *Parser) parseMainStatement() *ast.Statement {
	if p.isMidAssignment() {
		return p.parseMidAssignment()
	}
	switch p.token {
	case ast.KindERASE:
		pos := p.nodePos
		p.nextToken()
		listPos := p.nodePos
		var expressions []*ast.Node
		for {
			expressions = append(expressions, p.parseExpression())
			if p.token != ast.KindCOMMA {
				break
			}
			p.nextToken()
		}
		return p.finishNode(p.factory.NewEraseStatement(p.newNodeList(expressions, listPos, p.nodePos)), pos)
	case ast.KindFOR:
		return p.parseForStatement()
	case ast.KindDO:
		return p.parseDoStatement()
	case ast.KindWHILE:
		return p.parseWhileStatement()
	case ast.KindSELECT:
		return p.parseSelectStatement()
	case ast.KindWITH:
		return p.parseWithStatement()
	case ast.KindSET:
		return p.parseSetStatement()
	case ast.KindREDIM:
		return p.parseReDimStatement()
	case ast.KindEXIT:
		return p.parseExitStatement()
	case ast.KindEND:
		if !p.isEndBlock() {
			pos := p.nodePos
			p.nextToken()
			return p.finishNode(p.factory.NewToken(ast.KindEndStatement), pos)
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticStatementExpected, "Unexpected block terminator")
		p.nextToken()
		p.recoverStatement()
		return nil
	case ast.KindNumericLiteral, ast.KindMINUS:
		if p.insideSingleLineIf {
			return p.parseGotoTarget(p.nodePos)
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticStatementExpected, "Unexpected numeric statement")
		p.nextToken()
		p.recoverStatement()
		return nil
	case ast.KindIF:
		if p.insideSingleLineIf {
			p.parseErrorAtCurrentToken(ast.DiagnosticUnsupportedSyntax, "An If statement inside a single-line If is not supported")
			p.nextToken()
			p.recoverStatement()
			return nil
		}
		return p.parseIfStatement()
	case ast.KindLET:
		return p.parseLetStatement()
	case ast.KindCALL:
		return p.parseCallStatement()
	case ast.KindIdentifier, ast.KindME, ast.KindL_SQUARE_BRACKET, ast.KindDOT, ast.KindEXCLAMATIONPOINT:
		if p.isSpacedGosub() {
			pos := p.nodePos
			p.nextToken()
			node := p.parseGotoStatement()
			node.Kind = ast.KindGosubStatement
			node.Loc = core.NewTextRange(pos, node.End())
			return node
		}
		return p.parseAssignmentOrCallStatement()
	case ast.KindGOTO, ast.KindGOSUB:
		gosub := p.token == ast.KindGOSUB
		node := p.parseGotoStatement()
		if gosub {
			node.Kind = ast.KindGosubStatement
		}
		return node
	case ast.KindRETURN:
		pos := p.nodePos
		p.nextToken()
		return p.finishNode(p.factory.NewGosubReturnStatement(), pos)
	case ast.KindRESUME:
		return p.parseResumeStatement()
	case ast.KindERROR:
		pos := p.nodePos
		p.nextToken()
		return p.finishNode(p.factory.NewErrorStatement(p.parseExpression()), pos)
	case ast.KindON:
		return p.parseOnGotoStatement()
	case ast.KindDIM, ast.KindSTATIC, ast.KindCONST, ast.KindPRIVATE, ast.KindPUBLIC, ast.KindFRIEND, ast.KindGLOBAL:
		return p.parseVariableStatement(false)
	case ast.KindOPTION:
		return p.parseModuleOption(false)
	case ast.KindATTRIBUTE:
		return p.parseAttributeStatement()
	default:
		if isIdentifierToken(p.token) {
			return p.parseAssignmentOrCallStatement()
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticStatementExpected, "Statement expected; this syntax may not be supported yet")
		if !p.isRecoveryBoundary() {
			p.nextToken()
			p.recoverStatement()
		}
		return nil
	}
}

func (p *Parser) ParseBlockStatement() *ast.Statement {
	pos := p.nodePos
	var number, label *ast.Node
	if p.token == ast.KindLineNumber {
		number = p.factory.NewLineNumber(p.tokenValue, false)
		numberPos := p.nodePos
		p.nextToken()
		p.finishNode(number, numberPos)
	}
	if p.token == ast.KindIdentifierLabel {
		labelPos := p.nodePos
		label = p.factory.NewIdentifier(p.tokenValue)
		p.nextToken()
		p.finishNode(label, labelPos)
		p.parseExpected(ast.KindCOLON)
	} else if number != nil && p.token == ast.KindCOLON {
		p.nextToken()
	}
	var statement *ast.Node
	if (number == nil && label == nil) || !p.isRecoveryBoundary() {
		statement = p.parseMainStatement()
	}
	if number != nil || label != nil {
		return p.finishNode(p.factory.NewLabeledStatement(number, label, statement), pos)
	}
	return statement
}

func (p *Parser) parseVisibility() ast.Modifiers {
	var modifier ast.Modifiers
	switch p.token {
	case ast.KindPRIVATE:
		modifier = ast.ModifierPrivate
	case ast.KindFRIEND:
		modifier = ast.VisibiltiyFriend
	case ast.KindPUBLIC:
		modifier = ast.ModifierPublic
	case ast.KindGLOBAL:
		modifier = ast.ModifierGlobal
	default:
		return ast.ModifierNone
	}
	p.nextToken()
	return modifier
}

func (p *Parser) parseEndOfStatement() {
	if p.token == ast.KindNEWLINE || p.token == ast.KindCOLON {
		p.nextToken()
		return
	}
	if p.token == ast.KindEndOfFile {
		return
	}
	p.parseErrorAtCurrentToken(ast.DiagnosticEndOfStatementExpected, "End of statement expected")
	p.recoverStatement()
	if p.token == ast.KindNEWLINE || p.token == ast.KindCOLON {
		p.nextToken()
	}
}

func (p *Parser) parseBlock(context parsingContext) *ast.Node {
	pos := p.nodePos
	var nodes []*ast.Node
	listHadError := false
	for {
		if p.token == ast.KindCOLON || (context != pcSingleLineStatements && p.token == ast.KindNEWLINE) {
			p.nextToken()
			continue
		}
		if p.isListTerminator(context) {
			break
		}
		before := p.scanner.TokenStart()
		diagnosticsBefore, hadError := len(p.diagnostics), p.hasParseError
		var statement *ast.Node
		if context == pcSingleLineStatements {
			statement = p.parseMainStatement()
		} else {
			statement = p.ParseBlockStatement()
		}
		if statement != nil {
			nodes = append(nodes, statement)
		}
		if p.scanner.TokenStart() == before && !p.isListTerminator(context) {
			// An error production may insert a missing node without consuming a
			// token. List recovery must still advance, as in typescript-go.
			p.nextToken()
			p.recoverStatement()
		}
		if p.token == ast.KindCOLON || p.token == ast.KindNEWLINE || p.token == ast.KindEndOfFile {
			listHadError = p.finishStatementRecovery(statement, diagnosticsBefore, hadError) || listHadError
			continue
		}
		if context == pcSingleLineStatements && p.isListTerminator(context) {
			listHadError = p.finishStatementRecovery(statement, diagnosticsBefore, hadError) || listHadError
			break
		}
		if p.pendingNext && p.token == ast.KindCOMMA && p.isListTerminator(context) {
			listHadError = p.finishStatementRecovery(statement, diagnosticsBefore, hadError) || listHadError
			break
		}
		labelOnly := statement != nil && statement.Kind == ast.KindLabeledStatement && statement.AsLabeledStatement().Statement == nil
		if !labelOnly {
			p.parseErrorAtCurrentToken(ast.DiagnosticEndOfStatementExpected, "End of statement expected")
			p.recoverStatement()
		}
		listHadError = p.finishStatementRecovery(statement, diagnosticsBefore, hadError) || listHadError
	}
	if context == pcSingleLineStatements && len(nodes) == 0 {
		p.parseErrorAtCurrentToken(ast.DiagnosticStatementExpected, "Statement expected")
	}
	list := p.newNodeList(nodes, pos, p.nodePos)
	block := p.factory.NewBlock(list)
	if listHadError {
		block.Flags |= ast.NodeFlagsThisNodeHasError
	}
	return p.finishNode(block, pos)
}

func (p *Parser) parseParameterList() *ast.NodeList {
	return p.parseParameterListWithValidation(true)
}

func (p *Parser) parseParameterListWithValidation(validate bool) *ast.NodeList {
	p.parseExpected(ast.KindLPAREN)
	list := p.parseDelimitedList(ast.KindRPAREN, true, p.parseParameter)
	p.parseExpected(ast.KindRPAREN)
	if validate {
		p.validateParameters(list)
	}
	return list
}

func (p *Parser) ParseFunction() *ast.Statement {
	p.pendingNext = false
	defer func() { p.pendingNext = false }()
	pos := p.nodePos
	modifiers := p.parseVisibility()
	if p.token == ast.KindSTATIC {
		modifiers |= ast.ModifierStatic
		p.nextToken()
	}
	keyword := ast.KindFUNCTION
	kind := ast.KindFunctionDeclaration
	if p.token == ast.KindSUB {
		keyword = ast.KindSUB
		kind = ast.KindSubDeclaration
	} else if p.token == ast.KindPROPERTY {
		keyword = ast.KindPROPERTY
	}
	p.parseExpected(keyword)
	if keyword == ast.KindPROPERTY {
		switch p.token {
		case ast.KindGET:
			kind = ast.KindGetAccessor
		case ast.KindLET:
			kind = ast.KindLetAccessor
		case ast.KindSET:
			kind = ast.KindSetAccessor
		default:
			p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Get, Let, or Set expected after Property")
			kind = ast.KindGetAccessor
		}
		if isAccessorKeyword(p.token) {
			p.nextToken()
		}
	}
	name := p.parseDeclarationName()
	setter := kind == ast.KindLetAccessor || kind == ast.KindSetAccessor
	parameters := p.newNodeList(nil, p.nodePos, p.nodePos)
	if p.token == ast.KindLPAREN {
		if setter {
			parameters = p.parseParameterListWithValidation(false)
		} else {
			parameters = p.parseParameterList()
		}
	}
	if setter {
		p.validatePropertySetter(parameters, name)
	}
	returnType := p.parseTypeAnnotation(false)
	if (keyword == ast.KindSUB || setter) && returnType != nil {
		p.parseErrorAtNode(returnType, ast.DiagnosticInvalidDeclaration, "Only Function and Property Get may declare a return type")
	}
	p.parseEndOfStatement()
	body := p.parseBlock(pcFunctionStatements)
	if p.token == ast.KindEND {
		p.nextToken()
		if !p.parseExpected(keyword) {
			p.recoverStatement()
		}
	} else {
		message := "End Function expected"
		if keyword == ast.KindSUB {
			message = "End Sub expected"
		} else if keyword == ast.KindPROPERTY {
			message = "End Property expected"
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, message)
	}
	if p.token != ast.KindNEWLINE && p.token != ast.KindCOLON && p.token != ast.KindEndOfFile {
		p.parseErrorAtCurrentToken(ast.DiagnosticEndOfStatementExpected, "End of statement expected")
		p.recoverStatement()
	}
	node := p.factory.NewFunctionDeclaration(modifiers, name, parameters, body)
	node.Kind = kind
	node.AsFunctionDeclaration().Type = returnType
	return p.finishNode(node, pos)
}

// ParseSourceFile parses the supported VBA syntax. Malformed and unsupported input
// produces diagnostics and a recoverable syntax tree, rather than a panic.
func ParseSourceFile(input string) *ast.Node {
	p := &Parser{scanner: &scanner.ScannerState{Input: input}}
	p.Start()
	return p.ParseSourceFile()
}

func (p *Parser) ParseSourceFile() *ast.Node {
	var nodes []*ast.Node
	for p.token != ast.KindEndOfFile {
		if p.token == ast.KindNEWLINE || p.token == ast.KindCOLON {
			p.nextToken()
			continue
		}
		if p.isFunctionStart() {
			p.seenProcedure = true
			nodes = append(nodes, p.ParseFunction())
			continue
		}
		var declaration *ast.Node
		if p.isTypeDeclarationStart() {
			if p.seenProcedure {
				p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "Type and Enum declarations must precede procedures")
			}
			declaration = p.parseTypeDeclaration()
		} else {
			switch p.token {
			case ast.KindIMPLEMENTS:
				pos := p.nodePos
				p.nextToken()
				name := p.parseDeclarationName()
				for p.token == ast.KindDOT {
					p.nextToken()
					name = p.finishNode(p.factory.NewQualifiedName(name, p.parseDeclarationName()), pos)
				}
				declaration = p.finishNode(p.factory.NewImplementsStatement(name), pos)
			case ast.KindOPTION:
				declaration = p.parseModuleOption(true)
			case ast.KindATTRIBUTE:
				declaration = p.parseAttributeStatement()
			case ast.KindDIM, ast.KindSTATIC, ast.KindCONST, ast.KindPRIVATE, ast.KindPUBLIC, ast.KindFRIEND, ast.KindGLOBAL:
				if p.seenProcedure {
					p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "Module declarations must precede procedures")
				}
				declaration = p.parseVariableStatement(true)
			}
		}
		if declaration != nil {
			nodes = append(nodes, declaration)
			if p.token != ast.KindNEWLINE && p.token != ast.KindCOLON && p.token != ast.KindEndOfFile {
				p.parseErrorAtCurrentToken(ast.DiagnosticEndOfStatementExpected, "End of statement expected")
				p.recoverStatement()
				declaration.Flags |= ast.NodeFlagsThisNodeHasError | ast.NodeFlagsThisNodeOrAnySubNodesHasError
				p.hasParseError = false
			}
			continue
		}
		p.parseErrorAtCurrentToken(ast.DiagnosticDeclarationExpected, "Module declaration or procedure expected; this syntax may not be supported yet")
		// At module scope there is no enclosing block whose terminator to retain.
		for p.token != ast.KindNEWLINE && p.token != ast.KindCOLON && p.token != ast.KindEndOfFile {
			p.nextToken()
		}
		// The source file owns diagnostics for discarded module text; they must
		// not mark the next valid declaration as an erroneous node.
		p.hasParseError = false
	}
	list := p.newNodeList(nodes, 0, p.nodePos)
	eof := p.parseTokenNode()
	node := p.factory.NewSourceFile(list)
	file := node.AsSourceFile()
	file.Text, file.EndOfFileToken = p.scanner.Input, eof
	file.Comments = append([]ast.Comment(nil), p.scanner.Comments...)
	attachAnnotations(file)
	p.setModuleAttributes(file)
	file.ParseDiagnostics = p.diagnostics
	p.finishNodeWithEnd(node, 0, len(file.Text))
	if len(file.ParseDiagnostics) != 0 {
		node.Flags |= ast.NodeFlagsThisNodeHasError | ast.NodeFlagsThisNodeOrAnySubNodesHasError
	}
	for _, diagnostic := range file.ParseDiagnostics {
		diagnostic.File = file
	}
	return node
}

func (p *Parser) Start() {
	p.scanner.SetOnError(p.scanError)
	p.nextToken()
}
