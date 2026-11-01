package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

// Preserve the checker's source-order bindings. The ParamArray is an array of
// ByRef Variant parameters, not an array constructor or a spread operation.
func (b *procedureBuilder) paramArrayArgument(call *ast.Node, arg *checker.ArgumentBinding) ir.ValueID {
	if arg.Mode == checker.ArgumentMissing {
		return b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: ir.Type{Scalar: ir.Variant}, Integer: 2, Source: source(call)})
	}
	if arg.Expression == nil {
		b.unsupported(call, "missing ParamArray argument expression")
		return 0
	}
	// Deferred indexing of a Variant resolves to native array element storage,
	// as for an ordinary ByRef Variant formal. Keep its lock through the call.
	dynamic := arg.Mode == checker.ArgumentTemporary && !arg.ForcedValue && arg.Storage != nil && arg.Storage.Kind == checker.StorageDynamic && arg.Storage.Resolution != nil && b.checker.GetTypeAtLocation(arg.Storage.Resolution.Target).Kind == checker.TypeVariant
	if arg.Mode == checker.ArgumentReference || dynamic {
		ref := b.reference(arg.Expression, arg.Storage)
		if auto := b.autoReferences[ref]; auto.class != 0 || auto.builtin != 0 {
			b.readReference(arg.Expression, ref)
		}
		return b.variantReference(arg.Expression, ref)
	}
	if arg.Mode == checker.ArgumentValue || arg.Mode == checker.ArgumentTemporary {
		return b.conversion(arg.Expression, b.expression(arg.Expression), arg.Conversion)
	}
	b.unsupported(call, "ParamArray argument binding mode")
	return 0
}
func (b *procedureBuilder) variantReference(n *ast.Node, ref ir.ValueID) ir.ValueID {
	t := b.types[ref]
	if t == (ir.Type{Scalar: ir.Variant, Form: ir.Reference}) {
		return ref
	}
	if t.Form != ir.Reference || !((ir.ScalarValue(t) && t.Scalar != ir.Variant) || t.Scalar == ir.Object || ir.Array(t) && t.Scalar != ir.ArrayRecord) {
		b.unsupported(n, "Variant references to this storage type")
		return 0
	}
	if t.Scalar == ir.String {
		b.operation(n, ir.OpPinString, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.ManagesLifetime, ref)
	}
	in := ir.Instruction{Op: ir.OpVariantRef, Type: ir.Type{Scalar: ir.Variant, Form: ir.Reference}, Args: []ir.ValueID{ref}, Source: source(n)}
	if t.Scalar == ir.Object {
		declared := b.checker.GetTypeAtLocation(n)
		in.Class = b.classes[declared.Symbol]
		in.Integer = int64(virtualClass(declared.Symbol))
	}
	view := b.emit(in)
	if len(b.referenceTokens[ref]) != 0 {
		b.referenceTokens[view] = append([]ir.ValueID(nil), b.referenceTokens[ref]...)
	}
	return view
}
func (b *procedureBuilder) packParamArray(n *ast.Node, values []ir.ValueID) ir.ValueID {
	t := ir.Type{Scalar: ir.ArrayVariant}
	slot := b.addSlot(n, nil, "$paramarray", t, ir.Temporary, false)
	t.Form = ir.Reference
	ref := b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(n)})
	b.operation(n, ir.OpParamArrayPack, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime, append([]ir.ValueID{ref}, values...)...)
	return ref
}
