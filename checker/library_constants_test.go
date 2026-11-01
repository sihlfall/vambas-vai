package checker_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestAllLibraryConstantAliasesAndValues(t *testing.T) {
	var source strings.Builder
	source.WriteString("Option Explicit\nPublic Enum Values\n")
	for i, entry := range stdlib.Constants() {
		if entry.Value.Kind == stdlib.NumericLiteral {
			fmt.Fprintf(&source, " N%d = VBA.%s.%s\n", i, entry.Owner, entry.Name)
		}
	}
	source.WriteString("End Enum\nSub Run()\n Dim value As Variant\n")
	for _, entry := range stdlib.Constants() {
		for _, prefix := range []string{"", "VBA.", entry.Owner + ".", "vBa." + strings.ToUpper(entry.Owner) + "."} {
			fmt.Fprintf(&source, " value=%s%s\n", prefix, entry.Name)
		}
	}
	source.WriteString("End Sub\n")
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: source.String()})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	for i, entry := range stdlib.Constants() {
		var target *ast.Symbol
		for _, prefix := range []string{"", "VBA.", entry.Owner + ".", "vBa." + strings.ToUpper(entry.Owner) + "."} {
			kind := ast.KindPropertyAccessExpression
			if prefix == "" {
				kind = ast.KindIdentifier
			}
			n := find(t, file, kind, prefix+entry.Name)
			symbol := c.GetSymbolAtLocation(n)
			if symbol == nil || symbol.Library != entry.ID || !ast.IsConstantSymbol(symbol) || symbol.Parent.Name != entry.Owner {
				t.Fatalf("%s lost its identity or owner", prefix+entry.Name)
			}
			if target != nil && symbol != target {
				t.Fatalf("%s aliases have different declarations", entry.Name)
			}
			target = symbol
			want := checker.TypeLong
			if entry.Type.Kind == stdlib.TypeString {
				want = checker.TypeString
			}
			requireType(t, c, n, want)
		}
		if entry.Value.Kind == stdlib.NumericLiteral {
			text, base := entry.Value.Text, 10
			if strings.HasPrefix(text, "&H") {
				text, base = text[2:], 16
			}
			number, err := strconv.ParseInt(text, base, 64)
			if err != nil {
				t.Fatal(err)
			}
			want := int64(int32(number)) // Hex system colors carry signed Long bits.
			if value, ok := c.GetNumericConstantValue(target); !ok || value != float64(want) {
				t.Fatalf("%s: got %v, %v; want %d", entry.Name, value, ok, want)
			}
			if value, ok := c.GetEnumMemberValue(file.Locals.Lookup(fmt.Sprintf("N%d", i))); !ok || value != want {
				t.Fatalf("%s did not retain its value in a user enum: %d, %v", entry.Name, value, ok)
			}
			if target.Flags&ast.SymbolFlagsEnumMember != 0 {
				if value, ok := c.GetEnumMemberValue(target); !ok || value != want {
					t.Fatalf("%s lost its prepared enum value: %d, %v", entry.Name, value, ok)
				}
			}
		}
	}
}

func TestLibraryEnumsAreTypesAndRespectShadowing(t *testing.T) {
	var source strings.Builder
	source.WriteString("Option Explicit\nPublic Enum VbCompareMethod\n vbTextCompare = 42\nEnd Enum\nSub Run()\n Dim value As Long\n")
	for i, enum := range stdlib.Enums() {
		fmt.Fprintf(&source, " Dim a%d As %s, b%d As VBA.%s\n", i, enum.Name, i, enum.Name)
	}
	source.WriteString(" value=vbTextCompare\n value=VbCompareMethod.vbTextCompare\n value=VBA.VbCompareMethod.vbTextCompare\n value=KeyCodeConstants.vbKeyA\n value=VBA.KeyCodeConstants.vbKeyA\nEnd Sub\n")
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: source.String()},
		compiler.SourceFileInput{FileName: "KeyCodeConstants.bas", Text: "Public Const vbKeyA As Long = 99\n"})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	for _, enum := range stdlib.Enums() {
		for _, prefix := range []string{"", "VBA."} {
			requireType(t, c, find(t, file, ast.KindTypeReference, prefix+enum.Name), checker.TypeLong)
		}
	}
	for _, test := range []struct {
		text string
		id   stdlib.MemberID
	}{
		{"vbTextCompare", stdlib.None}, {"VbCompareMethod.vbTextCompare", stdlib.None},
		{"VBA.VbCompareMethod.vbTextCompare", stdlib.VBTextCompare},
		{"KeyCodeConstants.vbKeyA", stdlib.None}, {"VBA.KeyCodeConstants.vbKeyA", stdlib.VBKeyA},
	} {
		kind := ast.KindPropertyAccessExpression
		if test.text == "vbTextCompare" {
			kind = ast.KindIdentifier
		}
		symbol := c.GetSymbolAtLocation(find(t, file, kind, test.text))
		if symbol == nil || symbol.Library != test.id {
			t.Fatalf("%s lost its shadowing/qualified lookup", test.text)
		}
	}
}

func TestCatalogConstantsKeepTypesValuesAndShadowing(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Private Const vbObjectError As Long = 10
Private Const vbNullString As String = "local"
Public Enum Errors
 LocalBase = vbObjectError + 3
 LibraryBase = VBA.vbObjectError + 3
End Enum
Function Text() As String
 Text = vbNullString & VBA.vbNullString & VBA.vbNullChar
End Function
Function Compare() As Long
 Compare = VBA.vbBinaryCompare
End Function
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	for name, want := range map[string]int64{"LocalBase": 13, "LibraryBase": -2147221501} {
		if value, ok := c.GetEnumMemberValue(file.Locals.Lookup(name)); !ok || value != want {
			t.Fatalf("%s: got %d, %v; want %d", name, value, ok, want)
		}
	}
	for _, test := range []struct {
		name   string
		member stdlib.MemberID
		kind   checker.TypeKind
	}{
		{"vbObjectError", stdlib.ObjectError, checker.TypeLong},
		{"vbNullString", stdlib.NullString, checker.TypeString},
		{"vbNullChar", stdlib.NullChar, checker.TypeString},
		{"vbBinaryCompare", stdlib.BinaryCompare, checker.TypeLong},
	} {
		node := find(t, file, ast.KindPropertyAccessExpression, "VBA."+test.name)
		symbol := c.GetSymbolAtLocation(node)
		if symbol == nil || symbol.Library != test.member || !ast.IsConstantSymbol(symbol) || c.GetTypeAtLocation(node).Kind != test.kind {
			t.Fatalf("%s lost its library identity, type, or Const declaration", test.name)
		}
		if test.member == stdlib.ObjectError {
			if value, ok := c.GetNumericConstantValue(symbol); !ok || value != -2147221504 {
				t.Fatal("vbObjectError lost its signed value", value, ok)
			}
			initializer := symbol.ValueDeclaration.AsVariableDeclaration().Initializer
			if c.GetTypeAtLocation(initializer).Kind != checker.TypeLong {
				t.Fatal("vbObjectError initializer changed its declared arithmetic type")
			}
		}
	}
	for _, name := range []string{"vbObjectError", "vbNullString"} {
		local := file.Locals.Lookup(name)
		use := find(t, file, ast.KindIdentifier, name)
		if local.Library != stdlib.None || c.GetSymbolAtLocation(use) != local {
			t.Fatalf("library metadata replaced user constant %s", name)
		}
	}
}
