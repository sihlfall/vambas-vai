package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

func (b *procedureBuilder) errorCall(n *ast.Node, r *checker.CallResolution) ir.ValueID {
	if r.Signature.Declaration.Symbol.Library == stdlib.ErrClear {
		return b.operation(n, ir.OpErrClear, ir.Type{Scalar: ir.Void}, ir.WritesStorage|ir.ManagesLifetime|ir.WritesAmbientErr)
	}
	args := make([]ir.ValueID, 5)
	for _, a := range r.Arguments {
		if a.Mode == checker.ArgumentMissing {
			args[a.ParameterIndex] = b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: ir.Type{Scalar: ir.Variant}, Integer: 2, Source: source(n)})
		} else {
			args[a.ParameterIndex] = b.coerce(n, b.expression(a.Expression), ir.Type{Scalar: ir.Variant})
		}
	}
	return b.operation(n, ir.OpErrRaise, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime|ir.WritesAmbientErr, args...)
}

// These selectors belong to the existing Err IR/runtime contract, independently
// of catalog IDs and declaration ordering.
func errorField(id stdlib.MemberID) (int64, bool) {
	switch id {
	case stdlib.ErrNumber:
		return 0, true
	case stdlib.ErrDescription:
		return 1, true
	case stdlib.ErrSource:
		return 2, true
	case stdlib.ErrHelpFile:
		return 3, true
	case stdlib.ErrHelpContext:
		return 4, true
	}
	return 0, false
}

func (b *procedureBuilder) errorGet(n *ast.Node, field int64, t ir.Type) ir.ValueID {
	args := []ir.ValueID{b.literal(n, ir.Type{Scalar: ir.Long}, field)}
	effects := ir.MayRaise | ir.ReadsAmbientErr
	if ir.Managed(t) {
		effects |= ir.ManagesLifetime
	}
	return b.operation(n, ir.OpErrGet, t, effects, args...)
}
