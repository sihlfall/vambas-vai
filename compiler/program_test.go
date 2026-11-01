package compiler_test

import (
	"strings"
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/compiler"
)

func program(t *testing.T, sources ...compiler.SourceFileInput) *compiler.Program {
	t.Helper()
	p := compiler.NewProgram(compiler.ProgramOptions{ProjectName: "Demo", Sources: sources})
	for _, d := range p.ParseDiagnostics() {
		t.Errorf("%s: %s at %v", d.File.FileName, d.Message, d.Loc)
	}
	if t.Failed() {
		t.FailNow()
	}
	return p
}

func noErrors(t *testing.T, p *compiler.Program) {
	t.Helper()
	for _, d := range p.BindDiagnostics() {
		t.Errorf("%s: %s: %s at %v", d.File.FileName, d.Code, d.Message, d.Loc)
	}
}

func expression(t *testing.T, file *ast.SourceFile, kind ast.Kind, text string) *ast.Node {
	t.Helper()
	var found *ast.Node
	var walk func(*ast.Node) bool
	walk = func(node *ast.Node) bool {
		if node.Kind == kind && strings.TrimSpace(file.Text[node.Pos():node.End()]) == text {
			found = node
			return true
		}
		return node.ForEachChild(walk)
	}
	walk(file.AsNode())
	if found == nil {
		t.Fatalf("expression %q of kind %v not found in %s", text, kind, file.FileName)
	}
	return found
}

func TestProjectForwardReferencesAndQualification(t *testing.T) {
	client := compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef result As Long, Optional maximum As Long = Utilities.Limit)
result = Compute
result = utilities.Compute()
result = DEMO.Utilities.Compute()
result = Demo.Limit
result = Limit
Call Utilities.Reset
End Sub`}
	utilities := compiler.SourceFileInput{FileName: "other.bas", Text: `Attribute VB_Name = "Utilities"
Option Explicit
Option Private Module
Public Const Limit As Long = 10
Public Function Compute() As Long
Compute = Limit
End Function
Public Sub Reset()
End Sub`}
	for _, sources := range [][]compiler.SourceFileInput{{client, utilities}, {utilities, client}} {
		p := program(t, sources...)
		noErrors(t, p)
		main, utils := p.Module("MAIN"), p.Module("utilities")
		if main == nil || utils == nil || !utils.OptionPrivateModule || utils.Symbol.Parent != p.Symbol() || p.Symbol().Exports.Lookup("Utilities") != utils.Symbol {
			t.Fatal("project/module ownership or names are missing")
		}
		compute := utils.Locals.Lookup("compute")
		for _, text := range []string{"utilities.Compute", "DEMO.Utilities.Compute"} {
			node := expression(t, main, ast.KindPropertyAccessExpression, text)
			if node.Symbol != compute || node.AsPropertyAccessExpression().Name.Symbol != compute {
				t.Fatalf("%s did not resolve to the utility function", text)
			}
		}
		body := main.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		if body[0].AsLetStatement().RExpression.Symbol != compute || body[4].AsLetStatement().RExpression.Symbol != utils.Locals.Lookup("Limit") {
			t.Fatal("project-wide names did not resolve")
		}
		parameter := main.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Parameters.Nodes[1].AsParameterDeclaration()
		if parameter.Initializer.Symbol != utils.Locals.Lookup("Limit") || len(main.FunctionDeclarations.Nodes[0].Locals) != 2 {
			t.Fatal("qualified constant default created a local or lost its symbol")
		}
		if p.SourceFile("./MAIN.BAS") != main || p.Name() != "Demo" || !main.IsBound() {
			t.Fatal("program accessors or binding lifecycle are broken")
		}
	}
}

func TestPrivateNamesAndLocalShadowing(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(Helpers As Object, result As Long)
result = Helpers.F
Call Hidden
Call Library.Hidden
Call Library.Missing
End Sub
Sub Own()
End Sub`},
		compiler.SourceFileInput{FileName: "Helpers.bas", Text: `Public Function F() As Long
End Function`},
		compiler.SourceFileInput{FileName: "Library.bas", Text: `Private Sub Hidden()
End Sub`})
	diagnostics := p.BindDiagnostics()
	if len(diagnostics) != 3 || diagnostics[0].Code != ast.BindingUndeclaredName || diagnostics[1].Code != ast.BindingInaccessibleMember || diagnostics[2].Code != ast.BindingMemberNotFound {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	member := expression(t, p.Module("Main"), ast.KindPropertyAccessExpression, "Helpers.F")
	if member.Symbol != nil || member.AsPropertyAccessExpression().Expression.Symbol != p.Module("Main").FunctionDeclarations.Nodes[0].Locals.Lookup("Helpers") {
		t.Fatal("a local Object receiver did not shadow the standard module")
	}
	private := diagnostics[1]
	if private.RelatedFile != p.Module("Library") || private.RelatedNode != p.Module("Library").Locals.Lookup("Hidden").ValueDeclaration || private.RelatedFile.Text[private.RelatedLoc.Pos():private.RelatedLoc.End()] != "Hidden" {
		t.Fatal("cross-file diagnostic did not underline the declaration in its own file")
	}
}

func TestAmbiguousGlobalsRequireQualification(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long)
result = Shared
result = One.Shared
result = Two.Shared
result = Demo.Shared
End Sub`},
		compiler.SourceFileInput{FileName: "One.bas", Text: `Function Shared() As Long
Shared = 1
End Function
Sub Run(result As Long)
result = Shared
End Sub`},
		compiler.SourceFileInput{FileName: "Two.bas", Text: `Function Shared() As Long
End Function`})
	ds := p.BindDiagnostics()
	if len(ds) != 2 || ds[0].Code != ast.BindingAmbiguousName || ds[1].Code != ast.BindingAmbiguousName {
		t.Fatalf("expected ambiguity only at the unqualified uses: %+v", ds)
	}
	main := p.Module("Main")
	if main.FunctionDeclarations.Nodes[0].Locals.Lookup("Shared") != nil {
		t.Fatal("an ambiguous global became an implicit local")
	}
	for _, name := range []string{"One", "Two"} {
		if expression(t, main, ast.KindPropertyAccessExpression, name+".Shared").Symbol != p.Module(name).Locals.Lookup("Shared") {
			t.Fatal("qualified disambiguation failed")
		}
	}
}

func TestDeclaredClassReceiversAndProperties(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByVal item As Demo.Counter, result As Long, objects() As Counter)
item.Value = 1
result = item.Value
With item
.Value = result
result = .Value
End With
result = Make().Value
result = objects(0).Value
Set item = New Demo.Counter
End Sub
Function Make() As Counter
Set Make = New Counter
End Function`},
		compiler.SourceFileInput{FileName: "Counter.cls", Text: `Option Explicit
Private stored As Long
Public Property Get Value() As Long
Value = Me.stored
End Property
Public Property Let Value(ByVal rhs As Long)
Me.stored = rhs
End Property
Public Sub Reset()
Me.Value = 0
End Sub`})
	noErrors(t, p)
	main, counter := p.Module("Main"), p.Module("Counter")
	if counter.ModuleKind != ast.ModuleKindClass || counter.Symbol.Flags != ast.SymbolFlagsClass || counter.Symbol.Members.Lookup("stored") != counter.Locals.Lookup("stored") {
		t.Fatal("class module did not retain its instance members")
	}
	for _, text := range []string{"item.Value", ".Value", "Make().Value", "objects(0).Value"} {
		if expression(t, main, ast.KindPropertyAccessExpression, text).Symbol != counter.Locals.Lookup("Value") {
			t.Fatalf("%s did not resolve the class property", text)
		}
	}
	if expression(t, counter, ast.KindPropertyAccessExpression, "Me.stored").Symbol != counter.Locals.Lookup("stored") || expression(t, counter, ast.KindMeExpression, "Me").Symbol != counter.Symbol {
		t.Fatal("Me did not resolve the class's private members")
	}
	creation := expression(t, main, ast.KindNewExpression, "New Demo.Counter")
	if creation.Symbol != counter.Symbol || creation.AsNewExpression().Expression.AsQualifiedName().Right.Symbol != counter.Symbol {
		t.Fatal("project-qualified New type was not resolved")
	}
	annotation := main.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Parameters.Nodes[0].AsParameterDeclaration().Type.AsTypeReference()
	if annotation.TypeName.Symbol != counter.Symbol {
		t.Fatal("class type annotation was not resolved")
	}
}

func TestClassNamesDoNotLeakInstanceMembers(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(result As Long)
result = Value
result = Counter.Value
result = Me.Value
End Sub`},
		compiler.SourceFileInput{FileName: "Counter.cls", Text: `Public Value As Long`})
	ds := p.BindDiagnostics()
	if len(ds) != 3 || ds[0].Code != ast.BindingUndeclaredName || ds[1].Code != ast.BindingInvalidValueReference || ds[2].Code != ast.BindingInvalidValueReference {
		t.Fatalf("instance-only members/names were not diagnosed: %+v", ds)
	}
}

func TestClassTypesUseSeparateLookupFromLocalValues(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(Counter As Object)
Dim value As New Counter
Dim external As External.Widget
Set value = New Demo.Counter
Set external = New External.Widget
Call Counter.Reset
End Sub`},
		compiler.SourceFileInput{FileName: "Counter.cls", Text: `Sub Reset()
End Sub`})
	noErrors(t, p)
	main := p.Module("Main")
	fn := main.FunctionDeclarations.Nodes[0]
	annotation := fn.Locals.Lookup("value").ValueDeclaration.AsVariableDeclaration().Type.AsTypeReference()
	if annotation.TypeName.Symbol != p.Module("Counter").Symbol || expression(t, main, ast.KindNewExpression, "New Demo.Counter").Symbol != p.Module("Counter").Symbol {
		t.Fatal("a value named Counter shadowed the class type")
	}
	if expression(t, main, ast.KindNewExpression, "New External.Widget").Symbol != nil || expression(t, main, ast.KindPropertyAccessExpression, "Counter.Reset").Symbol != nil || len(fn.Locals) != 3 {
		t.Fatal("external types or dynamic members polluted the value scope")
	}
}

func TestHostModulesAndPredeclaredClasses(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(result As Long)
result = Sheet1.Value
result = Form1.Value
result = Singleton.Value
End Sub`},
		compiler.SourceFileInput{FileName: "Sheet1.cls", ModuleKind: ast.ModuleKindDocument, Text: `Public Value As Long`},
		compiler.SourceFileInput{FileName: "Form1.frm", Text: `Public Value As Long`},
		compiler.SourceFileInput{FileName: "Singleton.cls", Text: `Attribute VB_PredeclaredId = True
Public Value As Long`})
	noErrors(t, p)
	for _, name := range []string{"Sheet1", "Form1", "Singleton"} {
		file := p.Module(name)
		if !file.PredeclaredID || expression(t, p.Module("Main"), ast.KindPropertyAccessExpression, name+".Value").Symbol != file.Locals.Lookup("Value") {
			t.Fatal("predeclared instance did not resolve")
		}
	}
}

func TestQualifiedPropertyVisibilityAndWriteKinds(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(result As Variant, obj As Object)
result = Props.Value
Props.Value = result
Set Props.Value = obj
End Sub`},
		compiler.SourceFileInput{FileName: "Props.bas", Text: `Private Property Get Value() As Variant
End Property
Public Property Let Value(rhs As Variant)
End Property
Private Property Set Value(rhs As Object)
End Property`})
	ds := p.BindDiagnostics()
	if len(ds) != 2 || ds[0].Code != ast.BindingInaccessibleMember || ds[1].Code != ast.BindingInaccessibleMember {
		t.Fatalf("per-accessor visibility was not enforced: %+v", ds)
	}
	if ds[0].RelatedNode.Kind != ast.KindGetAccessor || ds[1].RelatedNode.Kind != ast.KindSetAccessor || ds[0].RelatedFile != p.Module("Props") {
		t.Fatal("accessor diagnostics lost their cross-module declaration")
	}
	flow := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode
	if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Antecedent.Flags&ast.FlowFlagsCall == 0 || flow.Antecedent.Node != flow.Node {
		t.Fatal("qualified setter did not create one call before its write")
	}
}

func TestDefaultInstancesCannotBeSet(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run()
Set Singleton = New Singleton
Set Demo.Singleton = Nothing
End Sub`},
		compiler.SourceFileInput{FileName: "Singleton.cls", Text: `Attribute VB_PredeclaredId = True`})
	ds := p.BindDiagnostics()
	if len(ds) != 2 || ds[0].Code != ast.BindingInvalidAssignmentTarget || ds[1].Code != ast.BindingInvalidAssignmentTarget {
		t.Fatalf("a predeclared default instance was accepted as a Set target: %+v", ds)
	}
}

func TestQualifiedConstantsArraysAndInvalidAssignmentTargets(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Private Const Size = Library.Size
Sub Run(result As Long)
Dim values(Library.Size) As Long
result = Library.Values(0)
Library.Size = 2
Library.F = 3
Library = 4
End Sub`},
		compiler.SourceFileInput{FileName: "Library.bas", Text: `Public Const Size = 3
Public Values(3) As Long
Public Function F() As Long
End Function`})
	ds := p.BindDiagnostics()
	if len(ds) != 4 || ds[0].Code != ast.BindingAssignmentToConstant || ds[1].Code != ast.BindingInvalidAssignmentTarget || ds[2].Code != ast.BindingInvalidValueReference || ds[3].Code != ast.BindingInvalidAssignmentTarget {
		t.Fatalf("unexpected qualified assignment diagnostics: %+v", ds)
	}
	flow := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode
	if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Antecedent.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("qualified array indexing or constant evaluation emitted a call")
	}
}

func TestDuplicateModulesAndFiles(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: `src\One.bas`, Text: `Attribute VB_Name = "Same"`},
		compiler.SourceFileInput{FileName: "./SRC/one.bas", Text: `' A longer prefix in a different file.
Attribute VB_Name = "sAME"`})
	ds := p.BindDiagnostics()
	if len(ds) != 2 || ds[0].Code != ast.BindingDuplicateFile || ds[1].Code != ast.BindingDuplicateModule {
		t.Fatalf("duplicate project inputs were not diagnosed: %+v", ds)
	}
	d := ds[1]
	if d.File == d.RelatedFile || d.File.Text[d.Loc.Pos():d.Loc.End()] != `"sAME"` || d.RelatedFile.Text[d.RelatedLoc.Pos():d.RelatedLoc.End()] != `"Same"` {
		t.Fatal("duplicate module spans used the wrong source text")
	}
	if p.SourceFile("SRC/ONE.BAS") != p.SourceFiles()[0] || p.Module("Same") != p.SourceFiles()[0] {
		t.Fatal("duplicate recovery did not retain the first source")
	}
}

func TestProgramInputsAndSnapshots(t *testing.T) {
	options := compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Original.cls", ModuleName: "HostName", ModuleKind: ast.ModuleKindStandard, Text: `Attribute VB_Name = "ExportName"`}}}
	first := compiler.NewProgram(options)
	second := compiler.NewProgram(options)
	file := first.Module("HostName")
	if first.Name() != "Project" || file == nil || file.ModuleKind != ast.ModuleKindStandard || file == second.Module("HostName") || file.Symbol == second.Module("HostName").Symbol {
		t.Fatal("programs shared mutable trees or ignored explicit host metadata")
	}
	files := first.SourceFiles()
	files[0] = nil
	if first.SourceFiles()[0] != file {
		t.Fatal("SourceFiles exposed the program's owned slice")
	}
	empty := compiler.NewProgram(compiler.ProgramOptions{})
	if len(empty.SourceFiles()) != 0 || len(empty.BindDiagnostics()) != 0 || empty.Module("missing") != nil {
		t.Fatal("empty project failed")
	}
}

func TestProgramConstructionAndCompletedTreesAreConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
				{FileName: "Main.bas", Text: "Option Explicit\nSub Run()\nCall Library.Work\nEnd Sub"},
				{FileName: "Library.bas", Text: "Sub Work()\nEnd Sub"},
			}})
			if len(p.BindDiagnostics()) != 0 {
				t.Error(p.BindDiagnostics())
			}
			for _, file := range p.SourceFiles() {
				binder.BindSourceFile(file)
			}
		}()
	}
	wg.Wait()
}

func TestQualifiedInvocationFlows(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(result As Long)
result = Library.Value
Library.Value = result
Call Library.Work(Argument())
result = Library.Values(0)
End Sub
Function Argument() As Long
End Function`},
		compiler.SourceFileInput{FileName: "Library.bas", Text: `Public Values(3) As Long
Public Property Get Value() As Long
End Property
Public Property Let Value(rhs As Long)
End Property
Sub Work(value As Long)
End Sub`})
	noErrors(t, p)
	flow := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode
	calls, assignments := 0, 0
	for ; flow != nil; flow = flow.Antecedent {
		if flow.Flags&ast.FlowFlagsCall != 0 {
			calls++
		}
		if flow.Flags&ast.FlowFlagsAssignment != 0 {
			assignments++
		}
	}
	if calls != 4 || assignments != 3 {
		t.Fatalf("cross-module calls were evaluated more than once: %d calls, %d writes", calls, assignments)
	}
}

func TestUnqualifiedGlobalsDoNotBecomeImplicitLocals(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run()
total = COUNT
Call Work
End Sub`},
		compiler.SourceFileInput{FileName: "Library.bas", Text: `Public count As Long
Public Sub Work()
End Sub`})
	noErrors(t, p)
	fn := p.Module("Main").FunctionDeclarations.Nodes[0]
	if len(fn.Locals) != 1 || fn.Locals.Lookup("total") == nil || expression(t, p.Module("Main"), ast.KindIdentifier, "COUNT").Symbol != p.Module("Library").Locals.Lookup("count") {
		t.Fatal("project declarations were mistaken for implicit procedure locals")
	}
}

func TestInvalidModuleIdentity(t *testing.T) {
	for _, input := range []compiler.SourceFileInput{
		{FileName: "bad-name.bas"}, {ModuleName: "1Invalid"},
		{ModuleName: strings.Repeat("A", 32)}, {ModuleName: "Demo"},
		{ModuleKind: ast.ModuleKind(255)},
	} {
		p := program(t, input)
		if ds := p.BindDiagnostics(); len(ds) != 1 || ds[0].Code != ast.BindingInvalidModule {
			t.Fatalf("invalid module identity was accepted: %+v: %+v", input, ds)
		}
	}
}

func FuzzProgramBinding(f *testing.F) {
	f.Add("Option Explicit\nSub Run()\nCall Library.Work\nEnd Sub", "Sub Work()\nEnd Sub")
	f.Add("Sub Run(item As Library)\nWith item\n.Value = 1\nEnd With\nEnd Sub", "Attribute VB_Name = \"Library\"\nProperty Let Value(rhs As Long)\nEnd Property")
	f.Add("Private Const N = Library.Size", "Public Const Size = 3")
	f.Add("Attribute VB_Name =", "Attribute VB_PredeclaredId = True")
	f.Fuzz(func(t *testing.T, main, library string) {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
			{FileName: "Main.bas", Text: main}, {FileName: "Library.cls", Text: library},
		}})
		for _, file := range p.SourceFiles() {
			if !file.IsBound() || file.Symbol.Parent != p.Symbol() {
				t.Fatal("incomplete project tree")
			}
		}
		for _, d := range p.BindDiagnostics() {
			if d.File == nil || !d.Loc.IsValid() || d.Loc.Pos() < 0 || d.Loc.End() > len(d.File.Text) {
				t.Fatal("invalid primary diagnostic location")
			}
			if d.RelatedNode != nil && (d.RelatedFile == nil || !d.RelatedLoc.IsValid() || d.RelatedLoc.Pos() < 0 || d.RelatedLoc.End() > len(d.RelatedFile.Text)) {
				t.Fatal("invalid cross-file diagnostic location")
			}
		}
	})
}
