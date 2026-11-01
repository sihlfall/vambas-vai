package checker

import (
	"strconv"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
)

func (c *Checker) collectDefaultMembers(node *ast.Node, file *ast.SourceFile) {
	if node.Kind == ast.KindAttributeStatement && file.ModuleKind.IsClass() {
		a := node.AsAttributeStatement()
		if a.Name.Kind == ast.KindPropertyAccessExpression && len(a.Values.Nodes) == 1 {
			name := a.Name.AsPropertyAccessExpression()
			if name.Expression != nil && name.Expression.Kind == ast.KindIdentifier && strings.EqualFold(name.Name.AsIdentifier().Text, "VB_UserMemId") && a.Values.Nodes[0].Kind == ast.KindNumericLiteral {
				value, err := strconv.ParseFloat(a.Values.Nodes[0].AsNumericLiteral().Text, 64)
				member := file.Locals.Lookup(name.Expression.AsIdentifier().Text)
				if err == nil && value == 0 && member != nil {
					if previous := c.defaults[file.Symbol]; previous != nil && previous != member {
						c.defaultConflicts[file.Symbol] = true
					} else {
						c.defaults[file.Symbol] = member
					}
				}
			}
		}
	}
	node.ForEachChild(func(child *ast.Node) bool { c.collectDefaultMembers(child, file); return false })
}

func (c *Checker) GetDefaultMember(t *Type) *ast.Symbol {
	if t == nil || c.defaultConflicts[t.Symbol] {
		return nil
	}
	if t.Symbol != nil && t.Symbol.DefaultMember != nil {
		return t.Symbol.DefaultMember
	}
	return c.defaults[t.Symbol]
}

func (c *Checker) memberReceiver(node *ast.Node) *ast.Node {
	m := node.AsPropertyAccessExpression()
	if m.Expression != nil {
		return m.Expression
	}
	if m.WithTarget != nil {
		return m.WithTarget.AsWithStatement().Expression
	}
	return nil
}

func (c *Checker) memberType(node *ast.Node) *Type {
	m := node.AsPropertyAccessExpression()
	receiver := c.memberReceiver(node)
	if receiver == nil {
		return c.intrinsic[TypeError]
	}
	if m.OperatorToken.Kind == ast.KindEXCLAMATIONPOINT {
		c.types[m.Name] = c.intrinsic[TypeString]
		return c.application(node, receiver, c.typeOfNode(receiver), []*ast.Node{m.Name})
	}
	if c.symbols[node] != nil {
		return c.typeOfReference(node)
	}
	t := c.typeOfNode(receiver)
	if t.Kind == TypeStructure {
		if symbol := t.Symbol.Members.Lookup(m.Name.AsIdentifier().Text); symbol != nil {
			c.symbols[node], c.symbols[m.Name] = symbol, symbol
			return c.typeOfReference(node)
		}
		if !c.boundErrors[m.Name] && !c.boundErrors[node] {
			c.report(DiagnosticMember, m.Name, "This user-defined type has no member with this name")
		}
		return c.intrinsic[TypeError]
	}
	if t.Kind == TypeClass {
		var symbol *ast.Symbol
		if ast.SourceFileOf(t.Symbol.ValueDeclaration) == c.owners[node] {
			symbol = t.Symbol.Members.Lookup(m.Name.AsIdentifier().Text)
		} else {
			symbol = t.Symbol.Exports.Lookup(m.Name.AsIdentifier().Text)
		}
		if symbol != nil && symbol.Flags&(ast.SymbolFlagsRegularEnum|ast.SymbolFlagsEnumMember|ast.SymbolFlagsUserDefinedType) == 0 {
			c.symbols[node], c.symbols[m.Name] = symbol, symbol
			return c.typeOfReference(node)
		}
		if !c.boundErrors[m.Name] && !c.boundErrors[node] {
			c.report(DiagnosticMember, m.Name, "No accessible member has this name")
		}
		return c.intrinsic[TypeError]
	}
	if t.Kind == TypeObject || t.Kind == TypeVariant || t.Kind == TypeUnknown {
		if calleeReference(node) {
			return c.intrinsic[TypeVariant]
		}
		// This is late-bound, not an implicit local or a fabricated signature.
		r := &CallResolution{Kind: CallDynamic, Node: node, Target: node, Receiver: receiver, WithTarget: m.WithTarget, ResultType: c.intrinsic[TypeVariant], Valid: true, Deferred: true}
		c.calls[node] = r
		if t.Kind == TypeUnknown {
			r.ResultType = t
		}
		return r.ResultType
	}
	if !c.boundErrors[m.Name] && !c.boundErrors[node] && t.Kind != TypeError {
		c.report(DiagnosticMember, m.Name, "The receiver has no members")
	}
	return c.intrinsic[TypeError]
}

func (c *Checker) defaultSignature(node *ast.Node, t *Type, kind ast.Kind) (*Signature, *ast.Symbol) {
	symbol := c.GetDefaultMember(t)
	var decl *ast.Node
	if symbol != nil {
		if symbol.Flags&ast.SymbolFlagsFunction != 0 && (kind == ast.KindGetAccessor && symbol.ValueDeclaration.Kind == ast.KindFunctionDeclaration || kind == ast.KindSubDeclaration) {
			decl = symbol.ValueDeclaration
		} else {
			decl = ast.PropertyAccessor(symbol, kind)
		}
	}
	if decl == nil || decl.AsFunctionDeclaration().Modifiers()&ast.ModifierPrivate != 0 {
		c.report(DiagnosticDefaultMember, node, "The class has no unambiguous public default member for this operation")
		return nil, symbol
	}
	return c.signatureOfDeclaration(decl), symbol
}

// application resolves array indexing, class default-member invocation, or a
// late-bound application after any preceding procedure invocation has completed.
func (c *Checker) application(node, target *ast.Node, t *Type, args []*ast.Node) *Type {
	if t.Kind == TypeArray {
		return c.indexCall(node, target, t, args)
	}
	if t.Kind == TypeClass {
		kind := ast.KindGetAccessor
		if node.Parent != nil && node.Parent.Kind == ast.KindCallStatement {
			kind = ast.KindSubDeclaration
		}
		var rhs *ast.Node
		if statement := directAssignment(node); statement != nil {
			kind = ast.KindLetAccessor
			if statement.Kind == ast.KindSetStatement {
				kind = ast.KindSetAccessor
			}
			rhs = statement.AsLetStatement().RExpression
		}
		sig, symbol := c.defaultSignature(node, t, kind)
		if sig != nil {
			return c.invocation(node, target, target, sig, args, rhs, symbol).ResultType
		}
		c.calls[node] = &CallResolution{Kind: CallInvalid, Node: node, Target: target, Receiver: target, ResultType: c.intrinsic[TypeError]}
		return c.intrinsic[TypeError]
	}
	if t.Kind == TypeObject || t.Kind == TypeVariant || t.Kind == TypeUnknown {
		result := c.intrinsic[TypeVariant]
		if t.Kind == TypeUnknown {
			result = t
		}
		r := &CallResolution{Kind: CallDynamic, Node: node, Target: target, Receiver: target, ResultType: result, Valid: true, Deferred: true}
		if node.Kind == ast.KindPropertyAccessExpression {
			r.WithTarget = node.AsPropertyAccessExpression().WithTarget
		}
		c.calls[node] = r
		for i, arg := range args {
			expr := arg
			if arg.Kind == ast.KindNamedArgument {
				expr = arg.AsNamedArgument().Expression
			}
			binding := &ArgumentBinding{Argument: arg, Expression: expr, ParameterIndex: -1, ParamArrayIndex: -1, SourceIndex: i, Mode: ArgumentDeferred}
			if node.Kind == ast.KindPropertyAccessExpression && node.AsPropertyAccessExpression().OperatorToken.Kind == ast.KindEXCLAMATIONPOINT && arg == node.AsPropertyAccessExpression().Name {
				text := arg.AsIdentifier().Text
				binding.StringValue = &text
			}
			r.Arguments = append(r.Arguments, binding)
		}
		return result
	}
	if t.Kind != TypeError {
		c.report(DiagnosticNotCallable, node, "This expression is neither callable nor an array or object")
	}
	c.calls[node] = &CallResolution{Kind: CallInvalid, Node: node, Target: target, ResultType: c.intrinsic[TypeError]}
	return c.intrinsic[TypeError]
}

func directAssignment(node *ast.Node) *ast.Node {
	if parent := node.Parent; parent != nil && (parent.Kind == ast.KindLetStatement || parent.Kind == ast.KindSetStatement) && parent.AsLetStatement().LExpression == node {
		return parent
	}
	return nil
}

// simpleType resolves a class default-value chain in a scalar evaluation context.
// The syntax's own type/symbol remains the object reference; the chain is separate.
func (c *Checker) simpleType(node *ast.Node) (t *Type) {
	if previous := c.valueTypes[node]; previous != nil {
		return previous
	}
	t = c.typeOfNode(node)
	defer func() { c.valueTypes[node] = t }()
	seen := make(map[*ast.Symbol]bool)
	for t.Kind == TypeClass {
		if seen[t.Symbol] {
			c.report(DiagnosticDefaultMember, node, "Default-member evaluation is circular")
			return c.intrinsic[TypeError]
		}
		seen[t.Symbol] = true
		sig, member := c.defaultSignature(node, t, ast.KindGetAccessor)
		if sig == nil {
			return c.intrinsic[TypeError]
		}
		// A value conversion may coexist with an explicit call on this syntax.
		// Keep each implicit default invocation in a separate operation record.
		old, oldSignature := c.calls[node], c.resolved[node]
		delete(c.calls, node)
		r := c.invocation(node, node, node, sig, nil, nil, member)
		c.calls[node], c.resolved[node] = old, oldSignature
		if old == nil {
			delete(c.calls, node)
		}
		if oldSignature == nil {
			delete(c.resolved, node)
		}
		c.valueCalls[node] = append(c.valueCalls[node], r)
		t = r.ResultType
	}
	return t
}
