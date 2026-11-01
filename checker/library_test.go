package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestLibraryPoliciesRespectShadowing(t *testing.T) {
	for _, test := range []struct {
		name, parameters, arguments string
		member                      stdlib.MemberID
		libraryResult               checker.TypeKind
	}{
		{"Len", "ByVal value As String", `"abc"`, stdlib.Len, checker.TypeLong},
		{"LenB", "ByVal value As String", `"abc"`, stdlib.LenB, checker.TypeLong},
		{"Mid", "ByVal value As String, ByVal start As Long", `"abc", 1`, stdlib.Mid, checker.TypeString},
		{"Mid$", "ByVal value As String, ByVal start As Long", `"abc", 1`, stdlib.MidString, checker.TypeString},
		{"InStr", "ByVal first As String, ByVal second As String", `"abc", "b"`, stdlib.InStr, checker.TypeVariant},
		{"CByte", "ByVal value As Variant", `255`, stdlib.CByte, checker.TypeByte},
		{"CDate", "ByVal value As Variant", `1.25`, stdlib.CDate, checker.TypeDate},
		{"CVDate", "ByVal value As Variant", `1.25`, stdlib.CVDate, checker.TypeVariant},
		{"CCur", "ByVal value As Variant", `"1.25"`, stdlib.CCur, checker.TypeCurrency},
		{"CDec", "ByVal value As Variant", `"1.25"`, stdlib.CDec, checker.TypeVariant},
		{"CVErr", "ByVal value As Variant", `2001`, stdlib.CVErr, checker.TypeVariant},
		{"IsError", "ByVal value As Variant", `CVErr(2001)`, stdlib.IsError, checker.TypeBoolean},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A user Mid function also shadows calls using the $ type character.
			name := test.name
			if name == "Mid$" {
				name = "Mid"
			}
			p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Option Explicit\nFunction " + name + "(" + test.parameters + ") As Variant\nEnd Function\nSub Probe()\nDim value As Variant\nvalue=" + test.name + "(" + test.arguments + ")\nvalue=VBA." + test.name + "(" + test.arguments + ")\nEnd Sub\n"})
			c := p.GetTypeChecker()
			noErrors(t, c)
			local := c.GetCallResolution(find(t, p.Module("Main"), ast.KindCallExpression, test.name+"("+test.arguments+")"))
			library := c.GetCallResolution(find(t, p.Module("Main"), ast.KindCallExpression, "VBA."+test.name+"("+test.arguments+")"))
			if local.Signature.Declaration.Symbol.Library != stdlib.None || local.ResultType.Kind != checker.TypeVariant || library.Signature.Declaration.Symbol.Library != test.member || library.ResultType.Kind != test.libraryResult {
				t.Fatal("library policy affected a shadowing user declaration", local, library)
			}
			if test.member == stdlib.InStr {
				if len(local.Arguments) != 2 || local.Arguments[0].ParameterIndex != 0 || local.Arguments[1].ParameterIndex != 1 || library.Arguments[0].ParameterIndex != 1 || library.Arguments[1].ParameterIndex != 2 {
					t.Fatal("InStr shorthand mapping affected a user function", local, library)
				}
			}
		})
	}
}

func TestLibraryPoliciesRetainInvalidCallInformation(t *testing.T) {
	for _, test := range []struct {
		call     string
		member   stdlib.MemberID
		result   checker.TypeKind
		declared checker.TypeKind
		code     checker.DiagnosticCode
	}{
		{`VBA.Len(Unknown:="abc")`, stdlib.Len, checker.TypeLong, checker.TypeVariant, checker.DiagnosticUnknownArgument},
		{`VBA.LenB(Unknown:="abc")`, stdlib.LenB, checker.TypeLong, checker.TypeVariant, checker.DiagnosticUnknownArgument},
		{`VBA.Mid(String:="abc", Start:=1, Unknown:=2)`, stdlib.Mid, checker.TypeString, checker.TypeVariant, checker.DiagnosticUnknownArgument},
		{`VBA.Mid$(String:="abc", Start:=1, Unknown:=2)`, stdlib.MidString, checker.TypeString, checker.TypeString, checker.DiagnosticUnknownArgument},
		{`VBA.InStr(String1:="abc", Compare:=0)`, stdlib.InStr, checker.TypeVariant, checker.TypeVariant, checker.DiagnosticArgumentCount},
	} {
		t.Run(test.call, func(t *testing.T) {
			p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Probe()\nDim value As Variant\nvalue=" + test.call + "\nEnd Sub\n"})
			c := p.GetTypeChecker()
			found := false
			for _, d := range c.GetDiagnostics(nil) {
				found = found || d.Code == test.code
			}
			if !found {
				t.Fatalf("invalid library call lost diagnostic %s", test.code)
			}
			r := c.GetCallResolution(find(t, p.Module("Main"), ast.KindCallExpression, test.call))
			if r == nil || r.Valid || r.Deferred || r.Signature == nil || r.Signature.Declaration.Symbol.Library != test.member || r.Signature.ReturnType.Kind != test.declared || r.ResultType.Kind != test.result {
				t.Fatal("invalid library call lost its selected signature or result type", r)
			}
		})
	}
}
