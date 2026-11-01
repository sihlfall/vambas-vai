package checker_test

import (
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"testing"
)

func TestStringByteArrayConversions(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run()
 Dim a() As Byte, s As String, v As Variant
 a=s
 s=a
 a=v
 v=a
 TakeString a
 TakeString (a)
End Sub
Sub TakeString(ByVal s As String)
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := procedureBody(p, "Main", "Run")
	for _, n := range body[1:5] {
		if conversion := c.GetAssignmentConversion(n); conversion == nil || conversion.Kind != checker.ConversionLet {
			t.Fatalf("missing Let conversion: %+v", conversion)
		}
	}
}

func TestStringByteArrayConversionRestrictions(t *testing.T) {
	for _, body := range []string{
		`Dim a(3) As Byte: a="AB"`,
		`Dim a() As Integer, s As String: a=s`,
		`Dim a() As Integer, s As String: s=a`,
		`Dim a() As Byte, b() As Integer: a=b`,
		`Dim a() As Byte: a=42`,
		`Dim a() As Byte: TakeString a`,
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Sub Run()\n" + body + "\nEnd Sub\nSub TakeString(ByRef s As String)\nEnd Sub"}}})
		if len(p.GetSemanticDiagnostics(nil)) == 0 {
			t.Fatalf("invalid assignment or ByRef coercion accepted: %s", body)
		}
	}
}
