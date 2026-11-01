package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func TestRecordsEnumsAndStorageAcrossModules(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef answer As Long)
    Dim value As Shapes.Box, other As Shapes.Box, choice As Demo.Color
    value.Inner.X = Shapes.Red
    With value.Inner
        .X = Color.Blue
    End With
    value.Items(1).X = Color.Green
    other = value
    Bump value.Inner.X
    Bump choice
    Copy value, other
    answer = value.Inner.X
End Sub
Sub Bump(ByRef value As Long)
    value = value + 1
End Sub
Sub Copy(ByRef target As Shapes.Box, ByRef source As Shapes.Box)
    target = source
End Sub
`},
		compiler.SourceFileInput{FileName: "Shapes.bas", Text: `Public Type Box
    Inner As Point
    Items(1 To 3) As Point
    DynamicItems() As Point
End Type
Public Type Point
    X As Long
    Caption As String * 12
End Type
Public Enum Color
    Red
    Green = &H100& Or 2
    Blue
End Enum
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	field := find(t, file, ast.KindPropertyAccessExpression, "value.Inner.X")
	requireType(t, c, field, checker.TypeLong)
	storage := c.GetStorageAtLocation(field)
	if !storage.Writable || !storage.Addressable || storage.Receiver == nil || storage.Symbol.Parent.Name != "Point" {
		t.Fatal(storage)
	}
	call := find(t, file, ast.KindCallExpression, "Bump value.Inner.X")
	if r := c.GetCallResolution(call); !r.Valid || r.Arguments[0].Mode != checker.ArgumentReference {
		t.Fatal(r)
	}
	for _, test := range []struct {
		name  string
		value int64
	}{{"Red", 0}, {"Green", 258}, {"Blue", 259}} {
		s := p.Module("Shapes").Locals.Lookup(test.name)
		if value, ok := c.GetEnumMemberValue(s); !ok || value != test.value {
			t.Fatal(test.name, value, ok)
		}
		if c.GetTypeOfSymbol(s).Kind != checker.TypeLong || s.Parent.Name != "Color" {
			t.Fatal(s)
		}
	}
	choice := find(t, file, ast.KindVariableDeclaration, "choice As Demo.Color")
	requireType(t, c, choice, checker.TypeLong)
}

func TestEnumValuesAndFailures(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Private Const Base As Long = 7
Enum Values
    Negative = -2
    Auto
    Mask = &HFFFF&
    AllBits = &HFFFFFFFF
    Shifted = Base * 2 ^ 3
    Rounded = 2.5
    NextValue
    MinValue = -2147483648
End Enum
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	for name, want := range map[string]int64{"Negative": -2, "Auto": -1, "Mask": 65535, "AllBits": -1, "Shifted": 56, "Rounded": 2, "NextValue": 3, "MinValue": -2147483648} {
		if got, ok := c.GetEnumMemberValue(p.Module("Main").Locals.Lookup(name)); !ok || got != want {
			t.Error(name, got, ok, want)
		}
	}
	for _, text := range []string{
		"Enum E\n A = 2147483648\nEnd Enum",
		"Enum E\n A = 2147483647\n B\nEnd Enum",
		"Enum E\n A = B\n B = 2\nEnd Enum",
		"Enum E\n A = 1 / 0\nEnd Enum",
		"Enum E\n A = 32767 + 1\nEnd Enum",
		"Enum E\n A = \"hello\"\nEnd Enum",
	} {
		p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: text})
		found := false
		for _, d := range p.GetSemanticDiagnostics(nil) {
			found = found || d.Code == checker.DiagnosticEnumValue
		}
		if !found {
			t.Error("invalid enum had no value diagnostic", text)
		}
	}
}

func TestNominalRecordsAndPrivateTypeLookup(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Private Type First
    X As Long
End Type
Private Type Second
    X As Long
End Type
Sub Run(ByVal First As Long)
    Dim a As First, b As Second, v As Variant
    a = b
    v = a
    Set a = b
End Sub
`})
	c := p.GetTypeChecker()
	count := 0
	for _, d := range c.GetDiagnostics(nil) {
		if d.Code == checker.DiagnosticTypeMismatch {
			count++
		} else {
			t.Error(d.Message)
		}
	}
	if count != 3 {
		t.Fatal("nominal record conversions were accepted", c.GetDiagnostics(nil))
	}
	p = program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Type Recursive\n Self As Recursive\nEnd Type\n"})
	if ds := p.GetSemanticDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticRecursiveType {
		t.Fatal(ds)
	}
}
