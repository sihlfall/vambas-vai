package llvm_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"testing"
)

func TestNativeObjectArraysOwnershipAndFields(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Private Type Tree
 children(0 To 1) As C
 objects() As Object
End Type
Private globalItems(2 To 3) As C
Function Main() As Long
 Dim a() As C,b() As C,v As Variant,node As Tree,counter As Variant,selected As C
 ReDim a(-1 To 1)
 Set a(-1)=New C
 a(-1).Value=5
 b=a
 b(-1).Value=7
 If a(-1).Value<>7 Then Exit Function
 If Not a(-1) Is b(-1) Then Exit Function
 ReDim Preserve a(-1 To 3)
 If Not a(3) Is Nothing Then Exit Function
 Erase b
 Set node.children(0)=a(-1)
 Set globalItems(2)=node.children(0)
 ReDim node.objects(0 To 1)
 Set node.objects(0)=globalItems(2)
 Set node.objects(1)=New Collection
 v=a
 If Not IsArray(v) Or VarType(v)<>8192+vbObject Then Exit Function
 Set v(-1)=New C
 v(-1).Value=12
 If a(-1).Value<>7 Then Exit Function
 For Each counter In v
  If Not counter Is Nothing Then Main=Main+counter.Value
 Next
 If Main<>12 Then Main=0:Exit Function
 For Each selected In v
  If Not selected Is Nothing Then
   If selected.Value<>12 Then Main=0:Exit Function
  End If
 Next
 Erase v
 ClearAndUse node.children(0),node.children
 If node.children(0).Value<>25 Then Main=0:Exit Function
 If globalItems(2).Value<>7 Then Main=0:Exit Function
 Main=42
End Function
Sub ClearAndUse(ByRef item As C,ByRef values() As C)
 Erase values
 If Not item Is Nothing Then Err.Raise 37
 Set item=New C
 item.Value=25
End Sub`},
		compiler.SourceFileInput{FileName: "C.cls", Text: "Public Value As Long\n"})
	classResult(t, p, 42)
}

func TestNativeObjectArraysNominalVariantsAndInvocation(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim a() As C,other() As Other,v As Variant,late As Object,c As C
 ReDim a(0 To 1)
 Set a(0)=New C
 a(0).Value=7
 v=a
 On Error Resume Next
 Set v(0)=New Other
 If Err.Number<>13 Then Exit Function
 Err.Clear
 If v(0).Value<>7 Then Exit Function
 ReDim other(0 To 1)
 Set other(0)=New Other
 other=v
 If Err.Number<>13 Then Exit Function
 Err.Clear
 If other(0) Is Nothing Then Exit Function
 Mutate a
 If a(0).Value<>11 Then Exit Function
 Set late=New Holder
 late.Items=a
 Set late.Items(0)=New Other
 If Err.Number<>13 Then Exit Function
 Err.Clear
 late.Change a
 If a(0).Value<>21 Then Exit Function
 late.Change other
 If Err.Number<>13 Then Exit Function
 Err.Clear
 a=late.Clone
 If a(0).Value<>21 Then Exit Function
 Erase a
 v=a
 other=v
 If Err.Number<>13 Then Exit Function
 Err.Clear
 a=v
 If Err.Number<>0 Then Exit Function
 On Error GoTo 0
 Main=42
End Function
Sub Mutate(ByRef values() As C)
 values(0).Value=11
End Sub`},
		compiler.SourceFileInput{FileName: "C.cls", Text: "Public Value As Long\n"},
		compiler.SourceFileInput{FileName: "Other.cls", Text: "Public Value As Long\n"},
		compiler.SourceFileInput{FileName: "Holder.cls", Text: `Public Items() As C
Public Sub Change(ByRef values() As C)
 values(0).Value=21
End Sub
Public Function Clone() As C()
 Clone=Items
End Function`})
	classResult(t, p, 42)
}

func TestNativeObjectArrayBoundsViewsAndSubtypes(t *testing.T) {
	p := lowerClasses(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim a() As C,b() As Collection,objects() As Object,v As Variant,element As Variant
 ReDim a(1 To 2,-1 To 1)
 Set a(2,0)=New C
 a(2,0).Value=17
 ReDim Preserve a(1 To 2,-1 To 3)
 If a(2,0).Value<>17 Or LBound(a,2)<>-1 Or UBound(a,2)<>3 Then Exit Function
 On Error Resume Next
 ReDim Preserve a(0 To 2,-1 To 3)
 If Err.Number<>9 Then Exit Function
 Err.Clear
 TryRetype a
 If Err.Number<>13 Then Exit Function
 Err.Clear
 LockAndResize a(2,0),a
 If Err.Number<>10 Then Exit Function
 Err.Clear
 v=a
 On Error GoTo 0
 ReDim v(0 To 1) As Object
 objects=v
 Set objects(0)=New C
 Set objects(1)=New Collection
 ReDim b(0 To 1)
 Set b(0)=New Collection
 b(0).Add 42
 v=b
 If VarType(v)<>8201 Then Exit Function
 On Error Resume Next
 Set v(0)=New C
 If Err.Number<>13 Then Exit Function
 Err.Clear
 b=v
 If Err.Number<>0 Then Exit Function
 If CLng(b(0).Item(1))<>42 Then Exit Function
 On Error GoTo 0
 Main=42
End Function
Sub TryRetype(ByRef value As Variant)
 ReDim value(0 To 1) As Object
End Sub
Sub LockAndResize(ByRef cell As C,ByRef a() As C)
 ReDim a(0 To 1)
End Sub`}, compiler.SourceFileInput{FileName: "C.cls", Text: "Public Value As Long\n"})
	classResult(t, p, 42)
}
