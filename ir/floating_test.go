package ir_test

import (
	"math"
	"testing"

	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/ir"
)

func floatingFixture() *ir.Program {
	source := ir.Source{FileName: "Main.bas", Range: core.NewTextRange(0, 10)}
	double := ir.Type{Scalar: ir.Double}
	return &ir.Program{Procedures: []*ir.Procedure{{ID: 1, Name: "Main.F", Source: source, ReturnType: double, ResultSlot: 1, Effects: ir.MayRaise | ir.WritesStorage,
		Slots: []ir.Slot{{ID: 1, Name: "F", Type: double, Kind: ir.FunctionResult, Source: source}},
		Statements: []*ir.Statement{{ID: 1, Retry: 1, End: ir.Return, Source: source, Instructions: []ir.Instruction{
			{ID: 1, Op: ir.OpConstant, Type: double, Float: 1.5, Source: source},
			{ID: 2, Op: ir.OpAddress, Type: ir.Type{Scalar: ir.Double, Form: ir.Reference}, Slot: 1, Source: source},
			{ID: 3, Op: ir.OpDivide, Type: ir.Type{Scalar: ir.Double, Form: ir.Result}, Args: []ir.ValueID{1, 1, 2}, Effects: ir.MayRaise | ir.WritesStorage, Source: source},
			{ID: 4, Op: ir.OpTry, Type: double, Args: []ir.ValueID{3}, OnError: ir.Propagate, Source: source},
		}}},
	}}}
}

func TestVerifierFloating(t *testing.T) {
	if d := ir.Verify(floatingFixture()); len(d) != 0 {
		t.Fatal(d)
	}
	for _, mutate := range []func(*ir.Program){
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Float = math.NaN() },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Float = math.Inf(1) },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Integer = 1 },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[2].Effects = ir.MayRaise },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[2].Args[2] = 1 },
		func(p *ir.Program) {
			s := p.Procedures[0].Statements[0]
			s.End = ir.Switch
			s.Selector = 1
			s.Target = 1
		},
	} {
		p := floatingFixture()
		mutate(p)
		if len(ir.Verify(p)) == 0 {
			t.Fatal("invalid floating IR accepted")
		}
	}
}

func TestVerifierSingleLiteralPrecision(t *testing.T) {
	p := floatingFixture()
	f := p.Procedures[0]
	f.ReturnType.Scalar, f.Slots[0].Type.Scalar = ir.Single, ir.Single
	f.Effects = ir.MayRaise
	for i := range f.Statements[0].Instructions {
		f.Statements[0].Instructions[i].Type.Scalar = ir.Single
	}
	f.Statements[0].Instructions[2].Args = []ir.ValueID{1, 1}
	f.Statements[0].Instructions[2].Effects = ir.MayRaise
	in := &f.Statements[0].Instructions[0]
	in.Float = float64(float32(.1))
	if d := ir.Verify(p); len(d) != 0 {
		t.Fatal(d)
	}
	in.Float = .1
	if len(ir.Verify(p)) == 0 {
		t.Fatal("unrounded binary32 literal accepted")
	}
}
