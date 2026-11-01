package binder

import (
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/stdlib"
)

func libraryType(f *ast.NodeFactory, t stdlib.Type) *ast.Node {
	var kind ast.Kind
	switch t.Kind {
	case stdlib.TypeVariant:
		kind = ast.KindVARIANT
	case stdlib.TypeInteger:
		kind = ast.KindINTEGER
	case stdlib.TypeLong:
		kind = ast.KindLONG
	case stdlib.TypeString:
		kind = ast.KindSTRING
	case stdlib.TypeBoolean:
		kind = ast.KindBOOLEAN
	case stdlib.TypeSingle:
		kind = ast.KindSINGLE
	case stdlib.TypeDouble:
		kind = ast.KindDOUBLE
	case stdlib.TypeObject:
		kind = ast.KindOBJECT
	case stdlib.TypeByte:
		kind = ast.KindBYTE
	case stdlib.TypeDate:
		kind = ast.KindDATE
	case stdlib.TypeCurrency:
		kind = ast.KindCURRENCY
	default:
		panic("unsupported catalog type")
	}
	return f.NewTypeReference(f.NewToken(kind), nil, t.Array, nil)
}

func libraryLiteral(f *ast.NodeFactory, value stdlib.Literal) *ast.Node {
	switch value.Kind {
	case stdlib.NoLiteral:
		return nil
	case stdlib.NumericLiteral:
		return f.NewNumericLiteral(value.Text)
	case stdlib.StringLiteral, stdlib.NullStringLiteral:
		return f.NewStringLiteral(value.Text)
	default:
		panic("unsupported catalog literal")
	}
}

// Constant signs are expressions, preserving the operand's declared type and
// the original vbObjectError declaration. Optional defaults retain their existing
// synthetic literal representation through libraryLiteral during migration.
func libraryConstantLiteral(f *ast.NodeFactory, value stdlib.Literal) *ast.Node {
	if value.Kind == stdlib.NumericLiteral && strings.HasPrefix(value.Text, "-") {
		return f.NewPrefixUnaryExpression(ast.KindMINUS, f.NewNumericLiteral(value.Text[1:]))
	}
	return libraryLiteral(f, value)
}

func libraryParameters(f *ast.NodeFactory, entries []stdlib.Parameter) []*ast.Node {
	var parameters []*ast.Node
	for _, p := range entries {
		mods := ast.ParameterByVal
		if p.ByRef {
			mods = ast.ParameterByRef
		}
		if p.Optional {
			mods |= ast.ParameterOptional
		}
		name := f.NewIdentifier(p.Name)
		decl := f.NewParameterDeclaration(mods, name, libraryType(f, p.Type), libraryLiteral(f, p.Default), nil)
		symbol := &ast.Symbol{Flags: ast.SymbolFlagsFunctionScopedVariable, Name: p.Name, ValueDeclaration: decl, Declarations: []*ast.Node{decl}}
		decl.Symbol, name.Symbol = symbol, symbol
		parameters = append(parameters, decl)
	}
	return parameters
}

func libraryProcedureSymbol(fn *ast.Node, name string, id stdlib.MemberID, flags ast.SymbolFlags, owner *ast.Symbol) *ast.Symbol {
	setParents(fn, nil)
	symbol := &ast.Symbol{Flags: flags, Name: name, ValueDeclaration: fn, Declarations: []*ast.Node{fn}, Parent: owner, Library: id}
	fn.Symbol, fn.AsFunctionDeclaration().Name().Symbol = symbol, symbol
	for _, p := range fn.AsFunctionDeclaration().Parameters.Nodes {
		p.Symbol.Parent = symbol
	}
	return symbol
}

// Materialize fresh declarations, defaults, and symbols per ambient scope. Only
// immutable scalar/string descriptor data is shared between project snapshots.
func addCatalogLibrary(f *ast.NodeFactory, exports ast.SymbolTable, namespace *ast.Symbol) {
	modules := make(ast.SymbolTable)
	for _, entry := range stdlib.Modules() {
		symbol := &ast.Symbol{Flags: ast.SymbolFlagsValueModule, Name: entry.Name, Parent: namespace, Exports: make(ast.SymbolTable)}
		key := ast.SymbolNameKey(entry.Name)
		modules[key], exports[key] = symbol, symbol
	}
	enums := make(ast.SymbolTable)
	for _, entry := range stdlib.Enums() {
		name := f.NewIdentifier(entry.Name)
		decl := f.NewEnumDeclaration(ast.ModifierPublic, name, f.NewNodeList(nil))
		members := make(ast.SymbolTable)
		symbol := &ast.Symbol{Flags: ast.SymbolFlagsRegularEnum, Name: entry.Name, Parent: namespace, ValueDeclaration: decl, Declarations: []*ast.Node{decl}, Members: members, Exports: members}
		decl.Symbol, name.Symbol = symbol, symbol
		key := ast.SymbolNameKey(entry.Name)
		enums[key], exports[key] = symbol, symbol
	}
	// Root exports alias canonical declarations in their defining module/enum.
	addExport := func(name, ownerName string, symbol *ast.Symbol) {
		key := strings.ToLower(name) // Preserve distinct library $ forms.
		symbol.Parent = namespace
		if owner := modules.Lookup(ownerName); owner != nil {
			symbol.Parent = owner
			owner.Exports[key] = symbol
		} else if owner := enums.Lookup(ownerName); owner != nil {
			symbol.Parent = owner
			owner.Exports[key] = symbol
		}
		exports[key] = symbol
	}
	for _, entry := range stdlib.Functions() {
		parameters := libraryParameters(f, entry.Parameters)
		fn := f.NewFunctionDeclaration(ast.ModifierPublic, f.NewIdentifier(entry.Name), f.NewNodeList(parameters), f.NewBlock(nil))
		fn.AsFunctionDeclaration().Type = libraryType(f, entry.Result)
		symbol := libraryProcedureSymbol(fn, entry.Name, entry.ID, ast.SymbolFlagsFunction, namespace)
		addExport(entry.Name, entry.Module, symbol)
	}
	for _, entry := range stdlib.Constants() {
		name := f.NewIdentifier(entry.Name)
		if owner := enums.Lookup(entry.Owner); owner != nil {
			decl := f.NewEnumMember(name, libraryConstantLiteral(f, entry.Value))
			symbol := &ast.Symbol{Flags: ast.SymbolFlagsEnumMember, Name: entry.Name, ValueDeclaration: decl, Declarations: []*ast.Node{decl}, Library: entry.ID}
			decl.Symbol, name.Symbol = symbol, symbol
			owner.ValueDeclaration.AsEnumDeclaration().Members.Nodes = append(owner.ValueDeclaration.AsEnumDeclaration().Members.Nodes, decl)
			addExport(entry.Name, entry.Owner, symbol)
			continue
		}
		decl := f.NewVariableDeclaration(name, libraryType(f, entry.Type), libraryConstantLiteral(f, entry.Value), nil)
		setParents(f.NewVariableStatement(ast.ModifierPublic, ast.KindCONST, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{decl}), true)), nil)
		symbol := &ast.Symbol{Flags: ast.SymbolFlagsFunctionScopedVariable, Name: entry.Name, ValueDeclaration: decl, Declarations: []*ast.Node{decl}, Parent: namespace, Library: entry.ID}
		decl.Symbol, name.Symbol = symbol, symbol
		addExport(entry.Name, entry.Owner, symbol)
	}
	for _, owner := range enums {
		setParents(owner.ValueDeclaration, nil)
	}
	for _, entry := range stdlib.Classes() {
		flags := ast.SymbolFlagsClass
		if entry.Singleton {
			flags = ast.SymbolFlagsValueModule
		}
		owner := &ast.Symbol{Flags: flags, Name: entry.Name, Library: entry.ID, Exports: make(ast.SymbolTable)}
		if !entry.Singleton {
			id := f.NewIdentifier(entry.Name)
			owner.ValueDeclaration, owner.Declarations = id, []*ast.Node{id}
			owner.Members = make(ast.SymbolTable)
			id.Symbol = owner
		}
		addExport(entry.Name, "", owner)
		for _, member := range entry.Members {
			name := f.NewIdentifier(member.Name)
			var symbol *ast.Symbol
			if member.Kind == stdlib.VariableMember {
				decl := f.NewVariableDeclaration(name, libraryType(f, member.Type), nil, nil)
				setParents(decl, nil)
				symbol = &ast.Symbol{Flags: ast.SymbolFlagsFunctionScopedVariable, Name: member.Name, Library: member.ID, Parent: owner, ValueDeclaration: decl, Declarations: []*ast.Node{decl}}
				decl.Symbol, name.Symbol = symbol, symbol
			} else {
				parameters := f.NewNodeList(libraryParameters(f, member.Parameters))
				var fn *ast.Node
				flags := ast.SymbolFlagsFunction
				switch member.Kind {
				case stdlib.SubMember:
					fn = f.NewSubDeclaration(ast.ModifierPublic, name, parameters, f.NewBlock(nil))
				case stdlib.GetMember:
					fn = f.NewGetAccessor(ast.ModifierPublic, name, parameters, f.NewBlock(nil))
					fn.AsFunctionDeclaration().Type = libraryType(f, member.Type)
					flags = ast.SymbolFlagsGetAccessor
				default:
					panic("unsupported catalog class member")
				}
				symbol = libraryProcedureSymbol(fn, member.Name, member.ID, flags, owner)
			}
			key := ast.SymbolNameKey(member.Name)
			owner.Exports[key] = symbol
			if owner.Members != nil {
				owner.Members[key] = symbol
			}
			if entry.DefaultMember == member.ID {
				owner.DefaultMember = symbol
			}
		}
	}
}

// Ambient namespaces and classes are identified by their owning library root,
// not a list of built-in names or intrinsic-number ranges. User projects and
// modules can shadow these names without becoming ambient containers themselves.
func (b *Binder) isAmbientContainer(symbol *ast.Symbol) bool {
	if symbol.Flags&(ast.SymbolFlagsValueModule|ast.SymbolFlagsClass) == 0 {
		return false
	}
	for owner := symbol; owner != nil; owner = owner.Parent {
		if owner.Parent == nil && owner.Flags&ast.SymbolFlagsValueModule != 0 && b.intrinsics.Lookup(owner.Name) == owner {
			return true
		}
	}
	return false
}
