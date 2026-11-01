package ir_test

import (
	"sihlfall.ch/vambas-vai/ir"
	"testing"
)

func controlFixture() *ir.Program {
	p := fixture()
	f := p.Procedures[0]
	first := f.Statements[0]
	first.End = ir.Branch
	first.Condition = 8
	first.Target = 1
	first.FalseTarget = 2
	first.Next = 2
	first.Instructions = append(first.Instructions, ir.Instruction{ID: 8, Op: ir.OpConstant, Type: ir.Type{Scalar: ir.Boolean}, Integer: -1, Source: first.Source})
	f.Statements = append(f.Statements, &ir.Statement{ID: 2, Retry: 2, End: ir.Return, Source: first.Source})
	return p
}

func TestVerifierControlEdges(t *testing.T) {
	if d := ir.Verify(controlFixture()); len(d) != 0 {
		t.Fatal(d)
	}
	for _, mutate := range []func(*ir.Program){
		func(p *ir.Program) { p.Procedures[0].Statements[0].Target = 0 },
		func(p *ir.Program) { p.Procedures[0].Statements[0].FalseTarget = 99 },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Condition = 1 },
		func(p *ir.Program) { p.Procedures[0].Statements[0].Scope = 1 },
		func(p *ir.Program) { p.Procedures[0].Scopes = []ir.Scope{{ID: 1, Parent: 1}} },
		func(p *ir.Program) { p.Procedures[0].Scopes = []ir.Scope{{ID: 1, Cleanup: []ir.SlotID{2}}} },
		func(p *ir.Program) { p.Procedures[0].Statements[0].End = ir.Jump },
		func(p *ir.Program) {
			f := p.Procedures[0]
			f.Statements[1].End = ir.Branch
			f.Statements[1].Condition = 8
			f.Statements[1].Target = 1
			f.Statements[1].FalseTarget = 2
		},
	} {
		p := controlFixture()
		mutate(p)
		if len(ir.Verify(p)) == 0 {
			t.Fatal("invalid control/resource edge accepted")
		}
	}
}

func FuzzVerifyControlGraph(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{255, 0, 255, 0, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := controlFixture()
		s := p.Procedures[0].Statements[0]
		for i, b := range data {
			switch i % 7 {
			case 0:
				s.End = ir.End(b)
			case 1:
				s.Target = ir.StatementID(b)
			case 2:
				s.FalseTarget = ir.StatementID(b)
			case 3:
				s.Next = ir.StatementID(b)
			case 4:
				s.Condition = ir.ValueID(b)
			case 5:
				s.Scope = ir.ScopeID(b)
			case 6:
				p.Procedures[0].Scopes = []ir.Scope{{ID: ir.ScopeID(b), Parent: ir.ScopeID(b), Cleanup: []ir.SlotID{ir.SlotID(b)}}}
			}
		}
		ir.Verify(p)
	})
}

func switchFixture() *ir.Program {
	p := controlFixture()
	s := p.Procedures[0].Statements[0]
	s.End, s.Condition, s.FalseTarget = ir.Switch, 0, 0
	s.Selector = 9
	s.Instructions = append(s.Instructions, ir.Instruction{ID: 9, Op: ir.OpConstant, Type: ir.Type{Scalar: ir.Integer}, Integer: 1, Source: s.Source})
	s.Cases = []ir.SwitchCase{{Value: 1, Target: 2}, {Value: 2, Target: 1}}
	return p
}

func TestVerifierSwitch(t *testing.T) {
	if d := ir.Verify(switchFixture()); len(d) != 0 {
		t.Fatal(d)
	}
	for _, mutate := range []func(*ir.Statement){
		func(s *ir.Statement) { s.Target = 0 },
		func(s *ir.Statement) { s.Cases[0].Target = 99 },
		func(s *ir.Statement) { s.Cases[1].Value = 1 },
		func(s *ir.Statement) { s.Cases[0].Value = 32768 },
		func(s *ir.Statement) { s.Selector = 8 },
		func(s *ir.Statement) { s.Selector = 99 },
		func(s *ir.Statement) { s.Condition = 8 },
		func(s *ir.Statement) { s.End = ir.Jump },
	} {
		p := switchFixture()
		mutate(p.Procedures[0].Statements[0])
		if len(ir.Verify(p)) == 0 {
			t.Fatal("invalid switch accepted")
		}
	}
}

func FuzzVerifySwitch(f *testing.F) {
	f.Add([]byte{0, 1, 2, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := switchFixture()
		s := p.Procedures[0].Statements[0]
		for i, v := range data {
			switch i % 5 {
			case 0:
				s.Cases[i%2].Value = int64(int8(v)) * 1000
			case 1:
				s.Cases[i%2].Target = ir.StatementID(v)
			case 2:
				s.Selector = ir.ValueID(v)
			case 3:
				s.Target = ir.StatementID(v)
			case 4:
				s.End = ir.End(v)
			}
		}
		ir.Verify(p)
	})
}
