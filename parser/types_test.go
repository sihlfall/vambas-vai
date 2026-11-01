package parser_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/parser"
)

func TestTypeAndEnumSyntax(t *testing.T) {
	text := `Public Type Point
    X As Long
    End As String * 12
    Type(1 To 3) As Double
    Children() As Point
End Type
Private Enum Color
    Red
    Green = &H100& Or 2
    Blue
End Enum
Sub Run()
End Sub
`
	file := parseDeclarations(t, text)
	if len(file.Statements.Nodes) != 3 || len(file.FunctionDeclarations.Nodes) != 1 {
		t.Fatal("declaration lists overlap")
	}
	record, enum := file.Statements.Nodes[0].AsTypeDeclaration(), file.Statements.Nodes[1].AsEnumDeclaration()
	if record.Name.AsIdentifier().Text != "Point" || record.Modifiers != ast.ModifierPublic || len(record.Members.Nodes) != 4 {
		t.Fatal(record)
	}
	if enum.Modifiers != ast.ModifierPrivate || len(enum.Members.Nodes) != 3 || enum.Members.Nodes[0].AsEnumMember().Initializer != nil || enum.Members.Nodes[2].AsEnumMember().Initializer != nil {
		t.Fatal("implicit enum syntax was synthesized")
	}
	if record.Members.Nodes[1].AsFieldDeclaration().Type.AsTypeReference().Length == nil || len(record.Members.Nodes[3].AsFieldDeclaration().Dimensions.Nodes) != 0 {
		t.Fatal("field shape lost")
	}
	other := parser.ParseSourceFile(text)
	if !file.AsNode().DeeplyEquals(other) {
		t.Fatal("declaration syntax equality failed")
	}
}

func TestTypeAndEnumRecovery(t *testing.T) {
	for _, text := range []string{
		"Type Broken\n X As Long\nSub Good()\nEnd Sub\n",
		"Enum Broken\n A =\nEnd Enum\nSub Good()\nEnd Sub\n",
		"Type Broken\n X As Long\nPublic Enum NextOne\n A\nEnd Enum\nSub Good()\nEnd Sub\n",
	} {
		file := parser.ParseSourceFile(text).AsSourceFile()
		if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) != 1 {
			t.Fatalf("recovery consumed next procedure: %q", text)
		}
		validateRanges(t, file)
	}
}
