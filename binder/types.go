package binder

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
)

func (b *Binder) declareType(node *ast.Node) {
	d := node.AsTypeDeclaration()
	flags := ast.SymbolFlagsUserDefinedType
	if node.Kind == ast.KindEnumDeclaration {
		flags = ast.SymbolFlagsRegularEnum
	}
	symbol := b.newSymbol(flags, d.Name.AsIdentifier().Text, node, b.file.Symbol)
	node.Symbol, d.Name.Symbol = symbol, symbol
	symbol.Members = make(ast.SymbolTable)
	symbol.Exports = symbol.Members
	key := ast.SymbolNameKey(symbol.Name)
	if symbol.Name != "" {
		if previous := b.file.Locals[key]; previous != nil {
			b.report(ast.BindingDuplicateDeclaration, d.Name, previous.ValueDeclaration, "Duplicate type declaration")
		} else {
			b.file.Locals[key] = symbol
			if d.Modifiers&ast.ModifierPrivate == 0 {
				b.file.Symbol.Exports[key] = symbol
			}
		}
	}
	for _, member := range d.Members.Nodes {
		name := ast.DeclarationName(member)
		memberFlags := ast.SymbolFlagsProperty
		if node.Kind == ast.KindEnumDeclaration {
			memberFlags = ast.SymbolFlagsEnumMember
		}
		m := b.newSymbol(memberFlags, name.AsIdentifier().Text, member, symbol)
		member.Symbol, name.Symbol = m, m
		if m.Name == "" {
			continue
		}
		key := ast.SymbolNameKey(m.Name)
		if previous := symbol.Members[key]; previous != nil {
			b.report(ast.BindingDuplicateDeclaration, name, previous.ValueDeclaration, "Duplicate type member")
		} else {
			symbol.Members[key] = m
		}
		if node.Kind == ast.KindEnumDeclaration {
			// Unlike TypeScript, VBA enum members also enter the module's
			// constant scope, without changing their enum as symbol parent.
			if b.enumAliases == nil {
				b.enumAliases = make(map[string][]*ast.Symbol)
			}
			b.enumAliases[key] = append(b.enumAliases[key], m)
			if previous := b.file.Locals[key]; previous != nil && previous.Flags&ast.SymbolFlagsEnumMember == 0 {
				b.report(ast.BindingDuplicateDeclaration, name, previous.ValueDeclaration, "Duplicate module constant")
			} else {
				if b.file.Locals[key] == nil {
					b.file.Locals[key] = m
				}
				if d.Modifiers&ast.ModifierPrivate == 0 && b.file.Symbol.Exports[key] == nil {
					b.file.Symbol.Exports[key] = m
				}
			}
		}
	}
}

func (b *Binder) moduleValue(file *ast.SourceFile, name string, use *ast.Node) (*ast.Symbol, bool) {
	table := file.Locals
	aliases := b.enumAliases
	if file != b.file {
		table = file.Symbol.Exports
		if b.project != nil {
			aliases = b.project.enumAliases[file]
		}
	}
	symbol := table.Lookup(name)
	if symbol == nil || symbol.Flags&ast.SymbolFlagsEnumMember == 0 {
		return symbol, false
	}
	var result *ast.Symbol
	for _, candidate := range aliases[ast.SymbolNameKey(name)] {
		if file != b.file && candidate.Parent.ValueDeclaration.AsEnumDeclaration().Modifiers&ast.ModifierPrivate != 0 {
			continue
		}
		if result != nil {
			b.report(ast.BindingAmbiguousName, use, result.ValueDeclaration, "Enum constant name is ambiguous; qualify it with its enum")
			return nil, true
		}
		result = candidate
	}
	return symbol, false
}

func (b *Binder) bindTypeDeclaration(node *ast.Node) {
	for _, member := range node.AsTypeDeclaration().Members.Nodes {
		if member.Kind == ast.KindEnumMember {
			b.bindConstantExpression(member.AsEnumMember().Initializer, true)
		} else {
			b.bindVariableDeclaration(member)
		}
	}
}

// Type lookup ignores procedure values, including parameters with the same name.
// A declaration's own module is used even while resolving an imported receiver's
// field/return annotation before that module's bodies have been bound.
func (b *Binder) resolveTypeName(node *ast.Node) *ast.Symbol {
	if node == nil {
		return nil
	}
	if node.Symbol != nil && node.Symbol.Flags&ast.SymbolFlagsType != 0 {
		return node.Symbol
	}
	file := ast.SourceFileOf(node)
	if file == nil {
		file = b.file
	}
	ambient := func(name string) *ast.Symbol {
		s := b.intrinsics.Lookup(name)
		if s != nil && s.Flags&ast.SymbolFlagsType != 0 {
			return s
		}
		return nil
	}
	lookup := func(name string) *ast.Symbol {
		if s := file.Locals.Lookup(name); s != nil && s.Flags&ast.SymbolFlagsType != 0 {
			return s
		}
		if b.project == nil {
			return ambient(name)
		}
		var result *ast.Symbol
		for _, candidate := range b.project.globals[ast.SymbolNameKey(name)] {
			if candidate.Flags&ast.SymbolFlagsType == 0 {
				continue
			}
			if result != nil {
				if file == b.file {
					b.report(ast.BindingAmbiguousName, node, result.ValueDeclaration, fmt.Sprintf("Type name %q is ambiguous", name))
				}
				return nil
			}
			result = candidate
		}
		if result != nil {
			return result
		}
		s := b.project.symbol.Exports.Lookup(name)
		if s != nil && s.Flags&ast.SymbolFlagsClass != 0 {
			return s
		}
		return ambient(name)
	}
	var symbol *ast.Symbol
	if node.Kind == ast.KindIdentifier {
		symbol = lookup(node.AsIdentifier().Text)
	} else if node.Kind == ast.KindQualifiedName && b.project == nil {
		q := node.AsQualifiedName()
		if q.Left.Kind == ast.KindIdentifier && file.Locals.Lookup(q.Left.AsIdentifier().Text) == nil {
			owner := b.intrinsics.Lookup(q.Left.AsIdentifier().Text)
			if owner != nil && owner.Flags&ast.SymbolFlagsValueModule != 0 {
				symbol = owner.Exports.Lookup(q.Right.AsIdentifier().Text)
				if symbol != nil && symbol.Flags&ast.SymbolFlagsType != 0 {
					q.Left.Symbol = owner
					q.Right.Symbol = symbol
				}
			}
		}
	} else if node.Kind == ast.KindQualifiedName && b.project != nil {
		q := node.AsQualifiedName()
		var namespace func(*ast.Node) *ast.Symbol
		namespace = func(n *ast.Node) *ast.Symbol {
			if n.Kind == ast.KindIdentifier {
				name := n.AsIdentifier().Text
				if ast.SymbolNameKey(name) == ast.SymbolNameKey(b.project.symbol.Name) {
					n.Symbol = b.project.symbol
				} else {
					n.Symbol = b.project.symbol.Exports.Lookup(name)
					if n.Symbol == nil {
						n.Symbol = b.intrinsics.Lookup(name)
					}
				}
			} else if n.Kind == ast.KindQualifiedName {
				if owner := namespace(n.AsQualifiedName().Left); owner == b.project.symbol {
					n.Symbol = owner.Exports.Lookup(n.AsQualifiedName().Right.AsIdentifier().Text)
					n.AsQualifiedName().Right.Symbol = n.Symbol
				}
			}
			return n.Symbol
		}
		owner := namespace(q.Left)
		if owner == b.project.symbol {
			name := q.Right.AsIdentifier().Text
			if s := owner.Exports.Lookup(name); s != nil && s.Flags&ast.SymbolFlagsClass != 0 {
				symbol = s
			} else {
				for _, candidate := range b.project.globals[ast.SymbolNameKey(name)] {
					if candidate.Flags&ast.SymbolFlagsType != 0 {
						if symbol != nil {
							if file == b.file {
								b.report(ast.BindingAmbiguousName, node, symbol.ValueDeclaration, "Qualified type name is ambiguous")
							}
							return nil
						}
						symbol = candidate
					}
				}
			}
		} else if owner != nil && (owner == b.intrinsics.Lookup("Rubberduck") || owner == b.intrinsics.Lookup("VBA")) {
			symbol = owner.Exports.Lookup(q.Right.AsIdentifier().Text)
		} else if owner != nil && owner.ValueDeclaration != nil && owner.ValueDeclaration.Kind == ast.KindSourceFile {
			table := owner.Exports
			if ast.SourceFileOf(owner.ValueDeclaration) == file {
				table = file.Locals
			}
			symbol = table.Lookup(q.Right.AsIdentifier().Text)
			// VBA permits module-qualified UDTs, but not enum type names.
			if symbol != nil && symbol.Flags&ast.SymbolFlagsUserDefinedType == 0 {
				symbol = nil
			}
		}
		if symbol != nil && symbol.Flags&ast.SymbolFlagsType != 0 {
			q.Right.Symbol = symbol
		}
	}
	if symbol != nil && symbol.Flags&ast.SymbolFlagsType != 0 {
		node.Symbol = symbol
		return symbol
	}
	return nil
}
