package llvm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/llvm"
	"testing"
)

func TestNativeObjectLifecycleAndLazyInstances(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Public initialized As Long
Public terminated As Long
Public saved As Reviving
Function Main() As Long
 Dim lazy As New C
 If initialized <> 0 Then Main=1:Exit Function
 If VarType(lazy) <> 9 Then Main=15:Exit Function
 If lazy.Value <> 42 Then Main=2:Exit Function
 If initialized <> 1 Then Main=3:Exit Function
 Set lazy = Nothing
 If terminated <> 1 Then Main=4:Exit Function
 If lazy Is Nothing Then Main=5:Exit Function
 If initialized <> 2 Then Main=6:Exit Function
 Dim alias As C
 Set alias = lazy
 Set lazy = Nothing
 If terminated <> 1 Then Main=7:Exit Function
 Set alias = Nothing
 If terminated <> 2 Then Main=8:Exit Function
 If Singleton.Value <> 42 Then Main=9:Exit Function
 If initialized <> 3 Then Main=10:Exit Function
 Dim r As Reviving
 Set r = New Reviving
 Set r = Nothing
 If saved Is Nothing Then Main=11:Exit Function
 If saved.Value <> 7 Then Main=12:Exit Function
 If terminated <> 3 Then Main=13:Exit Function
 Set saved = Nothing
 If terminated <> 3 Then Main=14
 Dim nextValue As New C
 If Not Packed(nextValue) Then Main=16
End Function
Function Packed(ParamArray values() As Variant) As Boolean
 Packed=Not (values(0) Is Nothing)
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Public Value As Long
Private Sub Class_Initialize()
 initialized=initialized+1
 Value=42
End Sub
Private Sub Class_Terminate()
 terminated=terminated+1
End Sub`},
		compiler.SourceFileInput{FileName: "Singleton.cls", Text: `Attribute VB_PredeclaredId = True
Public Value As Long
Private Sub Class_Initialize()
 initialized=initialized+1:Value=42
End Sub`},
		compiler.SourceFileInput{FileName: "Reviving.cls", Text: `Public Value As Long
Private Sub Class_Initialize()
 Value=7
End Sub
Private Sub Class_Terminate()
 terminated=terminated+1
 Set saved=Me
End Sub`})
	classResult(t, p, 0)
}

func TestNativeDynamicClassInvocation(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim o As Object, v As Variant, child As C
 Set o=New C
 o.Add 5
 Check CLng(o.Value)=5
 o.Value=10
 Check CLng(o.Sum(b:=4,a:=2))=16
 o.Item(2)=30
 Check CLng(o.Item(2))=30
 Set v=o
 v.Add 1
 Check CLng(v(2))=31
 Dim n As Long
 n=10
 o.Bump n
 Check n=11
 Check CLng(o.Missing())=1
 Check CLng(o.Total(2,3,4))=9
 Set o.Child=New C
 Set child=o.Child
 Check child.Value=0
 Check VarType(v)=9
 Check v Is o
 With o
  Set o=Nothing
  .Value=7
  Check CLng(.Value)=7
 End With
 Check CLng(v)=7
 Check CLng(v.Read(ClearVariant(v)))=7
 Check v Is Nothing
 Dim c As C
 Set c=New C
 c.Value=12
 n=c
 Check n=12
End Function
Sub Check(ByVal condition As Boolean)
 If Not condition Then Err.Raise 513
End Sub
Function ClearVariant(ByRef value As Variant) As Long
 Set value=Nothing
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Private count As Long
Public Child As C
Public Sub Add(ByVal n As Long)
 count=count+n
End Sub
Public Function Read(ByVal ignored As Long) As Long
 Read=count
End Function
Public Property Get Value() As Long
 Value=count
End Property
Public Property Let Value(ByVal n As Long)
 count=n
End Property
Public Property Get Item(Optional ByVal index As Long = 0) As Long
Attribute Item.VB_UserMemId = 0
 Item=count+index
End Property
Public Property Let Item(Optional ByVal index As Long = 0, ByVal value As Long)
 count=value-index
End Property
Public Function Sum(Optional ByVal a As Long = 1,Optional ByVal b As Long = 2) As Long
 Sum=count+a+b
End Function
Public Sub Bump(ByRef n As Long)
 n=n+1
End Sub
Public Function Missing(Optional value As Variant) As Long
 If IsMissing(value) Then Missing=1
End Function
Public Function Total(ParamArray values() As Variant) As Long
 Dim v As Variant
 For Each v In values
  Total=Total+CLng(v)
 Next
End Function`})
	classResult(t, p, 0)
}

func TestNativeInitializationFailurePreservesErrorAndCleansFields(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Public terminated As Long
Function Main() As Long
 On Error GoTo failed
 Dim owner As Bad
 Set owner=New Bad
 Main=1:Exit Function
failed:
 If Err.Number <> 37 Then Main=2:Exit Function
 If Err.Description <> "init failed" Then Main=3:Exit Function
 If Err.Source <> "Bad.init" Then Main=4:Exit Function
 If terminated <> 2 Then Main=5:Exit Function
 If Not (owner Is Nothing) Then Main=6
End Function`},
		compiler.SourceFileInput{FileName: "Bad.cls", Text: `Private text As String
Private child As Child
Private Sub Class_Initialize()
 text="owned" & ChrW(33)
 Set child=New Child
 Err.Raise 37,"Bad.init","init failed"
End Sub
Private Sub Class_Terminate()
 terminated=terminated+1
 Err.Clear
End Sub`},
		compiler.SourceFileInput{FileName: "Child.cls", Text: `Private Sub Class_Terminate()
 On Error Resume Next
 terminated=terminated+1
 Err.Raise 39
End Sub`})
	classResult(t, p, 0)
}

func TestNativeObjectVariantsFieldsAndByRef(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim c As C, old As C, v As Variant, values(0 To 1) As Variant
 Set c=New C
 Set old=c
 Set v=c
 Check v Is c
 Set values(0)=v
 Set c=Nothing
 Set c=values(0)
 Check c Is old
 Set c.Child=New C
 Set v=c.Child
 Check v Is c.Child
 ChangeAsObject c
 Check Not (c Is old)
 Set old=c
 Check AdaptedError(c)=37
 Check c Is old
 Check ViewError(c)=13
 Check c Is old
 Dim dynamic As Object
 Set dynamic=c
 dynamic.Change c
 Check Not (c Is old)
 Set old=c
 Check DynamicError(dynamic,c)=37
 Check c Is old
 c=9
 Check c.Value=9
 Set dynamic=c
 dynamic=11
 Check c.Value=11
 Set v=c
 Check CLng(v+1)=12
 Check CBool(v=11)
 Check CStr(v)="11"
End Function
Sub Check(ByVal condition As Boolean)
 If Not condition Then Err.Raise 513
End Sub
Sub ChangeAsObject(ByRef value As Object)
 Set value=New C
End Sub
Function AdaptedError(ByRef value As C) As Long
 On Error GoTo failed
 ChangeAndFail value
 Exit Function
failed:
 AdaptedError=Err.Number
End Function
Sub ChangeAndFail(ByRef value As Object)
 Set value=New C
 Err.Raise 37
End Sub
Function ViewError(ByRef value As C) As Long
 On Error GoTo failed
 ChangeVariant value
 Exit Function
failed:
 ViewError=Err.Number
End Function
Sub ChangeVariant(ByRef value As Variant)
 Set value=New Other
End Sub
Function DynamicError(ByVal target As Object,ByRef value As C) As Long
 On Error GoTo failed
 target.ChangeAndFail value
 Exit Function
failed:
 If Err.Source <> "C.method" Then DynamicError=98:Exit Function
 If Err.Description <> "method failed" Then DynamicError=99:Exit Function
 DynamicError=Err.Number
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Private count As Long
Public Child As C
Public Property Get Value() As Long
Attribute Value.VB_UserMemId = 0
 Value=count
End Property
Public Property Let Value(ByVal n As Long)
 count=n
End Property
Public Sub Change(ByRef value As Object)
 Set value=New C
End Sub
Public Sub ChangeAndFail(ByRef value As Object)
 Set value=New C
 Err.Raise 37,"C.method","method failed"
End Sub`},
		compiler.SourceFileInput{FileName: "Other.cls", Text: ""})
	classResult(t, p, 0)
}

func TestUnhandledTerminationCannotReachCallerHandler(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 On Error Resume Next
 Dim c As C
 Set c=New C
 Set c=Nothing
 Main=1
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Private Sub Class_Terminate()
 Err.Raise 37
End Sub`})
	dir := t.TempDir()
	source := filepath.Join(dir, "terminate.ll")
	if err := os.WriteFile(source, []byte(emit(t, p, llvm.Options{Entry: 1})), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, tool(t, "llvm-as"), source, "-o", filepath.Join(dir, "terminate.bc"))
	for _, level := range []string{"-O0", "-O2"} {
		binary := filepath.Join(dir, "terminate"+level)
		args := append([]string{level, source}, runtimeSources(t)...)
		command(t, tool(t, "clang"), append(args, "-lm", "-o", binary)...)
		out, err := exec.Command(binary).CombinedOutput()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 37 {
			t.Fatalf("termination was caught by the caller: %v %s", err, out)
		}
	}
}

func TestNativeLazyFieldsDynamicPublicArraysAndInvalidReceivers(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim o As Object, child As C
 Set o=New Holder
 Set child=o.Child
 If child.Value<>42 Then Main=1:Exit Function
 Set o.Child=Nothing
 If CLng(o.Child.Value)<>42 Then Main=2:Exit Function
 o.Values(1)=17
 If CLng(o.Values(1))<>17 Then Main=3:Exit Function
 If Probe(1)<>424 Then Main=4:Exit Function
 If Probe(2)<>424 Then Main=5:Exit Function
 If Probe(3)<>91 Then Main=6:Exit Function
End Function
Function Probe(ByVal mode As Long) As Long
 On Error GoTo failed
 Dim v As Variant, o As Object, flag As Boolean
 v=1
 If mode=1 Then flag=v Is Nothing
 If mode=2 Then
  With v
   .Missing
  End With
 End If
 If mode=3 Then o.Missing
 Exit Function
failed:
 Probe=Err.Number
End Function`},
		compiler.SourceFileInput{FileName: "Holder.cls", Text: `Public Child As New C
Public Values(1 To 2) As Long`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Public Value As Long
Private Sub Class_Initialize()
 Value=42
End Sub`})
	classResult(t, p, 0)
}

func TestNativeDynamicInvocationKinds(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Main() As Long
 Dim o As Object, child As Other, result As Other
 Set o=New C
 o.P=5
 If CLng(o.LetCount)<>5 Then Main=1:Exit Function
 Set child=New Other
 Set o.P=child
 If CLng(o.SetCount)<>1 Then Main=2:Exit Function
 Set result=o.P
 If Not (result Is child) Then Main=3:Exit Function
 If Probe(o)<>450 Then Main=4
End Function
Function Probe(ByVal o As Object) As Long
 On Error GoTo failed
 Dim n As Variant
 n=o.SubOnly()
 Exit Function
failed:
 Probe=Err.Number
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Public LetCount As Long
Public SetCount As Long
Private child As Object
Public Property Get P() As Variant
 Set P=child
End Property
Public Property Let P(ByVal n As Long)
 LetCount=n
End Property
Public Property Set P(ByVal value As Object)
 SetCount=SetCount+1
 Set child=value
End Property
Public Sub SubOnly()
End Sub`},
		compiler.SourceFileInput{FileName: "Other.cls", Text: ""})
	classResult(t, p, 0)
}

func TestNativeRecordObjectReplacementIsVisibleDuringTermination(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Type State
 Child As C
End Type
Public current As State
Public observed As Long
Function Main() As Long
 Dim nextValue As State, old As C, replacement As C
 Set old=New C
 old.ID=1
 Set current.Child=old
 Set old=Nothing
 Set replacement=New C
 replacement.ID=42
 Set nextValue.Child=replacement
 current=nextValue
 If observed<>42 Then Main=1
End Function`},
		compiler.SourceFileInput{FileName: "C.cls", Text: `Public ID As Long
Private Sub Class_Terminate()
 If Not (current.Child Is Nothing) Then observed=current.Child.ID
End Sub`})
	classResult(t, p, 0)
}

func TestNativeSetEvaluatesValueBeforeTarget(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Public typed As Holder
Public late As Object
Function Main() As Long
 Dim previous As Holder
 Set typed=New Holder
 Set previous=typed
 Set typed.Child=MoveTyped()
 If Not (previous.Child Is Nothing) Then Main=1:Exit Function
 If typed.Child Is Nothing Then Main=2:Exit Function
 Set previous=typed
 Set typed.P=MoveTyped()
 If Not (previous.P Is Nothing) Then Main=3:Exit Function
 If typed.P Is Nothing Then Main=4:Exit Function
 Set late=New Holder
 Set previous=late
 Set late.Child=MoveLate()
 If Not (previous.Child Is Nothing) Then Main=5:Exit Function
 If late.Child Is Nothing Then Main=6:Exit Function
 Set previous=late
 Set late.P=MoveLate()
 If Not (previous.P Is Nothing) Then Main=7:Exit Function
 If late.P Is Nothing Then Main=8
End Function
Function MoveTyped() As C
 Set typed=New Holder
 Set MoveTyped=New C
End Function
Function MoveLate() As C
 Set late=New Holder
 Set MoveLate=New C
End Function`},
		compiler.SourceFileInput{FileName: "Holder.cls", Text: `Public Child As C
Private value As C
Public Property Get P() As C
 Set P=value
End Property
Public Property Set P(ByVal rhs As C)
 Set value=rhs
End Property`},
		compiler.SourceFileInput{FileName: "C.cls", Text: ""})
	classResult(t, p, 0)
}
