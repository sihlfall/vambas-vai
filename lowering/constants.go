package lowering

import (
	"math"
	"math/big"
	"strings"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

func (b *procedureBuilder) numericLiteral(n *ast.Node, t ir.Type, value float64) ir.ValueID {
	if t.Scalar == ir.Currency && value == 0 {
		return b.literal(n, t, 0)
	}
	if t.Scalar == ir.Date || t.Scalar == ir.Currency {
		v := b.emit(ir.Instruction{Op: ir.OpConstant, Type: ir.Type{Scalar: ir.Double}, Float: value, Source: source(n)})
		return b.coerce(n, v, t)
	}
	if ir.Floating(t) {
		return b.emit(ir.Instruction{Op: ir.OpConstant, Type: t, Float: value, Source: source(n)})
	}
	return b.literal(n, t, int64(value))
}

// Currency literals retain their decimal spelling through exact constant
// evaluation; neither Double rounding nor runtime String allocation is involved.
func (b *procedureBuilder) currencyLiteral(site *ast.Node, spelling string) ir.ValueID {
	spelling = strings.TrimSuffix(spelling, "@")
	spelling = strings.ReplaceAll(strings.ReplaceAll(spelling, "d", "e"), "D", "e")
	value, ok := new(big.Rat).SetString(spelling)
	if !ok {
		b.unsupported(site, "Currency literal spelling")
		return 0
	}
	numerator := new(big.Int).Abs(value.Num())
	numerator.Mul(numerator, big.NewInt(10000))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(numerator, value.Denom(), rem)
	cmp := new(big.Int).Lsh(rem, 1).Cmp(value.Denom())
	if cmp > 0 || cmp == 0 && q.Bit(0) != 0 {
		q.Add(q, big.NewInt(1))
	}
	if value.Sign() < 0 {
		q.Neg(q)
	}
	if !q.IsInt64() {
		b.unsupported(site, "out-of-range Currency literal")
		return 0
	}
	return b.literal(site, ir.Type{Scalar: ir.Currency}, q.Int64())
}

func (b *procedureBuilder) validateDefault(p *checker.Parameter) {
	if p.Modifiers&ast.ParameterOptional == 0 {
		return
	}
	if p.Type.Kind == checker.TypeArray || p.Type.Kind == checker.TypeStructure {
		b.unsupported(p.Declaration, "array/UDT optional parameters")
		return
	}
	if p.Default == nil {
		return
	}
	if p.Type.Kind == checker.TypeDate || p.Type.Kind == checker.TypeCurrency || b.checker.GetTypeAtLocation(p.Default).Kind == checker.TypeCurrency {
		b.unsupported(p.Default, "explicit Date/Currency optional defaults")
		return
	}
	if objectType := p.Type.Kind == checker.TypeClass || p.Type.Kind == checker.TypeObject; objectType && p.Default.Kind == ast.KindNOTHING {
		return
	}
	if units, _, ok := b.constantString(p.Default, 0); ok {
		if p.Type.Kind == checker.TypeString || p.Type.Kind == checker.TypeVariant {
			return
		}
		text := string(utf16.Decode(units))
		if p.Type.Kind == checker.TypeBoolean && (strings.EqualFold(text, "True") || strings.EqualFold(text, "False")) {
			return
		}
	}
	if value, ok := b.checker.GetNumericConstantExpression(p.Default); ok {
		switch p.Type.Kind {
		case checker.TypeByte:
			value = math.RoundToEven(value)
			if value < 0 || value > 255 {
				b.unsupported(p.Default, "out-of-range optional default")
			}
		case checker.TypeInteger:
			value = math.RoundToEven(value)
			if value < math.MinInt16 || value > math.MaxInt16 {
				b.unsupported(p.Default, "out-of-range optional default")
			}
		case checker.TypeLong:
			value = math.RoundToEven(value)
			if value < math.MinInt32 || value > math.MaxInt32 {
				b.unsupported(p.Default, "out-of-range optional default")
			}
		case checker.TypeSingle:
			if math.IsInf(float64(float32(value)), 0) {
				b.unsupported(p.Default, "out-of-range optional default")
			}
		}
		return
	}
	if p.Type.Kind == checker.TypeVariant && (p.Default.Kind == ast.KindNULL || p.Default.Kind == ast.KindEMPTY) {
		return
	}
	b.unsupported(p.Default, "nonconstant or unsupported optional defaults")
}

// Materialize constants at the caller's source location. An omitted ByRef
// parameter receives a fresh activation-local temporary, never library storage.
func (b *procedureBuilder) defaultArgument(site *ast.Node, p *checker.Parameter) ir.ValueID {
	t := b.scalar(site, p.Type, false)
	n := p.Default
	if n == nil {
		if t.Scalar == ir.Object {
			return b.emit(ir.Instruction{Op: ir.OpObjectNull, Type: t, Source: source(site)})
		}
		if t.Scalar == ir.String {
			return b.stringLiteral(site, nil, false)
		}
		return b.numericLiteral(site, t, 0)
	}
	if n.Kind == ast.KindNOTHING && t.Scalar == ir.Object {
		return b.emit(ir.Instruction{Op: ir.OpObjectNull, Type: t, Source: source(site)})
	}
	if units, null, ok := b.constantString(n, 0); ok {
		if t.Scalar == ir.Boolean {
			text := string(utf16.Decode(units))
			if strings.EqualFold(text, "True") {
				return b.literal(site, t, -1)
			}
			if strings.EqualFold(text, "False") {
				return b.literal(site, t, 0)
			}
		}
		v := b.stringLiteral(site, units, null)
		if t.Scalar != ir.String && t.Scalar != ir.Variant {
			v = b.coerce(site, v, ir.Type{Scalar: ir.Variant})
		}
		return b.coerce(site, v, t)
	}
	if n.Kind == ast.KindNULL || n.Kind == ast.KindEMPTY {
		code := int64(0)
		if n.Kind == ast.KindNULL {
			code = 1
		}
		return b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: t, Integer: code, Source: source(site)})
	}
	value, ok := b.checker.GetNumericConstantExpression(n)
	if !ok {
		b.unsupported(site, "optional default value")
		return 0
	}
	// Coerce the constant's own declared value first, then the parameter type.
	from := b.scalar(site, b.checker.GetTypeAtLocation(n), false)
	if from.Scalar == ir.Boolean {
		if value != 0 {
			value = -1
		}
	}
	if from.Scalar == ir.Integer || from.Scalar == ir.Long {
		value = math.RoundToEven(value)
	}
	v := b.numericLiteral(site, from, value)
	if t.Scalar == ir.String {
		v = b.coerce(site, v, ir.Type{Scalar: ir.Variant})
	}
	return b.coerce(site, v, t)
}

func (b *procedureBuilder) argumentTemporary(site *ast.Node, value ir.ValueID, name string) ir.ValueID {
	t := b.types[value]
	slot := b.addSlot(site, nil, name, t, ir.Temporary, false)
	b.store(site, slot, value)
	t.Form = ir.Reference
	return b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(site)})
}
