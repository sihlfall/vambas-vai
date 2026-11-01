package llvm

import (
	"fmt"
	"math"

	"sihlfall.ch/vambas-vai/ir"
)

// LLVM's hexadecimal floating syntax stores the exact binary64 encoding, also
// for a binary32 value extended exactly to binary64. This preserves signed zero.
func floatingLiteral(v float64) string { return fmt.Sprintf("0x%016X", math.Float64bits(v)) }

func (f *function) nonfinite(id ir.ValueID, t ir.Type, value string) string {
	kind, typ := "f64", scalar(t)
	if t.Scalar == ir.Single {
		kind = "f32"
	}
	callee := "llvm.fabs." + kind
	f.module.intrinsics[callee] = fmt.Sprintf("declare %s @%s(%s)", typ, callee, typ)
	f.line("%%abs.fp%d = call %s @%s(%s %s)", id, typ, callee, typ, value)
	condition := fmt.Sprintf("%%nonfinite%d", id)
	f.line("%s = fcmp uge %s %%abs.fp%d, 0x7FF0000000000000", condition, typ, id)
	return condition
}

func (f *function) floatArithmetic(in ir.Instruction, args []operand) {
	name, typ := valueName(in.ID), scalar(in.Type)
	op := map[ir.Op]string{ir.OpAdd: "fadd", ir.OpSubtract: "fsub", ir.OpMultiply: "fmul", ir.OpDivide: "fdiv", ir.OpNegate: "fneg"}[in.Op]
	if in.Op == ir.OpNegate {
		f.line("%s = fneg %s %s", name, typ, args[0].text)
	} else {
		f.line("%s = %s %s %s, %s", name, op, typ, args[0].text, args[1].text)
	}
	r := result{condition: f.nonfinite(in.ID, in.Type, name), value: name, direct: true, source: in.Source}
	if in.Op == ir.OpDivide {
		f.line("%%div.zero%d = fcmp oeq %s %s, 0.000000e+00", in.ID, typ, args[1].text)
		f.line("%%div.nonzero%d = fcmp one %s %s, 0.000000e+00", in.ID, typ, args[0].text)
		f.line("%%div.zero.status%d = select i1 %%div.nonzero%d, i32 11, i32 6", in.ID, in.ID)
		r.status = fmt.Sprintf("%%div.status%d", in.ID)
		f.line("%s = select i1 %%div.zero%d, i32 %%div.zero.status%d, i32 6", r.status, in.ID, in.ID)
		r.recordError = true
		if len(args) == 3 {
			r.errorStore, r.errorValue, r.storeWhen = args[2].text, name, fmt.Sprintf("%%div.zero%d", in.ID)
		}
	}
	f.results[in.ID] = r
}

func (f *function) floatConvert(in ir.Instruction, args []operand) operand {
	name, from, to := valueName(in.ID), scalar(args[0].typ), scalar(in.Type)
	value := operand{text: name, typ: in.Type}
	if !ir.Floating(args[0].typ) {
		op := "sitofp"
		if args[0].typ.Scalar == ir.Byte {
			op = "uitofp"
		}
		f.line("%s = %s %s %s to %s", name, op, from, args[0].text, to)
	} else if in.Type.Scalar == ir.Double {
		f.line("%s = fpext float %s to double", name, args[0].text)
	} else if in.Type.Scalar == ir.Single {
		f.line("%s = fptrunc double %s to float", name, args[0].text)
		f.module.intrinsics["llvm.fabs.f64"] = "declare double @llvm.fabs.f64(double)"
		f.line("%%source.abs%d = call double @llvm.fabs.f64(double %s)", in.ID, args[0].text)
		condition := fmt.Sprintf("%%outside.single%d", in.ID)
		f.line("%s = fcmp ugt double %%source.abs%d, %s", condition, in.ID, floatingLiteral(math.MaxFloat32))
		f.results[in.ID] = result{condition: condition, value: name, direct: true, source: in.Source}
	} else {
		input := args[0].text
		if args[0].typ.Scalar == ir.Single {
			input = fmt.Sprintf("%%wide.fp%d", in.ID)
			f.line("%s = fpext float %s to double", input, args[0].text)
		}
		f.module.intrinsics["llvm.roundeven.f64"] = "declare double @llvm.roundeven.f64(double)"
		f.line("%%rounded%d = call double @llvm.roundeven.f64(double %s)", in.ID, input)
		lower, upper := -2147483648.0, 2147483648.0
		if in.Type.Scalar == ir.Integer {
			lower, upper = -32768, 32768
		}
		f.line("%%lower.fp%d = fcmp oge double %%rounded%d, %s", in.ID, in.ID, floatingLiteral(lower))
		f.line("%%upper.fp%d = fcmp olt double %%rounded%d, %s", in.ID, in.ID, floatingLiteral(upper))
		f.line("%%fits.fp%d = and i1 %%lower.fp%d, %%upper.fp%d", in.ID, in.ID, in.ID)
		condition := fmt.Sprintf("%%overflow.fp%d", in.ID)
		f.line("%s = xor i1 %%fits.fp%d, true", condition, in.ID)
		// fptosi on NaN/infinity/out-of-range is poison. It must occur only
		// after the guard, never speculatively before the Result/try edge.
		f.results[in.ID] = result{condition: condition, conversion: fmt.Sprintf("fptosi double %%rounded%d to %s", in.ID, to), source: in.Source}
	}
	return value
}

func (f *function) floatCompare(in ir.Instruction, args []operand) {
	typ := scalar(args[0].typ)
	condition := fmt.Sprintf("%%nan.compare%d", in.ID)
	f.line("%s = fcmp uno %s %s, %s", condition, typ, args[0].text, args[1].text)
	predicate := map[ir.Op]string{ir.OpEqual: "oeq", ir.OpNotEqual: "one", ir.OpLess: "olt", ir.OpLessEqual: "ole", ir.OpGreater: "ogt", ir.OpGreaterEqual: "oge"}[in.Op]
	f.line("%%compare.fp%d = fcmp %s %s %s, %s", in.ID, predicate, typ, args[0].text, args[1].text)
	value := fmt.Sprintf("%%boolean.fp%d", in.ID)
	f.line("%s = sext i1 %%compare.fp%d to i16", value, in.ID)
	f.results[in.ID] = result{condition: condition, value: value, direct: true, source: in.Source}
}

func (f *function) floatCondition(in ir.Instruction, arg operand) {
	condition := f.nonfinite(in.ID, arg.typ, arg.text)
	f.line("%%truth.fp%d = fcmp one %s %s, 0.000000e+00", in.ID, scalar(arg.typ), arg.text)
	value := fmt.Sprintf("%%boolean.fp%d", in.ID)
	f.line("%s = sext i1 %%truth.fp%d to i16", value, in.ID)
	f.results[in.ID] = result{condition: condition, value: value, direct: true, source: in.Source}
}
