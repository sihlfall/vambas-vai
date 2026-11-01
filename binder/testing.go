package binder

import "sihlfall.ch/vambas-vai/ast"

// Independently authored virtual declarations for the source-visible testing API.
// These are ordinary library symbols, separate from user declarations.
func addTestLibrary(f *ast.NodeFactory, scope ast.SymbolTable) {
	ns := &ast.Symbol{Flags: ast.SymbolFlagsValueModule, Name: "Rubberduck", Exports: ast.SymbolTable{}}
	scope["rubberduck"] = ns
	for _, c := range []struct {
		name string
		kind ast.IntrinsicKind
	}{
		{"AssertClass", ast.IntrinsicAssertClass}, {"PermissiveAssertClass", ast.IntrinsicPermissiveAssertClass}, {"FakesProvider", ast.IntrinsicFakesProvider},
	} {
		id := f.NewIdentifier(c.name)
		class := &ast.Symbol{Flags: ast.SymbolFlagsClass, Name: c.name, Intrinsic: c.kind, ValueDeclaration: id, Declarations: []*ast.Node{id}, Parent: ns, Members: ast.SymbolTable{}, Exports: ast.SymbolTable{}}
		id.Symbol = class
		ns.Exports[ast.SymbolNameKey(c.name)] = class
		if c.kind == ast.IntrinsicFakesProvider {
			continue
		} // Identity/storage only; no fake is silently simulated.
		for _, method := range []struct {
			name   string
			params []string
			object bool
		}{
			{"AreEqual", []string{"Expected", "Actual"}, false}, {"AreNotEqual", []string{"Expected", "Actual"}, false},
			{"AreSame", []string{"Expected", "Actual"}, true}, {"AreNotSame", []string{"Expected", "Actual"}, true},
			{"IsTrue", []string{"Condition"}, false}, {"IsFalse", []string{"Condition"}, false},
			{"IsNothing", []string{"Value"}, true}, {"IsNotNothing", []string{"Value"}, true},
			{"SequenceEquals", []string{"Expected", "Actual"}, false}, {"NotSequenceEquals", []string{"Expected", "Actual"}, false},
			{"Fail", nil, false}, {"Inconclusive", nil, false}, {"Succeed", nil, false},
		} {
			var ps []*ast.Node
			names := append(append([]string{}, method.params...), "Message")
			for i, name := range names {
				kind, mods := ast.KindVARIANT, ast.ParameterByVal
				if method.object {
					kind = ast.KindOBJECT
				}
				if i == len(names)-1 {
					kind = ast.KindSTRING
					mods |= ast.ParameterOptional
				}
				var def *ast.Node
				if i == len(names)-1 {
					def = f.NewStringLiteral("")
				}
				pn := f.NewIdentifier(name)
				p := f.NewParameterDeclaration(mods, pn, f.NewTypeReference(f.NewToken(kind), nil, false, nil), def, nil)
				s := &ast.Symbol{Flags: ast.SymbolFlagsFunctionScopedVariable, Name: name, ValueDeclaration: p, Declarations: []*ast.Node{p}}
				p.Symbol, pn.Symbol = s, s
				ps = append(ps, p)
			}
			fn := f.NewSubDeclaration(ast.ModifierPublic, f.NewIdentifier(method.name), f.NewNodeList(ps), f.NewBlock(nil))
			setParents(fn, nil)
			s := &ast.Symbol{Flags: ast.SymbolFlagsFunction, Name: method.name, Intrinsic: ast.IntrinsicTestAssert, Parent: class, ValueDeclaration: fn, Declarations: []*ast.Node{fn}}
			fn.Symbol, fn.AsFunctionDeclaration().Name().Symbol = s, s
			for _, p := range ps {
				p.Symbol.Parent = s
			}
			class.Members[ast.SymbolNameKey(method.name)], class.Exports[ast.SymbolNameKey(method.name)] = s, s
		}
	}
}
