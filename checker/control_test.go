package checker_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"testing"
)

func TestControlConditionsAndLoopVariables(t *testing.T) {
	for _, source := range []string{
		"Sub Run()\n Dim a() As Long\n If a Then\n End If\nEnd Sub",
		"Sub Run()\n Dim s As String\n For s=1 To 3\n Next\nEnd Sub",
		"Sub Run()\n Dim i As Long,a(2) As Long\n For Each i In a\n Next\nEnd Sub",
		"Sub Run()\n Dim i As Long\n For i=1 To 3\n For i=1 To 2\n Next\n Next\nEnd Sub",
		"Sub Run()\n Dim a() As Long\n Select Case a\n Case 1\n End Select\nEnd Sub",
		"Sub Run()\n Dim a() As Long\n Select Case 1\n Case 0 To a\n End Select\nEnd Sub",
		"Sub Run()\n Dim a() As Long\n On a GoTo Done\nDone: End Sub",
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: source}}})
		if len(p.GetSemanticDiagnostics(nil)) == 0 {
			t.Fatal("invalid control construct accepted", source)
		}
	}
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run()
 Dim v As Variant,i As Long
 If Null Then Exit Sub
 Do Until Null
  Exit Do
 Loop
 For i=1 To 3
 Next
End Sub`})
	noErrors(t, p.GetTypeChecker())
}
