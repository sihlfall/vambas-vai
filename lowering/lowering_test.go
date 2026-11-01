package lowering_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func lower(t *testing.T, sources ...compiler.SourceFileInput) *ir.Program {
	t.Helper()
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: sources})
	result, diagnostics := lowering.Lower(p)
	if len(diagnostics) != 0 {
		t.Fatalf("lowering: %+v", diagnostics)
	}
	if diagnostics := ir.Verify(result); len(diagnostics) != 0 {
		t.Fatalf("verification: %+v", diagnostics)
	}
	// Repeated lowering must not mutate checked syntax, symbols, or flow metadata.
	second, diagnostics := lowering.Lower(p)
	if len(diagnostics) != 0 || ir.Format(second) != ir.Format(result) {
		t.Fatalf("nondeterministic lowering: %+v", diagnostics)
	}
	return result
}

func module(text string) compiler.SourceFileInput {
	return compiler.SourceFileInput{FileName: "Main.bas", Text: text}
}

func TestByRefArithmeticAndResultStorage(t *testing.T) {
	p := lower(t, module(`Option Explicit
Public Function Bump(ByRef value As Long) As Long
    value = value + 1
    Bump = value
End Function
`))
	f := p.Procedures[0]
	if len(f.Parameters) != 1 || !f.Slots[0].ByRef || f.Slots[f.ResultSlot-1].Kind != ir.FunctionResult {
		t.Fatal("missing parameter alias or function result storage")
	}
	code := f.Statements[0].Instructions
	if len(code) != 6 || code[2].Op != ir.OpConvert || code[2].Type.Scalar != ir.Long || code[3].Op != ir.OpAdd || code[3].Type.Form != ir.Result || code[4].Op != ir.OpTry || code[4].OnError != ir.Propagate || code[5].Op != ir.OpStore {
		t.Fatal(ir.Format(p))
	}
	if f.Effects != ir.MayRaise|ir.WritesStorage || f.ReturnType != (ir.Type{Scalar: ir.Long}) {
		t.Fatal("effects changed language return type")
	}
	value := int64(41)
	result, failed := execute(t, p, 1, []argument{{cell: &value}})
	if failed || value != 42 || result != 42 {
		t.Fatal(value, result, failed)
	}
	value = 2147483647
	_, failed = execute(t, p, 1, []argument{{cell: &value}})
	if !failed || value != 2147483647 {
		t.Fatal("overflow committed pending ByRef assignment", value)
	}
}

func TestFailurePreservesEarlierEffectsAndSkipsCallerCommit(t *testing.T) {
	p := lower(t, module(`Option Explicit
Sub Run(ByRef hits As Long, ByRef result As Long)
    result = BumpThenFail(hits)
End Sub
Function BumpThenFail(ByRef hits As Long) As Long
    Dim limit As Long
    hits = hits + 1
    limit = 2147483647
    BumpThenFail = limit + 1
End Function
`))
	hits, result := int64(0), int64(11)
	_, failed := execute(t, p, 1, []argument{{cell: &hits}, {cell: &result}})
	if !failed || hits != 1 || result != 11 {
		t.Fatalf("failed call rolled back side effects or committed assignment: hits=%d result=%d failure=%v", hits, result, failed)
	}
}

func TestCallAliasesTemporariesAndSourceOrder(t *testing.T) {
	p := lower(t, module(`Option Explicit
Sub Run(ByRef value As Long)
    Change value, value
    Change (value), (value)
End Sub
Sub Change(ByRef a As Long, ByRef b As Long)
    a = a + 1
    b = b + 1
End Sub
Function Named(ByRef counter As Long) As Long
    Named = Combine(b:=Bump(counter), a:=Bump(counter))
End Function
Function Bump(ByRef value As Long) As Long
    value = value + 1
    Bump = value
End Function
Function Combine(ByVal a As Long, ByVal b As Long) As Long
    Combine = a * 10 + b
End Function
`))
	value := int64(5)
	_, failed := execute(t, p, 1, []argument{{cell: &value}})
	if failed || value != 7 {
		t.Fatal("alias identity or parenthesized temporary lost", value, failed)
	}
	value = 0
	result, failed := execute(t, p, 3, []argument{{cell: &value}})
	if failed || result != 21 || value != 2 {
		t.Fatal("named arguments were evaluated in parameter order", result, value, failed)
	}
}

func TestConversionsDefaultResultAndExplicitExit(t *testing.T) {
	p := lower(t, module(`Option Explicit
Function Narrow(ByVal value As Long) As Integer
    Narrow = value
End Function
Function DefaultResult() As Long
    Exit Function
    DefaultResult = 99
End Function
Sub SetValue(ByVal value As Long)
    value = 99
End Sub
`))
	for _, test := range []struct {
		value  int64
		failed bool
	}{{32767, false}, {32768, true}, {-32768, false}, {-32769, true}} {
		result, failed := execute(t, p, 1, []argument{{value: test.value}})
		if failed != test.failed || !failed && result != test.value {
			t.Fatal(test, result, failed)
		}
	}
	result, failed := execute(t, p, 2, nil)
	if failed || result != 0 {
		t.Fatal("Exit did not return default result", result, failed)
	}
	value := int64(7)
	execute(t, p, 3, []argument{{value: value}})
	if value != 7 {
		t.Fatal("ByVal parameter aliased caller")
	}
}

func TestForwardCrossModuleCallsEnumsAndColonRegions(t *testing.T) {
	p := lower(t,
		module("Function Answer() As Long\n Answer = Utilities.GetAnswer()\nEnd Function\n"),
		compiler.SourceFileInput{FileName: "Utilities.bas", Text: "Enum Values\n First = 41\nEnd Enum\nFunction GetAnswer() As Long\n Dim value As Long: value = Values.First: GetAnswer = value + 1\nEnd Function\n"})
	result, failed := execute(t, p, 1, nil)
	if failed || result != 42 {
		t.Fatal(result, failed)
	}
	regions := p.Procedures[1].Statements
	if len(regions) != 4 || regions[0].ID == regions[1].ID || regions[0].Source.Range.End() > regions[1].Source.Range.Pos() {
		t.Fatal("colon-separated statements share a recovery region")
	}
}

func TestRecursiveEffectPropagation(t *testing.T) {
	p := lower(t, module("Sub A(ByRef x As Long)\n B x\nEnd Sub\nSub B(ByRef x As Long)\n A x\n x = 1\nEnd Sub\n"))
	for _, f := range p.Procedures {
		if f.Effects != ir.MayRaise|ir.WritesStorage {
			t.Fatal("recursive effects not propagated", f.Effects)
		}
	}
}

func TestScalarOperatorsAndFunctionResultReads(t *testing.T) {
	p := lower(t, module(`Option Explicit
Function Logic(ByVal a As Boolean, ByVal b As Boolean) As Boolean
    Logic = Not (a And b) Xor (a Or b)
End Function
Function Compare(ByVal a As Integer, ByVal b As Long) As Boolean
    Compare = a < b
End Function
Function Arithmetic() As Long
    Arithmetic = 10
    Arithmetic = -Arithmetic * 2 - 1
End Function
Function ImplicitCall() As Long
    ImplicitCall = Arithmetic
End Function
Sub ExplicitCall(ByRef value As Long)
    Call Update(value)
End Sub
Sub Update(ByRef value As Long)
    value = value + 1
End Sub
`))
	result, failed := execute(t, p, 1, []argument{{value: -1}, {value: 0}})
	if failed || result != 0 {
		t.Fatal("Boolean bitwise representation", result, failed)
	}
	result, failed = execute(t, p, 2, []argument{{value: -5}, {value: 1}})
	if failed || result != -1 {
		t.Fatal("comparison did not widen operands or return VBA True", result, failed)
	}
	result, failed = execute(t, p, 4, nil)
	if failed || result != -21 {
		t.Fatal("function-result read became recursive call", result, failed)
	}
	value := int64(41)
	_, failed = execute(t, p, 5, []argument{{cell: &value}})
	if failed || value != 42 {
		t.Fatal("explicit Call parentheses lost alias", value, failed)
	}
}

func TestStrictCompilationGate(t *testing.T) {
	tests := []struct{ name, text, want string }{
		{"recovery", "Sub F()\n x =\nEnd Sub\n", ""},
		{"binding", "Option Explicit\nSub F()\n missing = 1\nEnd Sub\n", ""},
		{"semantic", "Sub F(ByVal x As Long)\n Set x = Nothing\nEnd Sub\n", ""},
		{"deferred", "Sub F(ByVal x As Variant)\n x = Not x\nEnd Sub\n", "Variant logical"},
		{"unsupported array element", "Sub F()\n Dim x() As LongLong\n ReDim x(3)\nEnd Sub\n", "type LongLong"},
		{"static", "Sub F()\n Static x As Long\nEnd Sub\n", "Static"},
		{"unsupported control", "Sub F()\n End\nEnd Sub\n", "statement kind"},
		{"unreachable unsupported", "Sub F()\n Exit Sub\n Dim x As LongLong\nEnd Sub\n", "type LongLong"},
		{"wide literal", "Function F() As LongLong\n F = &HFFFFFFFFFFFFFFFF^\nEnd Function\n", "type LongLong"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(test.text)}})
			result, diagnostics := lowering.Lower(p)
			if result != nil || len(diagnostics) == 0 {
				t.Fatal("partial/recovered program became executable", result, diagnostics)
			}
			if test.want != "" && !strings.Contains(diagnostics[0].Message, test.want) {
				t.Fatalf("expected %q: %+v", test.want, diagnostics)
			}
			if diagnostics[0].File == nil || !diagnostics[0].Loc.IsValid() {
				t.Fatal("diagnostic lost source location")
			}
		})
	}
}

func TestHostModulesAreRejected(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "C.cls", Text: "Sub F()\nEnd Sub\n", ModuleKind: ast.ModuleKindDocument}}})
	result, diagnostics := lowering.Lower(p)
	if result != nil || len(diagnostics) == 0 {
		t.Fatal("host procedure silently received private scalar ABI")
	}
}
