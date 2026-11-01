package parser

import "sihlfall.ch/vambas-vai/ast"

func isAccessorKeyword(kind ast.Kind) bool {
	return kind == ast.KindGET || kind == ast.KindLET || kind == ast.KindSET
}

func (p *Parser) validatePropertySetter(parameters *ast.NodeList, name *ast.Node) {
	nodes := parameters.Nodes
	if len(nodes) == 0 {
		p.parseErrorAtNode(name, ast.DiagnosticInvalidParameter, "Property Let/Set requires a final value parameter")
		return
	}
	// Optional and ParamArray index parameters precede the required assignment
	// value; the ordinary procedure rule would incorrectly reject that value.
	p.validateParameterNodes(nodes[:len(nodes)-1])
	value := nodes[len(nodes)-1]
	p.validateParameterNodes([]*ast.Node{value})
	parameter := value.AsParameterDeclaration()
	if parameter.Modifiers&(ast.ParameterOptional|ast.ParameterParamArray) != 0 || parameter.Initializer != nil {
		p.parseErrorAtNode(value, ast.DiagnosticInvalidParameter, "A property's assignment value cannot be Optional, ParamArray, or have a default")
	}
}
