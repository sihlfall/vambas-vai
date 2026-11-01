package llvm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

func TestSSANativeScalarTypes(t *testing.T) {
	for _, test := range []struct{ vba, native string }{
		{"Boolean", "i16"}, {"Byte", "i8"}, {"Integer", "i16"}, {"Long", "i32"},
		{"Single", "float"}, {"Double", "double"}, {"Date", "double"}, {"Currency", "i64"},
	} {
		t.Run(test.vba, func(t *testing.T) {
			p := lower(t, fmt.Sprintf(`Function Choose(ByVal flag As Boolean, ByVal a As %[1]s, ByVal b As %[1]s) As %[1]s
 Dim value As %[1]s
 If flag Then
  value = a
 Else
  value = b
 End If
 Choose = value
End Function`, test.vba))
			text := emit(t, p, llvm.Options{})
			if !strings.Contains(text, "phi "+test.native) || strings.Contains(text, "alloca") || strings.Contains(text, "vba.variant") {
				t.Fatalf("expected native scalar phi without stack slots or boxing:\n%s", text)
			}
			// Emit remains usable with no LLVM tools. Verification is a separate
			// integration check, just as for the ordinary emitter.
			path := filepath.Join(t.TempDir(), "ssa.ll")
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			command(t, tool(t, "opt"), "-passes=verify", "-disable-output", path)
		})
	}
}

func TestSSABoxesAtVariantBoundary(t *testing.T) {
	text := emit(t, lower(t, `Function Compute(ByVal flag As Boolean, ByVal a As Long, ByVal b As Long) As Variant
 Dim value As Long
 If flag Then
  value = a
 Else
  value = b
 End If
 Compute = value + 1
End Function`), llvm.Options{})
	start := strings.Index(text, "define internal i32 @vba.p1(")
	body := text[start:]
	if !strings.Contains(body, "phi i32") || !strings.Contains(body, "@llvm.sadd.with.overflow.i32") || strings.Count(body, "call i32 @vba_variant_box_number(") != 1 || strings.Contains(body, "call i32 @vba_variant_binary(") {
		t.Fatalf("scalar computation must precede the single Variant conversion:\n%s", body)
	}
	if strings.Index(body, "@vba_variant_box_number(") < strings.Index(body, "@llvm.sadd.with.overflow.i32(") {
		t.Fatal("boxed before completing scalar arithmetic")
	}
}

func TestSSANativeBehavior(t *testing.T) {
	tests := []struct {
		name, source string
		want         int
	}{
		{"loop", `Function Run() As Long
 Dim i As Long, total As Long
 While i < 10
  i = i + 1
  If i > 5 Then
   total = total + i
  Else
   total = total + 1
  End If
 Wend
 Run = total
End Function`, 45},
		{"default and dead cycle", `Function Run() As Long
 Exit Function
dead:
 Run = Run + 1
 GoTo dead
End Function`, 0},
		{"irreducible loop", `Function Run() As Long
 Dim n As Long
 GoTo second
first:
 n = n + 2
 If n > 8 Then GoTo done
second:
 n = n + 1
 If n < 6 Then GoTo first
done:
 Run = n
End Function`, 7},
		{"duplicate switch edges", `Function Run() As Long
 Dim n As Long, index As Integer
 n = 4
 index = 2
 On index GoTo same, same, other
 n = 99
same:
 n = n + 3
 GoTo done
other:
 n = 88
done:
 Run = n
End Function`, 7},
		{"byref alias and value temporary", `Function Run() As Long
 Dim n As Long
 n = 5
 Change n, n
 Change (n), (n)
 Run = n
End Function
Sub Change(ByRef a As Long, ByRef b As Long)
 a = a + 1
 b = b + a
End Sub`, 12},
		{"error retry", `Function Run() As Long
 Dim n As Long, divisor As Long
 n = 10
 On Error GoTo handler
 n = n \ divisor
 Run = n
 Exit Function
handler:
 divisor = 2
 Resume
End Function`, 5},
		{"resume next preserves failed assignment", `Function Run() As Long
 Dim n As Long, maximum As Long
 n = 7
 maximum = 2147483647
 On Error GoTo handler
 n = maximum + 1
 Run = n
 Exit Function
handler:
 n = n + 2
 Resume Next
End Function`, 9},
		{"gosub shares locals", `Function Run() As Long
 Dim n As Long
 n = 2
 GoSub outer
 GoSub inner
 Run = n
 Exit Function
outer:
 n = n + 3
 GoSub inner
 n = n + 1
 Return
inner:
 n = n * 2
 Return
End Function`, 22},
		{"variant subtype boundary", `Function Run() As Long
 Dim n As Long, boxed As Variant
 n = 4
 If n > 2 Then n = n + 5
 boxed = n
 Run = VarType(boxed) * 100 + boxed
End Function`, 309},
		{"managed cleanup", `Function Run() As Long
 Dim values(0 To 1) As Long, item As Variant, total As Long
 values(0) = 3
 values(1) = 8
 For Each item In values
  total = total + item
 Next
 Run = total
End Function`, 11},
		{"double error write", `Function Run() As Long
 Dim value As Double, divisor As Double
 value = 7
 On Error Resume Next
 value = 1 / divisor
 Run = Err.Number
End Function`, 11},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := lower(t, test.source)
			text := emit(t, p, llvm.Options{})
			text += fmt.Sprintf(`
define i32 @main() {
 %%error = alloca %%vba.error
 %%out = alloca i32
 store i32 0, ptr %%out
 %%status = call i32 @vba.p1(ptr %%error, ptr %%out)
 %%value = load i32, ptr %%out
 %%ok.status = icmp eq i32 %%status, 0
 %%ok.value = icmp eq i32 %%value, %d
 %%ok = and i1 %%ok.status, %%ok.value
 %%exit = select i1 %%ok, i32 0, i32 1
 ret i32 %%exit
}
`, test.want)
			native(t, text)
		})
	}
}
