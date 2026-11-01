package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestCatalogErrAliasesAndMissingArguments(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run()
 Dim n As Long
 n=Err
 n=VBA.Err
 Err.Number=6
 n=VBA.Err.Number
 Err.Raise Description:="custom", Number:=6
 VBA.Err.Raise Description:="custom", Number:=6
 Err.Clear
 VBA.Err.Clear
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	for _, text := range []string{"Err", "VBA.Err", "VBA.Err.Number"} {
		kind := ast.KindPropertyAccessExpression
		if text == "Err" {
			kind = ast.KindIdentifier
		}
		requireType(t, c, find(t, file, kind, text), checker.TypeLong)
	}
	var signature *checker.Signature
	for _, text := range []string{`Err.Raise Description:="custom", Number:=6`, `VBA.Err.Raise Description:="custom", Number:=6`} {
		r := c.GetCallResolution(find(t, file, ast.KindCallExpression, text))
		if r == nil || !r.Valid || r.Signature.Declaration.Symbol.Library != stdlib.ErrRaise || r.Signature.Declaration.Symbol.Intrinsic != ast.IntrinsicNone {
			t.Fatal("Err.Raise lost catalog identity or signature", r)
		}
		if signature != nil && signature != r.Signature {
			t.Fatal("qualified Err aliases created different signatures")
		}
		signature = r.Signature
		if len(r.Arguments) != 5 || r.Arguments[0].ParameterIndex != 2 || r.Arguments[1].ParameterIndex != 0 {
			t.Fatal("Err.Raise named argument order changed", r.Arguments)
		}
		for _, a := range r.Arguments[2:] {
			if a.Mode != checker.ArgumentMissing || a.Expression != nil || a.Parameter.Type.Kind != checker.TypeVariant {
				t.Fatal("omitted Err.Raise argument no longer retains Missing", a)
			}
		}
	}
}

func TestCatalogObjectsRespectUserShadowing(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Err.bas", Text: "Public Number As String\nPublic Sub Clear()\nEnd Sub\n"},
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Function CreateObject(ByVal Class As String) As Object
End Function
Sub Run()
 Dim obj As Object, text As String, n As Long
 text=Err.Number
 n=VBA.Err.Number
 Err.Clear
 VBA.Err.Clear
 Set obj=CreateObject("local")
 Set obj=VBA.Interaction.CreateObject(Class:="Rubberduck.AssertClass")
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	requireType(t, c, find(t, file, ast.KindPropertyAccessExpression, "Err.Number"), checker.TypeString)
	requireType(t, c, find(t, file, ast.KindPropertyAccessExpression, "VBA.Err.Number"), checker.TypeLong)
	for _, test := range []struct {
		text string
		id   stdlib.MemberID
	}{
		{"Err.Clear", stdlib.None},
		{"VBA.Err.Clear", stdlib.ErrClear},
		{`CreateObject("local")`, stdlib.None},
		{`VBA.Interaction.CreateObject(Class:="Rubberduck.AssertClass")`, stdlib.CreateObject},
	} {
		r := c.GetCallResolution(find(t, file, ast.KindCallExpression, test.text))
		if r == nil || !r.Valid || r.Signature.Declaration.Symbol.Library != test.id || r.Signature.Declaration.Symbol.Intrinsic != ast.IntrinsicNone {
			t.Fatalf("%s lost its declaration identity", test.text)
		}
	}
}
