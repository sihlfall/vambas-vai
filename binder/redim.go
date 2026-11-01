package binder

import "sihlfall.ch/vambas-vai/ast"

// Explicit declarations have already been collected from the whole procedure
// and all modules. ReDim declares a local array only when no such name exists,
// including under Option Explicit; bounds remain executable expressions.
func (b *Binder) predeclareReDims(node *ast.Node) {
	if node == nil || ast.IsProcedureDeclaration(node) {
		return
	}
	if node.Kind == ast.KindReDimDeclaration {
		d := node.AsReDimDeclaration()
		if d.Expression.Kind != ast.KindIdentifier || d.Expression.AsIdentifier().Text == "" {
			return
		}
		name := d.Expression.AsIdentifier().Text
		symbol := b.container.Locals.Lookup(name)
		if symbol == nil {
			var ambiguous bool
			symbol, ambiguous = b.moduleValue(b.file, name, d.Expression)
			if ambiguous {
				return
			}
		}
		if symbol == nil && b.project != nil {
			var ambiguous bool
			symbol, ambiguous = b.lookupProjectName(name, d.Expression)
			if ambiguous {
				return
			}
		}
		if symbol == nil {
			symbol = b.intrinsics.Lookup(name)
		}
		if symbol == nil {
			b.declareLocal(node, d.Expression)
		}
		return
	}
	node.ForEachChild(func(child *ast.Node) bool { b.predeclareReDims(child); return false })
}

func (b *Binder) bindReDim(node *ast.Node) {
	for _, decl := range node.AsReDimStatement().Declarations.Nodes {
		d := decl.AsReDimDeclaration()
		decl.FlowNode = b.currentFlow
		b.bind(d.Expression)
		decl.Symbol = d.Expression.Symbol
		for _, dimension := range d.Dimensions.Nodes {
			b.bind(dimension.AsArrayDimension().LowerBound)
			b.bind(dimension.AsArrayDimension().UpperBound)
		}
		b.bindTypeAnnotation(d.Type, false)
		if b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
			b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsArrayMutation, decl, b.currentFlow)
		}
	}
}
