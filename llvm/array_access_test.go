package llvm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

func TestInlineArrayAccessNativeCode(t *testing.T) {
	source, err := os.ReadFile("../examples/array-access/Main.bas")
	if err != nil {
		t.Fatal(err)
	}
	text := emit(t, lower(t, strings.TrimPrefix(string(source), "Option Explicit\n")), llvm.Options{})
	// Inspection must keep unreferenced private procedures, just as the
	// Compiler Explorer adapter does. No runtime bitcode or LTO is involved.
	text += "\n@llvm.used = appending global [2 x ptr] [ptr @vba.p1, ptr @vba.p2], section \"llvm.metadata\"\n"
	dir := t.TempDir()
	input, output := filepath.Join(dir, "array.ll"), filepath.Join(dir, "array.s")
	if err := os.WriteFile(input, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, tool(t, "clang"), "-S", "-O2", "-Wno-override-module", input, "-o", output)
	assembly, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	calls := regexp.MustCompile(`(?m)^\s*(call[a-z]*|bl|blr)\s`)
	for _, name := range []string{"vba.p1", "vba.p2"} {
		start := strings.Index(string(assembly), name+":")
		if start < 0 {
			t.Fatalf("missing native procedure %s", name)
		}
		body := string(assembly[start:])
		end := strings.Index(body, "Lfunc_end")
		if end < 0 {
			t.Fatalf("missing native procedure end for %s", name)
		}
		if calls.MatchString(body[:end]) {
			t.Fatalf("scalar summation/fill should have no runtime calls at O2:\n%s", body[:end])
		}
	}
}

func TestInlineArrayAccessSelection(t *testing.T) {
	tests := []struct {
		name, source string
		index, bound bool
	}{
		{"stable parameter", `Function Work(ByRef a() As Long) As Long
 Work = a(0) + UBound(a)
End Function`, true, true},
		// Snapshots reload after the ReDim; no whole-call stability is needed.
		{"resized parameter", `Function Work(ByRef a() As Long) As Long
 ReDim a(2)
 Work = a(0) + UBound(a)
End Function`, true, true},
		{"local after resize", `Function Work() As Long
 Dim a() As Long
 ReDim a(2)
 Work = a(0) + UBound(a)
End Function`, true, true},
		{"erased variant", `Function Work(ByRef a As Variant) As Long
 Work = a(0) + UBound(a)
End Function`, false, false},
		{"owned variant", `Function Work() As Long
 Dim a As Variant
 ReDim a(2) As Long
 Work = a(0) + UBound(a)
End Function`, false, false},
		{"global array", `Public values() As Long
Function Work() As Long
 Work = values(0) + UBound(values)
End Function`, true, true},
		{"managed array", `Function Work(ByRef a() As String) As Long
 Work = Len(a(0)) + UBound(a)
End Function`, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := emit(t, lower(t, test.source), llvm.Options{})
			if strings.Contains(text, "call { i32, ptr } @vba.array.snapshot.index.") != test.index ||
				strings.Contains(text, "call { i32, i32 } @vba.array.snapshot.bound.") != test.bound {
				t.Fatal(text)
			}
		})
	}
}

func TestNativeInlineArrayBoundariesAndLocks(t *testing.T) {
	text := emit(t, lower(t, `Function GetCell(ByRef a() As Long, ByVal index As Long) As Long
 GetCell = a(index)
End Function
Function Lower(ByRef a() As Long, ByVal dimension As Long) As Long
 Lower = LBound(a, dimension)
End Function
Function Upper(ByRef a() As Long, ByVal dimension As Long) As Long
 Upper = UBound(a, dimension)
End Function
Sub Change(ByRef a() As Long)
 a(0) = a(0) + 1
End Sub
Sub SetCell(ByRef a() As Long)
 a(0) = 99
End Sub
Sub SetChecked(ByRef a() As Long, ByVal value As Long)
 a(0) = value + 1
End Sub`), llvm.Options{})
	var harness strings.Builder
	harness.WriteString(`
define i32 @main() {
entry:
  %allocation = alloca [40 x i8], align 8
  store [40 x i8] zeroinitializer, ptr %allocation
  %tag = getelementptr i8, ptr %allocation, i64 4
  store i32 3, ptr %tag
  %array = getelementptr i8, ptr %allocation, i64 8
  %rank = getelementptr %vba.array, ptr %array, i32 0, i32 0
  store i16 1, ptr %rank, align 1
  %features = getelementptr %vba.array, ptr %array, i32 0, i32 1
  store i16 128, ptr %features, align 1
  %size = getelementptr %vba.array, ptr %array, i32 0, i32 2
  store i32 4, ptr %size, align 1
  %locks = getelementptr %vba.array, ptr %array, i32 0, i32 3
  %data = getelementptr %vba.array, ptr %array, i32 0, i32 4
  %count = getelementptr %vba.array, ptr %array, i32 0, i32 5, i64 0, i32 0
  %lower = getelementptr %vba.array, ptr %array, i32 0, i32 5, i64 0, i32 1
  %cell = alloca i32
  store ptr %cell, ptr %data, align 1
  %slot = alloca ptr
  %error = alloca %vba.error
  %out = alloca i32
  br label %test0
`)
	tests := []struct {
		procedure, argument      int
		count, lower, locks      int64
		null                     bool
		status, value, afterCell int
	}{
		{1, 0, 1, 0, 0, false, 0, 42, 42},
		{1, -2147483648, 1, -2147483648, 0, false, 0, 42, 42},
		{1, 2147483647, 1, -2147483648, 0, false, 9, -77, 42},
		{1, -1, 1, 0, 0, false, 9, -77, 42},
		{1, 0, 0, 0, 0, false, 9, -77, 42},
		{1, 0, 1, 0, 0, true, 9, -77, 42},
		{1, 0, 1, 0, 4294967295, false, 10, -77, 42},
		{1, 0, 1, 0, 4294967294, false, 0, 42, 42},
		{2, 1, 0, -2147483648, 0, false, 0, -2147483648, 42},
		{3, 1, 0, 0, 0, false, 0, -1, 42},
		{3, 1, 0, -2147483648, 0, false, 6, -77, 42},
		{3, 1, 4294967295, 0, 0, false, 6, -77, 42},
		{3, 0, 1, 0, 0, false, 9, -77, 42},
		{3, 2, 1, 0, 0, false, 9, -77, 42},
		{3, 1, 1, 0, 0, true, 9, -77, 42},
		{3, 1, 1, 0, 4294967295, false, 0, 0, 42},
		// Nested indexing must observe the LHS lock, even though its own
		// immediate read can elide its balanced lock/unlock pair.
		{4, 0, 1, 0, 4294967294, false, 10, -77, 42},
		{4, 0, 1, 0, 0, false, 0, -77, 43},
		{5, 0, 1, 0, 0, false, 0, -77, 99},
		{5, 0, 1, 0, 4294967295, false, 10, -77, 42},
		{6, 2147483647, 1, 0, 0, false, 6, -77, 42},
		{6, 2147483647, 1, 0, 4294967295, false, 10, -77, 42},
		{6, 2147483647, 0, 0, 0, false, 9, -77, 42},
		{6, 1, 1, 0, 0, false, 0, -77, 2},
	}
	for i, test := range tests {
		fmt.Fprintf(&harness, "test%d:\n  store i32 %d, ptr %%count, align 1\n  store i32 %d, ptr %%lower, align 1\n  store i32 %d, ptr %%locks, align 1\n  store i32 42, ptr %%cell\n  store i32 -77, ptr %%out\n", i, test.count, test.lower, test.locks)
		pointer := "%array"
		if test.null {
			pointer = "null"
		}
		fmt.Fprintf(&harness, "  store ptr %s, ptr %%slot\n", pointer)
		arguments := "ptr %error, ptr %out, ptr %slot"
		if test.procedure > 3 {
			arguments = "ptr %error, ptr %slot"
			if test.procedure == 6 {
				arguments += fmt.Sprintf(", i32 %d", test.argument)
			}
		} else {
			arguments += fmt.Sprintf(", i32 %d", test.argument)
		}
		fmt.Fprintf(&harness, "  %%status%d = call i32 @vba.p%d(%s)\n  %%value%d = load i32, ptr %%out\n  %%locks%d = load i32, ptr %%locks, align 1\n  %%cell%d = load i32, ptr %%cell\n", i, test.procedure, arguments, i, i, i)
		fmt.Fprintf(&harness, "  %%ok.status%d = icmp eq i32 %%status%d, %d\n  %%ok.value%d = icmp eq i32 %%value%d, %d\n  %%ok.locks%d = icmp eq i32 %%locks%d, %d\n  %%ok.cell%d = icmp eq i32 %%cell%d, %d\n", i, i, test.status, i, i, test.value, i, i, test.locks, i, i, test.afterCell)
		fmt.Fprintf(&harness, "  %%ok.first%d = and i1 %%ok.status%d, %%ok.value%d\n  %%ok.second%d = and i1 %%ok.locks%d, %%ok.cell%d\n  %%ok%d = and i1 %%ok.first%d, %%ok.second%d\n", i, i, i, i, i, i, i, i, i)
		next := fmt.Sprintf("test%d", i+1)
		if i+1 == len(tests) {
			next = "success"
		}
		fmt.Fprintf(&harness, "  br i1 %%ok%d, label %%%s, label %%failure\n", i, next)
	}
	harness.WriteString("success:\n  ret i32 0\nfailure:\n  ret i32 1\n}\n")
	native(t, text+harness.String())
}

func TestArrayLockElisionAcrossCalls(t *testing.T) {
	text := emit(t, lower(t, `Sub Touch(ByRef x As Long)
 x = x + 1
End Sub
Sub Both(ByRef x As Long, ByRef a() As Long)
 ReDim a(5)
 x = 7
End Sub
Function Run() As Long
 Dim a() As Long
 ReDim a(3)
 Touch a(1)
 If a(1) <> 1 Then Run = 1: Exit Function
 On Error Resume Next
 Both a(2), a
 If Err.Number <> 10 Then Run = 2: Exit Function
 On Error GoTo 0
 If UBound(a) <> 3 Then Run = 3: Exit Function
 If a(2) <> 0 Then Run = 4: Exit Function
End Function`), llvm.Options{Entry: 3})
	// Only the element passed alongside its resizable array keeps its lock:
	// the callee's ReDim must still observe it and fail with error 10.
	locks := regexp.MustCompile(`@vba\.array\.snapshot\.index\.1\.1\(ptr [^,]+, i64 4, i1 (true|false)`).FindAllStringSubmatch(text, -1)
	var got []string
	for _, lock := range locks {
		got = append(got, lock[1])
	}
	if strings.Join(got, " ") != "false false true false" {
		t.Fatalf("lock plan %v\n%s", got, text)
	}
	native(t, text)
}
