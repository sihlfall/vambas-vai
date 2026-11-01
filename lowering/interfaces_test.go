package lowering_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
)

func TestInterfaceIRValidatesDispatchContracts(t *testing.T) {
	makeProgram := func() *ir.Program {
		return lower(t,
			compiler.SourceFileInput{FileName: "Main.bas", Text: "Function Run() As Long\n Dim c As C, value As I\n Set c=New C\n Set value=c\n Run=value.GetValue(2)\nEnd Function"},
			compiler.SourceFileInput{FileName: "I.cls", Text: "Function GetValue(ByVal n As Long) As Long\nEnd Function"},
			compiler.SourceFileInput{FileName: "C.cls", Text: "Implements I\nPrivate state As Long\nPrivate Function I_GetValue(ByVal n As Long) As Long\n state=40\n I_GetValue=n+state\nEnd Function"})
	}
	p := makeProgram()
	var dispatch *ir.Instruction
	for _, s := range p.Procedures[0].Statements {
		for i := range s.Instructions {
			if s.Instructions[i].Op == ir.OpInterfaceCall {
				dispatch = &s.Instructions[i]
			}
		}
	}
	if dispatch == nil || len(p.Classes[1].Interfaces) != 1 {
		t.Fatal(ir.Format(p))
	}
	if dispatch.Effects&ir.WritesStorage == 0 {
		t.Fatal("implementation effects omitted")
	}
	for _, mutate := range []func(*ir.Program){
		func(p *ir.Program) { p.Classes[1].Interfaces[0].Methods = nil },
		func(p *ir.Program) { p.Classes[1].Interfaces[0].Class = 2 },
		func(p *ir.Program) { p.Classes[1].Interfaces[0].Methods[0].Implementation = 2 },
		func(p *ir.Program) {
			p.Classes[1].Interfaces = append(p.Classes[1].Interfaces, p.Classes[1].Interfaces[0])
		},
		func(p *ir.Program) { p.Procedures[2].Slots[1].ByRef = true },
		func(p *ir.Program) {
			for _, s := range p.Procedures[2].Statements {
				for i := range s.Instructions {
					if s.Instructions[i].Op == ir.OpClassCheck {
						s.Instructions[i].Op = ir.OpInterfaceCheck
					}
				}
			}
		},
		func(p *ir.Program) {
			for _, s := range p.Procedures[0].Statements {
				for i := range s.Instructions {
					if s.Instructions[i].Op == ir.OpInterfaceCall {
						s.Instructions[i].Effects = ir.MayRaise
					}
				}
			}
		},
	} {
		p := makeProgram()
		mutate(p)
		if len(ir.Verify(p)) == 0 {
			t.Fatal("malformed interface IR accepted", ir.Format(p))
		}
	}
}
