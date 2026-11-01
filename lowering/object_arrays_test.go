package lowering_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
	"strings"
	"testing"
)

func TestObjectArrayIRPreservesNominalConstraints(t *testing.T) {
	p := lower(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Private Type Node
 children(0 To 1) As C
End Type
Private roots(1 To 2) As C
Function Run() As Long
 Dim a() As C,v As Variant
 ReDim a(0 To 2)
 v=a
 a=v
End Function`}, compiler.SourceFileInput{FileName: "C.cls", Text: "Public Values() As Object\n"})
	want := ir.Type{Scalar: ir.ArrayObject, Class: 1}
	if p.Records[0].Fields[0].Type != want || p.Globals[0].Type != want || p.Classes[0].Fields[0].Type != (ir.Type{Scalar: ir.ArrayObject}) {
		t.Fatal(ir.Format(p))
	}
	for _, s := range p.Procedures[0].Statements {
		for i := range s.Instructions {
			in := &s.Instructions[i]
			if in.Op == ir.OpArrayResize {
				old := in.Class
				in.Class = 0
				if len(ir.Verify(p)) == 0 {
					t.Fatal("lost class constraint accepted")
				}
				in.Class = old
				return
			}
		}
	}
	t.Fatal("no object array resize")
}

func TestAutoInstantiatedArrayElementsAreExplicitlyDeferred(t *testing.T) {
	p, ds := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Sub Run()\n Dim a(1) As New C\nEnd Sub"}, {FileName: "C.cls", Text: ""}}}))
	if p != nil {
		t.Fatal("As New array silently became ordinary Nothing elements")
	}
	for _, d := range ds {
		if strings.Contains(d.Message, "As New array elements") {
			return
		}
	}
	t.Fatal(ds)
}
