package parser

import (
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
)

// The reference grammar admits these keywords as contextual identifiers.
// Statement boundaries and expression operators take precedence in their contexts.
func isIdentifierToken(kind ast.Kind) bool {
	switch kind {
	case ast.KindIdentifier, ast.KindABS, ast.KindADDRESSOF, ast.KindALIAS, ast.KindAND, ast.KindPROPERTY:
		return true
	case ast.KindANY, ast.KindARRAY, ast.KindATTRIBUTE, ast.KindBEGIN, ast.KindBEGINPROPERTY:
		return true
	case ast.KindBOOLEAN, ast.KindBYREF, ast.KindBYTE, ast.KindBYVAL, ast.KindCBOOL:
		return true
	case ast.KindCBYTE, ast.KindCCUR, ast.KindCDATE, ast.KindCDBL, ast.KindCDEC:
		return true
	case ast.KindCINT, ast.KindCLASS, ast.KindCLNG, ast.KindCLNGLNG, ast.KindCLNGPTR:
		return true
	case ast.KindCSNG, ast.KindCSTR, ast.KindCURRENCY, ast.KindCVAR, ast.KindCVERR:
		return true
	case ast.KindDATABASE, ast.KindDATE, ast.KindDEBUG, ast.KindDOEVENTS, ast.KindDOUBLE:
		return true
	case ast.KindEND, ast.KindENDPROPERTY, ast.KindEQV, ast.KindFALSE, ast.KindFIX:
		return true
	case ast.KindIMP, ast.KindIN, ast.KindINPUTB, ast.KindINT, ast.KindINTEGER:
		return true
	case ast.KindIS, ast.KindLBOUND, ast.KindLEN, ast.KindLENB, ast.KindLIB:
		return true
	case ast.KindLIKE, ast.KindLONG, ast.KindLONGLONG, ast.KindLONGPTR, ast.KindME:
		return true
	case ast.KindMID, ast.KindMIDB, ast.KindMOD, ast.KindNEW, ast.KindNOT:
		return true
	case ast.KindNOTHING, ast.KindNULL, ast.KindOBJECT, ast.KindOPTIONAL, ast.KindOR:
		return true
	case ast.KindPARAMARRAY, ast.KindPRESERVE, ast.KindPSET, ast.KindPTRSAFE, ast.KindREM:
		return true
	case ast.KindSGN, ast.KindSINGLE, ast.KindSPC, ast.KindSTRING, ast.KindTAB:
		return true
	case ast.KindTEXT, ast.KindTHEN, ast.KindTO, ast.KindTRUE, ast.KindTYPEOF:
		return true
	case ast.KindUBOUND, ast.KindUNTIL, ast.KindVARIANT, ast.KindVERSION, ast.KindWITHEVENTS:
		return true
	case ast.KindXOR, ast.KindSTEP, ast.KindON_ERROR, ast.KindERROR, ast.KindAPPEND:
		return true
	case ast.KindBINARY, ast.KindOUTPUT, ast.KindRANDOM, ast.KindRANDOMIZE, ast.KindACCESS:
		return true
	case ast.KindREAD, ast.KindWRITE, ast.KindREAD_WRITE, ast.KindSHARED, ast.KindLOCK_READ:
		return true
	case ast.KindLOCK_WRITE, ast.KindLOCK_READ_WRITE, ast.KindLINE_INPUT, ast.KindRESET, ast.KindWIDTH:
		return true
	case ast.KindPRINT, ast.KindGET, ast.KindPUT, ast.KindCLOSE, ast.KindINPUT:
		return true
	case ast.KindLOCK, ast.KindOPEN, ast.KindSEEK, ast.KindUNLOCK, ast.KindNAME:
		return true
	}
	return false
}

func typeHintText(kind ast.Kind) string {
	return map[ast.Kind]string{ast.KindPERCENT: "%", ast.KindAMPERSAND: "&", ast.KindPOW: "^", ast.KindEXCLAMATIONPOINT: "!", ast.KindHASH: "#", ast.KindAT: "@", ast.KindDOLLAR: "$"}[kind]
}

func (p *Parser) isReferenceTypeHint() bool {
	switch p.token {
	case ast.KindPERCENT, ast.KindHASH, ast.KindAT, ast.KindDOLLAR, ast.KindEXCLAMATIONPOINT:
		// In particular, obj!Member is dictionary access, including a keyword
		// or bracketed name after the bang, rather than a Single type suffix.
		next := p.lookAheadToken()
		return !isUnrestrictedIdentifierToken(next) && next != ast.KindL_SQUARE_BRACKET
	case ast.KindAMPERSAND, ast.KindPOW:
		// Preserve existing binary operator parsing when an operand follows.
		switch p.lookAheadToken() {
		case ast.KindEQ, ast.KindRPAREN, ast.KindCOMMA, ast.KindNEWLINE, ast.KindCOLON,
			ast.KindEndOfFile, ast.KindTHEN, ast.KindELSE, ast.KindLT, ast.KindLEQ,
			ast.KindGT, ast.KindGEQ, ast.KindNEQ, ast.KindGOTO:
			return true
		}
	}
	return false
}

func (p *Parser) parseDeclarationName() *ast.Node {
	name := p.parseIdentifier()
	if suffix := typeHintText(p.token); suffix != "" && name.AsIdentifier().Text != "" && name.AsIdentifier().TypeHint == ast.KindNONE && name.End() == p.scanner.TokenStart() {
		name.AsIdentifier().Text += suffix
		name.AsIdentifier().TypeHint = p.token
		p.nextToken()
		p.finishNode(name, name.Pos())
	}
	return name
}

// A delimited list retains missing elements and stops at an enclosing statement
// or procedure boundary, following typescript-go's parseDelimitedList pattern.
func (p *Parser) parseDelimitedList(terminator ast.Kind, allowEmpty bool, element func() *ast.Node) *ast.NodeList {
	pos := p.nodePos
	var nodes []*ast.Node
	for {
		atEnd := p.token == terminator || p.isRecoveryBoundary()
		if atEnd && (allowEmpty || len(nodes) != 0) {
			break
		}
		before := p.scanner.TokenStart()
		nodes = append(nodes, element())
		if p.token == ast.KindCOMMA {
			p.nextToken()
			if p.token == terminator || p.isRecoveryBoundary() {
				nodes = append(nodes, element())
				break
			}
			continue
		}
		if p.token == terminator || p.isRecoveryBoundary() {
			break
		}
		p.parseExpected(ast.KindCOMMA)
		if p.scanner.TokenStart() == before {
			p.nextToken()
		}
		for p.token != ast.KindCOMMA && p.token != terminator && !p.isRecoveryBoundary() {
			p.nextToken()
		}
		if p.token == ast.KindCOMMA {
			p.nextToken()
			continue
		}
		break
	}
	return p.newNodeList(nodes, pos, p.nodePos)
}

func (p *Parser) parseArrayDimensions() *ast.NodeList {
	p.parseExpected(ast.KindLPAREN)
	dimensions := p.parseDelimitedList(ast.KindRPAREN, true, func() *ast.Node {
		pos := p.nodePos
		upper := p.parseExpression()
		var lower *ast.Node
		if p.token == ast.KindTO {
			lower = upper
			p.nextToken()
			upper = p.parseExpression()
		}
		return p.finishNode(p.factory.NewArrayDimension(lower, upper), pos)
	})
	p.parseExpected(ast.KindRPAREN)
	return dimensions
}

func isBuiltInType(kind ast.Kind) bool {
	switch kind {
	case ast.KindBOOLEAN, ast.KindBYTE, ast.KindCURRENCY, ast.KindDATE, ast.KindDOUBLE,
		ast.KindINTEGER, ast.KindLONG, ast.KindLONGLONG, ast.KindLONGPTR, ast.KindSINGLE,
		ast.KindSTRING, ast.KindVARIANT, ast.KindOBJECT, ast.KindANY:
		return true
	}
	return false
}

func (p *Parser) parseTypeAnnotation(allowNew bool) *ast.Node {
	if p.token != ast.KindAS {
		return nil
	}
	p.nextToken()
	pos := p.nodePos
	var newKeyword *ast.Node
	if p.token == ast.KindNEW {
		newKeyword = p.parseTokenNode()
	}
	bracketed := p.token == ast.KindL_SQUARE_BRACKET
	if bracketed {
		p.nextToken()
	}
	var name *ast.Node
	if isBuiltInType(p.token) {
		name = p.parseTokenNode()
	} else {
		name = p.parseIdentifier()
	}
	if bracketed {
		p.parseExpected(ast.KindR_SQUARE_BRACKET)
	}
	for p.token == ast.KindDOT {
		p.nextToken()
		right := p.parseIdentifier()
		name = p.finishNode(p.factory.NewQualifiedName(name, right), name.Pos())
	}
	array := false
	if p.token == ast.KindLPAREN && p.lookAheadToken() == ast.KindRPAREN {
		p.nextToken()
		p.nextToken()
		array = true
	}
	var length *ast.Node
	if p.token == ast.KindMULT {
		p.nextToken()
		length = p.parsePrimaryExpression()
	}
	node := p.finishNode(p.factory.NewTypeReference(name, newKeyword, array, length), pos)
	if newKeyword != nil && !allowNew {
		p.parseErrorAtNode(newKeyword, ast.DiagnosticInvalidDeclaration, "As New is only valid for variable declarations")
		node.Flags |= ast.NodeFlagsThisNodeOrAnySubNodesHasError
	}
	if name.Kind == ast.KindANY {
		p.parseErrorAtNode(name, ast.DiagnosticInvalidDeclaration, "As Any is only valid for external procedure declarations")
		node.Flags |= ast.NodeFlagsThisNodeOrAnySubNodesHasError
	}
	return node
}

func (p *Parser) parseVariableStatement(module bool) *ast.Node {
	pos := p.nodePos
	modifiers := p.parseVisibility()
	keyword := p.token
	if keyword == ast.KindDIM || keyword == ast.KindSTATIC || keyword == ast.KindCONST {
		p.nextToken()
	} else {
		keyword = ast.KindNONE
	}
	if keyword == ast.KindSTATIC {
		modifiers |= ast.ModifierStatic
	}
	if !module && modifiers & ^ast.ModifierStatic != ast.ModifierNone {
		p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "Visibility modifiers are only valid at module level")
	}
	if module && keyword == ast.KindSTATIC {
		p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "Static variables must be declared inside a procedure")
	}
	if modifiers&ast.VisibiltiyFriend != 0 {
		p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "Friend is only valid on procedures")
	}
	listPos := p.nodePos
	constant := keyword == ast.KindCONST
	declarations := p.parseDelimitedList(ast.KindNONE, false, func() *ast.Node {
		pos := p.nodePos
		name := p.parseDeclarationName()
		var dimensions *ast.NodeList
		if !constant && p.token == ast.KindLPAREN {
			dimensions = p.parseArrayDimensions()
		}
		typeNode := p.parseTypeAnnotation(!constant)
		var initializer *ast.Node
		if constant {
			p.parseExpected(ast.KindEQ)
			initializer = p.parseExpression()
		} else if p.token == ast.KindEQ {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "VBA variable declarations do not have assignment initializers")
			p.nextToken()
			initializer = p.parseExpression()
		}
		return p.finishNode(p.factory.NewVariableDeclaration(name, typeNode, initializer, dimensions), pos)
	})
	list := p.finishNode(p.factory.NewVariableDeclarationList(declarations, constant), listPos)
	return p.finishNode(p.factory.NewVariableStatement(modifiers, keyword, list), pos)
}

func (p *Parser) parseParameter() *ast.Node {
	pos := p.nodePos
	var modifiers ast.ParameterModifiers
	for {
		flag := map[ast.Kind]ast.ParameterModifiers{ast.KindOPTIONAL: ast.ParameterOptional, ast.KindBYVAL: ast.ParameterByVal, ast.KindBYREF: ast.ParameterByRef, ast.KindPARAMARRAY: ast.ParameterParamArray}[p.token]
		if flag == 0 {
			break
		}
		if modifiers&flag != 0 {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidParameter, "Duplicate parameter modifier")
		}
		modifiers |= flag
		p.nextToken()
	}
	name := p.parseDeclarationName()
	var dimensions *ast.NodeList
	if p.token == ast.KindLPAREN {
		dimensions = p.parseArrayDimensions()
	}
	typeNode := p.parseTypeAnnotation(false)
	var initializer *ast.Node
	if p.token == ast.KindEQ {
		p.nextToken()
		initializer = p.parseExpression()
	}
	return p.finishNode(p.factory.NewParameterDeclaration(modifiers, name, typeNode, initializer, dimensions), pos)
}

func (p *Parser) validateParameters(list *ast.NodeList) {
	p.validateParameterNodes(list.Nodes)
}

func (p *Parser) validateParameterNodes(nodes []*ast.Node) {
	optional, paramArray := false, false
	for index, node := range nodes {
		parameter := node.AsParameterDeclaration()
		mods := parameter.Modifiers
		message := ""
		switch {
		case mods&(ast.ParameterByVal|ast.ParameterByRef) == ast.ParameterByVal|ast.ParameterByRef:
			message = "A parameter cannot be both ByVal and ByRef"
		case parameter.Initializer != nil && mods&ast.ParameterOptional == 0:
			message = "Only Optional parameters may have a default value"
		case dimensionsHaveBounds(parameter.Dimensions):
			message = "Parameter arrays must use empty parentheses"
		case parameter.Dimensions != nil && mods&ast.ParameterByVal != 0:
			message = "An array parameter cannot be ByVal"
		case paramArray:
			message = "ParamArray must be the last parameter"
		case mods&ast.ParameterParamArray != 0:
			paramArray = true
			if index != len(nodes)-1 || optional || mods&(ast.ParameterOptional|ast.ParameterByRef|ast.ParameterByVal) != 0 || parameter.Dimensions == nil || parameter.Name.AsIdentifier().TypeHint != ast.KindNONE {
				message = "ParamArray must be a final Variant array and cannot be combined with Optional, ByVal, or ByRef"
			} else if parameter.Type != nil && (parameter.Type.AsTypeReference().TypeName.Kind != ast.KindVARIANT || parameter.Type.AsTypeReference().IsArray) {
				message = "ParamArray must have Variant elements"
			}
		case optional && mods&ast.ParameterOptional == 0:
			message = "Required parameters cannot follow Optional parameters"
		}
		if mods&ast.ParameterOptional != 0 {
			optional = true
		}
		if message != "" {
			name := parameter.Name
			loc := core.NewTextRange(name.End()-len(name.AsIdentifier().Text), name.End())
			p.parseErrorAtRange(loc, ast.DiagnosticInvalidParameter, message)
			node.Flags |= ast.NodeFlagsThisNodeHasError | ast.NodeFlagsThisNodeOrAnySubNodesHasError
			p.hasParseError = false // The signature error belongs to this parameter.
		}
	}
}

func dimensionsHaveBounds(list *ast.NodeList) bool { return list != nil && len(list.Nodes) != 0 }

func (p *Parser) parseModuleOption(module bool) *ast.Node {
	pos := p.nodePos
	p.nextToken()
	if p.token == ast.KindPRIVATE {
		return p.parseOptionPrivateModule(pos, module)
	}
	if p.token == ast.KindIdentifier && strings.EqualFold(p.tokenValue, "Base") {
		p.nextToken()
		base := 0
		if p.token != ast.KindNumericLiteral || p.tokenValue != "0" && p.tokenValue != "1" {
			p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "Option Base requires 0 or 1")
			p.recoverStatement()
		} else {
			if p.tokenValue == "1" {
				base = 1
			}
			p.nextToken()
		}
		if !module || p.seenProcedure || p.seenOptionBase {
			p.parseErrorAtRange(core.NewTextRange(pos, p.nodePos), ast.DiagnosticInvalidDeclaration, "Option Base must appear once at module level before procedures")
		}
		if module {
			p.seenOptionBase = true
		}
		return p.finishNode(p.factory.NewOptionBaseStatement(base), pos)
	}
	if p.token != ast.KindIdentifier || !strings.EqualFold(p.tokenValue, "Explicit") {
		p.parseErrorAtCurrentToken(ast.DiagnosticUnsupportedSyntax, "Only Option Explicit, Option Base, and Option Private Module are supported")
		p.recoverStatement()
		return nil
	}
	p.nextToken()
	if !module || p.seenProcedure {
		p.parseErrorAtRange(core.NewTextRange(pos, p.nodePos), ast.DiagnosticInvalidDeclaration, "Option Explicit must appear before procedures at module level")
	}
	if module && p.seenOptionExplicit {
		p.parseErrorAtRange(core.NewTextRange(pos, p.nodePos), ast.DiagnosticInvalidDeclaration, "Duplicate Option Explicit statement")
	}
	if module {
		p.seenOptionExplicit = true
	}
	return p.finishNode(p.factory.NewOptionExplicitStatement(), pos)
}
