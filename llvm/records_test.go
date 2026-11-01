package llvm_test

import (
	"fmt"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/llvm"
	"sihlfall.ch/vambas-vai/lowering"
)

func TestNativeUDT(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         int
	}{
		{"numeric fields and aliases", `Type Point
 X As Long
 Y As Double
End Type
Function Run() As Long
 Dim p As Point,q As Point
 p.X=2:p.Y=3.5
 q=p
 MovePoint p,p.X
 Run=p.X+q.X+CLng(q.Y*10#)
End Function
Sub MovePoint(ByRef p As Point,ByRef x As Long)
 p.X=p.X+1
 x=x+2
End Sub`, 42},
		{"nested managed copies and with", `Type Child
 Text As String
 Value As Variant
End Type
Type Parent
 Nested As Child
 Count As Long
End Type
Function Run() As Long
 Dim a As Parent,b As Parent
 With a
  .Nested.Text="abc"
  .Nested.Value=7
  .Count=3
 End With
 b=a
 Mid$(a.Nested.Text,1,1)="x"
 a.Nested.Value=99
 If b.Nested.Text<>"abc" Then Exit Function
 Run=CLng(b.Nested.Value)+Len(b.Nested.Text)+b.Count+29
End Function`, 42},
		{"returns and value receiver", `Type Item
 Text As String
 Value As Long
End Type
Function Run() As Long
 Dim a As Item
 a=MakeItem(20)
 Run=a.Value+MakeItem(22).Value
End Function
Function MakeItem(ByVal n As Long) As Item
 MakeItem.Text="payload"
 MakeItem.Value=n
End Function`, 42},
		{"fixed dynamic and variant arrays copied", `Type Buffer
 Fixed(1 To 2) As String
 Values() As Double
 Extra As Variant
End Type
Function Run() As Long
 Dim a As Buffer,b As Buffer,item As Variant
 a.Fixed(1)="abc":a.Fixed(2)="defg"
 ReDim a.Values(1 To 2)
 a.Values(1)=10#:a.Values(2)=20#
 a.Extra=a.Values
 b=a
 a.Fixed(1)="changed"
 a.Values(1)=99#
 For Each item In b.Extra
  Run=Run+item
 Next
 Run=Run+Len(b.Fixed(1))+Len(b.Fixed(2))+UBound(b.Values)+3
End Function`, 42},
		{"forced values and forwarded record parameters", `Type State
 N As Long
 Text As String
End Type
Function Run() As Long
 Dim s As State
 s.N=40:s.Text="abc"
 Change (s)
 ForwardState s
 Run=s.N
End Function
Sub ForwardState(ByRef s As State)
 Change s
End Sub
Sub Change(ByRef s As State)
 s.N=s.N+2
 Mid$(s.Text,1,1)="x"
End Sub`, 42},
		{"aggregate replacement keeps field aliases", `Type State
 N As Long
 Text As String
End Type
Function Run() As Long
 Dim a As State,b As State
 a.Text="old":b.Text="new":b.N=30
 ReplaceState a,b,a.N,a.Text
 If a.Text="updated" Then Run=a.N+1
End Function
Sub ReplaceState(ByRef a As State,ByRef b As State,ByRef n As Long,ByRef text As Variant)
 a=b
 If text<>"new" Then Exit Sub
 n=n+11
 text="updated"
End Sub`, 42},
		{"field counters and nested byref records", `Type Inner
 Index As Long
 Item As Variant
End Type
Type Outer
 Child As Inner
 Values(0 To 2) As Long
End Type
Function Run() As Long
 Dim a As Outer
 With a
  For .Child.Index=0 To 2
   .Values(.Child.Index)=.Child.Index+1
  Next
  For Each .Child.Item In .Values
   Run=Run+.Child.Item
  Next
 End With
 AddValue a.Child
 Run=Run+a.Child.Index+32
End Function
Sub AddValue(ByRef a As Inner)
 a.Index=a.Index+1
End Sub`, 42},
		{"nested record field assignment", `Type Inner
 N As Long
 Text As String
End Type
Type Outer
 Child As Inner
End Type
Function Run() As Long
 Dim a As Outer,b As Inner
 b.N=42:b.Text="hello"
 a.Child=b
 b.N=99
 Run=a.Child.N
End Function`, 42},
		{"initialization precedes skipped dim", `Type State
 Values(2 To 4) As Long
 Text As String
End Type
Function Run() As Long
 GoTo body
 Dim s As State
body: Run=UBound(s.Values)+LBound(s.Values)+Len(s.Text)+36
End Function`, 42},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeUDTErrors(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"locked array blocks record replacement", `Dim v As Variant
 ReDim a.Values(1)
 For Each v In a.Values
  a=b
 Next`, 10},
		{"fixed field cannot redim through alias", `GrowArray a.Fixed`, 10},
		{"failure releases nested payloads", `Dim n As Integer
 a.Text="payload"
 ReDim a.Values(10)
 a.Extra=a.Values
 n=32767
 n=n+1`, 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `Type State
 Text As String
 Values() As Long
 Fixed(1) As Long
 Extra As Variant
End Type
Sub Run(ByRef observed As Long)
 Dim a As State,b As State
 observed=7
` + test.body + "\n observed=99\nEnd Sub\nSub GrowArray(ByRef a() As Long)\n ReDim a(4)\nEnd Sub"
			text := emit(t, lower(t, source), llvm.Options{})
			text += fmt.Sprintf(`
define i32 @main() {
 %%error=alloca %%vba.error
 %%out=alloca i32
 store i32 0,ptr %%out
 %%s=call i32 @vba.p1(ptr %%error,ptr %%out)
 %%n=load i32,ptr %%out
 %%a=icmp eq i32 %%s,%d
 %%b=icmp eq i32 %%n,7
 %%ok=and i1 %%a,%%b
 %%exit=select i1 %%ok,i32 0,i32 1
 ret i32 %%exit
}`, test.status)
			native(t, trackedStrings(text))
		})
	}
}

func TestNativeEnums(t *testing.T) {
	variantResult(t, `Enum Mode
 First=-2
 Second
 Mask=2 ^ 4 + 2
 Rounded=2.5
 MinValue=-2147483648
End Enum
Type State
 Current As Mode
End Type
Function Run() As Long
 Dim s As State,values(1) As Mode
 s.Current=Mode.Second
 SetMode s.Current
 values(0)=s.Current
 Select Case values(0)
 Case Mode.Mask: Run=40
 Case Else: Exit Function
 End Select
 Run=Run+Mode.Rounded
End Function
Sub SetMode(ByRef value As Mode)
 value=Mode.Mask
End Sub`, 42)
}

func TestNativeUDTAndEnumAcrossModules(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
		{FileName: "Main.bas", Text: `Option Explicit
Function Run() As Long
 Dim p As Model.Point
 p=Model.CreatePoint(Model.Ready)
 Model.Advance p
 Run=p.X
End Function`},
		{FileName: "Model.bas", Text: `Option Explicit
Public Enum Phase
 Ready=40
End Enum
Public Type Point
 X As Phase
 Text As String
End Type
Public Function CreatePoint(ByVal n As Phase) As Point
 CreatePoint.X=n
 CreatePoint.Text="model"
End Function
Public Sub Advance(ByRef p As Point)
 p.X=p.X+2
End Sub`},
	}})
	semantic, diagnostics := lowering.Lower(p)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	text := emit(t, semantic, llvm.Options{})
	text += `
define i32 @main() {
 %error=alloca %vba.error
 %out=alloca i32
 %s=call i32 @vba.p1(ptr %error,ptr %out)
 %n=load i32,ptr %out
 %a=icmp eq i32 %s,0
 %b=icmp eq i32 %n,42
 %ok=and i1 %a,%b
 %exit=select i1 %ok,i32 0,i32 1
 ret i32 %exit
}`
	native(t, trackedStrings(text))
}
