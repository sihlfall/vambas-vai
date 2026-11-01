package ir_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/ir"
)

func fixture() *ir.Program {
	source := ir.Source{FileName: "Main.bas", Range: core.NewTextRange(0, 10)}
	long := ir.Type{Scalar: ir.Long}
	ref := ir.Type{Scalar: ir.Long, Form: ir.Reference}
	void := ir.Type{Scalar: ir.Void}
	return &ir.Program{Procedures: []*ir.Procedure{
		{ID: 1, Name: "Main.F", Source: source, ReturnType: long, Parameters: []ir.SlotID{1}, ResultSlot: 2, Effects: ir.MayRaise | ir.WritesStorage,
			Slots: []ir.Slot{
				{ID: 1, Name: "x", Type: long, Kind: ir.Parameter, ByRef: true, Source: source},
				{ID: 2, Name: "F", Type: long, Kind: ir.FunctionResult, Source: source},
			},
			Statements: []*ir.Statement{{ID: 1, Retry: 1, Source: source, End: ir.Return, Instructions: []ir.Instruction{
				{ID: 1, Op: ir.OpLoad, Type: long, Slot: 1, Source: source},
				{ID: 2, Op: ir.OpConstant, Type: long, Integer: 42, Source: source},
				{ID: 3, Op: ir.OpAdd, Type: ir.Type{Scalar: ir.Long, Form: ir.Result}, Args: []ir.ValueID{1, 2}, Effects: ir.MayRaise, Source: source},
				{ID: 4, Op: ir.OpTry, Type: long, Args: []ir.ValueID{3}, OnError: ir.Propagate, Source: source},
				{Op: ir.OpStore, Type: void, Slot: 2, Args: []ir.ValueID{4}, Effects: ir.WritesStorage, Source: source},
				{ID: 5, Op: ir.OpAddress, Type: ref, Slot: 1, Source: source},
				{ID: 6, Op: ir.OpCall, Type: ir.Type{Scalar: ir.Void, Form: ir.Result}, Callee: 2, Args: []ir.ValueID{5}, Effects: ir.MayRaise, Source: source},
				{ID: 7, Op: ir.OpTry, Type: void, Args: []ir.ValueID{6}, OnError: ir.Propagate, Source: source},
			}}}},
		{ID: 2, Name: "Main.G", Source: source, ReturnType: void, Parameters: []ir.SlotID{1},
			Slots:      []ir.Slot{{ID: 1, Name: "x", Type: long, Kind: ir.Parameter, ByRef: true, Source: source}},
			Statements: []*ir.Statement{{ID: 1, Retry: 1, Source: source, End: ir.Return}},
		},
	}}
}

func TestVerifierRejectsBrokenSemantics(t *testing.T) {
	if d := ir.Verify(fixture()); len(d) != 0 {
		t.Fatal("invalid fixture", d)
	}
	tests := []struct {
		name   string
		mutate func(*ir.Program)
	}{
		{"missing try", func(p *ir.Program) {
			s := p.Procedures[0].Statements[0]
			s.Instructions = append(s.Instructions[:3], s.Instructions[4:]...)
		}},
		{"missing error continuation", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[3].OnError = ir.NoErrorContinuation }},
		{"unchecked overflow", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[2].Effects = 0 }},
		{"unresolved type", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Type.Scalar = ir.Invalid }},
		{"result used before success", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[4].Args[0] = 3 }},
		{"result reused", func(p *ir.Program) {
			s := p.Procedures[0].Statements[0]
			s.Instructions = append(s.Instructions, s.Instructions[3])
			s.Instructions[len(s.Instructions)-1].ID = 99
		}},
		{"forward reference", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[2].Args[0] = 4 }},
		{"duplicate value", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[1].ID = 1 }},
		{"unknown slot", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Slot = 99 }},
		{"load storage mismatch", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Type.Scalar = ir.Integer }},
		{"alias storage mismatch", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[5].Type.Scalar = ir.Integer }},
		{"ByRef gets a value", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[6].Args[0] = 1 }},
		{"missing callee", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[6].Callee = 0 }},
		{"effect lost at procedure", func(p *ir.Program) { p.Procedures[0].Effects = 0 }},
		{"wrong result slot", func(p *ir.Program) { p.Procedures[0].ResultSlot = 1 }},
		{"nonlocal ownership", func(p *ir.Program) { p.Procedures[0].Slots[1].ByRef = true }},
		{"missing parameter", func(p *ir.Program) { p.Procedures[0].Parameters = nil }},
		{"duplicate parameter", func(p *ir.Program) { p.Procedures[0].Parameters = []ir.SlotID{1, 1} }},
		{"bad retry", func(p *ir.Program) { p.Procedures[0].Statements[0].Retry = 2 }},
		{"bad next", func(p *ir.Program) { p.Procedures[0].Statements[0].Next = 1 }},
		{"fallthrough", func(p *ir.Program) { p.Procedures[0].Statements[0].End = ir.Continue }},
		{"empty body", func(p *ir.Program) { p.Procedures[0].Statements = nil }},
		{"cross region value", func(p *ir.Program) {
			f := p.Procedures[0]
			s := f.Statements[0]
			s.End, s.Next = ir.Continue, 2
			f.Statements = append(f.Statements, &ir.Statement{ID: 2, Retry: 2, End: ir.Return, Source: s.Source, Instructions: []ir.Instruction{s.Instructions[4]}})
		}},
		{"outside region", func(p *ir.Program) {
			p.Procedures[0].Statements[0].Instructions[0].Source.Range = core.NewTextRange(0, 11)
		}},
		{"missing source", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Source.FileName = "" }},
		{"invalid opcode", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Op = ir.OpInvalid }},
		{"out of range constant", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[1].Integer = 2147483648 }},
		{"unexpected payload", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[1].Callee = 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := fixture()
			test.mutate(p)
			if len(ir.Verify(p)) == 0 {
				t.Fatal("invalid semantic program passed verification")
			}
		})
	}
}

func TestVerifierHandlesMalformedContainers(t *testing.T) {
	for _, p := range []*ir.Program{nil, {Procedures: []*ir.Procedure{nil}}, {Procedures: []*ir.Procedure{{Statements: []*ir.Statement{nil}}}}} {
		if len(ir.Verify(p)) == 0 {
			t.Fatal("malformed container accepted")
		}
	}
	for op := ir.OpInvalid; op <= ir.OpTry; op++ {
		p := fixture()
		p.Procedures[0].Statements[0].Instructions = []ir.Instruction{{Op: op}}
		if len(ir.Verify(p)) == 0 {
			t.Fatalf("malformed %s accepted", op)
		}
	}
}

func FuzzVerify(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{255, 0, 255, 0, 255, 0, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := fixture()
		instructions := p.Procedures[0].Statements[0].Instructions
		for i, b := range data {
			in := &instructions[i%len(instructions)]
			switch i % 7 {
			case 0:
				in.Op = ir.Op(b)
			case 1:
				in.Type = ir.Type{Scalar: ir.Scalar(b), Form: ir.Form(b >> 4)}
			case 2:
				in.Args = []ir.ValueID{ir.ValueID(b)}
			case 3:
				in.Effects = ir.Effects(b)
			case 4:
				in.Callee = ir.ProcedureID(b)
			case 5:
				in.Slot = ir.SlotID(b)
			case 6:
				in.Args = nil
			}
		}
		ir.Verify(p) // Arbitrary instruction payloads must never panic.
	})
}
