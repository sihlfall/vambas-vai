package parser

import (
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
)

func (p *Parser) parseOptionPrivateModule(pos int, module bool) *ast.Node {
	p.nextToken() // Private
	if p.token != ast.KindIdentifier || !strings.EqualFold(p.tokenValue, "Module") {
		p.parseErrorAtCurrentToken(ast.DiagnosticExpectedToken, "Module expected after Option Private")
		p.recoverStatement()
	} else {
		p.nextToken()
	}
	if !module || p.seenProcedure {
		p.parseErrorAtRange(core.NewTextRange(pos, p.nodePos), ast.DiagnosticInvalidDeclaration, "Option Private Module must appear before procedures at module level")
	}
	if module && p.seenOptionPrivate {
		p.parseErrorAtRange(core.NewTextRange(pos, p.nodePos), ast.DiagnosticInvalidDeclaration, "Duplicate Option Private Module statement")
	}
	if module {
		p.seenOptionPrivate = true
	}
	return p.finishNode(p.factory.NewOptionPrivateModuleStatement(), pos)
}

func (p *Parser) parseAttributeStatement() *ast.Node {
	pos := p.nodePos
	p.nextToken()
	name := p.parseLExpression()
	p.parseExpected(ast.KindEQ)
	valuesPos := p.nodePos
	var values []*ast.Node
	for {
		values = append(values, p.parseExpression())
		if p.token != ast.KindCOMMA {
			break
		}
		p.nextToken()
	}
	return p.finishNode(p.factory.NewAttributeStatement(name, p.newNodeList(values, valuesPos, p.nodePos)), pos)
}

func (p *Parser) setModuleAttributes(file *ast.SourceFile) {
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindAttributeStatement {
			continue
		}
		attribute := node.AsAttributeStatement()
		if attribute.Name.Kind != ast.KindIdentifier {
			continue // Procedure/member metadata does not name the module.
		}
		name := strings.ToLower(attribute.Name.AsIdentifier().Text)
		if name != "vb_name" && name != "vb_predeclaredid" {
			continue
		}
		if len(attribute.Values.Nodes) != 1 {
			p.parseErrorAtNode(node, ast.DiagnosticInvalidDeclaration, "Module attribute requires one value")
			continue
		}
		value := attribute.Values.Nodes[0]
		if name == "vb_name" {
			if value.Kind != ast.KindStringLiteral || value.AsStringLiteral().Text == "" {
				p.parseErrorAtNode(value, ast.DiagnosticInvalidDeclaration, "VB_Name requires a nonempty string")
			} else {
				file.ModuleName = value.AsStringLiteral().Text
			}
		} else if value.Kind == ast.KindTRUE || value.Kind == ast.KindFALSE {
			file.PredeclaredID = value.Kind == ast.KindTRUE
		} else {
			p.parseErrorAtNode(value, ast.DiagnosticInvalidDeclaration, "VB_PredeclaredId requires True or False")
		}
	}
}
