package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func TestImplementsContractDiagnostics(t *testing.T) {
	tests := []struct {
		name, contract, implementation string
		code                           checker.DiagnosticCode
	}{
		{"missing", "Sub Run()\nEnd Sub", "", checker.DiagnosticImplements},
		{"wrong kind", "Sub Run()\nEnd Sub", "Private Function I_Run() As Long\nEnd Function", checker.DiagnosticImplements},
		{"return type", "Function Run() As Long\nEnd Function", "Private Function I_Run() As String\nEnd Function", checker.DiagnosticInterfaceSignature},
		{"parameter type", "Sub Run(ByVal x As Long)\nEnd Sub", "Private Sub I_Run(ByVal x As Integer)\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"byref", "Sub Run(ByRef x As Long)\nEnd Sub", "Private Sub I_Run(ByVal x As Long)\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"optional", "Sub Run(Optional x As Long)\nEnd Sub", "Private Sub I_Run(x As Long)\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"default value", "Sub Run(Optional x As Long = 3)\nEnd Sub", "Private Sub I_Run(Optional x As Long = 4)\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"string default", "Sub Run(Optional x As String = \"a\")\nEnd Sub", "Private Sub I_Run(Optional x As String = \"b\")\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"paramarray", "Sub Run(ParamArray x() As Variant)\nEnd Sub", "Private Sub I_Run(x() As Variant)\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"nominal array", "Sub Run(x() As I)\nEnd Sub", "Private Sub I_Run(x() As C)\nEnd Sub", checker.DiagnosticInterfaceSignature},
		{"missing setter", "Public Value As Long", "Private Property Get I_Value() As Long\nEnd Property", checker.DiagnosticImplements},
		{"field type", "Public Value As Long", "Private Property Get I_Value() As String\nEnd Property\nPrivate Property Let I_Value(ByVal rhs As Long)\nEnd Property", checker.DiagnosticInterfaceSignature},
		{"array field", "Public Values() As Long", "", checker.DiagnosticImplements},
		{"underscore", "Sub Bad_Name()\nEnd Sub", "Private Sub I_Bad_Name()\nEnd Sub", checker.DiagnosticImplements},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := program(t, compiler.SourceFileInput{FileName: "I.cls", Text: tt.contract}, compiler.SourceFileInput{FileName: "C.cls", Text: "Implements I\n" + tt.implementation})
			for _, d := range p.GetTypeChecker().GetDiagnostics(nil) {
				if d.Code == tt.code {
					return
				}
			}
			t.Fatalf("missing %s: %+v", tt.code, p.GetTypeChecker().GetDiagnostics(nil))
		})
	}
}

func TestImplementsDeclarationDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name    string
		sources []compiler.SourceFileInput
		code    checker.DiagnosticCode
	}{
		{"standard module", []compiler.SourceFileInput{{FileName: "M.bas", Text: "Implements I"}, {FileName: "I.cls"}}, checker.DiagnosticImplements},
		{"unknown interface", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements Missing"}}, checker.DiagnosticUnknownType},
		{"standard target", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements M"}, {FileName: "M.bas"}}, checker.DiagnosticUnknownType},
		{"builtin target", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements Collection"}}, checker.DiagnosticImplements},
		{"duplicate", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements I\nImplements i"}, {FileName: "I.cls"}}, checker.DiagnosticImplements},
		{"self", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements C"}}, checker.DiagnosticImplements},
		{"cycle", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements I"}, {FileName: "I.cls", Text: "Implements C"}}, checker.DiagnosticImplements},
		{"prefix", []compiler.SourceFileInput{{FileName: "C.cls", Text: "Implements I\nImplements I_Other"}, {FileName: "I.cls"}, {FileName: "I_Other.cls"}}, checker.DiagnosticImplements},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := program(t, tt.sources...)
			for _, d := range p.GetTypeChecker().GetDiagnostics(nil) {
				if d.Code == tt.code {
					return
				}
			}
			t.Fatalf("missing %s: %+v", tt.code, p.GetTypeChecker().GetDiagnostics(nil))
		})
	}
}

func TestImplementsUsesOnlyPublicContractsAndReturnsIsolatedMetadata(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "I.cls", Text: `Private Sub Hidden()
End Sub
Friend Sub Internal()
End Sub
Public Sub Run(Optional ByVal value As Long = 42)
End Sub
Public Property Get Value() As String
End Property
Private Property Let Value(ByVal rhs As String)
End Property`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Private Sub I_Run(Optional ByVal renamed As Long = 40 + 2)
End Sub
Implements Demo.I
Private Property Get I_Value() As String
End Property`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	class := p.Module("C").Symbol
	got := c.GetImplementedInterfaces(class)
	if len(got) != 1 || len(got[0].Members) != 2 || got[0].Interface != p.Module("I").Symbol || got[0].Members[0].Kind != ast.KindSubDeclaration {
		t.Fatal(got)
	}
	got[0].Members[0].Implementation = nil
	if c.GetImplementedInterfaces(class)[0].Members[0].Implementation == nil {
		t.Fatal("mutable checker metadata escaped")
	}
}
