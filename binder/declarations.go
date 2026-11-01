package binder

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
)

func (b *Binder) declareLocal(declaration, name *ast.Node) *ast.Symbol {
	if name == nil || name.AsIdentifier().Text == "" {
		return nil
	}
	text := name.AsIdentifier().Text
	key := ast.SymbolNameKey(text)
	symbol := b.newSymbol(ast.SymbolFlagsFunctionScopedVariable, text, declaration, b.container.Symbol)
	declaration.Symbol, name.Symbol = symbol, symbol
	if previous := b.container.Locals[key]; previous != nil {
		b.report(ast.BindingDuplicateDeclaration, name, previous.ValueDeclaration, fmt.Sprintf("Duplicate declaration of %q", text))
	} else {
		b.container.Locals[key] = symbol
	}
	return symbol
}

// VBA declarations inside If branches still belong to the procedure, and are
// visible before their textual declaration. Do not create TypeScript block scopes.
func (b *Binder) predeclareVariables(node *ast.Node) {
	if node == nil || ast.IsProcedureDeclaration(node) {
		return
	}
	if node.Kind == ast.KindVariableStatement {
		stmt := node.AsVariableStatement()
		for _, declaration := range stmt.DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
			symbol := b.declareLocal(declaration, declaration.AsVariableDeclaration().Name)
			if symbol != nil && b.container.Kind == ast.KindSourceFile && stmt.Modifiers&(ast.ModifierPublic|ast.ModifierGlobal) != 0 && b.container.Locals.Lookup(symbol.Name) == symbol {
				b.file.Symbol.Exports[ast.SymbolNameKey(symbol.Name)] = symbol
			}
		}
		return
	}
	node.ForEachChild(func(child *ast.Node) bool { b.predeclareVariables(child); return false })
}

func (b *Binder) bindTypeAnnotation(node *ast.Node, moduleOnly bool) {
	if node == nil || node.Kind != ast.KindTypeReference {
		return
	}
	b.resolveTypeName(node.AsTypeReference().TypeName)
	// Unknown library/type names remain for the checker, without implicit locals.
	b.bindConstantExpression(node.AsTypeReference().Length, moduleOnly)
}

func (b *Binder) bindVariableDeclaration(node *ast.Node) {
	decl := node.AsVariableDeclaration()
	decl.Name.FlowNode = b.currentFlow
	b.bindTypeAnnotation(decl.Type, false)
	if decl.Dimensions != nil {
		for _, dimension := range decl.Dimensions.Nodes {
			dim := dimension.AsArrayDimension()
			b.bindConstantExpression(dim.LowerBound, false)
			b.bindConstantExpression(dim.UpperBound, false)
		}
	}
	if ast.IsConstantSymbol(node.Symbol) {
		b.bindConstantExpression(decl.Initializer, false)
	} else {
		b.bind(decl.Initializer) // Retain usable children of invalid Dim initializers.
	}
}

func (b *Binder) bindConstantExpression(node *ast.Node, moduleOnly bool) {
	if node == nil {
		return
	}
	switch node.Kind {
	case ast.KindIdentifier:
		name := node.AsIdentifier().Text
		if name == "" {
			return
		}
		var symbol *ast.Symbol
		if !moduleOnly {
			symbol = b.container.Locals.Lookup(name)
		}
		if symbol == nil || b.container.Kind == ast.KindSourceFile {
			var ambiguous bool
			symbol, ambiguous = b.moduleValue(b.file, name, node)
			if ambiguous {
				return
			}
		}
		if symbol == nil && b.project != nil {
			var ambiguous bool
			symbol, ambiguous = b.lookupProjectName(name, node)
			if ambiguous {
				return
			}
		}
		if symbol == nil {
			symbol = b.intrinsics.Lookup(name)
		}
		node.Symbol = symbol
		if !ast.IsConstantSymbol(symbol) {
			b.report(ast.BindingConstantExpressionExpected, node, nil, fmt.Sprintf("%q is not a constant", name))
		}
	case ast.KindPropertyAccessExpression:
		symbol := b.resolveConstantMember(node, moduleOnly)
		if symbol != nil && !ast.IsConstantSymbol(symbol) {
			b.report(ast.BindingConstantExpressionExpected, node, nil, "Constant expression expected")
		}
	case ast.KindNumericLiteral, ast.KindStringLiteral, ast.KindTRUE, ast.KindFALSE, ast.KindEMPTY, ast.KindNULL, ast.KindNOTHING:
		return
	case ast.KindParenthesizedExpression:
		b.bindConstantExpression(node.AsParenthesizedExpression().Inner, moduleOnly)
	case ast.KindPrefixUnaryExpression:
		b.bindConstantExpression(node.AsPrefixUnaryExpression().Operand, moduleOnly)
	case ast.KindBinaryExpression:
		expression := node.AsBinaryExpression()
		b.bindConstantExpression(expression.Left, moduleOnly)
		b.bindConstantExpression(expression.Right, moduleOnly)
	default:
		b.report(ast.BindingConstantExpressionExpected, node, nil, "Constant expression expected")
	}
}
