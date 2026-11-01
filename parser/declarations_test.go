package parser_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func parseDeclarations(t *testing.T, source string) *ast.SourceFile {
	t.Helper()
	file := parser.ParseSourceFile(source).AsSourceFile()
	if len(file.ParseDiagnostics) != 0 {
		for _, diagnostic := range file.ParseDiagnostics {
			t.Errorf("%s [%d,%d) %q: %s", diagnostic.Code, diagnostic.Pos(), diagnostic.End(), source[diagnostic.Pos():diagnostic.End()], diagnostic.Message)
		}
		t.FailNow()
	}
	validateRanges(t, file)
	return file
}

func TestModuleDeclarationsAndProcedureSignatures(t *testing.T) {
	source := "Option Explicit\nPrivate total As Long\nPublic Const Limit As Long = 10, Greeting As String = \"hello\"\n" +
		"Private Function Compute(ByVal input As Long, ByRef output As Long, Optional factor As Long = Limit) As Long\n" +
		"Dim local As Long\nlocal = input * factor\noutput = local\nCompute = local\nEnd Function\n" +
		"Public Static Sub Reset()\ntotal = 0\nEnd Sub"
	file := parseDeclarations(t, source)
	if len(file.Statements.Nodes) != 5 || len(file.FunctionDeclarations.Nodes) != 2 {
		t.Fatal("source elements were not retained in source order")
	}
	if file.Statements.Nodes[0].Kind != ast.KindOptionExplicitStatement || file.Statements.Nodes[1].Kind != ast.KindVariableStatement {
		t.Fatal("module declarations have incorrect kinds")
	}
	constant := file.Statements.Nodes[2].AsVariableStatement()
	if constant.Modifiers != ast.ModifierPublic || constant.DeclarationList.Flags&ast.NodeFlagsConst == 0 || len(constant.DeclarationList.AsVariableDeclarationList().Declarations.Nodes) != 2 {
		t.Fatal("Const declaration list was not retained")
	}
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	if fn.Type.AsTypeReference().TypeName.Kind != ast.KindLONG || len(fn.Parameters.Nodes) != 3 {
		t.Fatal("function signature was not parsed")
	}
	first, second, third := fn.Parameters.Nodes[0].AsParameterDeclaration(), fn.Parameters.Nodes[1].AsParameterDeclaration(), fn.Parameters.Nodes[2].AsParameterDeclaration()
	if first.IsByRef() || !second.IsByRef() || !third.IsByRef() || third.Modifiers&ast.ParameterOptional == 0 || third.Initializer.AsIdentifier().Text != "Limit" {
		t.Fatal("parameter passing modes or default value were lost")
	}
	sub := file.FunctionDeclarations.Nodes[1]
	if sub.Kind != ast.KindSubDeclaration || sub.AsSubDeclaration().Modifiers() != ast.ModifierPublic|ast.ModifierStatic {
		t.Fatal("Sub signature or modifiers were lost")
	}
	binder.BindSourceFile(file)
	if len(file.BindDiagnostics) != 0 || !file.OptionExplicit {
		t.Fatalf("binding diagnostics: %+v", file.BindDiagnostics)
	}
	if sub.AsSubDeclaration().ResultSymbol != nil || sub.Locals.Lookup("Reset") != nil {
		t.Fatal("Sub unexpectedly has a function result variable")
	}
}

func TestIndependentDeclaratorTypesAndArrayBounds(t *testing.T) {
	file := parseDeclarations(t, "Const Size = 4\nDim first, second As Long, rows(-1 To Size, 2) As Double, dynamic() As String, names As String(), label As String * 12\nSub S()\nStatic cached As New Scripting.Dictionary\nEnd Sub")
	decls := file.Statements.Nodes[1].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(decls) != 6 || decls[0].AsVariableDeclaration().Type != nil || decls[1].AsVariableDeclaration().Type.AsTypeReference().TypeName.Kind != ast.KindLONG {
		t.Fatal("an As clause incorrectly applied to multiple declarators")
	}
	array := decls[2].AsVariableDeclaration()
	if len(array.Dimensions.Nodes) != 2 || array.Dimensions.Nodes[0].AsArrayDimension().LowerBound.Kind != ast.KindPrefixUnaryExpression || array.Dimensions.Nodes[1].AsArrayDimension().LowerBound != nil {
		t.Fatal("multidimensional bounds were not retained")
	}
	if decls[3].AsVariableDeclaration().Dimensions == nil || len(decls[3].AsVariableDeclaration().Dimensions.Nodes) != 0 || !decls[4].AsVariableDeclaration().Type.AsTypeReference().IsArray || decls[5].AsVariableDeclaration().Type.AsTypeReference().Length.AsNumericLiteral().Text != "12" {
		t.Fatal("dynamic arrays, array types, or fixed-length strings were lost")
	}
	local := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().Body.AsBlock().Statements.Nodes[0].AsVariableStatement()
	typeNode := local.DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].AsVariableDeclaration().Type.AsTypeReference()
	if local.Modifiers&ast.ModifierStatic == 0 || typeNode.NewKeyword == nil || typeNode.TypeName.AsQualifiedName().Right.AsIdentifier().Text != "Dictionary" {
		t.Fatal("Static As New qualified type was not retained")
	}
	binder.BindSourceFile(file)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	if file.Locals.Lookup("Scripting") != nil || file.Locals.Lookup("Dictionary") != nil {
		t.Fatal("type names became value declarations")
	}
}

func TestParameterVariantsAndTypeHints(t *testing.T) {
	for _, source := range []string{
		"Function F(ByRef name As String, Optional ByVal count As Long = 2) As String\nF = name\nEnd Function",
		"Sub S(ByVal Optional name As String = \"hello\")\nEnd Sub",
		"Sub S(values() As Long, ParamArray rest() As Variant)\nEnd Sub",
		"Sub S(ParamArray args())\nEnd Sub",
		"Sub S(ParamArray args() As [Variant])\nEnd Sub",
		"Function F() As Byte()\nEnd Function",
		"Function F$(ByVal value$)\nF$ = value$\nEnd Function",
		"Function F%(value%)\nF = value\nEnd Function",
		"Sub S()\nDim i%, l&, d#, s!, c@, ll^\ni = 1\nEnd Sub",
		"Function F%(ByVal value%)\nDim localValue%\nlocalValue% = value%\nF% = localValue%\nEnd Function",
		"Function F&(ByVal value&)\nF& = value&\nEnd Function",
		"Sub S()\nDim x As Long, result As Long\nresult = x^2\nEnd Sub",
		"Const Pi As Double = 3.14159, Small = .5, Large = 1E+4, Precise = 2d-3, HexValue = &HFF, OctalValue = &o77\nSub S(Optional ratio As Double = .25)\nEnd Sub",
	} {
		t.Run(source, func(t *testing.T) {
			file := parseDeclarations(t, source)
			binder.BindSourceFile(file)
			if len(file.BindDiagnostics) != 0 {
				t.Fatal(file.BindDiagnostics)
			}
		})
	}
}

func TestDeclarationErrorsRecoverIntoNextProcedure(t *testing.T) {
	for name, damaged := range map[string]string{
		"missing variable name":     "Dim As Long\n",
		"missing variable type":     "Dim x As\n",
		"missing const initializer": "Const Count As Long =\n",
		"Dim initializer":           "Dim x = 1\n",
		"missing comma":             "Dim x As Long y As String\n",
		"trailing comma":            "Dim x As Long,\n",
		"bad bound":                 "Dim rows(1 To) As Long\n",
		"missing parameter name":    "Sub Bad(ByVal As Long)\nEnd Sub\n",
		"bad default":               "Sub Bad(Optional n As Long =)\nEnd Sub\n",
		"required after optional":   "Sub Bad(Optional x, y)\nEnd Sub\n",
		"default on required":       "Sub Bad(x = 1)\nEnd Sub\n",
		"conflicting modes":         "Sub Bad(ByRef ByVal x)\nEnd Sub\n",
		"ByVal array":               "Sub Bad(ByVal x() As Long)\nEnd Sub\n",
		"fixed parameter bounds":    "Sub Bad(x(10) As Long)\nEnd Sub\n",
		"ParamArray not array":      "Sub Bad(ParamArray x)\nEnd Sub\n",
		"ParamArray wrong type":     "Sub Bad(ParamArray x() As Long)\nEnd Sub\n",
		"ParamArray nested array":   "Sub Bad(ParamArray x() As Variant())\nEnd Sub\n",
		"ParamArray not final":      "Sub Bad(ParamArray x(), y)\nEnd Sub\n",
		"ParamArray Optional":       "Sub Bad(Optional x, ParamArray y())\nEnd Sub\n",
		"ParamArray ByRef":          "Sub Bad(ByRef ParamArray x())\nEnd Sub\n",
		"Sub return type":           "Sub Bad() As Long\nEnd Sub\n",
		"wrong terminator":          "Sub Bad()\nEnd Function\n",
		"missing terminator":        "Sub Bad()\nDim x As Long\n",
		"As New parameter":          "Sub Bad(x As New Widget)\nEnd Sub\n",
		"As New return type":        "Function Bad() As New Widget\nEnd Function\n",
		"As Any":                    "Sub Bad(x As Any)\nEnd Sub\n",
		"duplicate option":          "Option Explicit\nOption Explicit\n",
		"trailing option text":      "Option Explicit Off\n",
		"local option":              "Sub Bad()\nOption Explicit\nEnd Sub\n",
		"module static":             "Static n As Long\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := parser.ParseSourceFile(damaged + "Sub Good()\nDim value As Long\nvalue = 1\nEnd Sub").AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("malformed declaration produced no diagnostic")
			}
			validateRanges(t, file)
			binder.BindSourceFile(file)
			if file.Locals.Lookup("Good") == nil || file.Locals.Lookup("") != nil {
				t.Fatal("recovery lost the valid procedure or declared an empty name")
			}
			if file.Locals.Lookup("Good").ValueDeclaration.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
				t.Fatal("declaration recovery marked the following valid Sub as erroneous")
			}
		})
	}
}

func TestDeclarationContinuationAndEquality(t *testing.T) {
	source := "Option _\r\n Explicit\r\nConst Limit As Long = 2\r\nFunction F( _\r\n ByVal input As Long, _\r\n Optional factor As Long = Limit _\r\n) As Long\r\nDim value As Long, _\r\n other As String\r\nF = input * factor\r\nEnd Function"
	file := parseDeclarations(t, source)
	plain := parser.ParseSourceFile(source)
	binder.BindSourceFile(file)
	if !file.AsNode().DeeplyEquals(plain) {
		t.Fatal("declaration binding changed syntax equality")
	}
	other := parser.ParseSourceFile(strings.Replace(source, "ByVal input", "ByRef input", 1))
	if file.AsNode().DeeplyEquals(other) {
		t.Fatal("syntax equality ignored parameter passing modes")
	}
}
