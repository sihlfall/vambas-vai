package lowering

import (
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

func (b *procedureBuilder) libraryCall(n *ast.Node, r *checker.CallResolution, signature ir.LibrarySignature) ir.ValueID {
	args := make([]ir.ValueID, len(r.Signature.Parameters))
	variant := ir.Type{Scalar: ir.Variant}
	// CallResolution retains source order even for named arguments. Evaluate and
	// snapshot every argument, including both IIf alternatives, before the call.
	for _, a := range r.Arguments {
		var value ir.ValueID
		if a.Mode == checker.ArgumentDefault {
			if a.Default.Kind == ast.KindStringLiteral {
				value = b.stringLiteral(n, utf16.Encode([]rune(a.Default.AsStringLiteral().Text)), false)
			} else if v, ok := b.checker.GetNumericLiteralValue(a.Default); ok {
				value = b.literal(n, ir.Type{Scalar: ir.Long}, int64(v))
			} else {
				b.unsupported(n, "library default argument")
				return 0
			}
		} else if a.Expression != nil {
			value = b.expression(a.Expression)
			if a.Parameter.Type.Kind == checker.TypeLong {
				if b.types[value].Scalar == ir.String || b.types[value].Scalar == ir.Boolean {
					value = b.coerce(n, value, variant)
				}
				value = b.coerce(n, value, ir.Type{Scalar: ir.Long})
			}
		} else {
			b.unsupported(n, "library argument form")
			return 0
		}
		args[a.ParameterIndex] = b.coerce(n, value, variant)
	}
	result := signature.Result
	result.Form = ir.Result
	id := b.emit(ir.Instruction{Op: ir.OpLibraryCall, Library: r.Signature.Declaration.Symbol.Library, Type: result, Args: args, Effects: signature.Effects, Source: source(n)})
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: signature.Result, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
}
