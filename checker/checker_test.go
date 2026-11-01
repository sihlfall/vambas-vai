package checker_test

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func program(t *testing.T, sources ...compiler.SourceFileInput) *compiler.Program {
	t.Helper()
	p := compiler.NewProgram(compiler.ProgramOptions{ProjectName: "Demo", Sources: sources})
	if ds := p.ParseDiagnostics(); len(ds) != 0 {
		for _, d := range ds {
			t.Errorf("%s:%d: %s", d.File.FileName, d.Pos(), d.Message)
		}
		t.FailNow()
	}
	if ds := p.BindDiagnostics(); len(ds) != 0 {
		t.Fatalf("bind errors: %+v", ds)
	}
	return p
}

func noErrors(t *testing.T, c *checker.Checker) {
	t.Helper()
	for _, d := range c.GetDiagnostics(nil) {
		t.Errorf("%s:%d: %s: %s", d.File.FileName, d.Pos(), d.Code, d.Message)
	}
}

func find(t *testing.T, file *ast.SourceFile, kind ast.Kind, text string) *ast.Node {
	t.Helper()
	var result *ast.Node
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == kind && strings.TrimSpace(file.Text[n.Pos():n.End()]) == text {
			result = n
			return true
		}
		return n.ForEachChild(visit)
	}
	visit(file.AsNode())
	if result == nil {
		t.Fatalf("%q (kind %v) not found in %s", text, kind, file.FileName)
	}
	return result
}

func requireType(t *testing.T, c *checker.Checker, node *ast.Node, kind checker.TypeKind) {
	t.Helper()
	if got := c.GetTypeAtLocation(node); got.Kind != kind {
		t.Fatalf("node %v: got %s, want kind %v", node.Kind, got, kind)
	}
}

func TestTwoModuleSemanticQueries(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef result As Long)
result = Utilities.Add(1, 2)
End Sub`},
		compiler.SourceFileInput{FileName: "Utilities.bas", Text: `Option Explicit
Public Function Add(ByVal a As Long, ByVal b As Long) As Long
Add = a + b
End Function`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	main, utils := p.Module("Main"), p.Module("Utilities")
	call := find(t, main, ast.KindCallExpression, "Utilities.Add(1, 2)")
	member := call.AsCallExpression().Expression
	declaration := utils.Locals.Lookup("Add").ValueDeclaration
	requireType(t, c, call, checker.TypeLong)
	if c.GetSymbolAtLocation(member) != declaration.Symbol || c.GetSymbolAtLocation(member.AsPropertyAccessExpression().Name) != declaration.Symbol {
		t.Fatal("cross-module member symbol queries disagree")
	}
	sig := c.GetResolvedSignature(call)
	if sig != c.GetSignatureFromDeclaration(declaration) || sig.ReturnType.Kind != checker.TypeLong || len(sig.Parameters) != 2 || sig.Parameters[0].IsByRef() {
		t.Fatal("procedure signature was not resolved")
	}
	fn := declaration.AsFunctionDeclaration()
	if c.GetTypeOfSymbol(fn.ResultSymbol).Kind != checker.TypeLong || c.GetTypeOfSymbol(declaration.Symbol).Kind != checker.TypeProcedure {
		t.Fatal("function result storage was confused with the procedure")
	}
	assignment := main.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0]
	conversion := c.GetAssignmentConversion(assignment)
	if conversion.Kind != checker.ConversionIdentity || conversion.From.Kind != checker.TypeLong || conversion.To.Kind != checker.TypeLong {
		t.Fatal("assignment did not retain its semantic conversion")
	}
}

func TestDeclaredScalarsHintsAndDefaults(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Dim flag As Boolean, octet As Byte, small As Integer, count As Long
Dim large As LongLong, pointer As LongPtr, real As Single, precise As Double
Dim money As Currency, day As Date, text As String, anything As Variant, obj As Object
Dim inferred, a%, b&, c^, d!, e#, f@, g$
Sub Run(ByVal value As Long, Optional other As String = "text")
End Sub
Sub Many(ParamArray rest() As Variant)
End Sub
Function TextResult$()
TextResult = "value"
End Function`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	cases := map[string]checker.TypeKind{
		"flag": checker.TypeBoolean, "octet": checker.TypeByte, "small": checker.TypeInteger, "count": checker.TypeLong,
		"large": checker.TypeLongLong, "pointer": checker.TypeLongPtr, "real": checker.TypeSingle, "precise": checker.TypeDouble,
		"money": checker.TypeCurrency, "day": checker.TypeDate, "text": checker.TypeString, "anything": checker.TypeVariant,
		"obj": checker.TypeObject, "inferred": checker.TypeVariant, "a": checker.TypeInteger, "b": checker.TypeLong,
		"c": checker.TypeLongLong, "d": checker.TypeSingle, "e": checker.TypeDouble, "f": checker.TypeCurrency, "g": checker.TypeString,
	}
	for name, want := range cases {
		if got := c.GetTypeOfSymbol(file.Locals.Lookup(name)); got.Kind != want {
			t.Errorf("%s: got %s, want kind %v", name, got, want)
		}
	}
	sig := c.GetSignatureFromDeclaration(file.FunctionDeclarations.Nodes[0])
	if sig.ReturnType.Kind != checker.TypeVoid || sig.Parameters[0].IsByRef() || !sig.Parameters[1].IsByRef() || sig.Parameters[1].Default == nil {
		t.Fatal("parameter and Sub signature details were lost")
	}
	variadic := c.GetSignatureFromDeclaration(file.FunctionDeclarations.Nodes[1]).Parameters[0]
	if variadic.Type.Kind != checker.TypeArray || variadic.Type.ElementType.Kind != checker.TypeVariant || variadic.IsByRef() {
		t.Fatal("ParamArray signature details were lost")
	}
	if c.GetSignatureFromDeclaration(file.FunctionDeclarations.Nodes[2]).ReturnType.Kind != checker.TypeString {
		t.Fatal("function type character was ignored")
	}
}

func TestLiteralAndConstantTypes(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Public Const First = Other.Later
Const Small = 32767, Large = 32768, Huge = 2147483648, Real = 1.5, Scientific = 1D2
Const HexSmall = &HFFFF, HexLarge = &HFFFFFFFF, Octal = &O177777
Const Text = "hello", Flag = True, EmptyValue = Empty, NullValue = Null
Const Typed As Currency = 2, Hint$ = "hint"`},
		compiler.SourceFileInput{FileName: "Other.bas", Text: `Public Const Later = "later"`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	for name, want := range map[string]checker.TypeKind{
		"First": checker.TypeString, "Small": checker.TypeInteger, "Large": checker.TypeLong,
		"Huge": checker.TypeDouble, "Real": checker.TypeDouble, "Scientific": checker.TypeDouble,
		"HexSmall": checker.TypeInteger, "HexLarge": checker.TypeLong, "Octal": checker.TypeInteger,
		"Text": checker.TypeString, "Flag": checker.TypeBoolean, "EmptyValue": checker.TypeVariant,
		"NullValue": checker.TypeVariant, "Typed": checker.TypeCurrency, "Hint": checker.TypeString,
	} {
		if got := c.GetTypeOfSymbol(file.Locals.Lookup(name)); got.Kind != want {
			t.Errorf("%s: got %s, want kind %v", name, got, want)
		}
	}
}

func TestScalarOperatorTables(t *testing.T) {
	source := `Option Explicit
Sub Run(b As Byte, i As Integer, l As Long, ll As LongLong, s As Single, d As Double, c As Currency, dt As Date, text As String, flag As Boolean, v As Variant, result As Variant)
`
	cases := []struct {
		expression string
		want       checker.TypeKind
	}{
		{"b + b", checker.TypeByte}, {"b + i", checker.TypeInteger}, {"i + l", checker.TypeLong},
		{"ll + i", checker.TypeLongLong}, {"s + i", checker.TypeSingle}, {"s + l", checker.TypeDouble},
		{"c + d", checker.TypeCurrency}, {"dt + i", checker.TypeDate}, {"dt - dt", checker.TypeDouble}, {"text + text", checker.TypeString},
		{"text + i", checker.TypeDouble}, {"c * s", checker.TypeDouble}, {"c * i", checker.TypeCurrency},
		{"dt * i", checker.TypeDouble}, {"s / i", checker.TypeSingle}, {"i / i", checker.TypeDouble},
		{"c / i", checker.TypeDouble}, {`d \ i`, checker.TypeLong}, {"d Mod ll", checker.TypeLong},
		{"i ^ i", checker.TypeDouble}, {"text & i", checker.TypeString},
		{"i < l", checker.TypeBoolean}, {"v < i", checker.TypeVariant}, {"v + i", checker.TypeVariant},
		{"v & text", checker.TypeVariant}, {"flag And flag", checker.TypeBoolean}, {"flag And i", checker.TypeInteger},
		{"b Or b", checker.TypeByte}, {"d Xor flag", checker.TypeLong}, {"d Imp ll", checker.TypeLongLong},
		{"Not flag", checker.TypeBoolean}, {"Not s", checker.TypeLong}, {"-b", checker.TypeInteger},
		{"-text", checker.TypeDouble}, {"Not v", checker.TypeVariant},
	}
	for _, test := range cases {
		source += "result = " + test.expression + "\n"
	}
	source += "End Sub"
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: source})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	for i, test := range cases {
		if got := c.GetTypeAtLocation(body[i].AsLetStatement().RExpression); got.Kind != test.want {
			t.Errorf("%s: got %s, want kind %v", test.expression, got, test.want)
		}
	}
}

func TestConversionsPreserveRuntimeCoercions(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(n As Long, text As String, v As Variant)
n = text
n = "not a number"
v = n
n = v
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	for _, statement := range p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes {
		conversion := c.GetAssignmentConversion(statement)
		if conversion == nil || conversion.Kind != checker.ConversionLet {
			t.Fatal("a potentially failing runtime Let-coercion was rejected or erased")
		}
	}
}

func TestInvalidScalarAssignmentsAndOperands(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(n As Long, obj As Object, values() As Long)
Set n = obj
Set obj = n
n = values + 1
n = Not values
n = n Is obj
End Sub`})
	c := p.GetTypeChecker()
	ds := c.GetDiagnostics(nil)
	if len(ds) != 5 {
		t.Fatalf("got %d diagnostics: %+v", len(ds), ds)
	}
	for i, d := range ds {
		want := checker.DiagnosticInvalidOperand
		if i < 2 {
			want = checker.DiagnosticTypeMismatch
		}
		if string(d.Code) != string(want) {
			t.Errorf("%d: %s, want %s", i, d.Code, want)
		}
	}
}

func TestClassesArraysAndSeparateTypeLookup(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(Counter As Object, item As Demo.Counter, result As Long, items() As Counter)
Set item = New Demo.Counter
result = item.Value
result = items(0).Value
result = item Is Nothing
End Sub`},
		compiler.SourceFileInput{FileName: "Counter.cls", Text: `Private stored As Long
Property Get Value() As Long
Value = stored
End Property`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	main, class := p.Module("Main"), p.Module("Counter")
	fn := main.FunctionDeclarations.Nodes[0]
	if typ := c.GetTypeOfSymbol(fn.Locals.Lookup("item")); typ.Kind != checker.TypeClass || typ.Symbol != class.Symbol {
		t.Fatal("class annotation resolved through the value namespace")
	}
	creation := find(t, main, ast.KindNewExpression, "New Demo.Counter")
	if c.GetTypeAtLocation(creation).Symbol != class.Symbol {
		t.Fatal("New class type not resolved")
	}
	array := c.GetTypeOfSymbol(fn.Locals.Lookup("items"))
	if array.Kind != checker.TypeArray || array.ElementType.Symbol != class.Symbol {
		t.Fatal("class array type not retained")
	}
	requireType(t, c, find(t, main, ast.KindCallExpression, "items(0)"), checker.TypeClass)
	requireType(t, c, find(t, main, ast.KindPropertyAccessExpression, "item.Value"), checker.TypeLong)
	requireType(t, c, find(t, main, ast.KindBinaryExpression, "item Is Nothing"), checker.TypeBoolean)
}

func TestPropertyTypesUseSelectedAccessor(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long, obj As Object)
result = Props.Value
Props.Value = "input"
Set Props.Value = obj
End Sub`},
		compiler.SourceFileInput{FileName: "Props.bas", Text: `Property Get Value() As Long
End Property
Property Let Value(rhs As String)
End Property
Property Set Value(rhs As Object)
End Property`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	body := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	for i, kind := range []ast.Kind{ast.KindGetAccessor, ast.KindLetAccessor, ast.KindSetAccessor} {
		node := body[i].AsLetStatement().LExpression
		if i == 0 {
			node = body[i].AsLetStatement().RExpression
		}
		if sig := c.GetResolvedSignature(node); sig == nil || sig.Declaration.Kind != kind {
			t.Fatal("selected property signature is wrong")
		}
	}
	if c.GetAssignmentConversion(body[1]).To.Kind != checker.TypeString || c.GetAssignmentConversion(body[2]).Kind != checker.ConversionSet {
		t.Fatal("property write used the getter return type")
	}
}

func TestNestedApplicationsIndexFunctionResult(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long)
result = Values()(0)
End Sub
Function Values() As Long()
End Function`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	inner := find(t, file, ast.KindCallExpression, "Values()")
	outer := find(t, file, ast.KindCallExpression, "Values()(0)")
	requireType(t, c, inner, checker.TypeArray)
	requireType(t, c, outer, checker.TypeLong)
	if c.GetResolvedSignature(inner) == nil || c.GetResolvedSignature(outer) != nil {
		t.Fatal("indexing was mistaken for a second procedure invocation")
	}
}

func TestProcedureReferencesPreserveInvocationContext(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long)
result = Compute
result = Compute()
Call Accept(AddressOf Compute)
Call Accept(ByVal True)
End Sub
Function Compute() As Long
End Function
Sub Accept(callback As Variant)
End Sub`})
	c := p.GetTypeChecker()
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || ds[0].Code != checker.DiagnosticInvalidArgument {
		t.Fatalf("internal call-site ByVal was not diagnosed: %+v", ds)
	}
	body := p.Module("Main").FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	bare := body[0].AsLetStatement().RExpression
	call := body[1].AsLetStatement().RExpression
	address := body[2].AsCallStatement().Expression.AsCallExpression().Arguments.Nodes[0]
	requireType(t, c, bare, checker.TypeLong)
	requireType(t, c, call, checker.TypeLong)
	requireType(t, c, call.AsCallExpression().Expression, checker.TypeProcedure)
	requireType(t, c, address.AsPrefixUnaryExpression().Operand, checker.TypeProcedure)
	requireType(t, c, body[3].AsCallStatement().Expression.AsCallExpression().Arguments.Nodes[0], checker.TypeBoolean)
	if c.GetResolvedSignature(bare) == nil || c.GetResolvedSignature(address.AsPrefixUnaryExpression().Operand) != nil {
		t.Fatal("AddressOf was confused with a bare function invocation")
	}
}

func TestUnknownErrorAndVariantStayDistinct(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Dim typed As External.Library.Widget
Sub Run(obj As Object, result As Variant)
result = obj.Member
End Sub`})
	c := p.GetTypeChecker()
	file := p.Module("Main")
	member := find(t, file, ast.KindPropertyAccessExpression, "obj.Member")
	requireType(t, c, member, checker.TypeVariant)
	if resolution := c.GetCallResolution(member); resolution == nil || !resolution.Deferred {
		t.Fatal("a late-bound member was treated as fully resolved")
	}
	if c.GetTypeAtLocation(nil).Kind != checker.TypeUnknown {
		t.Fatal("unknown query type was lost")
	}
	if c.GetTypeOfSymbol(file.FunctionDeclarations.Nodes[0].Locals.Lookup("result")).Kind != checker.TypeVariant || c.GetTypeOfSymbol(file.Locals.Lookup("typed")).Kind != checker.TypeError {
		t.Fatal("Variant, unknown and erroneous types were conflated")
	}
	if ds := c.GetDiagnostics(nil); len(ds) != 1 || string(ds[0].Code) != string(checker.DiagnosticUnknownType) {
		t.Fatalf("unknown type diagnostics: %+v", ds)
	}
}

func TestTypedAndUntypedConstantCycles(t *testing.T) {
	for _, annotation := range []string{"", " As Long"} {
		p := program(t,
			compiler.SourceFileInput{FileName: "One.bas", Text: "Public Const A" + annotation + " = Two.B"},
			compiler.SourceFileInput{FileName: "Two.bas", Text: "Public Const B" + annotation + " = One.A"})
		c := p.GetTypeChecker()
		if ds := c.GetDiagnostics(nil); len(ds) != 1 || string(ds[0].Code) != string(checker.DiagnosticCircularConstant) {
			t.Fatalf("cycle diagnostics: %+v", ds)
		}
		for _, file := range p.SourceFiles() {
			if got := c.GetTypeOfSymbol(file.Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].Symbol); got.Kind != checker.TypeError {
				t.Fatal("cycle did not produce an error type")
			}
		}
	}
}

func TestConflictingHintsAndAsNewScalar(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Dim x$ As Long, obj As New Object`})
	ds := p.GetSemanticDiagnostics(nil)
	if len(ds) != 2 || string(ds[0].Code) != string(checker.DiagnosticConflictingType) || string(ds[1].Code) != string(checker.DiagnosticInvalidNew) {
		t.Fatalf("declaration diagnostics: %+v", ds)
	}
}

func TestConstantTypesCannotBeObjectsOrArrays(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Const Obj As Object = Nothing
Const Values As Long() = 1`})
	ds := p.GetSemanticDiagnostics(nil)
	if len(ds) != 2 || ds[0].Code != checker.DiagnosticInvalidConstantType || ds[1].Code != checker.DiagnosticInvalidConstantType {
		t.Fatalf("invalid constant types were accepted: %+v", ds)
	}
}

func TestDiagnosticsHaveCorrectFileSpansAndStableQueries(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "One.bas", Text: "Dim value As Missing"},
		compiler.SourceFileInput{FileName: "Two.bas", Text: "' Longer prefix\nDim value As AnotherMissing"})
	c := p.GetTypeChecker()
	before := c.GetDiagnostics(nil)
	if len(before) != 2 || len(p.GetSemanticDiagnostics(p.Module("Two"))) != 1 {
		t.Fatal("diagnostics lost file ownership")
	}
	for _, d := range before {
		if d.Node.Kind != ast.KindIdentifier || d.File.Text[d.Pos():d.End()] != d.Node.AsIdentifier().Text {
			t.Fatal("diagnostic spans include trivia or refer to another file")
		}
	}
	c.GetTypeAtLocation(nil)
	c.GetTypeOfSymbol(nil)
	c.GetResolvedSignature(nil)
	c.GetSymbolAtLocation(nil)
	if !reflect.DeepEqual(before, c.GetDiagnostics(nil)) {
		t.Fatal("semantic queries changed diagnostics")
	}
	before[0].Message = "mutated copy"
	if c.GetDiagnostics(nil)[0].Message == "mutated copy" {
		t.Fatal("diagnostics exposed their owned slice")
	}
}

func TestCheckerDoesNotMutateBoundTreesAndSupportsConcurrentQueries(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Sub Run(result As Long)
result = 1 + 2
End Sub`})
	type metadata struct {
		symbol *ast.Symbol
		flow   *ast.FlowNode
		flags  ast.NodeFlags
	}
	before := make(map[*ast.Node]metadata)
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		before[n] = metadata{n.Symbol, n.FlowNode, n.Flags}
		return n.ForEachChild(visit)
	}
	visit(p.Module("Main").AsNode())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := p.GetTypeChecker()
			for node := range before {
				c.GetTypeAtLocation(node)
				c.GetSymbolAtLocation(node)
				c.GetResolvedSignature(node)
			}
			if len(c.GetDiagnostics(nil)) != 0 {
				t.Error("unexpected diagnostics")
			}
		}()
	}
	wg.Wait()
	for node, old := range before {
		if old != (metadata{node.Symbol, node.FlowNode, node.Flags}) {
			t.Fatal("checker mutated binding metadata")
		}
	}
	other := checker.NewChecker(p)
	first := p.GetTypeChecker()
	if first != p.GetTypeChecker() || first == other || first.GetTypeOfSymbol(p.Module("Main").Symbol) == other.GetTypeOfSymbol(p.Module("Main").Symbol) {
		t.Fatal("checker ownership or Program lifecycle is incorrect")
	}
}

func FuzzCheckRecoveredProjects(f *testing.F) {
	f.Add("Sub Run()\n Dim n As Long\n Select Case n\n Case 1 To 3, Is > 7\n On n GoTo First, 100\n Case Else\n End Select\nFirst: Exit Sub\n100: Exit Sub\nEnd Sub", "")
	f.Add("Sub Run()\n Select Case\n Case Is,\n On GoTo ,\nEnd Sub", "")
	f.Add("Sub Run(ByRef s As String)\n Mid$(s,1,2)=Mid$(s,Start:=2)\nEnd Sub", "")
	f.Add("Sub Run()\n Mid$(1,)=\nEnd Sub", "")
	f.Add("Option Explicit\nSub Run(result As Long)\nresult = Library.Add(1, 2)\nEnd Sub", "Function Add(a As Long, b As Long) As Long\nAdd = a + b\nEnd Function")
	f.Add("Public Const A As Long = Library.B", "Public Const B As Long = Main.A")
	f.Add("Sub Run(item As Library)\nitem.Value = 1\nEnd Sub", "Property Let Value(rhs As Long)\nEnd Property")
	f.Add("Function Broken(As\nvalue =\nEnd Function", "Attribute VB_Name =")
	f.Add("Dim obj As External.Library.Widget", "Dim arr(1 To 3) As Long")
	f.Add("Sub Run(obj As Library, n As Long)\nWith obj\nCall obj.Update(.Value, n)\nobj.Update .Value, n\n.Update .Value, n\nEnd With\nEnd Sub", "Public Value As Long\nSub Update(ByVal first As Long, ByVal second As Long)\nEnd Sub")
	f.Add("Option Explicit\nEnum Errors\n Base=vbObjectError+3000\n NextError=Base+1\nEnd Enum\nSub Run(ByVal n As Long)\n ReDim values(0 To n) As Long\n ReDim Preserve values(0 To n+1), other(n)\nEnd Sub", "Private Type State\n Buffer() As Long\nEnd Type\nSub Run(ByRef state As State,n As Long)\n With state\n ReDim .Buffer(0 To n)\n End With\nEnd Sub")
	f.Add("Sub Broken()\n ReDim Preserve .(, To\nEnd Sub", "")
	f.Add("Type Point\n X As Long\n Children() As Point\nEnd Type\nEnum E\n A = &HFFFF&\n B\nEnd Enum\nSub Run()\n Dim p As Point\n p.X=E.B\nEnd Sub", "Private Type State\n Value As Long\nEnd Type")
	f.Add("Enum E\n A = B\n B = A\nEnd Enum\nType T\n Self As T\nEnd Type", "Enum Broken\n Value=\nSub NextOne()\nEnd Sub")
	f.Add("Sub Run(obj As Library, n As Long)\nobj!Key = n\nn = obj(\"Key\")\nEnd Sub", "Property Get Item(ByVal key As String) As Long\nAttribute Item.VB_UserMemId = 0\nEnd Property\nProperty Let Item(ByVal key As String, ByVal rhs As Long)\nEnd Property")
	f.Add("Sub Run(obj As Library, n As Long)\nn = obj\nEnd Sub", "Property Get Value() As Library\nAttribute Value.VB_UserMemId = 0\nEnd Property")
	f.Add("Sub Run(n As Long)\nCall Target(value:=n)\nCall Target((n))\nEnd Sub\nSub Target(ByRef value As Long)\nEnd Sub", "")
	f.Fuzz(func(t *testing.T, main, library string) {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
			{FileName: "Main.bas", Text: main}, {FileName: "Library.cls", Text: library},
		}})
		c := p.GetTypeChecker()
		for _, file := range p.SourceFiles() {
			var visit func(*ast.Node) bool
			visit = func(n *ast.Node) bool {
				if c.GetTypeAtLocation(n) == nil {
					t.Fatal("query returned a nil type")
				}
				if got, bound := c.GetSymbolAtLocation(n), n.Symbol; bound != nil && got != bound {
					t.Fatal("checker changed a bound symbol")
				}
				c.GetResolvedSignature(n)
				c.GetStorageAtLocation(n)
				c.GetReDimResolution(n)
				c.GetValueResolutions(n)
				if resolution := c.GetCallResolution(n); resolution != nil && resolution.ResultType == nil {
					t.Fatal("call resolution has no result type")
				}
				return n.ForEachChild(visit)
			}
			visit(file.AsNode())
		}
		for _, d := range c.GetDiagnostics(nil) {
			if d.Node == nil || d.File == nil || !d.Loc.IsValid() || d.End() > len(d.File.Text) || ast.SourceFileOf(d.Node) != d.File {
				t.Fatal("invalid semantic diagnostic ownership/span")
			}
		}
	})
}
