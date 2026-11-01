package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"unicode/utf16"
)

func (b *procedureBuilder) captureInvocation(n *ast.Node, value ir.ValueID) ir.ValueID {
	if b.types[value].Scalar != ir.Variant {
		value = b.coerce(n, value, ir.Type{Scalar: ir.Variant})
	}
	return b.operation(n, ir.OpInvocationCapture, ir.Type{Scalar: ir.Variant}, ir.MayRaise|ir.ManagesLifetime, value)
}

func (b *procedureBuilder) dynamicInvoke(n *ast.Node, r *checker.CallResolution, receiver ir.ValueID) ir.ValueID {
	assignment := directAssignment(n)
	setValue := ir.ValueID(0)
	if assignment != nil && assignment.Kind == ast.KindSetStatement {
		setValue = b.coerce(assignment, b.objectExpression(assignment.AsLetStatement().RExpression), ir.Type{Scalar: ir.Object})
		setValue = b.coerce(assignment, setValue, ir.Type{Scalar: ir.Variant})
	}
	member := ""
	target := r.Target
	with := r.WithTarget
	var base *ast.Node
	if target != nil && target.Kind == ast.KindPropertyAccessExpression {
		m := target.AsPropertyAccessExpression()
		if with == nil {
			with = m.WithTarget
		}
		base = m.Expression
		if m.OperatorToken.Kind == ast.KindDOT {
			member = m.Name.AsIdentifier().Text
		}
	} else {
		base = target
	}
	if receiver == 0 && r.Prefix != nil {
		receiver = b.resolvedCall(n, r.Prefix, 0)
	}
	if receiver == 0 {
		if with != nil && b.objectCaptures[with.AsWithStatement().Expression] != 0 {
			receiver = b.load(n, b.objectCaptures[with.AsWithStatement().Expression])
		} else if base != nil {
			if base == n && b.checker.GetSymbolAtLocation(base) != nil && b.checker.GetSymbolAtLocation(base).Flags&ast.SymbolFlagsFunctionScopedVariable != 0 {
				receiver = b.objectReceiver(n, base, nil)
			} else {
				storage := b.checker.GetStorageAtLocation(base)
				if b.checker.GetTypeAtLocation(base).Kind == checker.TypeVariant && storage != nil && storage.Addressable {
					receiver = b.reference(base, storage)
				} else {
					receiver = b.objectExpression(base)
				}
			}
		} else {
			b.unsupported(n, "unresolved invocation receiver")
			return 0
		}
	}
	receiver = b.captureInvocation(n, receiver)
	args := []ir.ValueID{receiver}
	var names []string
	forced := n.Kind == ast.KindCallExpression && n.AsCallExpression().HasParentheses && n.Parent != nil && n.Parent.Kind == ast.KindCallStatement && n.Parent.AsCallStatement().CallKeyword == nil && len(r.Arguments) == 1
	for _, arg := range r.Arguments {
		name := ""
		if arg.Argument != nil && arg.Argument.Kind == ast.KindNamedArgument {
			name = arg.Argument.AsNamedArgument().Name.AsIdentifier().Text
		}
		var value ir.ValueID
		if arg.StringValue != nil {
			value = b.stringLiteral(n, utf16.Encode([]rune(*arg.StringValue)), false)
		} else if arg.Expression == nil || arg.Expression.Kind == ast.KindOmittedExpression {
			value = b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: ir.Type{Scalar: ir.Variant}, Integer: 2, Source: source(n)})
		} else {
			storage := b.checker.GetStorageAtLocation(arg.Expression)
			if !forced && !arg.ForcedValue && storage != nil && storage.Addressable && b.checker.GetTypeAtLocation(arg.Expression).Kind != checker.TypeStructure {
				value = b.reference(arg.Expression, storage)
				if auto := b.autoReferences[value]; auto.class != 0 || auto.builtin != 0 {
					b.readReference(arg.Expression, value)
				}
				value = b.variantReference(arg.Expression, value)
			} else {
				value = b.objectExpression(arg.Expression)
			}
		}
		if b.types[value].Form != ir.Reference {
			value = b.coerce(n, value, ir.Type{Scalar: ir.Variant})
		}
		args = append(args, value)
		names = append(names, name)
	}
	kind := ir.InvokeMethod | ir.InvokeGet
	if n.Parent != nil && n.Parent.Kind == ast.KindCallStatement {
		kind = ir.InvokeMethod | ir.InvokeDiscard
	}
	if assignment != nil {
		a := assignment.AsLetStatement()
		kind = ir.InvokeLet
		var value ir.ValueID
		if assignment.Kind == ast.KindSetStatement {
			kind = ir.InvokeSet
			value = setValue
		} else {
			value = b.expression(a.RExpression)
		}
		args = append(args, b.coerce(assignment, value, ir.Type{Scalar: ir.Variant}))
		names = append(names, "\x01value")
	}
	t := ir.Type{Scalar: ir.Variant, Form: ir.Result}
	id := b.emit(ir.Instruction{Op: ir.OpObjectInvoke, Type: t, Member: member, Invocation: kind, Names: names, Args: args, Effects: ir.MayRaise | ir.ManagesLifetime | ir.WritesStorage, Source: source(n)})
	t.Form = ir.Value
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
}

func directAssignment(n *ast.Node) *ast.Node {
	if n.Parent != nil && (n.Parent.Kind == ast.KindLetStatement || n.Parent.Kind == ast.KindSetStatement) && n.Parent.AsLetStatement().LExpression == n {
		return n.Parent
	}
	return nil
}
