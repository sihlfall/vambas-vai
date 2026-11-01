package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestMidPlansAndLibraryCalls(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef value As String)
    Mid$(value, 2, 3) = Mid$(value, Start:=1, Length:=3)
    Mid(value, 1) = "x"
End Sub
Function Read(ByVal value As String) As String
    Read = VBA.Mid(value,2)
End Function
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	for i, n := range body {
		r := c.GetMidAssignmentResolution(n)
		if r == nil || !r.Valid || r.Deferred || r.Storage.Type.Kind != checker.TypeString || !r.Storage.Addressable || r.Start.To.Kind != checker.TypeLong || r.Replacement.To.Kind != checker.TypeString {
			t.Fatal("missing Mid storage/coercion plan", r)
		}
		if (r.Length == nil) != (i == 1) {
			t.Fatal("omitted length was not preserved")
		}
	}
	rhs := body[0].AsMidAssignmentStatement().Expression
	if c.GetTypeAtLocation(rhs).Kind != checker.TypeString || c.GetCallResolution(rhs).Signature.Declaration.Symbol.Library != stdlib.MidString || c.GetCallResolution(rhs).Signature.ReturnType.Kind != checker.TypeString {
		t.Fatal("Mid$ library call did not resolve")
	}
}

func TestMidRequiresBoundStringStorage(t *testing.T) {
	for _, statement := range []string{"Mid$(1,1)=\"x\"", "Mid$((s),1)=\"x\"", "Mid$(n,1)=\"x\"", "Mid$(\"abc\",1)=\"x\""} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Sub Run(ByRef s As String, ByRef n As Long)\n" + statement + "\nEnd Sub\n"}}})
		if len(p.GetSemanticDiagnostics(nil)) == 0 {
			t.Fatal("invalid Mid target accepted", statement)
		}
	}
}

func TestMidVariantAndDollarSignatures(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run(ByVal input As Variant)\n Dim boxed As Variant, text As String\n boxed = Mid(input,1)\n text = Mid$(input,1)\nEnd Sub\n"})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	for i, want := range []checker.TypeKind{checker.TypeVariant, checker.TypeString} {
		rhs := body[i+1].AsLetStatement().RExpression
		if c.GetTypeAtLocation(rhs).Kind != want || c.GetCallResolution(rhs).Signature.ReturnType.Kind != want {
			t.Fatal("Mid and Mid$ declarations lost their return distinction", i)
		}
	}
}
