package lowering_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func TestGlobalStorageVerification(t *testing.T) {
	p := lower(t, module(`Private counter As Long
Function Run() As Long
 counter=counter+1
 Run=counter
End Function`))
	if len(p.Globals) != 1 {
		t.Fatal("missing project storage")
	}
	var alias *ir.Slot
	for i := range p.Procedures[0].Slots {
		s := &p.Procedures[0].Slots[i]
		if s.Kind == ir.GlobalAlias {
			alias = s
			break
		}
	}
	if alias == nil || alias.Global != 1 || alias.Type != p.Globals[0].Type {
		t.Fatal("global identity lost")
	}
	old := *alias
	for _, mutate := range []func(){func() { alias.Global = 99 }, func() { alias.Type.Scalar = ir.Integer }, func() { alias.Kind = ir.Local }, func() { alias.ByRef = true }} {
		mutate()
		if len(ir.Verify(p)) == 0 {
			t.Fatal("malformed global alias accepted")
		}
		*alias = old
	}
	p.Globals[0].Bounds = []ir.ArrayBound{{Lower: 1, Upper: 2}}
	if len(ir.Verify(p)) == 0 {
		t.Fatal("bounds accepted on scalar global")
	}
}

func TestOptionalDefaultValidationIncludesUnusedProcedures(t *testing.T) {
	for _, text := range []string{
		"Sub Run(Optional ByVal n As Integer=32768)\nEnd Sub",
		"Sub Run(Optional ByVal n As Single=1e100)\nEnd Sub",
		"Dim g As Long\nSub Run(Optional ByVal n As Long=g)\nEnd Sub",
		"Sub Run(Optional ByVal n As Long=Bad())\nEnd Sub\nFunction Bad() As Long\nEnd Function",
		"Sub Run(Optional ByVal n As Long=\"bad\")\nEnd Sub",
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(text)}})
		if result, ds := lowering.Lower(p); result != nil || len(ds) == 0 {
			t.Fatal("invalid optional default accepted", text)
		}
	}
}
