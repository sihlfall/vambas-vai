package binder

import "sihlfall.ch/vambas-vai/ast"

// Assemble an ambient scope separately from user module/project symbols.
// The catalog adapter and separate testing library materialize ordinary
// declarations and symbols, fresh per project and without source locations.
func newIntrinsicScope() ast.SymbolTable {
	var f ast.NodeFactory
	exports := make(ast.SymbolTable)
	namespace := &ast.Symbol{Flags: ast.SymbolFlagsValueModule, Name: "VBA", Exports: exports}
	addCatalogLibrary(&f, exports, namespace)
	scope := ast.SymbolTable{"vba": namespace}
	for name, s := range exports {
		scope[name] = s
	}
	addTestLibrary(&f, scope)
	return scope
}
