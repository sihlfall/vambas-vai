package llvm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

// Instrument actual remaining allocations without relying on optimizer-specific
// allocation counts. Every completed test activation must release all owners.
func trackedStrings(text string) string {
	if !strings.Contains(text, "@malloc(") {
		text += "\ndeclare ptr @malloc(i64)\n"
	}
	if !strings.Contains(text, "@free(") {
		text += "\ndeclare void @free(ptr)\n"
	}
	text = strings.ReplaceAll(text, "call ptr @malloc(i64", "call ptr @test.tracked.alloc(i64")
	text = strings.ReplaceAll(text, "call void @free(ptr", "call void @test.tracked.free(ptr")
	text = strings.ReplaceAll(text, "define i32 @main()", "define internal i32 @test.body()")
	return text + `
@test.live = internal global i64 0
define ptr @test.tracked.alloc(i64 %n) noinline {
entry:
  %p = call ptr @malloc(i64 %n)
  %ok = icmp ne ptr %p, null
  %increment = zext i1 %ok to i64
  %old = load i64, ptr @test.live
  %new = add i64 %old, %increment
  store i64 %new, ptr @test.live
  ret ptr %p
}
define void @test.tracked.free(ptr %p) noinline {
entry:
  %old = load i64, ptr @test.live
  %nonnull = icmp ne ptr %p, null
  %decrement = zext i1 %nonnull to i64
  %new = sub i64 %old, %decrement
  store i64 %new, ptr @test.live
  call void @free(ptr %p)
  ret void
}
define i32 @main() {
entry:
  %status = call i32 @test.body()
  %live = load i64, ptr @test.live
  %clean = icmp eq i64 %live, 0
  %exit = select i1 %clean, i32 %status, i32 99
  ret i32 %exit
}
`
}

func TestNativeStringValues(t *testing.T) {
	p := lower(t, `Private Const Prefix As String = "Hel" & "lo"
Function Greeting(ByVal name As String) As String
    Greeting = Prefix & name
End Function
Function Main() As Long
    Dim s As String
    Call Greeting("ignored")
    s = Greeting("!") & vbNullChar & ChrW(55357) & ChrW(56832)
    Main = Len(s) + LenB(s) - 27
End Function
`)
	native(t, trackedStrings(emit(t, p, llvm.Options{Entry: 2})))
}

func TestNativeStringMutationAndSnapshots(t *testing.T) {
	p := lower(t, `Function Main() As Long
    Dim s As String, copy As String, observed As Long
    s = "ab" & ChrW(99)
    copy = s
    Change s, s
    CopyChange copy
    observed = Observe(copy, Replace(copy))
    TemporaryChange (copy)
    s = s
    Main = Len(s) + Len(copy) + observed - 15
End Function
Sub Change(ByRef a As String, ByRef b As String)
    a = a & "x"
    b = b & "y"
End Sub
Sub CopyChange(ByVal value As String)
    value = "z"
End Sub
Function Observe(ByVal snapshot As String, ByVal n As Long) As Long
    Observe = Len(snapshot & ChrW(33)) - 1
End Function
Function Replace(ByRef value As String) As Long
    value = "1234567"
    Replace = 0
End Function
Sub TemporaryChange(ByRef value As String)
    value = "q"
End Sub
`)
	// s is mutable through two aliases (length 5); copy initially remains length
	// 3, then Replace changes its slot to length 7. Observe must retain the old
	// value evaluated before Replace. A parenthesized ByRef is separate storage.
	text := emit(t, p, llvm.Options{Entry: 1})
	native(t, trackedStrings(text))
}

func TestNativeStringFailurePreservesMutationAndCleansUp(t *testing.T) {
	p := lower(t, `Sub Run(ByRef changed As String, ByRef pending As String)
    pending = ("owned" & ChrW(33)) & Fail(changed)
End Sub
Function Fail(ByRef value As String) As String
    Dim limit As Long, local As String
    local = "temporary" & ChrW(0)
    value = value & ChrW(33)
    limit = 2147483647
    limit = limit + 1
    Fail = local
End Function
`)
	text := emit(t, p, llvm.Options{}) + `define i32 @main() {
entry:
  %error = alloca %vba.error
  %a = alloca %vba.string
  %b = alloca %vba.string
  store %vba.string {ptr null, ptr null, i32 1, i16 65}, ptr %a
  store %vba.string {ptr null, ptr null, i32 1, i16 66}, ptr %b
  %status = call i32 @vba.p1(ptr %error, ptr %a, ptr %b)
  %av = load %vba.string, ptr %a
  %bv = load %vba.string, ptr %b
  %length = call i32 @vba.str.len(%vba.string %av)
  %unit = call i16 @vba.str.unit(%vba.string %bv, i32 0)
  %ok1 = icmp eq i32 %status, 6
  %ok2 = icmp eq i32 %length, 2
  %ok3 = icmp eq i16 %unit, 66
  %ok4 = and i1 %ok1, %ok2
  %ok5 = and i1 %ok4, %ok3
  call void @vba.str.clear(ptr %a)
  call void @vba.str.clear(ptr %b)
  %exit = select i1 %ok5, i32 0, i32 1
  ret i32 %exit
}
`
	native(t, trackedStrings(text))
}

func TestNativeStringAllocationFailure(t *testing.T) {
	p := lower(t, `Sub Run(ByRef value As String)
    Dim local As String
    local = "abc" & ChrW(100)
    value = local & ChrW(101)
End Sub
`)
	// Fault injection affects the allocator call, not its annotation or the
	// language's failure path. The old mutable variable must remain unchanged.
	text := emit(t, p, llvm.Options{})
	text = strings.ReplaceAll(text, "call ptr @malloc(i64", "call ptr @test.allocate(i64")
	text += `@test.attempts = internal global i32 0
define internal ptr @test.allocate(i64 %n) noinline {
entry:
  %attempt = load i32, ptr @test.attempts
  %next = add i32 %attempt, 1
  store i32 %next, ptr @test.attempts
  %fail = icmp ne i32 %attempt, 0
  br i1 %fail, label %failure, label %success
failure:
  ret ptr null
success:
  %p = call ptr @malloc(i64 %n)
  ret ptr %p
}
define i32 @main() {
entry:
  %error = alloca %vba.error
  %value = alloca %vba.string
  store %vba.string {ptr null, ptr null, i32 1, i16 88}, ptr %value
  %status = call i32 @vba.p1(ptr %error, ptr %value)
  %s = load %vba.string, ptr %value
  %unit = call i16 @vba.str.unit(%vba.string %s, i32 0)
  %ok1 = icmp eq i32 %status, 14
  %ok2 = icmp eq i16 %unit, 88
  %ok = and i1 %ok1, %ok2
  %exit = select i1 %ok, i32 0, i32 1
  ret i32 %exit
}
`
	native(t, trackedStrings(text))
}

func TestLLVMEliminatesStringAllocationPaths(t *testing.T) {
	for _, expression := range []string{`Len(ChrW(65) & "") - 1`, `Len("ab" & "cd") - 4`} {
		t.Run(expression, func(t *testing.T) {
			p := lower(t, "Function Main() As Long\n Main = "+expression+"\nEnd Function\n")
			text := emit(t, p, llvm.Options{Entry: 1})
			optimizer := tool(t, "opt")
			dir := t.TempDir()
			input, output := filepath.Join(dir, "character.ll"), filepath.Join(dir, "optimized.ll")
			if err := os.WriteFile(input, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			command(t, optimizer, "-passes=default<O2>", "-S", input, "-o", output)
			optimized, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(optimized), "@malloc(") || strings.Contains(string(optimized), "@free(") {
				t.Fatalf("constant/character path retained allocation machinery:\n%s", optimized)
			}
			native(t, text)
		})
	}
}

func TestNativeChrWErrorsAndNullStrings(t *testing.T) {
	for _, test := range []struct {
		expression string
		result     int
	}{
		{"Len(vbNullString)", 0}, {"Len(\"\")", 0}, {"LenB(vbNullChar)", 2},
		{"Len(ChrW(-32768))", 1}, {"Len(ChrW(65535))", 1},
	} {
		text := fmt.Sprintf("Function Main() As Long\n Main = %s - %d\nEnd Function\n", test.expression, test.result)
		native(t, trackedStrings(emit(t, lower(t, text), llvm.Options{Entry: 1})))
	}
	for _, value := range []int{65536, -32769} {
		p := lower(t, fmt.Sprintf("Function Main() As String\n Main = ChrW(%d)\nEnd Function\n", value))
		text := emit(t, p, llvm.Options{}) + `define i32 @main() {
entry:
  %error = alloca %vba.error
  %output = alloca %vba.string
  store %vba.string zeroinitializer, ptr %output
  %status = call i32 @vba.p1(ptr %error, ptr %output)
  %ok = icmp eq i32 %status, 5
  %exit = select i1 %ok, i32 0, i32 1
  ret i32 %exit
}
`
		native(t, trackedStrings(text))
	}
}

func TestNativeUTF16StringContents(t *testing.T) {
	p := lower(t, `Function Make() As String
    Make = "A" & vbNullChar & ChrW(55357) & ChrW(56832) & "é" & ChrW(-1)
End Function
`)
	var harness strings.Builder
	harness.WriteString(`define i32 @main() {
entry:
  %error = alloca %vba.error
  %output = alloca %vba.string
  store %vba.string zeroinitializer, ptr %output
  %status = call i32 @vba.p1(ptr %error, ptr %output)
  %s = load %vba.string, ptr %output
  %length = call i32 @vba.str.len(%vba.string %s)
  %ok.status = icmp eq i32 %status, 0
  %ok.length = icmp eq i32 %length, 6
  %initial = and i1 %ok.status, %ok.length
`)
	previous := "%initial"
	for i, unit := range []uint16{65, 0, 55357, 56832, 233, 65535} {
		fmt.Fprintf(&harness, "  %%unit%d = call i16 @vba.str.unit(%%vba.string %%s, i32 %d)\n  %%check%d = icmp eq i16 %%unit%d, %d\n  %%all%d = and i1 %s, %%check%d\n", i, i, i, i, unit, i, previous, i)
		previous = fmt.Sprintf("%%all%d", i)
	}
	fmt.Fprintf(&harness, "  call void @vba.str.clear(ptr %%output)\n  %%exit = select i1 %s, i32 0, i32 1\n  ret i32 %%exit\n}\n", previous)
	native(t, trackedStrings(emit(t, p, llvm.Options{})+harness.String()))
}

func TestNativeStringSnapshotRetainsContents(t *testing.T) {
	p := lower(t, `Function Capture(ByVal snapshot As String, ByVal ignored As Long) As String
    Capture = snapshot
End Function
Function Replace(ByRef value As String) As Long
    value = "replacement"
    Replace = 0
End Function
Function Run() As String
    Dim s As String
    s = "ab" & ChrW(99)
    Run = Capture(s, Replace(s))
End Function
`)
	text := emit(t, p, llvm.Options{}) + `define i32 @main() {
entry:
  %error = alloca %vba.error
  %output = alloca %vba.string
  store %vba.string zeroinitializer, ptr %output
  %status = call i32 @vba.p3(ptr %error, ptr %output)
  %s = load %vba.string, ptr %output
  %a = call i16 @vba.str.unit(%vba.string %s, i32 0)
  %b = call i16 @vba.str.unit(%vba.string %s, i32 1)
  %c = call i16 @vba.str.unit(%vba.string %s, i32 2)
  %ok0 = icmp eq i32 %status, 0
  %ok1 = icmp eq i16 %a, 97
  %ok2 = icmp eq i16 %b, 98
  %ok3 = icmp eq i16 %c, 99
  %x = and i1 %ok0, %ok1
  %y = and i1 %x, %ok2
  %ok = and i1 %y, %ok3
  call void @vba.str.clear(ptr %output)
  %exit = select i1 %ok, i32 0, i32 1
  ret i32 %exit
}
`
	native(t, trackedStrings(text))
}
