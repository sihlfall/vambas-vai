package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
	"strings"
)

func virtualClass(s *ast.Symbol) uint8 {
	if s != nil {
		if s.Library == stdlib.Collection {
			return 4
		}
		switch s.Intrinsic {
		case ast.IntrinsicAssertClass:
			return 1
		case ast.IntrinsicPermissiveAssertClass:
			return 2
		case ast.IntrinsicFakesProvider:
			return 3
		}
	}
	return 0
}
func (l *lowerer) autoNew(n *ast.Node) uint8 {
	if n.Kind != ast.KindVariableDeclaration {
		return 0
	}
	a := n.AsVariableDeclaration().Type
	if a != nil && a.Kind == ast.KindTypeReference && a.AsTypeReference().NewKeyword != nil {
		return virtualClass(l.checker.GetTypeOfSymbol(n.Symbol).Symbol)
	}
	return 0
}

func (l *lowerer) autoClass(n *ast.Node) ir.ClassID {
	if n.Kind != ast.KindVariableDeclaration {
		return 0
	}
	a := n.AsVariableDeclaration().Type
	if a != nil && a.Kind == ast.KindTypeReference && a.AsTypeReference().NewKeyword != nil {
		return l.classes[l.checker.GetTypeOfSymbol(n.Symbol).Symbol]
	}
	return 0
}
func (b *procedureBuilder) testLibraryCall(n *ast.Node, r *checker.CallResolution) (ir.ValueID, bool) {
	if r == nil || !r.Valid {
		return 0, false
	}
	var member, receiver, with *ast.Node
	if r.Target != nil && r.Target.Kind == ast.KindPropertyAccessExpression {
		member = r.Target
		receiver = member.AsPropertyAccessExpression().Expression
		with = member.AsPropertyAccessExpression().WithTarget
	}
	if member == nil || receiver == nil && with == nil {
		return 0, false
	}
	static := r.Signature != nil && r.Signature.Declaration.Symbol != nil && r.Signature.Declaration.Symbol.Intrinsic == ast.IntrinsicTestAssert
	receiverType := receiver
	if receiverType == nil {
		receiverType = with.AsWithStatement().Expression
	}
	dynamic := r.Kind == checker.CallDynamic && b.checker.GetTypeAtLocation(receiverType).Kind == checker.TypeObject
	if !static && !dynamic {
		return 0, false
	}
	method := strings.ToLower(member.AsPropertyAccessExpression().Name.AsIdentifier().Text)
	methods := []string{"areequal", "arenotequal", "aresame", "arenotsame", "istrue", "isfalse", "isnothing", "isnotnothing", "sequenceequals", "notsequenceequals", "fail", "inconclusive", "succeed"}
	code := 0
	for i, m := range methods {
		if m == method {
			code = i + 1
		}
	}
	if code == 0 {
		b.unsupported(n, "virtual object member "+method)
		return 0, true
	}
	// A late-bound call remains deferred in the checker. Only the backend's private
	// virtual handle can receive it; the runtime validates method/argument binding.
	args := []ir.ValueID{b.objectReceiver(n, receiver, with)}
	names := []string{}
	for _, a := range r.Arguments {
		if a.SourceIndex < 0 {
			continue
		} // Optional Message is absent, rather than re-evaluated.
		if a.Expression == nil {
			b.unsupported(n, "omitted assertion arguments")
			return 0, true
		}
		name := ""
		if a.Argument != nil && a.Argument.Kind == ast.KindNamedArgument {
			name = a.Argument.AsNamedArgument().Name.AsIdentifier().Text
		}
		objectArgument := false
		if code == 3 || code == 4 || code == 7 || code == 8 {
			limit := 1
			if code == 3 || code == 4 {
				limit = 2
			}
			objectArgument = name == "" && a.SourceIndex < limit || strings.EqualFold(name, "Value") && limit == 1 || (strings.EqualFold(name, "Expected") || strings.EqualFold(name, "Actual")) && limit == 2
		}
		var v ir.ValueID
		if objectArgument {
			v = b.objectExpression(a.Expression)
		} else {
			v = b.expression(a.Expression)
		}
		if b.types[v].Scalar != ir.Object {
			v = b.coerce(a.Expression, v, ir.Type{Scalar: ir.Variant})
		}
		args = append(args, v)
		names = append(names, name)
	}
	t := ir.Type{Scalar: ir.Void, Form: ir.Result}
	id := b.emit(ir.Instruction{Op: ir.OpTestAssert, Type: t, Args: args, Names: names, Integer: int64(code), Effects: ir.MayRaise | ir.ManagesLifetime | ir.WritesStorage, Source: source(n)})
	t.Form = ir.Value
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)}), true
}
