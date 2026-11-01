package llvm_test

import (
	"fmt"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
	"testing"
)

func interfaceResult(t *testing.T, p *ir.Program, want int) {
	t.Helper()
	harness := fmt.Sprintf(`
@interface.format = private constant [47 x i8] c"status=%%d result=%%d proc=%%d stmt=%%d offset=%%d\0A\00"
declare i32 @printf(ptr, ...)
define i32 @main() {
 %%e = alloca %%vba.error
 store %%vba.error zeroinitializer, ptr %%e
 %%out = alloca i32
 store i32 0, ptr %%out
 %%status = call i32 @vba.p1(ptr %%e, ptr %%out)
 %%value = load i32, ptr %%out
 %%error = load %%vba.error, ptr %%e
 %%proc = extractvalue %%vba.error %%error, 1
 %%stmt = extractvalue %%vba.error %%error, 2
 %%offset = extractvalue %%vba.error %%error, 3
 %%ok.status = icmp eq i32 %%status, 0
 %%ok.value = icmp eq i32 %%value, %d
 %%ok = and i1 %%ok.status, %%ok.value
 br i1 %%ok, label %%done, label %%failure
failure:
 call i32 (ptr, ...) @printf(ptr @interface.format, i32 %%status, i32 %%value, i32 %%proc, i32 %%stmt, i32 %%offset)
 br label %%done
done:
 call void @vba.project.clear()
 %%exit = select i1 %%ok, i32 0, i32 1
 ret i32 %%exit
}
`, want)
	native(t, trackedStrings(emit(t, p, llvm.Options{})+harness))
}

func TestNativeImplementsMethodsPropertiesFieldsAndArrays(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Function Main() As Long
 Dim concrete As Worker, first As IWork, second As IOther, data As IData
 Dim boxed As Variant, untyped As Object, items() As IWork, count As Long
 Set concrete = New Worker
 Set first = concrete
 Set second = first
 Set data = first
 Check first Is concrete
 Check second Is first
 Check first.Evaluate(amount:=2) = 42
 Check second.Evaluate(2) = 102
 Check first.Evaluate() = 43
 first.Bump count
 Check count = 1
 Check CLng(first.Pack(20, 22)) = 42
 Check first.Self Is concrete
 second.Item(3) = 39
 Check second.Item(3) = 42
 Check second(3) = 42
 data.Label = "hello" & ChrW(33)
 Check data.Label = "hello!"
 BumpText data.Label
 Check data.Label = "hello!"
 Set data.Peer = concrete
 Check data.Peer Is first
 data.Payload = 42
 Check CLng(data.Payload) = 42
 Set data.Payload = concrete
 Set untyped = data.Payload
 Check untyped Is concrete
 Set data.Peer = Nothing
 data.Payload = Empty
 Set data = New IData
 data.Label = "fallback"
 Check data.Label = "fallback"
 data.Payload = 42
 Check CLng(data.Payload) = 42
 Set boxed = first
 Set first = boxed
 Check first.Evaluate(2) = 42
 ReDim items(1 To 2)
 Set items(1) = concrete
 ReDim Preserve items(1 To 3)
 Check items(1).Evaluate(2) = 42
 boxed = items
 Set first = boxed(1)
 Check first.Self Is concrete
 Check concrete.Evaluate(2) = 202
 Set first = New IWork
 Check first.Evaluate(2) = 302
 Set first = concrete
 On Error Resume Next
 Set first = New Other
 Check Err.Number = 13
 Err.Clear
 Check first Is concrete
 Set first = Nothing
 count = first.Evaluate(1)
 Check Err.Number = 91
 Err.Clear
 On Error GoTo 0
 Main = 42
End Function
Sub Check(ByVal ok As Boolean)
 If Not ok Then Err.Raise 5
End Sub
Sub BumpText(ByRef text As String)
 text = "changed"
End Sub`},
		compiler.SourceFileInput{FileName: "IWork.cls", Text: `Public Function Evaluate(Optional ByVal amount As Long = 3) As Long
 Evaluate = amount + 300
End Function
Public Sub Bump(ByRef count As Long)
End Sub
Public Function Pack(ParamArray values() As Variant) As Variant
End Function
Public Property Get Self() As IWork
End Property`},
		compiler.SourceFileInput{FileName: "IOther.cls", Text: `Public Function Evaluate(ByVal amount As Long) As Long
End Function
Public Property Get Item(ByVal index As Long) As Long
Attribute Item.VB_UserMemId = 0
End Property
Public Property Let Item(ByVal index As Long, value As Long)
End Property`},
		compiler.SourceFileInput{FileName: "IData.cls", Text: `Public Label As String
Public Peer As Object
Public Payload As Variant`},
		compiler.SourceFileInput{FileName: "Other.cls", Text: `Public Value As Long`},
		compiler.SourceFileInput{FileName: "Worker.cls", Text: `Implements IWork
Implements IOther
Implements IData
Private text As String
Private peerValue As Object
Private payloadValue As Variant
Private itemValue As Long
Private Function IWork_Evaluate(Optional ByVal value As Long = 1 + 2) As Long
 IWork_Evaluate = value + 40
End Function
Private Sub IWork_Bump(ByRef value As Long)
 value = value + 1
End Sub
Private Function IWork_Pack(ParamArray args() As Variant) As Variant
 IWork_Pack = args(0) + args(1)
End Function
Private Property Get IWork_Self() As IWork
 Set IWork_Self = Me
End Property
Private Function IOther_Evaluate(ByVal value As Long) As Long
 IOther_Evaluate = value + 100
End Function
Private Property Get IOther_Item(ByVal index As Long) As Long
 IOther_Item = itemValue + index
End Property
Private Property Let IOther_Item(ByVal index As Long, ByVal rhs As Long)
 itemValue = rhs
End Property
Private Property Get IData_Label() As String
 IData_Label = text
End Property
Private Property Let IData_Label(ByVal rhs As String)
 text = rhs
End Property
Private Property Get IData_Peer() As Object
 Set IData_Peer = peerValue
End Property
Private Property Set IData_Peer(ByVal rhs As Object)
 Set peerValue = rhs
End Property
Private Property Get IData_Payload() As Variant
 If VarType(payloadValue) = vbObject Then
  Set IData_Payload = payloadValue
 Else
  IData_Payload = payloadValue
 End If
End Property
Private Property Let IData_Payload(ByVal rhs As Variant)
 payloadValue = rhs
End Property
Private Property Set IData_Payload(ByVal rhs As Variant)
 Set payloadValue = rhs
End Property
Public Function Evaluate(ByVal amount As Long) As Long
 Evaluate = amount + 200
End Function`})
	interfaceResult(t, p, 42)
}

func TestNativeImplementsDefaultValuesByRefErrorsAndTermination(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Private destroyed As Long
Private root As IValue
Function Main() As Long
 Dim concrete As Worker, value As IValue, other As Alternate, boxed As Variant, count As Long
 Dim values() As IValue, emptyView As IEmpty
 Set concrete = New Worker
 Set value = concrete
 Set emptyView = concrete
 Check emptyView Is concrete
 Set emptyView = Nothing
 Set root = concrete
 Check CLng(value) = 42
 Check CStr(value) = "42"
 Check root.Value = 42
 ReDim values(1 To 1)
 value.Fill values
 Check values(1) Is concrete
 boxed = values
 Check VarType(boxed) = vbArray + vbObject
 Erase values
 boxed = Empty
 Set other = New Alternate
 On Error Resume Next
 Replace concrete, other
 Check Err.Number = 13
 Err.Clear
 Check concrete Is value
 value.Fail count
 Check Err.Number = 123
 Check Err.Description = "interface failure"
 Check count = 7
 Err.Clear
 On Error GoTo 0
 Replace value, other
 Check value.Value = 99
 Check Not (value Is concrete)
 Set value = Nothing
 Set root = Nothing
 Check destroyed = 0
 Set concrete = Nothing
 Check destroyed = 1
 Set value = New IValue
 Check value.Value = 1
 Check value.Value = 2
 Main = 42
End Function
Sub Check(ByVal ok As Boolean)
 If Not ok Then Err.Raise 5
End Sub
Sub Replace(ByRef value As IValue, ByVal replacement As IValue)
 Set value = replacement
End Sub
Public Sub RecordTermination()
 destroyed = destroyed + 1
End Sub
Public Sub Increment(ByRef count As Long)
 count = count + 1
End Sub`},
		compiler.SourceFileInput{FileName: "IValue.cls", Text: `Private backing As Long
Public Property Get Value() As Long
Attribute Value.VB_UserMemId = 0
 Increment Me.backing
 Value = backing
End Property
Public Sub Fail(ByRef count As Long)
End Sub
Public Sub Fill(ByRef values() As IValue)
End Sub`},
		compiler.SourceFileInput{FileName: "IEmpty.cls", Text: ""},
		compiler.SourceFileInput{FileName: "Worker.cls", Text: `Implements IValue
Implements IEmpty
Private Property Get IValue_Value() As Long
 IValue_Value = 42
End Property
Private Sub IValue_Fail(ByRef count As Long)
 count = 7
 Err.Raise 123, "worker", "interface failure"
End Sub
Private Sub IValue_Fill(ByRef values() As IValue)
 Set values(1) = Me
End Sub
Private Sub Class_Terminate()
 RecordTermination
End Sub`},
		compiler.SourceFileInput{FileName: "Alternate.cls", Text: `Implements IValue
Private Property Get IValue_Value() As Long
 IValue_Value = 99
End Property
Private Sub IValue_Fail(ByRef count As Long)
End Sub
Private Sub IValue_Fill(ByRef values() As IValue)
 Set values(1) = Me
End Sub`})
	interfaceResult(t, p, 42)
}
