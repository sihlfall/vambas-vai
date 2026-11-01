package lowering_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func TestUDTDefinitionsAndFieldReferences(t *testing.T) {
	p := lower(t, module(`Enum Mode
 Ready=42
End Enum
Type Inner
 Value As Mode
End Type
Type Outer
 Child As Inner
 Values(1 To 2) As Long
End Type
Function Run() As Long
 Dim a As Outer,b As Outer
 a.Child.Value=Mode.Ready
 b=a
 Run=b.Child.Value
End Function`))
	if len(p.Records) != 2 || p.Records[1].Fields[0].Type != (ir.Type{Scalar: ir.Record, Record: 1}) || len(p.Records[1].Fields[1].Bounds) != 1 {
		t.Fatal(ir.Format(p))
	}
	fields, reads, copies := 0, 0, 0
	for _, s := range p.Procedures[0].Statements {
		for _, in := range s.Instructions {
			if in.Op == ir.OpFieldRef {
				fields++
			}
			if in.Op == ir.OpReadReference {
				reads++
			}
			if in.Op == ir.OpStoreManaged && in.Slot != 0 && p.Procedures[0].Slots[in.Slot-1].Type.Scalar == ir.Record {
				copies++
			}
		}
	}
	if fields < 4 || reads == 0 || copies == 0 {
		t.Fatal(ir.Format(p))
	}
}

func TestUnsupportedUDTFormsAreDiagnosed(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{"Type Node\n Children() As Node\nEnd Type\nSub Run()\nEnd Sub", "recursive"},
		{"Type State\n Name As String * 8\nEnd Type\nSub Run()\nEnd Sub", "fixed-length String UDT"},
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(test.source)}})
		semantic, diagnostics := lowering.Lower(p)
		if semantic != nil || len(diagnostics) == 0 {
			t.Fatal("unsupported UDT silently compiled", test.source)
		}
		found := false
		for _, d := range diagnostics {
			found = found || strings.Contains(d.Message, test.want)
		}
		if !found {
			t.Fatal(test.want, diagnostics)
		}
	}
}
