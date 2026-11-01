package checker_test

import (
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"testing"
)

func TestFixedBoundsAndArrayCopies(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Base 1
Sub Run()
 Dim fixed(2, -2 To 2+1) As Long, dynamic() As Long, v As Variant
 v=fixed
 dynamic=v
 Erase fixed,dynamic,v
 Call Change((dynamic))
End Sub
Sub Change(ByRef a() As Long)
End Sub
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := procedureBody(p, "Main", "Run")
	decl := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	bounds := c.GetArrayBounds(decl)
	if len(bounds) != 2 || bounds[0] != (checker.ArrayStaticBound{Lower: 1, Upper: 2}) || bounds[1] != (checker.ArrayStaticBound{Lower: -2, Upper: 3}) {
		t.Fatal(bounds)
	}
	for _, n := range body[1:3] {
		if r := c.GetAssignmentConversion(n); r == nil || r.Kind != checker.ConversionLet {
			t.Fatal("array boxing/coercion was deferred", r)
		}
	}
	r := c.GetCallResolution(statementCall(body[4]))
	if r == nil || r.Deferred || r.Arguments[0].Mode != checker.ArgumentTemporary {
		t.Fatal("parenthesized array did not retain a value temporary", r)
	}
}

func TestInvalidArrayDeclarationsAndUses(t *testing.T) {
	for _, source := range []string{
		"Sub Run(n As Long)\n Dim a(n) As Long\nEnd Sub",
		"Sub Run()\n Dim a(2 To 1) As Long\nEnd Sub",
		"Sub Run(ByVal a() As Long)\nEnd Sub",
		"Sub Run()\n Dim n As Long\n Erase n\nEnd Sub",
		"Sub Run()\n Dim a() As Long,b() As Integer\n a=b\nEnd Sub",
		"Sub Run()\n Dim a(2) As Long,b() As Long\n a=b\nEnd Sub",
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: source}}})
		if len(p.GetSemanticDiagnostics(nil)) == 0 {
			t.Fatal("invalid array accepted", source)
		}
	}
}
