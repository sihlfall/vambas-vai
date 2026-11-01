package lowering_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
	"strings"
	"testing"
)

func TestParamArrayPackingKeepsCheckerBindings(t *testing.T) {
	p := lower(t, module(`Sub Run(n As Long)
 Call Pack(1, n, (n), , "last")
 Call Pack(1)
End Sub
Sub Pack(ByVal first As Long, ParamArray rest() As Variant)
End Sub
`))
	formal := p.Procedures[1].Slots[1]
	if !formal.ParamArray || !formal.ByRef || formal.Type != (ir.Type{Scalar: ir.ArrayVariant}) {
		t.Fatal(formal)
	}
	if formal.Source.Node.AsParameterDeclaration().IsByRef() {
		t.Fatal("lowering changed source parameter passing mode")
	}
	packs := 0
	for _, s := range p.Procedures[0].Statements {
		types := map[ir.ValueID]ir.Type{}
		for _, in := range s.Instructions {
			types[in.ID] = in.Type
			if in.Op != ir.OpParamArrayPack {
				continue
			}
			packs++
			if packs == 1 {
				if len(in.Args) != 5 || types[in.Args[0]] != (ir.Type{Scalar: ir.ArrayVariant, Form: ir.Reference}) || types[in.Args[1]] != (ir.Type{Scalar: ir.Variant, Form: ir.Reference}) {
					t.Fatal(in, types)
				}
				for _, arg := range in.Args[2:] {
					if types[arg] != (ir.Type{Scalar: ir.Variant}) {
						t.Fatal("temporary/omitted argument became alias", in, types)
					}
				}
			} else if len(in.Args) != 1 {
				t.Fatal("empty pack contains values", in)
			}
		}
	}
	if packs != 2 || !strings.Contains(ir.Format(p), "paramarray_pack") {
		t.Fatal(ir.Format(p))
	}
	p.Procedures[1].Slots[1].ByRef = false
	if len(ir.Verify(p)) == 0 {
		t.Fatal("invalid ParamArray ownership accepted")
	}
}
func TestParamArrayAcceptsPrivateObjectViews(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(`Sub Run(ByRef value As Object)
 Pack value
End Sub
Sub Pack(ParamArray p() As Variant)
End Sub
`)}})
	result, ds := lowering.Lower(p)
	if result == nil || len(ds) != 0 {
		t.Fatal("native Object binding was rejected", ds)
	}
}
