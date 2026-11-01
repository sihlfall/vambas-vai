package checker_test

import (
	"fmt"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestLibraryModuleAliasesShareCallSignatures(t *testing.T) {
	for _, test := range []struct {
		module, name, arguments string
		member                  stdlib.MemberID
	}{
		{"Strings", "Left$", `"abc",1`, stdlib.LeftString},
		{"Strings", "Left", `"abc",1`, stdlib.Left},
		{"Strings", "InStr", `"abc","b"`, stdlib.InStr},
		{"Strings", "InStr", `String2:="b",String1:="abc"`, stdlib.InStr},
		{"Strings", "Mid$", `"abc",Start:=2`, stdlib.MidString},
		{"Strings", "Len", `Expression:="abc"`, stdlib.Len},
		{"Conversion", "CInt", `Expression:=12`, stdlib.CInt},
		{"Conversion", "CByte", `Expression:=255`, stdlib.CByte},
		{"Conversion", "CDate", `Expression:=1.25`, stdlib.CDate},
		{"Conversion", "CVDate", `Expression:="2024-02-29"`, stdlib.CVDate},
		{"Conversion", "CCur", `Expression:="922337203685477.5807"`, stdlib.CCur},
		{"Conversion", "CDec", `Expression:="79228162514264337593543950335"`, stdlib.CDec},
		{"Conversion", "CVErr", `Expression:=2001`, stdlib.CVErr},
		{"Information", "IsError", `Arg:=CVErr(2001)`, stdlib.IsError},
		{"Information", "VarType", `VarName:=12`, stdlib.VarType},
		{"Interaction", "IIf", `True,1,2`, stdlib.IIf},
		{"Interaction", "CreateObject", `Class:="Rubberduck.AssertClass"`, stdlib.CreateObject},
	} {
		t.Run(test.module+"."+test.name+"/"+test.arguments, func(t *testing.T) {
			prefixes := []string{"", "VBA.", test.module + ".", "vBa." + strings.ToUpper(test.module) + "."}
			var source strings.Builder
			source.WriteString("Option Explicit\nSub Run()\nDim value As Variant\n")
			for _, prefix := range prefixes {
				fmt.Fprintf(&source, "value=%s%s(%s)\n", prefix, test.name, test.arguments)
			}
			source.WriteString("End Sub\n")
			p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: source.String()})
			c := p.GetTypeChecker()
			noErrors(t, c)
			var signature *checker.Signature
			for _, prefix := range prefixes {
				call := find(t, p.Module("Main"), ast.KindCallExpression, prefix+test.name+"("+test.arguments+")")
				r := c.GetCallResolution(call)
				if r == nil || !r.Valid || r.Deferred || r.Signature.Declaration.Symbol.Library != test.member {
					t.Fatalf("%s lost its catalog call resolution", prefix+test.name)
				}
				if signature == nil {
					signature = r.Signature
				} else if signature != r.Signature {
					t.Fatal("qualified aliases created duplicate declarations/signatures")
				}
				if r.Signature.Declaration.Symbol.Parent.Name != test.module {
					t.Fatal("library declaration lost its canonical module owner")
				}
			}
		})
	}
}

func TestLibraryModuleConstantsShareDeclarations(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run()
 Dim value As Variant
 value=vbNullString
 value=VBA.vbNullString
 value=Constants.vbNullString
 value=VBA.Constants.vbNullString
 value=VBA.Constants.vbObjectError
End Sub
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	var target *ast.Symbol
	for _, text := range []string{"vbNullString", "VBA.vbNullString", "Constants.vbNullString", "VBA.Constants.vbNullString"} {
		kind := ast.KindPropertyAccessExpression
		if text == "vbNullString" {
			kind = ast.KindIdentifier
		}
		n := find(t, p.Module("Main"), kind, text)
		symbol := c.GetSymbolAtLocation(n)
		if symbol == nil || symbol.Library != stdlib.NullString || !ast.IsConstantSymbol(symbol) {
			t.Fatalf("%s lost its constant identity", text)
		}
		if target == nil {
			target = symbol
		} else if symbol != target {
			t.Fatal("constant qualification created duplicate declarations")
		}
	}
	value := c.GetSymbolAtLocation(find(t, p.Module("Main"), ast.KindPropertyAccessExpression, "VBA.Constants.vbObjectError"))
	if number, ok := c.GetNumericConstantValue(value); !ok || number != -2147221504 {
		t.Fatal("module-qualified signed constant changed its value", number, ok)
	}
}

func TestUserModuleShadowsLibraryModule(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Strings.bas", Text: "Public Function Left(ByVal value As String, ByVal length As Long) As String\nLeft=\"local\"\nEnd Function\n"},
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run()
 Dim value As String
 value=Left$("abc",1)
 value=Strings.Left$("abc",1)
 value=VBA.Left$("abc",1)
 value=VBA.Strings.Left$("abc",1)
End Sub
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	for _, prefix := range []string{"", "Strings.", "VBA.", "VBA.Strings."} {
		r := c.GetCallResolution(find(t, p.Module("Main"), ast.KindCallExpression, prefix+`Left$("abc",1)`))
		if strings.HasPrefix(prefix, "VBA.") {
			if r.Signature.Declaration.Symbol.Library != stdlib.LeftString {
				t.Fatal("project module shadowed an explicitly qualified library call")
			}
		} else if r.Signature.Declaration.Symbol != p.Module("Strings").Locals.Lookup("Left") {
			t.Fatal("ambient module replaced a user module/function")
		}
	}
}

func TestLibraryModuleLookupRejectsWrongOwnersAndValues(t *testing.T) {
	for _, test := range []struct {
		expression string
		code       ast.BindingDiagnosticCode
	}{
		{`VBA.Strings.CInt(1)`, ast.BindingMemberNotFound},
		{`VBA.Conversion.Left$("abc",1)`, ast.BindingMemberNotFound},
		{`VBA.Strings.vbNullString`, ast.BindingMemberNotFound},
		{`VBA.Constants.vbBinaryCompare`, ast.BindingMemberNotFound},
		{`VBA.Information.LBound(values)`, ast.BindingMemberNotFound},
		{`VBA.Strings.Unknown`, ast.BindingMemberNotFound},
		{`VBA.Strings`, ast.BindingInvalidValueReference},
		{`Strings`, ast.BindingInvalidValueReference},
		{`VBA.VbCompareMethod.vbSunday`, ast.BindingMemberNotFound},
		{`VBA.KeyCodeConstants.vbRed`, ast.BindingMemberNotFound},
		{`VBA.VbCompareMethod`, ast.BindingInvalidValueReference},
		{`VbCompareMethod`, ast.BindingInvalidValueReference},
		{`VBA.ColorConstants`, ast.BindingInvalidValueReference},
	} {
		t.Run(test.expression, func(t *testing.T) {
			p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Option Explicit\nSub Run()\nDim value As Variant,values(0) As Long\nvalue=" + test.expression + "\nEnd Sub\n"}}})
			if ds := p.ParseDiagnostics(); len(ds) != 0 {
				t.Fatal(ds)
			}
			for _, d := range p.BindDiagnostics() {
				if d.Code == test.code {
					return
				}
			}
			t.Fatalf("%s lost diagnostic %s", test.expression, test.code)
		})
	}
}
