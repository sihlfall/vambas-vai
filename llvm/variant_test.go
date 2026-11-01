package llvm_test

import (
	"fmt"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

func variantResult(t *testing.T, source string, want int) {
	t.Helper()
	text := emit(t, lower(t, source), llvm.Options{})
	text += fmt.Sprintf(`
define i32 @main() {
  %%error = alloca %%vba.error
  %%out = alloca i32
  store i32 0, ptr %%out
  %%status = call i32 @vba.p1(ptr %%error, ptr %%out)
  %%value = load i32, ptr %%out
  %%ok1 = icmp eq i32 %%status, 0
  %%ok2 = icmp eq i32 %%value, %d
  %%ok = and i1 %%ok1, %%ok2
  %%exit = select i1 %%ok, i32 0, i32 1
  ret i32 %%exit
}
`, want)
	native(t, trackedStrings(text))
}

func TestNativeVariantValues(t *testing.T) {
	tests := []struct {
		name, source string
		want         int
	}{
		{"empty", `Function Run() As Long
 Dim v As Variant
 Run = VarType(v) + CLng(IsEmpty(v)) + CLng(v)
End Function`, -1},
		{"null", `Function Run() As Long
 Dim v As Variant, w As Variant
 v = Null
 w = v + 1
 Run = VarType(w) + CLng(IsNull(w)) + CLng(IsEmpty(w))
End Function`, 0},
		{"retag", `Function Run() As Long
 Dim v As Variant, copy As Variant, a As Long
 v = 42
 a = VarType(v)
 copy = v
 v = "abc" & vbNullChar & ChrW(55357)
 Run = a * 100 + VarType(v) * 10 + Len(v) + CLng(copy)
End Function`, 327},
		{"byref", `Function Run() As Long
 Dim n As Long
 n = 1
 Change n, n
 Run = n
End Function
Sub Change(ByRef a As Variant, ByRef b As Long)
 a = "21"
 b = b + 2
 a = a + b
End Sub`, 46},
		{"temporary", `Function Run() As Long
 Dim n As Long, v As Variant
 n = 9
 v = 7
 Change (n)
 CopyChange v
 Run = n * 10 + v
End Function
Sub Change(ByRef a As Variant)
 a = "retagged"
End Sub
Sub CopyChange(ByVal a As Variant)
 a = "private"
End Sub`, 97},
		{"forward", `Function Run() As Long
 Dim n As Integer, v As Variant
 n = 4
 v = 5
 Forward n
 Forward v
 Run = n * 10 + v + VarType(v)
End Function
Sub Forward(ByRef v As Variant)
 Change v
End Sub
Sub Change(ByRef v As Variant)
 v = v + 1
End Sub`, 58},
		{"promotion", `Function Run() As Long
 Dim a As Variant, b As Variant
 a = 32767
 a = a + 1
 b = 2147483647
 b = b + 1
 Run = VarType(a) * 10 + VarType(b)
End Function`, 35},
		{"length", `Function Run() As Long
 Dim v As Variant
 v = 123
 Run = Len(v) * 10 + LenB(v)
End Function`, 36},
		{"boolean subtype", `Function Run() As Long
 Dim v As Variant, b As Boolean
 b = True
 v = b
 Run = VarType(v) * 10 + CLng(v)
End Function`, 109},
		{"untyped results", `Function Run() As Long
 Run = CLng(Add(1,2))
End Function
Function Add(a, b)
 Add = a + b
End Function`, 3},
		{"variant aliases retag", `Function Run() As Long
 Dim v As Variant
 v = 1
 Change v, v
 Run = CLng(v) * 10 + VarType(v)
End Function
Sub Change(ByRef a As Variant, ByRef b As Variant)
 a = "2"
 b = b & "3"
 a = CLng(a) + 1
End Sub`, 243},
		{"rounding", `Function Run() As Long
 Run = CInt("2.5") * 10 + CInt("3.5")
End Function`, 24},
		{"missing", `Function Run() As Long
 Run = MissingValue() * 100 + PresentValue(Empty) * 10 + MissingForward()
End Function
Function MissingValue(Optional ByVal v As Variant) As Long
 MissingValue = VarType(v) + CLng(IsMissing(v))
End Function
Function PresentValue(Optional v As Variant) As Long
 PresentValue = CLng(IsMissing(v)) + CLng(IsEmpty(v))
End Function
Function MissingForward(Optional v As Variant) As Long
 MissingForward = MissingValue(v)
 v = "retagged missing temporary"
End Function`, 899},
		{"snapshot", `Function Run() As Long
 Dim v As Variant
 v = "123"
 Run = Observe(v, Replace(v)) + v
End Function
Function Observe(ByVal snapshot As Variant, ByVal ignored As Long) As Long
 snapshot = snapshot & "4"
 Observe = snapshot
End Function
Function Replace(ByRef v As Variant) As Long
 v = "7"
 Replace = 0
End Function`, 1241},
		{"string return from exposed result", `Function Run() As Long
 Run = CLng(MakeString())
End Function
Function MakeString() As String
 MakeString = "123"
 Change MakeString
End Function
Sub Change(ByRef v As Variant)
 v = "456"
End Sub`, 456},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeVariantFailurePreservesTypedWrites(t *testing.T) {
	for _, test := range []struct {
		name, typ, nativeType, bad string
		error                      int
	}{
		{"mismatch", "Long", "i32", `"invalid"`, 13},
		{"overflow", "Integer", "i16", `32768`, 6},
		{"null", "Long", "i32", `Null`, 94},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := emit(t, lower(t, fmt.Sprintf(`Sub Run(ByRef n As %s)
 Change n
 n = 99
End Sub
Sub Change(ByRef v As Variant)
 v = v + 1
 v = %s
End Sub`, test.typ, test.bad)), llvm.Options{})
			text += fmt.Sprintf(`
define i32 @main() {
 %%error = alloca %%vba.error
 %%n = alloca %[1]s
 store %[1]s 7, ptr %%n
 %%status = call i32 @vba.p1(ptr %%error, ptr %%n)
 %%n.value = load %[1]s, ptr %%n
 %%ok1 = icmp eq i32 %%status, %[2]d
 %%ok2 = icmp eq %[1]s %%n.value, 8
 %%origin.ptr = getelementptr %%vba.error, ptr %%error, i32 0, i32 1
 %%origin = load i32, ptr %%origin.ptr
 %%ok3 = icmp eq i32 %%origin, 2
 %%ok4 = and i1 %%ok1, %%ok2
 %%ok = and i1 %%ok4, %%ok3
 %%exit = select i1 %%ok, i32 0, i32 1
 ret i32 %%exit
}
`, test.nativeType, test.error)
			native(t, trackedStrings(text))
		})
	}
}

func TestNativeVariantStringAliases(t *testing.T) {
	variantResult(t, `Function Run() As Long
 Dim s As String, snapshot As String, v As Variant, n As Long
 s = "123"
 snapshot = s
 v = s
 Change s, s
 n = CLng(s) + Len(s) + CLng(snapshot) + Len(v)
 s = "9"
 Change s, s
 Run = n + CLng(s) + Len(s) + Len(v)
End Function
Sub Change(ByRef a As Variant, ByRef b As String)
 a = "45"
 b = b & "6"
 Mid$(b, 1, 1) = "7"
 a = a & "8"
End Sub`, 15273)
}
