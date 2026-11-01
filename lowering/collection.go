package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
	"strings"
	"unicode/utf16"
)

func (b *procedureBuilder) objectExpression(n *ast.Node) ir.ValueID {
	if n.Kind == ast.KindParenthesizedExpression {
		return b.objectExpression(n.AsParenthesizedExpression().Inner)
	}
	if r := b.checker.GetCallResolution(n); r != nil && r.Kind != checker.CallDynamic {
		if id, ok := b.collectionCall(n, r, ir.Type{Scalar: ir.Object}); ok {
			return id
		}
	}
	return b.rawExpression(n)
}
func (b *procedureBuilder) objectReceiver(n, receiver, with *ast.Node) ir.ValueID {
	if with != nil && (receiver == nil || b.objectCaptures[with.AsWithStatement().Expression] != 0) {
		if slot := b.objectCaptures[with.AsWithStatement().Expression]; slot != 0 {
			return b.load(n, slot)
		}
	}
	if receiver == n && receiver != nil {
		if symbol := b.checker.GetSymbolAtLocation(receiver); symbol != nil && symbol.Flags&ast.SymbolFlagsFunctionScopedVariable != 0 {
			storage := &checker.Storage{Kind: checker.StorageVariable, Node: receiver, Symbol: symbol, Type: b.checker.GetTypeOfSymbol(symbol), Addressable: true, Writable: true}
			if receiver.Kind == ast.KindPropertyAccessExpression {
				storage.Receiver = receiver.AsPropertyAccessExpression().Expression
				storage.WithTarget = receiver.AsPropertyAccessExpression().WithTarget
			}
			return b.coerce(n, b.readReference(n, b.reference(n, storage)), ir.Type{Scalar: ir.Object})
		}
	}
	if receiver == nil || receiver == n {
		b.unsupported(n, "unresolved object receiver")
		return 0
	}
	return b.coerce(n, b.objectExpression(receiver), ir.Type{Scalar: ir.Object})
}
func (b *procedureBuilder) collectionCall(n *ast.Node, r *checker.CallResolution, want ir.Type) (ir.ValueID, bool) {
	if r == nil || !r.Valid {
		return 0, false
	}
	code := 0
	var receiver *ast.Node
	with := r.WithTarget
	if r.Signature != nil && r.Signature.Declaration.Symbol != nil {
		switch r.Signature.Declaration.Symbol.Library {
		case stdlib.CollectionAdd:
			code = 1
		case stdlib.CollectionItem:
			code = 2
		case stdlib.CollectionRemove:
			code = 3
		case stdlib.CollectionCount:
			code = 4
		}
		if code != 0 {
			receiver = r.Receiver
		}
	}
	if code != 0 && r.Target != nil && r.Target.Kind == ast.KindPropertyAccessExpression {
		m := r.Target.AsPropertyAccessExpression()
		receiver = m.Expression
		if with == nil {
			with = m.WithTarget
		}
	}
	if code == 0 && r.Kind == checker.CallDynamic && r.Target != nil {
		if r.Target.Kind == ast.KindPropertyAccessExpression {
			m := r.Target.AsPropertyAccessExpression()
			receiver = m.Expression
			if with == nil {
				with = m.WithTarget
			}
			t := b.checker.GetTypeAtLocation(receiver)
			if receiver == nil && with != nil {
				t = b.checker.GetTypeAtLocation(with.AsWithStatement().Expression)
			}
			if t.Kind == checker.TypeObject {
				code = map[string]int{"add": 1, "item": 2, "remove": 3, "count": 4}[strings.ToLower(m.Name.AsIdentifier().Text)]
			}
		} else if b.checker.GetTypeAtLocation(r.Target).Kind == checker.TypeObject {
			code = 2
			receiver = r.Target
		}
	}
	if code == 0 {
		return 0, false
	}
	if r.Prefix != nil {
		b.unsupported(n, "chained Collection default application")
		return 0, true
	}
	t := ir.Type{Scalar: ir.Void}
	if code == 2 {
		t = ir.Type{Scalar: ir.Variant}
		if want.Scalar == ir.Object {
			t = want
		}
	}
	if code == 4 {
		t = b.scalar(n, r.ResultType, false)
	}
	args := []ir.ValueID{b.objectReceiver(n, receiver, with)}
	names := []string{}
	for _, a := range r.Arguments {
		if a.SourceIndex < 0 {
			continue
		}
		name := ""
		if a.Argument != nil && a.Argument.Kind == ast.KindNamedArgument {
			name = a.Argument.AsNamedArgument().Name.AsIdentifier().Text
		}
		names = append(names, name)
		var value ir.ValueID
		if a.StringValue != nil {
			value = b.coerce(n, b.stringLiteral(n, utf16.Encode([]rune(*a.StringValue)), false), ir.Type{Scalar: ir.Variant})
		} else if a.Mode == checker.ArgumentMissing || a.Expression != nil && a.Expression.Kind == ast.KindOmittedExpression {
			value = b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: ir.Type{Scalar: ir.Variant}, Integer: 2, Source: source(n)})
		} else if a.Expression != nil {
			value = b.expression(a.Expression)
			if b.types[value].Scalar != ir.Object {
				value = b.coerce(a.Expression, value, ir.Type{Scalar: ir.Variant})
			}
		} else {
			b.unsupported(n, "Collection argument expression")
			return 0, true
		}
		args = append(args, value)
	}
	result := t
	result.Form = ir.Result
	id := b.emit(ir.Instruction{Op: ir.OpCollectionInvoke, Type: result, Integer: int64(code), Args: args, Names: names, Effects: ir.MayRaise | ir.ManagesLifetime | ir.WritesStorage, Source: source(n)})
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)}), true
}
func (b *procedureBuilder) objectWith(node *ast.Node) {
	n := node.AsWithStatement()
	value := b.coerce(node, b.objectExpression(n.Expression), ir.Type{Scalar: ir.Object})
	t := ir.Type{Scalar: ir.Object}
	slot := b.addSlot(node, nil, "$with.object", t, ir.LoopState, false)
	b.store(node, slot, value)
	parent := b.scope
	scope := ir.ScopeID(len(b.procedure.Scopes) + 1)
	b.procedure.Scopes = append(b.procedure.Scopes, ir.Scope{ID: scope, Parent: parent, Cleanup: []ir.SlotID{slot}})
	done := b.block(node)
	if b.objectCaptures == nil {
		b.objectCaptures = map[*ast.Node]ir.SlotID{}
	}
	b.objectCaptures[n.Expression] = slot
	b.scope = scope
	body := b.block(n.Statement)
	b.jump(body)
	b.link(b.statement, done, 0)
	b.activate(body)
	b.sequence(n.Statement)
	b.fallthroughTo(done)
	delete(b.objectCaptures, n.Expression)
	b.scope = parent
	b.activate(done)
}
