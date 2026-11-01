package llvm_test

import (
	"fmt"
	"sihlfall.ch/vambas-vai/llvm"
	"testing"
)

func TestNativeControlFlow(t *testing.T) {
	tests := []struct {
		name, source string
		want         int
	}{
		{"for positive", `Function Run() As Long
 Dim i As Long,n As Long
 For i=1 To 5
  n=n+i
 Next i
 Run=n+i*100
End Function`, 615},
		{"for negative", `Function Run() As Long
 Dim i As Integer,n As Long
 For i=5 To 1 Step -2
  n=n+i
 Next
 Run=n+i*100
End Function`, -91},
		{"for empty", `Function Run() As Long
 Dim i As Long
 For i=5 To 1
  Run=99
 Next
 Run=Run+i
End Function`, 5},
		{"for capture", `Function Run() As Long
 Dim i As Long,limit As Long,increment As Long,n As Long
 limit=3
 increment=1
 For i=1 To limit Step increment
  n=n+i
  limit=0
  increment=-1
 Next
 Run=n+i*10
End Function`, 46},
		{"for evaluation order", `Function Run() As Long
 Dim i As Long,n As Long
 i=5
 For i=Record(n,1) To Record(n,i) Step Record(n,1)
 Next
 Run=n*10+i
End Function
Function Record(ByRef n As Long,ByVal value As Long) As Long
 n=n*10+value
 Record=value
End Function`, 1516},
		{"while and if", `Function Run() As Long
 Dim i As Long,n As Long
 While i<5
  i=i+1
  If i<3 Then
   n=n+1
  ElseIf i=3 Then
   n=n+10
  Else
   n=n+100
  End If
 Wend
 Run=n
End Function`, 212},
		{"do variants", `Function Run() As Long
 Dim n As Long
 Do While n<2
  n=n+1
 Loop
 Do Until n=4
  n=n+1
 Loop
 Do
  n=n+1
 Loop While n<6
 Do
  n=n+1
 Loop Until n=8
 Do
  n=n+1
  Exit Do
 Loop
 Run=n
End Function`, 9},
		{"goto labels and line numbers", `Function Run() As Long
 Dim n As Long
 GoTo Start
100 Run=99
 Exit Function
Start: n=n+1
 If n<3 Then GoTo Start
 GoTo 200
 Run=-99
200 Run=n
End Function`, 3},
		{"exit for skips increment", `Function Run() As Long
 Dim i As Long
 For i=1 To 10
  If i=3 Then Exit For
 Next
 Run=i
End Function`, 3},
		{"nested for next list", `Function Run() As Long
 Dim i As Long,j As Long,n As Long
 For i=1 To 2
  For j=1 To 3
   n=n+1
  Next j,i
 Run=n+i*10+j*100
End Function`, 436},
		{"goto initialized for body", `Function Run() As Long
 Dim i As Long,n As Long
 For i=1 To 1
body: n=n+1
 Next
 If n=1 Then GoTo body
 Run=n*10+i
End Function`, 23},
		{"variant for", `Function Run() As Long
 Dim n As Long,i As Variant
 For i=1 To 4
  n=n+i
 Next
 Run=n+i
End Function`, 15},
		{"variant conditions and comparisons", `Function Run() As Long
 Dim v As Variant,n As Long
 v=0
 While v<4
  v=v+1
  If v>2 Then n=n+10
 Wend
 If Null Then n=999
 Do While Null
  n=999
 Loop
 Do Until Null
  n=n+1
  Exit Do
 Loop
 If "12"="12" Then n=n+1
 If "False" Then n=999
 Run=n
End Function`, 22},
		{"nested declarations and goto", `Function Run() As Long
 GoTo inside
 If False Then
  Dim n As Long
inside: n=n+7
 End If
 Run=n
End Function`, 7},
		{"managed loops and goto cleanup", `Function Run() As Long
 Dim i As Long,s As String
 For i=1 To 4
  s=s&ChrW(48+i)
  If i=3 Then GoTo done
 Next
done: Run=CLng(s)
End Function`, 123},
		{"zero step and body counter mutation", `Function Run() As Long
 Dim i As Long,n As Long
 For i=2 To 3 Step 0
  n=n+1
  If n=3 Then Exit For
 Next
 For i=1 To 5
  n=n+i
  i=i+2
 Next
 Run=n+i*10
End Function`, 78},
		{"exit outer do through for", `Function Run() As Long
 Dim i As Long,n As Long
 Do
  For i=1 To 3
   While True
    n=n+1
    Exit Do
   Wend
  Next
 Loop
 Run=n+i
End Function`, 2},
		{"variant for subtype promotion", `Function Run() As Long
 Dim i As Variant,n As Long
 For i=32767 To 32768
  n=n+VarType(i)
 Next
 Run=n+VarType(i)
End Function`, 8},
		{"variant for double promotion", `Function Run() As Long
 Dim i As Variant,n As Long,limit As Variant
 limit=CVar(2147483647)+1
 For i=2147483647 To limit
  n=n+1
 Next
 Run=n+VarType(i)
End Function`, 7},
		{"comparison declared types and numeric coercion", `Function Run() As Long
 Dim a As Variant,b As Variant,n As Long
 a=0
 b="0.5"
 If a<b Then n=n+1
 If 0=b Then n=n+10
 If "0"=a Then n=n+100
 Run=n
End Function`, 111},
		{"boolean expressions evaluate both operands", `Function Run() As Long
 Dim n As Long
 If False And Hit(n) Then n=99
 If True Or Hit(n) Then Run=n
End Function
Function Hit(ByRef n As Long) As Boolean
 n=n+1
 Hit=True
End Function`, 2},
		{"recursive branches", `Function Run() As Long
 Run=Factorial(5)
End Function
Function Factorial(ByVal n As Long) As Long
 If n<=1 Then
  Factorial=1
 Else
  Factorial=n*Factorial(n-1)
 End If
End Function`, 120},
		{"boolean bounds coerce numerically", `Function Run() As Long
 Dim i As Long,n As Long
 For i=0 To True Step True
  n=n+i
 Next
 Run=n+i*10
End Function`, -21},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeForEachArrays(t *testing.T) {
	tests := []struct {
		name, source string
		want         int
	}{
		{"array mutation and column order", `Function Run() As Long
 Dim a(1 To 2,1 To 2) As Long,v As Variant,n As Long
 a(1,1)=1
 a(2,1)=2
 a(1,2)=3
 a(2,2)=4
 For Each v In a
  n=n*10+v
  a(2,2)=5
 Next
 Run=n+v
End Function`, 1240},
		{"string elements and exit", `Function Run() As Long
 Dim a(2) As String,v As Variant,s As String
 a(0)="1"
 a(1)="2"
 a(2)="3"
 For Each v In a
  s=s&v
  If Len(s)=2 Then Exit For
 Next
 Run=CLng(s)+CLng(v)
End Function`, 14},
		{"unallocated array", `Function Run() As Long
 Dim a() As Long,v As Variant
 v=42
 For Each v In a
  Run=99
 Next
 Run=v
End Function`, 42},
		{"goto out releases array lock", `Function Run() As Long
 Dim a() As Long,v As Variant
 ReDim a(2)
 a(0)=7
 For Each v In a
  GoTo done
 Next
done: ReDim a(3)
 Run=v+UBound(a)
End Function`, 10},
		{"value collection owned lifetime", `Function Run() As Long
 Dim v As Variant,n As Long
 For Each v In MakeArray()
  n=n+v
 Next
 Run=n
End Function
Function MakeArray() As Long()
 Dim a(1 To 2) As Long
 a(1)=12
 a(2)=30
 MakeArray=a
End Function`, 42},
		{"nested scope exit", `Function Run() As Long
 Dim a() As Variant,b() As Long,v As Variant,w As Variant,n As Long
 ReDim a(1)
 ReDim b(1)
 b(0)=7
 For Each v In a
  For Each w In b
   n=w
   GoTo done
  Next
 Next
done: Erase a,b
 Run=n
End Function`, 7},
		{"return releases borrowed descriptor", `Function Run() As Long
 Dim a() As Long
 ReDim a(1)
 a(0)=7
 Run=First(a)
 ReDim a(4)
 Run=Run+UBound(a)
End Function
Function First(ByRef a() As Long) As Long
 Dim v As Variant
 For Each v In a
  First=v
  Exit Function
 Next
End Function`, 11},
		{"exit do unwinds enumeration", `Function Run() As Long
 Dim a() As Long,v As Variant
 ReDim a(1)
 a(0)=7
 Do
  For Each v In a
   Exit Do
  Next
 Loop
 ReDim a(4)
 Run=v+UBound(a)
End Function`, 11},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeControlFlowErrors(t *testing.T) {
	tests := []struct {
		name, body, helpers string
		status, observed    int
	}{
		{"next overflow", "Dim i As Integer\n For i=32767 To 32767\n observed=i\n Next", "", 6, 32767},
		{"goto for not initialized", "Dim i As Long\n GoTo body\n For i=1 To 3\nbody: observed=7\n Next", "", 92, 7},
		{"goto explicit counter still not initialized", "Dim i As Long\n GoTo body\n For i=1 To 3\nbody: i=2\n observed=i\n Next", "", 92, 2},
		{"invalid variant bounds after evaluation", "Dim i As Variant,limit As Variant\n limit=Null\n For i=1 To limit Step Mark(observed)\n Next", `Function Mark(ByRef observed As Long) As Long
 observed=7
 Mark=1
End Function`, 13, 7},
		{"enumeration resize locked", "Dim a() As Long,v As Variant\n ReDim a(1)\n For Each v In a\n observed=7\n ReDim a(3)\n Next", "", 10, 7},
		{"iteration error cleanup", "Dim a() As String,v As Variant,n As Integer\n ReDim a(1)\n a(0)=\"123\"\n For Each v In a\n observed=CLng(v)\n n=32767\n n=n+1\n Next", "", 6, 123},
		{"condition type mismatch", "Dim v As Variant\n v=\"invalid\"\n observed=7\n While v\n observed=99\n Wend", "", 13, 7},
		{"select eager upper bound failure", "observed=7\n Select Case 1\n Case 3 To Fail(observed)\n observed=99\n End Select", `Function Fail(ByRef observed As Long) As Long
 Dim n As Integer
 observed=8
 n=32767
 Fail=n+1
End Function`, 6, 8},
		{"select comparison type mismatch", "Dim v As Variant\n observed=7\n v=\"invalid\"\n Select Case v\n Case 1\n observed=99\n End Select", "", 13, 7},
		{"on goto negative", "observed=7\n On -1 GoTo done\ndone:", "", 5, 7},
		{"on goto over 255", "observed=7\n On 256 GoTo done\ndone:", "", 5, 7},
		{"on goto coercion overflow", "observed=7\n On 32768 GoTo done\ndone:", "", 6, 7},
		{"on goto null", "observed=7\n On Null GoTo done\ndone:", "", 94, 7},
		{"on goto invalid string", "observed=7\n On \"invalid\" GoTo done\ndone:", "", 13, 7},
		{"on goto boolean true is negative", "observed=7\n On True GoTo done\ndone:", "", 5, 7},
		{"on goto fallthrough retains lock", "Dim a() As Long,v As Variant\n ReDim a(1)\n For Each v In a\n observed=7\n On 0 GoTo done\n ReDim a(3)\n Next\ndone:", "", 10, 7},
		{"nested enumeration failure cleanup", "Dim a(1) As String,b() As Long,v As Variant,w As Variant\n a(0)=\"12\"\n ReDim b(1)\n For Each v In a\n For Each w In b\n observed=CLng(v)\n ReDim b(3)\n Next\n Next", "", 10, 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := emit(t, lower(t, "Sub Run(ByRef observed As Long)\n"+test.body+"\n observed=99\nEnd Sub\n"+test.helpers), llvm.Options{})
			text += fmt.Sprintf(`
define i32 @main() {
 %%error=alloca %%vba.error
 %%observed=alloca i32
 store i32 0,ptr %%observed
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
