package ir_test

import (
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/ir"
	"testing"
)

func recordFixture() *ir.Program {
	source := ir.Source{FileName: "Main.bas", Range: core.NewTextRange(0, 10)}
	return &ir.Program{Records: []ir.RecordDefinition{{ID: 1, Name: "Main.Item", Source: source, Fields: []ir.RecordField{{Name: "N", Type: ir.Type{Scalar: ir.Long}, Source: source}}}}, Procedures: []*ir.Procedure{{ID: 1, Name: "Main.Run", Source: source, ReturnType: ir.Type{Scalar: ir.Void}, Parameters: []ir.SlotID{1}, Slots: []ir.Slot{{ID: 1, Name: "item", Kind: ir.Parameter, ByRef: true, Type: ir.Type{Scalar: ir.Record, Record: 1}, Source: source}}, Statements: []*ir.Statement{{ID: 1, Retry: 1, Source: source, End: ir.Return, Instructions: []ir.Instruction{
		{ID: 1, Op: ir.OpAddress, Type: ir.Type{Scalar: ir.Record, Record: 1, Form: ir.Reference}, Slot: 1, Source: source},
		{ID: 2, Op: ir.OpFieldRef, Type: ir.Type{Scalar: ir.Long, Form: ir.Reference}, Args: []ir.ValueID{1}, Source: source},
	}}}}}}
}

func TestVerifierUDTNominalShapes(t *testing.T) {
	if ds := ir.Verify(recordFixture()); len(ds) != 0 {
		t.Fatal(ds)
	}
	for _, mutate := range []func(*ir.Program){
		func(p *ir.Program) { p.Records[0].Fields[0].Type = ir.Type{Scalar: ir.Record, Record: 1} },
		func(p *ir.Program) { p.Records[0].Fields[0].Type = ir.Type{Scalar: ir.Record, Record: 2} },
		func(p *ir.Program) { p.Records[0].Fields = nil },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[1].Field = 1 },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[1].Type.Scalar = ir.Integer },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[1].Args = nil },
		func(p *ir.Program) { p.Procedures[0].Slots[0].Type.Record = 2 },
		func(p *ir.Program) { p.Procedures[0].Slots[0].ByRef = false },
		func(p *ir.Program) { p.Records[0].Fields[0].Type.Record = 1 },
	} {
		p := recordFixture()
		mutate(p)
		if len(ir.Verify(p)) == 0 {
			t.Fatal("invalid UDT IR accepted")
		}
	}
}

func FuzzVerifyUDT(f *testing.F) {
	f.Add([]byte{0, 1, 2, 255, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := recordFixture()
		for i, v := range data {
			switch i % 5 {
			case 0:
				p.Records[0].Fields[0].Type = ir.Type{Scalar: ir.Scalar(v), Record: ir.RecordID(v)}
			case 1:
				p.Procedures[0].Statements[0].Instructions[1].Field = uint32(v)
			case 2:
				p.Procedures[0].Statements[0].Instructions[1].Args = []ir.ValueID{ir.ValueID(v)}
			case 3:
				p.Records[0].ID = ir.RecordID(v)
			case 4:
				p.Procedures[0].Slots[0].Type.Record = ir.RecordID(v)
			}
		}
		ir.Verify(p)
	})
}
