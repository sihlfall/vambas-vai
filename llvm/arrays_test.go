package llvm_test

import (
	"fmt"
	"sihlfall.ch/vambas-vai/llvm"
	"strings"
	"testing"
)

func TestNativeArrays(t *testing.T) {
	tests := []struct {
		name, source string
		want         int
	}{
		{"resize and descriptor aliases", `Function Run() As Long
 Dim a() As Long
 ReDim a(1 To 3)
 a(1) = 10
 a(3) = 20
 Grow a(), a
 Run = a(1) + a(3) + a(4) + LBound(a) * 100 + UBound(a)
End Function
Sub Grow(ByRef first() As Long, ByRef second() As Long)
 ReDim Preserve first(1 To 5)
 second(4) = 12
End Sub`, 147},
		{"fixed multidimensional", `Function Run() As Long
 Dim a(1 To 2, -2 To 0) As Long
 a(1,-2) = 5
 a(2,-1) = 7
 Run = a(1,-2) + a(2,-1) + UBound(a,1)*100 + LBound(a,2)*10
End Function`, 192},
		{"string elements and aliases", `Function Run() As Long
 Dim a() As String, snapshot As String
 ReDim a(1 To 2)
 a(1) = "123"
 snapshot = a(1)
 Change a(1), a(1)
 ReDim Preserve a(1 To 3)
 Run = CLng(a(1)) + CLng(snapshot) + Len(a(3))
End Function
Sub Change(ByRef v As Variant, ByRef s As String)
 v = 45
 s = s & "6"
 Mid$(s,1,1) = "7"
 v = v & "8"
End Sub`, 7691},
		{"variant elements", `Function Run() As Long
 Dim a() As Variant
 ReDim a(2)
 a(1) = Null
 a(2) = "45"
 Change a(2)
 Run = VarType(a(0)) + CLng(IsNull(a(1))) + a(2)
End Function
Sub Change(ByRef v As Variant)
 v = CLng(v) + 1
End Sub`, 45},
		{"boxed array and element reference", `Function Run() As Long
 Dim a() As Long, b As Variant, c() As Long
 ReDim a(1)
 a(0) = 3
 b = a
 b(0) = 10
 Change b(0)
 c = b
 c(0) = 15
 Run = a(0)*100 + b(0)*10 + c(0)
End Function
Sub Change(ByRef v As Variant)
 v = v + 2
End Sub`, 435},
		{"variant byval and parenthesized arrays", `Function Run() As Long
 Dim a() As Long
 ReDim a(1)
 a(0) = 7
 CopyChange a
 Change (a)
 Run = a(0)
End Function
Sub CopyChange(ByVal v As Variant)
 v(0) = 99
End Sub
Sub Change(ByRef a() As Long)
 ReDim a(5)
 a(0) = 123
End Sub`, 7},
		{"typed array through byref variant", `Function Run() As Long
 Dim a() As Long
 ReDim a(1 To 2)
 a(1) = 20
 Forward a
 Run = a(1) + a(3) + UBound(a)
End Function
Sub Forward(ByRef v As Variant)
 Grow v
End Sub
Sub Grow(ByRef v As Variant)
 ReDim Preserve v(1 To 3)
 v(3) = 10
End Sub`, 33},
		{"variant redim", `Function Run() As Long
 Dim v As Variant
 ReDim v(-1 To 1) As String
 v(-1) = "12"
 ReDim Preserve v(-1 To 2)
 Run = CLng(v(-1)) + UBound(v) + CLng(IsArray(v)) + VarType(v)
End Function`, 8213},
		{"erase fixed and dynamic", `Function Run() As Long
 Dim a(1 To 2) As String, b() As Variant, v As Variant
 a(1) = "123"
 ReDim b(2)
 b(1) = "456"
 v = b
 Erase a, b, v
 ReDim b(3)
 Run = Len(a(1)) + UBound(a) + CLng(IsEmpty(b(1))) + CLng(IsEmpty(v))
End Function`, 1},
		{"option base and implicit redim declaration", `Option Base 1
Function Run() As Long
 ReDim a(3) As Long
 a(1) = 7
 Run = a(1) + LBound(a)*10 + UBound(a)
End Function`, 20},
		{"preserve last dimension", `Function Run() As Long
 Dim a() As Long
 ReDim a(-1 To 0, 2 To 3)
 a(-1,2) = 11
 a(0,3) = 22
 ReDim Preserve a(-1 To 0,2 To 5)
 Run = a(-1,2) + a(0,3) + a(0,5)
End Function`, 33},
		{"read releases descriptor lock", `Function Run() As Long
 Dim a() As Long
 ReDim a(1)
 a(0)=7
 Run = a(0) + Resize(a)
End Function
Function Resize(ByRef a() As Long) As Long
 ReDim a(5)
 Resize=UBound(a)
End Function`, 12},
		{"byval element and later redim", `Function Run() As Long
 Dim a() As String
 ReDim a(1)
 a(0)="123"
 Run = Observe(a(0),Resize(a))
End Function
Function Observe(ByVal snapshot As String,ByVal ignored As Long) As Long
 Observe=CLng(snapshot)
End Function
Function Resize(ByRef a() As String) As Long
 ReDim a(3)
 Resize=0
End Function`, 123},
		{"nested variant arrays", `Function Run() As Long
 Dim v As Variant, a() As Long
 ReDim a(1)
 a(0)=42
 ReDim v(1)
 v(0)=a
 Run=v(0)(0)
End Function`, 42},
		{"parenthesized element has no original lock", `Function Run() As Long
 Dim a() As Long
 ReDim a(1)
 a(0)=7
 Run = Observe((a(0)),Resize(a))
End Function
Function Observe(ByRef value As Long,ByVal ignored As Long) As Long
 value=value+1
 Observe=value
End Function
Function Resize(ByRef a() As Long) As Long
 ReDim a(3)
 Resize=0
End Function`, 8},
		{"array function results", `Function Run() As Long
 Dim a() As Long
 a=MakeArray()
 Run=a(0)
End Function
Function MakeArray() As Long()
 Dim a() As Long
 ReDim a(1)
 a(0)=42
 MakeArray=a
End Function`, 42},
		{"direct mid on string element", `Function Run() As Long
 Dim a(1 To 2) As String
 a(1)="123"
 a(2)="4"
 Mid$(a(1),2,1)="5"
 Mid$(a(2),1,1)="6"
 Run=CLng(a(1))+CLng(a(2))
End Function`, 159},
		{"boolean and integer element layouts", `Function Run() As Long
 Dim flags(1) As Boolean, numbers(1) As Integer
 flags(0)=True
 numbers(1)=12
 StepNumber numbers(1)
 ClearFlag flags
 Run=numbers(1)+CLng(flags(0))+VarType(flags)-8203
End Function
Sub StepNumber(ByRef v As Variant)
 v=v+1
End Sub
Sub ClearFlag(ByRef v As Variant)
 v(0)=0
End Sub`, 13},
		{"unallocated array type queries", `Function Run() As Long
 Dim a() As Long
 Run=CLng(IsArray(a))+VarType(a)-8195
End Function`, -1},
		{"erase retains variant array subtype", `Function Run() As Long
 Dim v As Variant
 ReDim v(2) As Long
 Erase v
 Run=VarType(v)+CLng(IsArray(v))+CLng(IsEmpty(v))
 ReDim v(1)
 Run=Run+VarType(v)
End Function`, 16389},
		{"erase fixed cells with live aliases", `Function Run() As Long
 Dim a(2) As String
 a(0)="123"
 Run=Reset(a(0),a)
End Function
Function Reset(ByRef cell As String,ByRef a() As String) As Long
 Erase a
 Reset=Len(cell)
 cell="456"
 Reset=Reset+CLng(a(0))
End Function`, 456},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeArrayAllocationFailurePreservesByRefMutation(t *testing.T) {
	text := emit(t, lower(t, `Sub Run(ByRef a() As Long)
 a(1)=43
 ReDim Preserve a(1 To 3)
End Sub`), llvm.Options{})
	text += `
@test.fail.next = internal global i1 false
define i32 @main() {
 %error=alloca %vba.error
 %array=alloca ptr
 %bounds=alloca [2 x i32]
 %index=alloca i32
 %element=alloca %vba.element
 store ptr null,ptr %array
 store [2 x i32] [i32 1,i32 2],ptr %bounds
 %initialized=call i32 @vba_array_redim(ptr %array,i32 3,i32 3,i32 0,i32 1,ptr %bounds)
 %before=load ptr,ptr %array
 store i1 true,ptr @test.fail.next
 %status=call i32 @vba.p1(ptr %error,ptr %array)
 %after=load ptr,ptr %array
 store i32 1,ptr %index
 %indexed=call i32 @vba_array_index(ptr %element,ptr %array,i32 3,i32 3,i32 1,ptr %index)
 %address=call ptr @vba_element_address(ptr %element,i32 3)
 %value=load i32,ptr %address
 call void @vba_element_clear(ptr %element)
 %ok1=icmp eq i32 %status,7
 %ok2=icmp eq ptr %before,%after
 %ok3=icmp eq i32 %value,43
 %ok4=or i32 %initialized,%indexed
 %ok5=icmp eq i32 %ok4,0
 %ok6=and i1 %ok1,%ok2
 %ok7=and i1 %ok3,%ok5
 %ok=and i1 %ok6,%ok7
 call void @vba_array_clear(ptr %array)
 %exit=select i1 %ok,i32 0,i32 1
 ret i32 %exit
}
`
	text = trackedStrings(text)
	text = strings.Replace(text, `define ptr @test.tracked.alloc(i64 %n) noinline {
entry:
  %p`, `define ptr @test.tracked.alloc(i64 %n) noinline {
entry:
  %fail = load i1, ptr @test.fail.next
  store i1 false, ptr @test.fail.next
  br i1 %fail, label %failure, label %allocate
failure:
  ret ptr null
allocate:
  %p`, 1)
	native(t, text)
}

func TestNativeArrayErrors(t *testing.T) {
	tests := []struct {
		name, body, helpers string
		status, observed    int
	}{
		{"unallocated", "Dim a() As Long\n observed=a(0)", "", 9, 7},
		{"out of bounds", "Dim a() As Long\n ReDim a(1 To 2)\n observed=a(0)", "", 9, 7},
		{"wrong rank", "Dim a() As Long\n ReDim a(2,3)\n observed=a(0)", "", 9, 7},
		{"invalid bounds", "Dim a() As Long\n ReDim a(2 To 1)", "", 9, 7},
		{"preserve lower bound", "Dim a() As Long\n ReDim a(1 To 2)\n ReDim Preserve a(0 To 3)", "", 9, 7},
		{"preserve nonlast bound", "Dim a() As Long\n ReDim a(2,3)\n ReDim Preserve a(3,4)", "", 9, 7},
		{"preserve rank", "Dim a() As Long\n ReDim a(2)\n ReDim Preserve a(2,3)", "", 9, 7},
		{"fixed array through parameter", "Dim a(2) As Long\n Resize a", `Sub Resize(ByRef a() As Long)
 ReDim a(3)
End Sub`, 10, 7},
		{"typed view cannot change element type", "Dim a() As Long\n ReDim a(2)\n Change a", `Sub Change(ByRef v As Variant)
 ReDim v(3) As String
End Sub`, 13, 7},
		{"preserve variant subtype", "Dim v As Variant\n ReDim v(2) As String\n ReDim Preserve v(3) As Long", "", 13, 7},
		{"element reference in callee", "Dim a() As Long\n ReDim a(2)\n a(0)=7\n Change a(0),a,observed", `Sub Change(ByRef value As Long,ByRef a() As Long,ByRef observed As Long)
 value=value+1
 observed=value
 ReDim a(3)
End Sub`, 10, 8},
		{"element reference during later argument", "Dim a() As Long\n ReDim a(2)\n a(0)=7\n Change a(0),Resize(a,observed)", `Sub Change(ByRef value As Long,ByVal unused As Long)
 value=99
End Sub
Function Resize(ByRef a() As Long,ByRef observed As Long) As Long
 observed=8
 ReDim a(3)
End Function`, 10, 8},
		{"assignment reference during RHS", "Dim a() As Long\n ReDim a(2)\n a(0)=Resize(a,observed)", `Function Resize(ByRef a() As Long,ByRef observed As Long) As Long
 observed=8
 ReDim a(3)
End Function`, 10, 8},
		{"erase while borrowed", "Dim a() As String\n ReDim a(2)\n a(0)=\"123\"\n Change a(0),a", `Sub Change(ByRef value As String,ByRef a() As String)
 value="456"
 Erase a
End Sub`, 10, 7},
		{"replace locked variant array", "Dim v As Variant\n ReDim v(2)\n Change v(0),v", `Sub Change(ByRef element As Variant,ByRef v As Variant)
 element=42
 v=Empty
End Sub`, 10, 7},
		{"dimension zero", "Dim a() As Long\n ReDim a(2)\n observed=UBound(a,0)", "", 9, 7},
		{"array to scalar", "Dim a() As Long\n ReDim a(2)\n observed=CLng(a)", "", 13, 7},
		{"erase nonarray variant", "Dim v As Variant\n Erase v", "", 13, 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := emit(t, lower(t, "Sub Run(ByRef observed As Long)\n observed=7\n"+test.body+"\n observed=99\nEnd Sub\n"+test.helpers), llvm.Options{})
			text += fmt.Sprintf(`
define i32 @main() {
 %%error=alloca %%vba.error
 %%observed=alloca i32
 store i32 0, ptr %%observed
 %%status=call i32 @vba.p1(ptr %%error,ptr %%observed)
 %%got=load i32,ptr %%observed
 %%a=icmp eq i32 %%status,%d
 %%b=icmp eq i32 %%got,%d
 %%ok=and i1 %%a,%%b
 %%exit=select i1 %%ok,i32 0,i32 1
 ret i32 %%exit
}
`, test.status, test.observed)
			native(t, trackedStrings(text))
		})
	}
}
