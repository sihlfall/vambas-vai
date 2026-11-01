package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

func (b *procedureBuilder) managedStorage(n *ast.Node, op ir.Op, t ir.Type, slot ir.SlotID, args ...ir.ValueID) ir.ValueID {
	effects := ir.MayRaise | ir.ManagesLifetime
	if op == ir.OpStoreManaged || op == ir.OpLoadManaged && slot > 0 && (b.procedure.Slots[slot-1].AutoNew != 0 || b.procedure.Slots[slot-1].AutoClass != 0) {
		effects |= ir.WritesStorage
	}
	resultType := t
	resultType.Form = ir.Result
	id := b.emit(ir.Instruction{Op: op, Type: resultType, Slot: slot, Args: args, Effects: effects, Source: source(n)})
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
}

func (b *procedureBuilder) variantLibraryCall(n *ast.Node, r *checker.CallResolution) ir.ValueID {
	a := r.Arguments[0].Expression
	kind := r.Signature.Declaration.Symbol.Library
	if kind == stdlib.VarType || kind == stdlib.IsEmpty || kind == stdlib.IsNull || kind == stdlib.IsMissing || kind == stdlib.IsArray {
		op, t := ir.OpVarType, ir.Type{Scalar: ir.Integer}
		if kind == stdlib.IsEmpty {
			op, t = ir.OpIsEmpty, ir.Type{Scalar: ir.Boolean}
		}
		if kind == stdlib.IsNull {
			op, t = ir.OpIsNull, ir.Type{Scalar: ir.Boolean}
		}
		if kind == stdlib.IsMissing {
			op, t = ir.OpIsMissing, ir.Type{Scalar: ir.Boolean}
		}
		if kind == stdlib.IsArray {
			op, t = ir.OpIsArray, ir.Type{Scalar: ir.Boolean}
		}
		if b.checker.GetTypeAtLocation(a).Kind != checker.TypeVariant {
			inputType := b.scalar(a, b.checker.GetTypeAtLocation(a), false)
			if inputType.Scalar == ir.Object {
				b.objectExpression(a)
			} else if b.checker.GetCallResolution(a) != nil || a.Kind != ast.KindIdentifier {
				b.expression(a)
			}
			value := int64(0)
			if ir.Array(inputType) && op == ir.OpIsArray {
				value = -1
			}
			if op == ir.OpVarType {
				value = map[ir.Scalar]int64{ir.Boolean: 11, ir.Byte: 17, ir.Integer: 2, ir.Long: 3, ir.Single: 4, ir.Double: 5, ir.String: 8, ir.Object: 9, ir.Date: 7, ir.Currency: 6}[inputType.Scalar]
				if ir.Array(inputType) {
					value = 8192 + arrayTag(ir.ElementType(inputType))
				}
			}
			return b.literal(n, t, value)
		}
		var input ir.ValueID
		if s := b.checker.GetStorageAtLocation(a); s != nil && s.Kind == checker.StorageVariable && s.Addressable {
			input = b.reference(a, s)
		} else {
			input = b.expression(a)
		}
		return b.operation(n, op, t, 0, input)
	}
	value := b.expression(a)
	value = b.coerce(n, value, ir.Type{Scalar: ir.Variant})
	// Explicit scalar conversions have their own catalog helper contract: unlike
	// implicit Let coercion, they can extract the code from an Error Variant.
	return value // CVar preserves the input value and subtype.
}

func (b *procedureBuilder) store(n *ast.Node, slot ir.SlotID, value ir.ValueID) {
	if ir.Managed(b.types[value]) {
		b.managedStorage(n, ir.OpStoreManaged, ir.Type{Scalar: ir.Void}, slot, value)
		return
	}
	b.emit(ir.Instruction{Op: ir.OpStore, Type: ir.Type{Scalar: ir.Void}, Slot: slot, Args: []ir.ValueID{value}, Effects: ir.WritesStorage, Source: source(n)})
}
