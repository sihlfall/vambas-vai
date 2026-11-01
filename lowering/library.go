package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

// Catalog identity selects both helper-backed and specialized calls. Keep
// storage-aware operations separate from the Variant-snapshot helper ABI.
func (b *procedureBuilder) catalogCall(n *ast.Node, r *checker.CallResolution) ir.ValueID {
	kind := r.Signature.Declaration.Symbol.Library
	if signature, ok := ir.LookupLibraryCall(kind); ok {
		return b.libraryCall(n, r, signature)
	}
	switch kind {
	case stdlib.ErrRaise, stdlib.ErrClear:
		return b.errorCall(n, r)
	case stdlib.CreateObject:
		if len(r.Arguments) != 1 || r.Arguments[0].Expression == nil {
			b.unsupported(n, "CreateObject arguments")
			return 0
		}
		value := b.coerce(n, b.expression(r.Arguments[0].Expression), ir.Type{Scalar: ir.String})
		return b.operation(n, ir.OpCreateObject, ir.Type{Scalar: ir.Object}, ir.MayRaise|ir.ManagesLifetime, value)
	case stdlib.LBound, stdlib.UBound:
		return b.arrayBound(n, r)
	case stdlib.Mid, stdlib.MidString:
		return b.midCall(n, r)
	}
	if len(r.Arguments) != 1 || r.Arguments[0].Expression == nil {
		b.unsupported(n, "library argument form")
		return 0
	}
	switch kind {
	case stdlib.VarType, stdlib.IsEmpty, stdlib.IsNull,
		stdlib.IsMissing, stdlib.IsArray, stdlib.CVar:
		return b.variantLibraryCall(n, r)
	case stdlib.Len, stdlib.LenB:
		return b.stringLength(n, r)
	case stdlib.ChrW:
		a := r.Arguments[0]
		value := b.conversion(a.Expression, b.expression(a.Expression), a.Conversion)
		return b.operation(n, ir.OpChrW, ir.Type{Scalar: ir.String}, ir.MayRaise, value)
	}
	b.unsupported(n, "runtime library function")
	return 0
}
