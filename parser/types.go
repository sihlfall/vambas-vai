package parser

import "sihlfall.ch/vambas-vai/ast"

func (p *Parser) isTypeDeclarationStart() bool {
	kind := p.token
	if kind == ast.KindPUBLIC || kind == ast.KindPRIVATE || kind == ast.KindGLOBAL || kind == ast.KindFRIEND {
		kind = p.lookAheadToken()
	}
	return kind == ast.KindTYPE || kind == ast.KindENUM
}

func (p *Parser) parseTypeDeclaration() *ast.Node {
	pos := p.nodePos
	modifiers := p.parseVisibility()
	kind := p.token
	p.nextToken()
	name := p.parseIdentifier()
	if name.AsIdentifier().TypeHint != ast.KindNONE {
		p.parseErrorAtNode(name, ast.DiagnosticInvalidDeclaration, "Type and Enum names cannot have type characters")
	}
	p.parseEndOfStatement()
	listPos := p.nodePos
	var members []*ast.Node
	for p.token != ast.KindEndOfFile {
		if p.token == ast.KindNEWLINE || p.token == ast.KindCOLON {
			p.nextToken()
			continue
		}
		fieldStart := kind == ast.KindTYPE && isUnrestrictedIdentifierToken(p.token) && (p.lookAheadToken() == ast.KindAS || p.lookAheadToken() == ast.KindLPAREN)
		if !fieldStart && (p.token == ast.KindEND || p.isFunctionStart() || p.isTypeDeclarationStart()) {
			break
		}
		before := p.scanner.TokenStart()
		memberPos := p.nodePos
		var member *ast.Node
		if kind == ast.KindENUM {
			memberName := p.parseDeclarationName()
			var initializer *ast.Node
			if p.token == ast.KindEQ {
				p.nextToken()
				initializer = p.parseExpression()
			}
			member = p.factory.NewEnumMember(memberName, initializer)
			if memberName.AsIdentifier().TypeHint != ast.KindNONE {
				p.parseErrorAtNode(memberName, ast.DiagnosticInvalidDeclaration, "Enum member names cannot have type characters")
			}
		} else {
			memberName := p.parseIdentifierWorker(true)
			if memberName.AsIdentifier().TypeHint != ast.KindNONE {
				p.parseErrorAtNode(memberName, ast.DiagnosticInvalidDeclaration, "Type field names cannot have type characters")
			}
			var dimensions *ast.NodeList
			if p.token == ast.KindLPAREN {
				dimensions = p.parseArrayDimensions()
			}
			typeNode := p.parseTypeAnnotation(false)
			if typeNode == nil && dimensions == nil {
				p.parseErrorAtCurrentToken(ast.DiagnosticInvalidDeclaration, "A Type field requires an As type or array dimensions")
			}
			member = p.factory.NewFieldDeclaration(memberName, typeNode, dimensions)
		}
		members = append(members, p.finishNode(member, memberPos))
		if p.scanner.TokenStart() == before {
			p.nextToken()
		}
		p.parseEndOfStatement()
	}
	list := p.newNodeList(members, listPos, p.nodePos)
	if p.token == ast.KindEND && p.lookAheadToken() == kind {
		p.nextToken()
		p.nextToken()
	} else {
		p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Matching End Type or End Enum expected")
	}
	var node *ast.Node
	if kind == ast.KindENUM {
		node = p.factory.NewEnumDeclaration(modifiers, name, list)
	} else {
		node = p.factory.NewTypeDeclaration(modifiers, name, list)
	}
	if len(members) == 0 {
		p.parseErrorAtNode(name, ast.DiagnosticInvalidDeclaration, "Type and Enum declarations require at least one member")
	}
	if modifiers&ast.VisibiltiyFriend != 0 || modifiers&ast.ModifierGlobal != 0 && kind != ast.KindENUM {
		p.parseErrorAtNode(name, ast.DiagnosticInvalidDeclaration, "Type and Enum declarations allow only Public or Private visibility")
	}
	return p.finishNode(node, pos)
}
