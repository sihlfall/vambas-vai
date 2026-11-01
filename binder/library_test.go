package binder

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestCatalogScopesOwnTheirDeclarations(t *testing.T) {
	first, second := newIntrinsicScope(), newIntrinsicScope()
	entries := make(map[string]stdlib.MemberID)
	owners := make(map[string]string)
	for _, fn := range stdlib.Functions() {
		entries[fn.Name] = fn.ID
		owners[fn.Name] = fn.Module
	}
	for _, c := range stdlib.Constants() {
		entries[c.Name] = c.ID
		owners[c.Name] = c.Owner
	}
	firstNodes := make(map[*ast.Node]bool)
	for name, id := range entries {
		left, right := first.Lookup(name), second.Lookup(name)
		if left == nil || right == nil || left == right || left.Library != id || right.Library != id {
			t.Fatalf("%s lost its catalog identity or scope isolation", name)
		}
		if left.Intrinsic != ast.IntrinsicNone || right.Intrinsic != ast.IntrinsicNone {
			t.Fatalf("%s still depends on a legacy intrinsic identity", name)
		}
		for _, item := range []struct {
			scope  ast.SymbolTable
			symbol *ast.Symbol
		}{{first, left}, {second, right}} {
			root := item.scope.Lookup("VBA")
			owner := root
			if module := root.Exports.Lookup(owners[name]); module != nil {
				owner = module
			}
			if item.symbol.Parent != owner || owner.Exports.Lookup(name) != item.symbol || root.Exports.Lookup(name) != item.symbol {
				t.Fatalf("%s lost its namespace owner or qualified alias", name)
			}
		}
		for index, symbol := range []*ast.Symbol{left, right} {
			root := symbol.ValueDeclaration
			for root.Parent != nil {
				root = root.Parent
			}
			var visit func(*ast.Node) bool
			visit = func(n *ast.Node) bool {
				if index == 0 {
					firstNodes[n] = true
				} else if firstNodes[n] {
					t.Fatalf("%s shares declaration/default syntax between scopes", name)
				}
				return n.ForEachChild(func(child *ast.Node) bool {
					if child.Parent != n {
						t.Fatalf("%s has a broken declaration parent link", name)
					}
					return visit(child)
				})
			}
			visit(root)
		}
		if left.Flags&ast.SymbolFlagsFunction != 0 {
			lp := left.ValueDeclaration.AsFunctionDeclaration().Parameters.Nodes
			rp := right.ValueDeclaration.AsFunctionDeclaration().Parameters.Nodes
			for i := range lp {
				if lp[i].Symbol == rp[i].Symbol || lp[i].Symbol.Parent != left || rp[i].Symbol.Parent != right {
					t.Fatalf("%s shares or misowns a parameter symbol", name)
				}
			}
		}
	}
	for _, module := range stdlib.Modules() {
		left, right := first.Lookup(module.Name), second.Lookup(module.Name)
		if left == nil || right == nil || left == right || left.Parent != first.Lookup("VBA") || right.Parent != second.Lookup("VBA") {
			t.Fatalf("%s lost its module ownership or scope isolation", module.Name)
		}
		if left != first.Lookup("VBA").Exports.Lookup(module.Name) || right != second.Lookup("VBA").Exports.Lookup(module.Name) {
			t.Fatalf("%s lost its module alias", module.Name)
		}
	}
	for _, enum := range stdlib.Enums() {
		left, right := first.Lookup(enum.Name), second.Lookup(enum.Name)
		if left == nil || right == nil || left == right || left.Parent != first.Lookup("VBA") || right.Parent != second.Lookup("VBA") {
			t.Fatalf("%s lost its enum ownership or scope isolation", enum.Name)
		}
		if left.Flags != ast.SymbolFlagsRegularEnum || left.ValueDeclaration.Kind != ast.KindEnumDeclaration || len(left.Members) == 0 {
			t.Fatalf("%s is not a populated enum declaration", enum.Name)
		}
		for _, member := range left.ValueDeclaration.AsEnumDeclaration().Members.Nodes {
			if member.Symbol.Parent != left || member.Symbol.Flags != ast.SymbolFlagsEnumMember || left.Members.Lookup(member.Symbol.Name) != member.Symbol || left.Exports.Lookup(member.Symbol.Name) != member.Symbol {
				t.Fatalf("%s has an incorrectly owned enum member", enum.Name)
			}
		}
	}
	if first.Lookup("Left") == first.Lookup("Left$") || first.Lookup("String") == first.Lookup("String$") {
		t.Fatal("catalog materialization merged distinct $ forms")
	}
}

func TestCatalogClassesOwnTheirDeclarations(t *testing.T) {
	first, second := newIntrinsicScope(), newIntrinsicScope()
	firstNodes := make(map[*ast.Node]bool)
	for _, class := range stdlib.Classes() {
		left, right := first.Lookup(class.Name), second.Lookup(class.Name)
		if left == nil || right == nil || left == right || left.Library != class.ID || right.Library != class.ID || left.Intrinsic != ast.IntrinsicNone || right.Intrinsic != ast.IntrinsicNone {
			t.Fatalf("%s lost its catalog identity or scope isolation", class.Name)
		}
		for index, item := range []struct {
			scope ast.SymbolTable
			owner *ast.Symbol
		}{{first, left}, {second, right}} {
			root, owner := item.scope.Lookup("VBA"), item.owner
			if owner.Parent != root || root.Exports.Lookup(class.Name) != owner {
				t.Fatalf("%s lost its owner or qualified alias", class.Name)
			}
			if class.Singleton != (owner.Flags&ast.SymbolFlagsValueModule != 0) {
				t.Fatalf("%s changed its singleton/instance model", class.Name)
			}
			var visit func(*ast.Node) bool
			visit = func(n *ast.Node) bool {
				if index == 0 {
					firstNodes[n] = true
				} else if firstNodes[n] {
					t.Fatalf("%s shares syntax between scopes", class.Name)
				}
				return n.ForEachChild(func(child *ast.Node) bool {
					if child.Parent != n {
						t.Fatalf("%s has a broken parent link", class.Name)
					}
					return visit(child)
				})
			}
			if owner.ValueDeclaration != nil {
				visit(owner.ValueDeclaration)
			}
			for _, member := range class.Members {
				symbol := owner.Exports.Lookup(member.Name)
				if symbol == nil || symbol.Library != member.ID || symbol.Intrinsic != ast.IntrinsicNone || symbol.Parent != owner {
					t.Fatalf("%s.%s lost its identity or owner", class.Name, member.Name)
				}
				if !class.Singleton && owner.Members.Lookup(member.Name) != symbol {
					t.Fatal("class member/export aliases diverged")
				}
				if root.Exports.Lookup(member.Name) == symbol || item.scope.Lookup(member.Name) == symbol {
					t.Fatal("class member leaked into root exports")
				}
				if member.ID == class.DefaultMember && owner.DefaultMember != symbol {
					t.Fatal("default member lost its declaration identity")
				}
				decl := symbol.ValueDeclaration
				if symbol.Declarations[0] != decl || decl.Symbol != symbol || ast.DeclarationName(decl).Symbol != symbol {
					t.Fatal("member declaration/symbol links diverged")
				}
				visit(decl)
				if member.Kind != stdlib.VariableMember {
					for i, p := range decl.AsFunctionDeclaration().Parameters.Nodes {
						if p.Symbol.Parent != symbol || p.Symbol.ValueDeclaration != p || ast.DeclarationName(p).Symbol != p.Symbol {
							t.Fatal("member parameter symbol lost its owner")
						}
						if index == 1 && p.Symbol == left.Exports.Lookup(member.Name).ValueDeclaration.AsFunctionDeclaration().Parameters.Nodes[i].Symbol {
							t.Fatal("member parameter symbol shared across scopes")
						}
					}
				}
			}
		}
	}
}
