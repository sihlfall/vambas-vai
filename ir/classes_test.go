package ir_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func classFixture() *ir.Program {
	source := ir.Source{FileName: "Classes.bas", Range: core.NewTextRange(0, 10)}
	object, void := ir.Type{Scalar: ir.Object}, ir.Type{Scalar: ir.Void}
	return &ir.Program{
		Classes: []ir.ClassDefinition{{ID: 1, Name: "C", Source: source, Fields: []ir.RecordField{{Name: "Count", Type: ir.Type{Scalar: ir.Long}, Source: source}}, Members: []ir.ProcedureID{2}}},
		Procedures: []*ir.Procedure{
			{ID: 1, Name: "Main.Run", Source: source, ReturnType: void, Effects: ir.MayRaise | ir.ManagesLifetime, Statements: []*ir.Statement{{ID: 1, Retry: 1, Source: source, End: ir.Return, Cleanup: []ir.ValueID{2}, Instructions: []ir.Instruction{
				{ID: 1, Op: ir.OpClassNew, Class: 1, Type: ir.Type{Scalar: ir.Object, Form: ir.Result}, Effects: ir.MayRaise | ir.ManagesLifetime, Source: source},
				{ID: 2, Op: ir.OpTry, Type: object, Args: []ir.ValueID{1}, OnError: ir.Propagate, Effects: ir.ManagesLifetime, Ownership: ir.Owned, Source: source},
				{ID: 3, Op: ir.OpClassCheck, Class: 1, Type: ir.Type{Scalar: ir.Void, Form: ir.Result}, Args: []ir.ValueID{2}, Effects: ir.MayRaise, Source: source},
				{ID: 4, Op: ir.OpTry, Type: void, Args: []ir.ValueID{3}, OnError: ir.Propagate, Source: source},
				{ID: 5, Op: ir.OpClassFieldRef, Class: 1, Type: ir.Type{Scalar: ir.Long, Form: ir.Reference}, Args: []ir.ValueID{2}, Source: source},
				{ID: 6, Op: ir.OpCall, Callee: 2, Type: ir.Type{Scalar: ir.Void, Form: ir.Result}, Args: []ir.ValueID{2}, Effects: ir.MayRaise | ir.ManagesLifetime, Source: source},
				{ID: 7, Op: ir.OpTry, Type: void, Args: []ir.ValueID{6}, OnError: ir.Propagate, Source: source},
			}}}},
			{ID: 2, Name: "C.Run", Class: 1, Receiver: 1, Source: source, ReturnType: void, Effects: ir.ManagesLifetime, Parameters: []ir.SlotID{1}, Slots: []ir.Slot{{ID: 1, Name: "$me", Type: object, Kind: ir.Parameter, Source: source}}, Statements: []*ir.Statement{{ID: 1, Retry: 1, Source: source, End: ir.Return}}},
		},
	}
}

func TestVerifierClassIdentityAndBorrowing(t *testing.T) {
	if ds := ir.Verify(classFixture()); len(ds) != 0 {
		t.Fatal(ds)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ir.Program)
	}{
		{"unknown class", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[0].Class = 2 }},
		{"nullable receiver", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[2].Op = ir.OpClassCast }},
		{"unchecked field", func(p *ir.Program) {
			s := p.Procedures[0].Statements[0]
			s.Instructions = append(s.Instructions[:2], s.Instructions[4:]...)
		}},
		{"missing field", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[4].Field = 1 }},
		{"wrong field type", func(p *ir.Program) { p.Procedures[0].Statements[0].Instructions[4].Type.Scalar = ir.String }},
		{"missing owner", func(p *ir.Program) { p.Procedures[0].Statements[0].Cleanup = nil }},
		{"wrong receiver alias", func(p *ir.Program) { p.Procedures[1].Slots[0].ByRef = true }},
		{"missing receiver", func(p *ir.Program) { p.Procedures[1].Receiver = 0 }},
		{"missing member", func(p *ir.Program) { p.Classes[0].Members = nil }},
		{"duplicate member", func(p *ir.Program) { p.Classes[0].Members = []ir.ProcedureID{2, 2} }},
		{"unsupported field", func(p *ir.Program) { p.Classes[0].Fields[0].Type.Scalar = ir.GosubAddress }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := classFixture()
			tc.mutate(p)
			if len(ir.Verify(p)) == 0 {
				t.Fatal("invalid class contract accepted")
			}
		})
	}
}

func TestVerifierInvocationMetadataAndCaptures(t *testing.T) {
	fixture := func() *ir.Program {
		p, ds := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
			{FileName: "Main.bas", Text: "Sub Run()\nDim c As New C, o As Object\nSet o=c\no.Value=1\nEnd Sub"},
			{FileName: "C.cls", Text: "Public Value As Long\nPublic Function F(Optional ByVal n As Long=1) As Long\nF=n\nEnd Function"},
		}}))
		if len(ds) != 0 {
			t.Fatal(ds)
		}
		return p
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ir.Program)
	}{
		{"missing member name", func(p *ir.Program) { p.Procedures[1].Invocation.Name = "" }},
		{"invalid invocation kind", func(p *ir.Program) { p.Procedures[1].Invocation.Kind = 0 }},
		{"parameter shape", func(p *ir.Program) { p.Procedures[1].Invocation.Parameters[0].Type = ir.Type{Scalar: ir.Variant} }},
		{"missing default", func(p *ir.Program) { p.Procedures[1].Invocation.Parameters[0].Default = 999 }},
		{"missing parameter", func(p *ir.Program) { p.Procedures[1].Invocation.Parameters = nil }},
		{"invalid field constraint", func(p *ir.Program) { p.Classes[0].Fields[0].AutoClass = 2 }},
		{"invalid lazy class", func(p *ir.Program) { p.Procedures[0].Slots[0].AutoClass = 999 }},
		{"uncaptured receiver", func(p *ir.Program) {
			for _, s := range p.Procedures[0].Statements {
				var value ir.ValueID
				for i := range s.Instructions {
					in := &s.Instructions[i]
					if in.Op == ir.OpInvocationCapture {
						value = in.Args[0]
					}
					if in.Op == ir.OpObjectInvoke {
						in.Args[0] = value
					}
				}
			}
		}},
		{"argument names", func(p *ir.Program) {
			for _, s := range p.Procedures[0].Statements {
				for i := range s.Instructions {
					if s.Instructions[i].Op == ir.OpObjectInvoke {
						s.Instructions[i].Names = nil
					}
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture()
			tc.mutate(p)
			if len(ir.Verify(p)) == 0 {
				t.Fatal("invalid invocation contract accepted")
			}
		})
	}
}
