package binder_test

import (
	"strings"
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func bindDeclarations(t *testing.T, source string) *ast.SourceFile {
	t.Helper()
	file := parser.ParseSourceFile(source).AsSourceFile()
	for _, diagnostic := range file.ParseDiagnostics {
		t.Errorf("%s [%d,%d): %s", diagnostic.Code, diagnostic.Pos(), diagnostic.End(), diagnostic.Message)
	}
	if len(file.ParseDiagnostics) != 0 {
		t.FailNow()
	}
	binder.BindSourceFile(file)
	return file
}

func requireNoBindDiagnostics(t *testing.T, file *ast.SourceFile) {
	t.Helper()
	for _, diagnostic := range file.BindDiagnostics {
		t.Errorf("%s: %s", diagnostic.Code, diagnostic.Message)
	}
}

func TestOptionExplicitDeclaresNamesWithoutInventingLocals(t *testing.T) {
	file := bindDeclarations(t, "Option Explicit\nDim sharedValue As Long\nFunction F(ByVal argument As Long) As Long\nDim localValue As Long\nlocalValue = argument + sharedValue\nmisspelled = unknown + 1\nF = localValue\nEnd Function")
	if !file.OptionExplicit || len(file.BindDiagnostics) != 2 {
		t.Fatalf("unexpected diagnostics: %+v", file.BindDiagnostics)
	}
	fn := file.FunctionDeclarations.Nodes[0]
	for _, diagnostic := range file.BindDiagnostics {
		if diagnostic.Code != ast.BindingUndeclaredName || diagnostic.Node.Symbol != nil ||
			!diagnostic.Loc.IsValid() || diagnostic.Loc.Len() != len(diagnostic.Node.AsIdentifier().Text) {
			t.Fatal("undeclared-name diagnostic has a bogus symbol or location")
		}
	}
	if fn.Locals.Lookup("misspelled") != nil || fn.Locals.Lookup("unknown") != nil || file.ClassifiableNames.Has("misspelled") || len(fn.Locals) != 3 {
		t.Fatal("Option Explicit introduced implicit locals")
	}
	if fn.Locals.Lookup("argument").ValueDeclaration.Kind != ast.KindParameter || fn.Locals.Lookup("localValue").ValueDeclaration.Kind != ast.KindVariableDeclaration {
		t.Fatal("declaration symbols do not point to their syntax nodes")
	}
}

func TestProcedureWideDeclarationsPrecedeReferencesAndShadowModules(t *testing.T) {
	file := bindDeclarations(t, "Option Explicit\nDim value As Long\nSub S(condition As Boolean)\nvalue = 1\nIf condition Then\nDim value As Long\nEnd If\nvalue = 2\nEnd Sub")
	requireNoBindDiagnostics(t, file)
	fn := file.FunctionDeclarations.Nodes[0]
	body := fn.AsSubDeclaration().Body.AsBlock().Statements.Nodes
	local := fn.Locals.Lookup("value")
	if local == nil || local == file.Locals.Lookup("value") || body[0].AsLetStatement().LExpression.Symbol != local || body[2].AsLetStatement().LExpression.Symbol != local {
		t.Fatal("local declaration was not procedure-wide or failed to shadow the module")
	}
	branch := body[1].AsIfStatement().ThenStatement
	if branch.Locals != nil || local.ValueDeclaration != branch.AsBlock().Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0] {
		t.Fatal("branch declaration did not retain its procedure scope")
	}
}

func TestImplicitNamesAndExplicitForwardDeclarationsRemainDistinct(t *testing.T) {
	file := bindDeclarations(t, "Sub S()\nexplicitValue = 1\nimplicitValue = 2\nDim explicitValue As Long\nEnd Sub")
	requireNoBindDiagnostics(t, file)
	fn := file.FunctionDeclarations.Nodes[0]
	if len(fn.Locals) != 2 || fn.Locals.Lookup("explicitValue").ValueDeclaration.Kind != ast.KindVariableDeclaration || fn.Locals.Lookup("implicitValue").ValueDeclaration.Kind != ast.KindIdentifier {
		t.Fatal("explicit and implicit declarations were not distinguished")
	}
}

func TestModuleExportsAndForwardReferences(t *testing.T) {
	file := bindDeclarations(t, "Option Explicit\nPublic exposed As Long\nGlobal globalValue As Long\nDim hidden As Long\nPrivate privateValue As Long\nPublic Const Visible = 2\nConst PrivateConstant = Visible + 1\nFunction F() As Long\nF = PrivateConstant + exposed\nEnd Function\nPrivate Sub HiddenProcedure()\nEnd Sub")
	requireNoBindDiagnostics(t, file)
	for _, name := range []string{"exposed", "globalValue", "Visible", "F"} {
		if file.Symbol.Exports.Lookup(name) != file.Locals.Lookup(name) || file.Symbol.Exports.Lookup(name) == nil {
			t.Errorf("missing export %q", name)
		}
	}
	for _, name := range []string{"hidden", "privateValue", "PrivateConstant", "HiddenProcedure"} {
		if file.Symbol.Exports.Lookup(name) != nil || file.Locals.Lookup(name) == nil {
			t.Errorf("incorrect visibility for %q", name)
		}
	}
}

func TestDeclarationConflicts(t *testing.T) {
	for name, source := range map[string]string{
		"module names":           "Dim Value As Long\nConst vALUE = 2",
		"module procedure":       "Dim F As Long\nFunction F()\nEnd Function",
		"parameter spellings":    "Sub S(value As Long, VALUE As Long)\nEnd Sub",
		"parameter and local":    "Sub S(value As Long)\nDim value As Long\nEnd Sub",
		"constant and parameter": "Sub S(value As Long)\nConst value = 1\nEnd Sub",
		"result and parameter":   "Function F(f As Long)\nEnd Function",
		"result and local":       "Function F()\nDim f As Long\nEnd Function",
		"branches":               "Sub S(condition)\nIf condition Then\nDim value As Long\nElse\nDim VALUE As String\nEnd If\nEnd Sub",
		"type characters":        "Sub S()\nDim value%, VALUE As Long\nEnd Sub",
	} {
		t.Run(name, func(t *testing.T) {
			file := bindDeclarations(t, source)
			if len(file.BindDiagnostics) != 1 || file.BindDiagnostics[0].Code != ast.BindingDuplicateDeclaration || file.BindDiagnostics[0].RelatedNode == nil || !file.BindDiagnostics[0].RelatedLoc.IsValid() {
				t.Fatalf("missing declaration conflict: %+v", file.BindDiagnostics)
			}
		})
	}
}

func TestConstantsAndParameterDefaultsUseConstantSymbols(t *testing.T) {
	file := bindDeclarations(t, "Option Explicit\nConst First = Later + 1, Later = 2\nFunction F(Optional value As Long = First) As Long\nConst LocalConstant = First + 3\nDim rows(0 To LocalConstant) As Long\nF = value + LocalConstant\nEnd Function")
	requireNoBindDiagnostics(t, file)
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	defaultValue := fn.Parameters.Nodes[0].AsParameterDeclaration().Initializer
	if defaultValue.Symbol != file.Locals.Lookup("First") || !ast.IsConstantSymbol(defaultValue.Symbol) || defaultValue.FlowNode != nil {
		t.Fatal("default value was not a compile-time reference to a module constant")
	}
	local := file.FunctionDeclarations.Nodes[0].Locals.Lookup("LocalConstant")
	if !ast.IsConstantSymbol(local) {
		t.Fatal("local constant was not distinguished from a variable")
	}
	first := file.Locals.Lookup("First").ValueDeclaration.AsVariableDeclaration()
	if first.Initializer.AsBinaryExpression().Left.Symbol != file.Locals.Lookup("Later") {
		t.Fatal("forward constant reference did not resolve")
	}
}

func TestInvalidConstantExpressionsAndReadOnlyAssignments(t *testing.T) {
	for name, test := range map[string]struct {
		source string
		code   ast.BindingDiagnosticCode
	}{
		"assign constant":       {"Const Answer = 42\nSub S()\nAnswer = 0\nEnd Sub", ast.BindingAssignmentToConstant},
		"local assign constant": {"Sub S()\nConst Answer = 42\nAnswer = 0\nEnd Sub", ast.BindingAssignmentToConstant},
		"variable initializer":  {"Dim value As Long\nConst Answer = value", ast.BindingConstantExpressionExpected},
		"function initializer":  {"Const Answer = F\nFunction F()\nF = 1\nEnd Function", ast.BindingConstantExpressionExpected},
		"unknown initializer":   {"Const Answer = absent", ast.BindingConstantExpressionExpected},
		"parameter default":     {"Sub S(value, Optional other = value)\nEnd Sub", ast.BindingConstantExpressionExpected},
		"local default":         {"Sub S(Optional value = LocalConstant)\nConst LocalConstant = 1\nEnd Sub", ast.BindingConstantExpressionExpected},
		"variable bound":        {"Dim size As Long, rows(size) As Long", ast.BindingConstantExpressionExpected},
		"Sub value":             {"Function F()\nF = S\nEnd Function\nSub S()\nEnd Sub", ast.BindingInvalidValueReference},
	} {
		t.Run(name, func(t *testing.T) {
			file := bindDeclarations(t, test.source)
			if len(file.BindDiagnostics) != 1 || file.BindDiagnostics[0].Code != test.code {
				t.Fatalf("unexpected diagnostics: %+v", file.BindDiagnostics)
			}
			if file.Locals.Lookup("absent") != nil {
				t.Fatal("constant expression introduced an implicit module variable")
			}
		})
	}
}

func TestSignaturesAndAsNewDoNotExecuteAtTheirDeclarations(t *testing.T) {
	file := bindDeclarations(t, "Option Explicit\nConst DefaultValue = 2\nFunction F(Optional arg As Long = DefaultValue) As Project.Result\nDim value As New Project.Widget\nEnd Function\nSub S(ParamArray args() As Variant)\nEnd Sub")
	requireNoBindDiagnostics(t, file)
	for _, node := range file.FunctionDeclarations.Nodes {
		fn := node.AsFunctionDeclaration()
		if fn.EndFlowNode != node.FlowNode || node.FlowNode.Flags&ast.FlowFlagsStart == 0 {
			t.Fatal("declarations introduced executable assignment or call flows")
		}
		for _, parameter := range fn.Parameters.Nodes {
			if parameter.Symbol == nil || parameter.AsParameterDeclaration().Name.Symbol != parameter.Symbol || parameter.FlowNode != nil {
				t.Fatal("parameter declaration is not connected to its symbol or was executed")
			}
		}
		if node.Locals.Lookup("Project") != nil || node.Locals.Lookup("Result") != nil || node.Locals.Lookup("Widget") != nil {
			t.Fatal("type-space names introduced value symbols")
		}
	}
}

func TestOptionExplicitPlacementAndScope(t *testing.T) {
	for _, source := range []string{"Sub S()\nEnd Sub\nOption Explicit", "Sub S()\nOption Explicit\nunknown = 1\nEnd Sub", "Sub S()\nPublic value As Long\nEnd Sub"} {
		file := parser.ParseSourceFile(source).AsSourceFile()
		if len(file.ParseDiagnostics) == 0 {
			t.Fatal("invalid declaration placement was accepted")
		}
		binder.BindSourceFile(file)
		if strings.Contains(source, "unknown") && file.OptionExplicit {
			t.Fatal("a local Option Explicit affected the whole module")
		}
	}
}

func TestConcurrentBindingOfDeclarationRichFiles(t *testing.T) {
	source := "Option Explicit\nConst Size = 2\nSub S(ByVal arg As Long)\nDim value As Long\nvalue = arg + Size\nEnd Sub"
	files := make([]*ast.SourceFile, 20)
	for i := range files {
		files[i] = parser.ParseSourceFile(source).AsSourceFile()
	}
	var workers sync.WaitGroup
	for _, file := range files {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 3 {
				binder.BindSourceFile(file)
			}
		}()
	}
	workers.Wait()
	for _, file := range files {
		requireNoBindDiagnostics(t, file)
		if !file.OptionExplicit || file.Locals.Lookup("S") == nil {
			t.Fatal("incomplete concurrent binding")
		}
	}
	if files[0].Locals.Lookup("Size") == files[1].Locals.Lookup("Size") {
		t.Fatal("symbol ownership leaked between files")
	}
}
