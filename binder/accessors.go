package binder

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
)

func (b *Binder) declareAccessor(node *ast.Node) {
	declaration := node.AsFunctionDeclaration()
	name := declaration.Name().AsIdentifier().Text
	key := ast.SymbolNameKey(name)
	flags := ast.SymbolFlagsGetAccessor
	if node.Kind == ast.KindLetAccessor {
		flags = ast.SymbolFlagsLetAccessor
	}
	if node.Kind == ast.KindSetAccessor {
		flags = ast.SymbolFlagsSetAccessor
	}
	previous := b.file.Locals[key]
	var symbol *ast.Symbol
	if name != "" && previous != nil && previous.Flags&ast.SymbolFlagsAccessor != 0 && ast.PropertyAccessor(previous, node.Kind) == nil {
		// Like typescript-go's accessor merging, retain each declaration and
		// union its flags. VBA permits both Let and Set on the same property.
		symbol = previous
		symbol.Flags |= flags
		symbol.Declarations = append(symbol.Declarations, node)
		if node.Kind == ast.KindGetAccessor {
			symbol.ValueDeclaration = node
		}
	} else {
		symbol = b.newSymbol(flags, name, node, b.file.Symbol)
		if name != "" && previous != nil {
			related := ast.PropertyAccessor(previous, node.Kind)
			if related == nil {
				related = previous.ValueDeclaration
			}
			b.report(ast.BindingDuplicateDeclaration, declaration.Name(), related, fmt.Sprintf("Conflicting property declaration %q", name))
			// Keep the first table entry; the invalid declaration still owns a
			// symbol so its parameters, body, and diagnostics can be bound.
			node.Symbol, declaration.Name().Symbol = symbol, symbol
			return
		}
		if name != "" {
			b.file.Locals[key] = symbol
		}
	}
	node.Symbol, declaration.Name().Symbol = symbol, symbol
	if name != "" && declaration.Modifiers()&ast.ModifierPrivate == 0 {
		b.file.Symbol.Exports[key] = symbol
	}
}

func (b *Binder) bindPropertyWrite(left *ast.Node, symbol *ast.Symbol) {
	wanted := ast.SymbolFlagsLetAccessor
	kind := ast.KindLetAccessor
	if left.Parent != nil && left.Parent.Kind == ast.KindSetStatement {
		wanted = ast.SymbolFlagsSetAccessor
		kind = ast.KindSetAccessor
	}
	if symbol.Flags&wanted == 0 {
		b.report(ast.BindingInvalidAssignmentTarget, left, nil, "Property has no accessor for this kind of assignment")
		return
	}
	if !b.checkAccessorVisibility(left, symbol, kind) {
		return
	}
	left.Symbol = symbol
	if b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
		// Index and RHS expressions have already been evaluated. Record the
		// setter invocation before the write, without invoking a getter.
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsCall, left, b.currentFlow)
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsAssignment, left, b.currentFlow)
	}
}
