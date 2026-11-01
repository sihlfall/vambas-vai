package binder

import "sihlfall.ch/vambas-vai/ast"

func isAssignmentTarget(node *ast.Node) bool {
	if node.Parent == nil {
		return false
	}
	return (node.Parent.Kind == ast.KindLetStatement || node.Parent.Kind == ast.KindSetStatement) && node.Parent.AsLetStatement().LExpression == node ||
		node.Parent.Kind == ast.KindForEachStatement && node.Parent.AsForEachStatement().Initializer == node
}

func isAssignmentReference(node *ast.Node) bool {
	if isAssignmentTarget(node) {
		return true
	}
	call := invocationForCallee(node)
	return call != nil && isAssignmentTarget(call)
}

func skipParenthesizedParents(node *ast.Node) *ast.Node {
	for node.Parent != nil && node.Parent.Kind == ast.KindParenthesizedExpression {
		node = node.Parent
	}
	return node
}

func invocationForCallee(node *ast.Node) *ast.Node {
	node = skipParenthesizedParents(node)
	if parent := node.Parent; parent != nil && parent.Kind == ast.KindCallExpression && parent.AsCallExpression().Expression == node {
		return parent
	}
	return nil
}

func isAddressOfOperand(node *ast.Node) bool {
	node = skipParenthesizedParents(node)
	return node.Parent != nil && node.Parent.Kind == ast.KindPrefixUnaryExpression && node.Parent.AsPrefixUnaryExpression().Operator == ast.KindADDRESSOF
}

func isArraySymbol(symbol *ast.Symbol) bool {
	if symbol == nil || symbol.ValueDeclaration == nil {
		return false
	}
	var dimensions *ast.NodeList
	var typeNode *ast.Node
	switch declaration := symbol.ValueDeclaration; declaration.Kind {
	case ast.KindVariableDeclaration, ast.KindFieldDeclaration:
		dimensions, typeNode = declaration.AsVariableDeclaration().Dimensions, declaration.AsVariableDeclaration().Type
	case ast.KindParameter:
		dimensions, typeNode = declaration.AsParameterDeclaration().Dimensions, declaration.AsParameterDeclaration().Type
	case ast.KindReDimDeclaration:
		return true // A declarative ReDim introduces a resizable array.
	}
	return dimensions != nil || typeNode != nil && typeNode.Kind == ast.KindTypeReference && typeNode.AsTypeReference().IsArray
}

func (b *Binder) bindCallExpression(node *ast.Node) {
	call := node.AsCallExpression()
	b.bind(call.Expression)
	if call.Arguments != nil {
		for _, argument := range call.Arguments.Nodes {
			b.bind(argument)
		}
	}
	target := ast.SkipParentheses(call.Expression)
	if target == nil || target.Kind == ast.KindIdentifier && target.AsIdentifier().Text == "" {
		return
	}
	if target.Symbol != nil && target.Symbol.Flags&ast.SymbolFlagsAccessor != 0 {
		node.Symbol = target.Symbol
	}
	// Known arrays are indexed without a call. Other applications may invoke a
	// procedure or a default member; model their possible side effects as calls.
	if !isAssignmentTarget(node) && !isArraySymbol(target.Symbol) && b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsCall, node, b.currentFlow)
	}
}

func (b *Binder) bindNewExpression(node *ast.Node) {
	// The class name is type-space syntax. Never create an implicit variable,
	// invoke a same-named procedure, or issue an Option Explicit value error.
	node.Symbol = b.resolveClassName(node.AsNewExpression().Expression)
	name := node.AsNewExpression().Expression
	if name != nil && name.Kind == ast.KindQualifiedName {
		name = name.AsQualifiedName().Right
	}
	if name == nil || name.Kind == ast.KindIdentifier && name.AsIdentifier().Text == "" {
		return
	}
	if b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsCall, node, b.currentFlow)
	}
}

func (b *Binder) bindAssignment(node *ast.Node) {
	statement := node.AsLetStatement()
	left := statement.LExpression
	b.bind(left)
	b.bind(statement.RExpression)
	b.bindAssignmentTargetFlow(left)
}

func (b *Binder) bindAssignmentTargetFlow(left *ast.Node) {
	if left.Kind == ast.KindIdentifier && (left.AsIdentifier().Text == "" || left.Symbol == nil) {
		return // Recovery placeholders and undeclared names were already handled.
	}
	base := left
	applications := 0
	for base.Kind == ast.KindCallExpression {
		applications++
		base = ast.SkipParentheses(base.AsCallExpression().Expression)
	}
	if applications <= 1 && base.Symbol != nil && base.Symbol.Flags&ast.SymbolFlagsAccessor != 0 {
		b.bindPropertyWrite(left, base.Symbol)
		return
	}
	if ast.IsConstantSymbol(base.Symbol) {
		b.report(ast.BindingAssignmentToConstant, left, nil, "Cannot assign to a constant")
		return
	}
	valid := false
	switch left.Kind {
	case ast.KindIdentifier:
		valid = left.Symbol.Flags&ast.SymbolFlagsFunctionScopedVariable != 0
	case ast.KindPropertyAccessExpression:
		valid = left.Symbol == nil || left.Symbol.Flags&(ast.SymbolFlagsFunctionScopedVariable|ast.SymbolFlagsProperty) != 0
	case ast.KindCallExpression:
		valid = base.Kind == ast.KindPropertyAccessExpression && base.Symbol == nil || base.Symbol != nil && base.Symbol.Flags&(ast.SymbolFlagsFunctionScopedVariable|ast.SymbolFlagsProperty) != 0
		// A function result may supply an array element or an object's default
		// setter. The checker resolves the argument queue and actual storage.
		valid = valid || base.Symbol != nil && base.Symbol.ValueDeclaration != nil && base.Symbol.ValueDeclaration.Kind == ast.KindFunctionDeclaration
		// P()(i) writes to a value returned by a getter, rather than invoking
		// P's setter. Its array/default-member compatibility needs a checker.
		valid = valid || applications > 1 && base.Symbol != nil && base.Symbol.Flags&ast.SymbolFlagsGetAccessor != 0
	}
	if !valid {
		b.report(ast.BindingInvalidAssignmentTarget, left, nil, "Assignment target is not a variable or property")
	} else if b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsAssignment, left, b.currentFlow)
	}
}
