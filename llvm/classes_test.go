package llvm_test

import (
	"fmt"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
	"sihlfall.ch/vambas-vai/lowering"
)

func lowerClasses(t *testing.T, sources ...compiler.SourceFileInput) *ir.Program {
	t.Helper()
	p, ds := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: sources}))
	if len(ds) != 0 {
		t.Fatalf("class lowering: %+v", ds)
	}
	return p
}

func classResult(t *testing.T, p *ir.Program, want int) {
	t.Helper()
	cleanup := ""
	if len(p.Globals) > 0 || ir.UsesErrors(p) {
		cleanup = "call void @vba.project.clear()"
	}
	harness := fmt.Sprintf(`
define i32 @main() {
 %%e = alloca %%vba.error
 %%out = alloca i32
 %%status = call i32 @vba.p1(ptr %%e, ptr %%out)
 %%value = load i32, ptr %%out
 %%ok.status = icmp eq i32 %%status, 0
 %%ok.value = icmp eq i32 %%value, %d
 %%ok = and i1 %%ok.status, %%ok.value
 %s
 %%exit = select i1 %%ok, i32 0, i32 1
 ret i32 %%exit
}
`, want, cleanup)
	native(t, trackedStrings(emit(t, p, llvm.Options{})+harness))
}

func TestNativeClassCounterAcceptance(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Public Function Main() As Long
 Dim counter As Counter
 Set counter = New Counter
 counter.Increment
 Main = counter.Value
End Function`},
		compiler.SourceFileInput{FileName: "Counter.cls", Text: `Private count As Long
Public Sub Increment()
 count = count + 1
End Sub
Public Property Get Value() As Long
 Value = count
End Property`})
	classResult(t, p, 1)
}

func TestNativeClassIdentityFieldsAndProperties(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Function Main() As Long
 Dim a As Counter, b As Counter, alias As Counter, peer As Peer
 Set a = New Counter
 Set b = New Counter
 Set alias = a
 a.Increment
 Check a.Value = 1
 Check b.Value = 0
 Check a Is alias
 Check Not (a Is b)
 Set alias = a.Self()
 Check a Is alias
 a.Value = 10
 Check a.Sum(b:=4) = 15
 Check a.Total(3, 4) = 7
 Check Make().Value = 0
 a.Raw = 4
 Bump a.Raw
 Bump a.Value
 Check a.Raw = 5
 Check a.Value = 10
 a.Item(2) = 17
 Check a.Item(2) = 17
 Check b.Item(2) = 0
 Set b.CopyFrom = a
 Check b.Value = 10
 Set peer = New Peer
 Set alias = a.Through(peer, a)
 Check alias Is a
 a.Text = "first" & ChrW(33)
 a.Payload = a.Text
 a.Buffer(1) = 11
 Check CStr(a.Payload) = "first!"
 Check a.Buffer(1) = 11
 Check a.EmptyTextLength = 0
 Set a = Nothing
 Check alias.Value = 10
 Set b = Nothing
 Set alias = Nothing
 Set a = New Counter
 a.Value = 42
 Check a.ReadWith(Clear(a)) = 42
 Check a Is Nothing
 Set a = New Counter
 a.Text = "borrowed" & ChrW(33)
 Borrow a.Text, a
 Check a Is Nothing
 Set a = New Counter
 a.Value = 9
 Check a.ReadAfterClear(a) = 9
 Check a Is Nothing
 Check NullFailure(1) = 91
 Check NullFailure(2) = 91
 Check NullFailure(3) = 91
End Function
Sub Check(ByVal condition As Boolean)
 If Not condition Then Err.Raise 513
End Sub
Sub Bump(ByRef n As Long)
 n = n + 1
End Sub
Function Make() As Counter
 Set Make = New Counter
End Function
Function Clear(ByRef owner As Counter) As Long
 Set owner = Nothing
End Function
Sub Borrow(ByRef text As String, ByRef owner As Counter)
 Set owner = Nothing
 text = text & "alive"
 Check text = "borrowed!alive"
End Sub
Function NullFailure(ByVal mode As Long) As Long
 On Error GoTo failed
 Dim c As Counter, n As Long
 If mode = 1 Then c.Increment
 If mode = 2 Then n = c.Raw
 If mode = 3 Then c.Value = 3
 Exit Function
failed:
 NullFailure = Err.Number
End Function`},
		compiler.SourceFileInput{FileName: "Counter.cls", Text: `Option Explicit
Private count As Long
Public Raw As Long
Public Text As String
Public Payload As Variant
Private numbers(2 To 3) As Long
Public Buffer(1 To 2) As Long
Private emptyText As String
Public Sub Increment()
 AddOne
End Sub
Private Sub AddOne()
 Value = Value + 1
End Sub
Public Property Get Value() As Long
 Value = Me.count
End Property
Public Property Let Value(ByVal nextValue As Long)
 count = nextValue
End Property
Public Property Get Item(ByVal index As Long) As Long
 Item = numbers(index)
End Property
Public Property Let Item(ByVal index As Long, ByVal value As Long)
 numbers(index) = value
End Property
Public Property Set CopyFrom(ByVal source As Counter)
 count = source.Value
End Property
Public Function Self() As Counter
 Set Self = Me
End Function
Public Function Through(ByVal peer As Peer, ByVal value As Counter) As Counter
 Set Through = peer.Echo(value)
End Function
Public Function ReadWith(ByVal ignored As Long) As Long
 ReadWith = count
End Function
Public Function ReadAfterClear(ByRef owner As Counter) As Long
 Set owner = Nothing
 ReadAfterClear = count
End Function
Public Property Get EmptyTextLength() As Long
 EmptyTextLength = Len(emptyText)
End Property
Public Function Sum(Optional ByVal a As Long = 1, Optional ByVal b As Long = 2) As Long
 Sum = count + a + b
End Function
Public Function Total(ParamArray values() As Variant) As Long
 Dim value As Variant
 For Each value In values
  Total = Total + CLng(value)
 Next
End Function`},
		compiler.SourceFileInput{FileName: "Peer.cls", Text: `Public Function Echo(ByVal value As Counter) As Counter
 Set Echo = value
End Function`})
	classResult(t, p, 0)
}

func TestNativeClassManagedFieldsOnError(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Type State
 Text As String
 Values() As String
End Type
Function Main() As Long
 On Error GoTo failed
 Dim owner As C
 Set owner = New C
 owner.Fail owner
 Exit Function
failed:
 If owner Is Nothing Then Main = Err.Number
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Private text As String
Private payload As Variant
Private values() As String
Private state As State
Private index As Long
Public Sub Fail(ByRef owner As C)
 text = "owned" & ChrW(33)
 payload = text
 ReDim values(1 To 2)
 values(1) = text
 state.Text = text
 ReDim state.Values(1 To 2)
 state.Values(2) = text
 For index = 1 To 2
  values(index) = values(index) & "!"
 Next
 Set owner = Nothing
 Err.Raise 37
End Sub`})
	classResult(t, p, 37)
}

func TestNativeClassNominalCastsAndBuiltins(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim a As C, b As C, value As Object, items As New Collection
 Set a = New C
 Set value = a
 Set b = value
 If Not (a Is b) Then Main = 1: Exit Function
 items.Add a
 Set a = Nothing
 Set b = items(1)
 If Not (value Is b) Then Main = 2: Exit Function
 If BadCast(1) <> 13 Then Main = 3: Exit Function
 If BadCast(2) <> 13 Then Main = 4: Exit Function
 Set value = Nothing
 Set b = value
 If Not (b Is Nothing) Then Main = 5
End Function
Function BadCast(ByVal mode As Long) As Long
 On Error GoTo failed
 Dim value As Object, a As C
 If mode = 1 Then Set value = New Other
 If mode = 2 Then Set value = New Collection
 Set a = value
 Exit Function
failed:
 BadCast = Err.Number
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: ""},
		compiler.SourceFileInput{FileName: "Other.cls", Text: ""})
	classResult(t, p, 0)
}

func TestNativeClassBorrowedFieldScopes(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Type State
 Text As String
End Type
Function Main() As Long
 Dim owner As C, item As Variant, text As String
 Set owner = New C
 owner.Values(1) = "a"
 owner.Values(2) = "b"
 For Each item In owner.Values
  Set owner = Nothing
  text = text & CStr(item)
 Next
 If text <> "ab" Then Main = 1: Exit Function
 Set owner = New C
 owner.State.Text = "held"
 With owner.State
  Set owner = Nothing
  .Text = .Text & "!"
  If .Text <> "held!" Then Main = 2: Exit Function
 End With
 Set owner = New C
 owner.States(1).Text = "element"
 With owner.States(1)
  Set owner = Nothing
  If .Text <> "element" Then Main = 3
 End With
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Public Values(1 To 2) As String
Public State As State
Public States(1 To 2) As State`})
	classResult(t, p, 0)
}
