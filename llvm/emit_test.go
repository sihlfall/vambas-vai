package llvm_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/internal/runtimebuild"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
	"sihlfall.ch/vambas-vai/lowering"
)

func lower(t *testing.T, text string) *ir.Program {
	t.Helper()
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Option Explicit\n" + text}}})
	semantic, diagnostics := lowering.Lower(p)
	if len(diagnostics) != 0 {
		t.Fatalf("lowering: %+v", diagnostics)
	}
	return semantic
}

func emit(t *testing.T, p *ir.Program, options llvm.Options) string {
	t.Helper()
	before := ir.Format(p)
	text, diagnostics := llvm.Emit(p, options)
	if len(diagnostics) != 0 {
		t.Fatalf("emission: %+v", diagnostics)
	}
	second, diagnostics := llvm.Emit(p, options)
	if len(diagnostics) != 0 || second != text || ir.Format(p) != before {
		t.Fatal("emitter mutated input or produced unstable output")
	}
	return text
}

func tool(t *testing.T, name string) string {
	t.Helper()
	for _, candidate := range []string{name + "-20", name} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	if os.Getenv("VBA_LLVM_TEST_REQUIRED") != "" {
		t.Fatalf("required LLVM tool %s not installed", name)
	}
	t.Skipf("LLVM tool %s not installed (VBA_LLVM_TEST_REQUIRED=1 makes this fatal)", name)
	return ""
}

func command(t *testing.T, path string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", path, args, err, output)
	}
	return output
}

func runtimeSources(t *testing.T) []string {
	t.Helper()
	sources, err := runtimebuild.Sources("../runtime")
	if err != nil {
		t.Fatal(err)
	}
	return sources
}

var compiledRuntime struct {
	sync.Once
	text string
}

// The runtime is unchanged between fixtures. Compile each translation unit once
// per test process, then give every fixture its own IR for instrumentation.
func runtimeLLVM(t *testing.T) string {
	t.Helper()
	compiledRuntime.Do(func() {
		dir := t.TempDir()
		var modules []string
		for _, source := range runtimeSources(t) {
			module := filepath.Join(dir, filepath.Base(source)+".ll")
			command(t, tool(t, "clang"), "-std=c11", "-O1", "-S", "-emit-llvm", source, "-o", module)
			modules = append(modules, module)
		}
		output := filepath.Join(dir, "runtime.ll")
		command(t, tool(t, "llvm-link"), append(append([]string{"-S"}, modules...), "-o", output)...)
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		compiledRuntime.text = string(data)
	})
	return compiledRuntime.text
}

// Assemble, verify, JIT, optimize, and compile the same module. Executing both
// native optimization levels checks that overflow/error and alias semantics
// survive optimization rather than depending on the unoptimized block layout.
func native(t *testing.T, text string) {
	t.Helper()
	assembler, optimizer, interpreter, clang := tool(t, "llvm-as"), tool(t, "opt"), tool(t, "lli"), tool(t, "clang")
	dir := t.TempDir()
	input, bitcode, optimized := filepath.Join(dir, "test.ll"), filepath.Join(dir, "test.bc"), filepath.Join(dir, "optimized.bc")
	if err := os.WriteFile(input, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, assembler, input, "-o", bitcode)
	if strings.Contains(text, "declare i32 @vba_variant_load") {
		runtimeIR := filepath.Join(dir, "runtime.ll")
		runtimeText := runtimeLLVM(t)
		if strings.Contains(text, "@test.live") {
			runtimeText = strings.ReplaceAll(runtimeText, "@malloc(", "@test.tracked.alloc(")
			runtimeText = strings.ReplaceAll(runtimeText, "@free(", "@test.tracked.free(")
		}
		if err := os.WriteFile(runtimeIR, []byte(runtimeText), 0600); err != nil {
			t.Fatal(err)
		}
		merged := filepath.Join(dir, "linked.ll")
		command(t, tool(t, "llvm-link"), "-S", input, runtimeIR, "-o", merged)
		input = merged
		command(t, assembler, input, "-o", bitcode)
	}
	command(t, optimizer, "-passes=verify", "-disable-output", bitcode)
	command(t, interpreter, bitcode)
	command(t, optimizer, "-passes=default<O2>,verify", bitcode, "-o", optimized)
	command(t, interpreter, optimized)
	for _, level := range []string{"-O0", "-O2"} {
		executable := filepath.Join(dir, "native"+level)
		command(t, clang, level, input, "-lm", "-o", executable)
		command(t, executable)
	}
}

func TestRejectsInvalidIRAndEntry(t *testing.T) {
	if text, diagnostics := llvm.Emit(nil, llvm.Options{}); text != "" || len(diagnostics) == 0 {
		t.Fatal("invalid IR emitted")
	}
	p := lower(t, "Function F(ByVal x As Long) As Long\n F = x + 1\nEnd Function\n")
	for _, id := range []ir.ProcedureID{1, 2} {
		if text, diagnostics := llvm.Emit(p, llvm.Options{Entry: id}); text != "" || len(diagnostics) == 0 {
			t.Fatal("invalid entry emitted")
		}
	}
	variant := lower(t, "Function V() As Variant\nEnd Function\n")
	if text, diagnostics := llvm.Emit(variant, llvm.Options{Entry: 1}); text != "" || len(diagnostics) == 0 {
		t.Fatal("Variant entry must require a scalar wrapper")
	}
	for i := range p.Procedures[0].Statements[0].Instructions {
		in := &p.Procedures[0].Statements[0].Instructions[i]
		if in.Op == ir.OpTry {
			in.OnError = ir.NoErrorContinuation
		}
	}
	if text, diagnostics := llvm.Emit(p, llvm.Options{}); text != "" || len(diagnostics) == 0 {
		t.Fatal("invalid error path emitted")
	}
}

func TestNativeEntryAndDefaultResult(t *testing.T) {
	for _, text := range []string{
		"Sub Run()\nEnd Sub\n",
		"Function Run() As Long\n Exit Function\n Run = 99\nEnd Function\n",
		"Function Run() As Integer\nEnd Function\n",
		"Function Run() As Boolean\nEnd Function\n",
	} {
		p := lower(t, text)
		native(t, emit(t, p, llvm.Options{Entry: 1}))
	}
}

func TestNativeAliasesCallsAndFailure(t *testing.T) {
	p := lower(t, `Sub Run(ByRef value As Long)
    Change value, value
    Change (value), (value)
    CopyChange value
    Call Bump(value)
End Sub
Sub Change(ByRef a As Long, ByRef b As Long)
    a = a + 1
    b = b + 1
End Sub
Function Bump(ByRef value As Long) As Long
    value = value + 1
    Bump = value
End Function
Function Named(ByRef counter As Long) As Long
    Named = Combine(b:=Bump(counter), a:=Bump(counter))
End Function
Function Combine(ByVal a As Long, ByVal b As Long) As Long
    Combine = a * 10 + b
End Function
Sub Failing(ByRef hits As Long, ByRef result As Long)
    result = BumpThenFail(hits)
End Sub
Function BumpThenFail(ByRef hits As Long) As Long
    Dim limit As Long
    hits = hits + 1
    limit = 2147483647
    BumpThenFail = limit + 1
End Function
Sub CopyChange(ByVal value As Long)
    value = 99
End Sub
`)
	text := emit(t, p, llvm.Options{}) + `define i32 @main() {
entry:
  %error = alloca %vba.error
  %x = alloca i32
  %result = alloca i32
  store i32 5, ptr %x
  %status1 = call i32 @vba.p1(ptr %error, ptr %x)
  %x1 = load i32, ptr %x
  %c1 = icmp eq i32 %status1, 0
  %c2 = icmp eq i32 %x1, 8
  store i32 0, ptr %x
  %status2 = call i32 @vba.p4(ptr %error, ptr %result, ptr %x)
  %value2 = load i32, ptr %result
  %x2 = load i32, ptr %x
  %c3 = icmp eq i32 %status2, 0
  %c4 = icmp eq i32 %value2, 21
  %c5 = icmp eq i32 %x2, 2
  store i32 0, ptr %x
  store i32 11, ptr %result
  %status3 = call i32 @vba.p6(ptr %error, ptr %x, ptr %result)
  %x3 = load i32, ptr %x
  %value3 = load i32, ptr %result
  %code = load i32, ptr %error
  %origin.ptr = getelementptr %vba.error, ptr %error, i32 0, i32 1
  %origin = load i32, ptr %origin.ptr
  %statement.ptr = getelementptr %vba.error, ptr %error, i32 0, i32 2
  %statement = load i32, ptr %statement.ptr
  %c6 = icmp eq i32 %status3, 6
  %c7 = icmp eq i32 %x3, 1
  %c8 = icmp eq i32 %value3, 11
  %c9 = icmp eq i32 %code, 6
  %c10 = icmp eq i32 %origin, 7
  %c11 = icmp eq i32 %statement, 4
  %a1 = and i1 %c1, %c2
  %a2 = and i1 %a1, %c3
  %a3 = and i1 %a2, %c4
  %a4 = and i1 %a3, %c5
  %a5 = and i1 %a4, %c6
  %a6 = and i1 %a5, %c7
  %a7 = and i1 %a6, %c8
  %a8 = and i1 %a7, %c9
  %a9 = and i1 %a8, %c10
  %a10 = and i1 %a9, %c11
  %exit = select i1 %a10, i32 0, i32 1
  ret i32 %exit
}
`
	native(t, text)
}

func TestNativeScalarOperations(t *testing.T) {
	tests := []struct {
		expression, input, output string
		a, b, want                int64
		error                     bool
	}{
		{"a + b", "Integer", "Integer", 32766, 1, 32767, false},
		{"a + b", "Integer", "Integer", 32767, 1, 0, true},
		{"a + b", "Long", "Long", 2147483647, 1, 0, true},
		{"a + b", "Long", "Long", -2147483648, -1, 0, true},
		{"a - b", "Integer", "Integer", -32768, 1, 0, true},
		{"a - b", "Long", "Long", 2147483647, -1, 0, true},
		{"a - b", "Long", "Long", -2, 3, -5, false},
		{"a * b", "Integer", "Integer", 200, 200, 0, true},
		{"a * b", "Long", "Long", 2147483647, 2, 0, true},
		{"a * b", "Long", "Long", -7, 6, -42, false},
		{"-a", "Integer", "Integer", -32768, 0, 0, true},
		{"-a", "Long", "Long", -2147483648, 0, 0, true},
		{"-a", "Long", "Long", 42, 0, -42, false},
		{"a", "Long", "Integer", 32768, 0, 0, true},
		{"a", "Long", "Integer", -32769, 0, 0, true},
		{"a", "Long", "Integer", -32768, 0, -32768, false},
		{"a", "Long", "Integer", 32767, 0, 32767, false},
		{"a", "Integer", "Long", -32768, 0, -32768, false},
		{"Not a", "Boolean", "Boolean", 0, 0, -1, false},
		{"a And b", "Boolean", "Boolean", -1, 0, 0, false},
		{"a Or b", "Boolean", "Boolean", -1, 0, -1, false},
		{"a Xor b", "Boolean", "Boolean", -1, -1, 0, false},
		{"Not a", "Integer", "Integer", 32767, 0, -32768, false},
		{"a And b", "Long", "Long", 255, 3, 3, false},
		{"a = b", "Long", "Boolean", -1, 0, 0, false},
		{"a <> b", "Long", "Boolean", -1, 0, -1, false},
		{"a < b", "Long", "Boolean", -1, 0, -1, false},
		{"a <= b", "Long", "Boolean", -1, -1, -1, false},
		{"a > b", "Long", "Boolean", 0, -1, -1, false},
		{"a >= b", "Long", "Boolean", -1, 0, 0, false},
	}
	var source, harness strings.Builder
	harness.WriteString("define i32 @main() {\nentry:\n  %error = alloca %vba.error\n")
	previous := "true"
	for i, test := range tests {
		fmt.Fprintf(&source, "Function F%d(ByVal a As %s, ByVal b As %s) As %s\n F%d = %s\nEnd Function\n", i, test.input, test.input, test.output, i, test.expression)
		input, output := "i16", "i16"
		if test.input == "Long" {
			input = "i32"
		}
		if test.output == "Long" {
			output = "i32"
		}
		status, want := 0, test.want
		if test.error {
			status, want = 6, 123
		} // Failure must leave output untouched.
		fmt.Fprintf(&harness, "  %%result%d = alloca %s\n  store %s 123, ptr %%result%d\n", i, output, output, i)
		fmt.Fprintf(&harness, "  %%status%d = call i32 @vba.p%d(ptr %%error, ptr %%result%d, %s %d, %s %d)\n", i, i+1, i, input, test.a, input, test.b)
		fmt.Fprintf(&harness, "  %%value%d = load %s, ptr %%result%d\n  %%ok.status%d = icmp eq i32 %%status%d, %d\n  %%ok.value%d = icmp eq %s %%value%d, %d\n", i, output, i, i, i, status, i, output, i, want)
		fmt.Fprintf(&harness, "  %%check%d = and i1 %%ok.status%d, %%ok.value%d\n  %%all%d = and i1 %s, %%check%d\n", i, i, i, i, previous, i)
		previous = fmt.Sprintf("%%all%d", i)
	}
	fmt.Fprintf(&harness, "  %%exit = select i1 %s, i32 0, i32 1\n  ret i32 %%exit\n}\n", previous)
	native(t, emit(t, lower(t, source.String()), llvm.Options{})+harness.String())
}

func TestNativeEntryExitCodes(t *testing.T) {
	interpreter := tool(t, "lli")
	for _, test := range []struct {
		body string
		want int
	}{
		{"Run = 42", 42},
		{"Dim value As Long\n value = 2147483647\n Run = value + 1", 6},
	} {
		text := emit(t, lower(t, "Function Run() As Long\n"+test.body+"\nEnd Function\n"), llvm.Options{Entry: 1})
		path := filepath.Join(t.TempDir(), "entry.ll")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		output, err := exec.CommandContext(ctx, interpreter, path).CombinedOutput()
		cancel()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != test.want {
			t.Fatalf("expected exit %s, got %v\n%s", strconv.Itoa(test.want), err, output)
		}
	}
}
