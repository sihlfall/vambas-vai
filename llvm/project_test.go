package llvm_test

import (
	"fmt"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
	"sihlfall.ch/vambas-vai/lowering"
)

// A project outlives individual procedure activations. Explicit clear is the
// private embedding boundary; trackedStrings verifies no owners survive it.
func projectResult(t *testing.T, p *ir.Program, wants ...int) {
	t.Helper()
	var main strings.Builder
	main.WriteString("\ndefine i32 @main() {\n %e=alloca %vba.error\n %out=alloca i32\n")
	for i, want := range wants {
		fmt.Fprintf(&main, ` %%s%d=call i32 @vba.p1(ptr %%e,ptr %%out)
 %%v%d=load i32,ptr %%out
 %%a%d=icmp eq i32 %%s%d,0
 %%b%d=icmp eq i32 %%v%d,%d
 %%ok%d=and i1 %%a%d,%%b%d
`, i, i, i, i, i, i, want, i, i, i)
		if i == 0 {
			main.WriteString(" %all0=and i1 true,%ok0\n")
		} else {
			fmt.Fprintf(&main, " %%all%d=and i1 %%all%d,%%ok%d\n", i, i-1, i)
		}
	}
	main.WriteString(" call void @vba.project.clear()\n")
	fmt.Fprintf(&main, ` %%reset=call i32 @vba.p1(ptr %%e,ptr %%out)
 %%reset.value=load i32,ptr %%out
 %%reset.status.ok=icmp eq i32 %%reset,0
 %%reset.value.ok=icmp eq i32 %%reset.value,%d
 %%reset.ok=and i1 %%reset.status.ok,%%reset.value.ok
 %%ok=and i1 %%all%d,%%reset.ok
 call void @vba.project.clear()
 %%exit=select i1 %%ok,i32 0,i32 1
 ret i32 %%exit
}
`, wants[0], len(wants)-1)
	native(t, trackedStrings(emit(t, p, llvm.Options{})+main.String()))
}

func TestNativeModuleGlobals(t *testing.T) {
	for _, test := range []struct {
		name, source string
		wants        []int
	}{
		{"scalar identity and aliasing", `Private counter As Long
Private flag As Boolean
Function Run() As Long
 If Not flag Then counter=40
 flag=True
 Bump counter,counter
 Run=counter
End Function
Sub Bump(ByRef a As Long,ByRef b As Long)
 a=a+1:b=b+1
End Sub`, []int{42, 44, 46}},
		{"strings variants arrays and record fields persist", `Private text As String
Private values() As Long
Private boxed As Variant
Private initialized As Boolean
Private state As StateType
Type StateType
 Text As String
 Fixed(1 To 2) As String
End Type
Function Run() As Long
 If Not initialized Then
  text="AB":boxed="12"
  ReDim values(1 To 2)
  values(1)=40
  With state
   .Text="xy"
   .Fixed(1)="fixed"
  End With
  initialized=True
 End If
 Mid$(text,2)="C"
 ChangeText text
 values(1)=values(1)+1
 If text<>"AC" Or state.Fixed(1)<>"fixed" Then Exit Function
 If state.Text<>"xy" Then Exit Function
 Run=values(1)+Len(text)-1
End Function
Sub ChangeText(ByRef v As Variant)
 v="AC"
End Sub`, []int{42, 43, 44}},
		{"fixed global arrays option base erase and no reinitialization", `Option Base 1
Private values(2, -1 To 1) As Long
Private n As Long
Function Run() As Long
 If LBound(values)<>1 Or UBound(values,2)<>1 Then Exit Function
 values(1,-1)=values(1,-1)+1
 n=n+1
 If values(1,-1)<>n Then Exit Function
 Run=values(1,-1)+41
End Function`, []int{42, 43, 44}},
	} {
		t.Run(test.name, func(t *testing.T) { projectResult(t, lower(t, test.source), test.wants...) })
	}
}

func TestNativeNumericConstantsAndOptionalDefaults(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"constants scopes widths and floating", `Private Const Base As Long=&H100&
Private Const Enabled As Boolean=7
Private Const S As Single=1.0000000596046448!
Function Run() As Long
 Const Mask As Long=Base Or 3
 Const Rounded As Integer=3.5
 Const Negative As Integer=&HFFFF
 Const Twice As Double=2 ^ 3 / 2
 If Mask<>259 Or Rounded<>4 Or Negative<>-1 Or Twice<>4 Then Exit Function
 If S<=1! Then Exit Function
 Run=CLng(Enabled)+43
End Function`},
		{"typed defaults omitted positional and named", `Private Const Answer As Long=40+2
Function Run() As Long
 If Defaults()<>42 Then Exit Function
 If Defaults(,2)<>44 Then Exit Function
 If Defaults(count:=3,flag:=False)<>45 Then Exit Function
 Run=Defaults()
End Function
Function Defaults(Optional ByVal flag As Boolean=True,Optional ByVal count As Long=0,Optional ByVal s As String="a" & "b") As Long
 If Len(s)<>2 Then Exit Function
 Defaults=Answer+count
End Function`},
		{"byref defaults are fresh writable temporaries", `Function Run() As Long
 Dim s As String,n As Long
 s="caller":n=7
 If Change()<>42 Then Exit Function
 If Change()<>42 Then Exit Function
 If Change(s,n)<>8 Then Exit Function
 If s<>"changed" Or n<>8 Then Exit Function
 Run=42
End Function
Function Change(Optional ByRef s As String="default",Optional ByRef n As Long=41) As Long
 n=n+1:s="changed"
 Change=n
End Function`},
		{"typed implicit defaults and variant explicit versus missing", `Function Run() As Long
 If ZeroDefaults()<>42 Then Exit Function
 If CheckVariant()<>42 Then Exit Function
 If CheckVariant(1,Null)<>0 Then Exit Function
 Run=42
End Function
Function ZeroDefaults(Optional ByVal n As Long,Optional ByVal b As Boolean,Optional ByVal s As String) As Long
 If n=0 And b=False And Len(s)=0 Then ZeroDefaults=42
End Function
Function CheckVariant(Optional missing As Variant,Optional supplied As Variant=7) As Long
 If IsMissing(missing) And Not IsMissing(supplied) Then CheckVariant=supplied+35
End Function`},
		{"rounding and ByRef expression temporaries", `Function Run() As Long
 Dim b As Boolean,n As Long,i As Integer
 b=2:n=b:i=b
 If n<>-1 Or i<>-1 Then Exit Function
 b=0
 If b Then Exit Function
 b=-32768:n=b
 If n<>-1 Then Exit Function
 n=41
 Call DefaultInteger((n))
 If n<>41 Or DefaultInteger()<>42 Then Exit Function
 If DefaultFloat()<>42 Then Exit Function
 Run=42
End Function
Function DefaultInteger(Optional ByRef n As Integer=41.5) As Long
 DefaultInteger=n:n=7
End Function
Function DefaultFloat(Optional ByVal n As Single=42.25) As Long
 DefaultFloat=n
End Function`},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, 42) })
	}
}

func TestNativeGlobalsAndConstantsAcrossModules(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
		{FileName: "Main.bas", Text: `Option Explicit
Function Run() As Long
 Data.Increment Data.Count
 With Data.State
  .Value=Data.PublicValue
 End With
 Run=Data.Count+Data.State.Value
End Function`},
		{FileName: "Data.bas", Text: `Option Explicit
Public Count As Long
Public State As Item
Public Const PublicValue As Long=41
Private Const Hidden As Long=999
Type Item
 Value As Long
End Type
Public Sub Increment(ByRef n As Long,Optional ByVal stepValue As Long=1)
 n=n+stepValue
End Sub`},
	}})
	semantic, ds := lowering.Lower(p)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	projectResult(t, semantic, 42, 43)
}

func TestNativeGlobalInitializationFailureAndReset(t *testing.T) {
	text := emit(t, lower(t, `Private fixed(1 To 2) As Long
Private state As StateType
Type StateType
 Tag As Long
 First(1 To 2) As String
 Second(1 To 2) As Long
End Type
Function Run() As Long
 If state.Tag=0 Then state.Tag=41
 state.Tag=state.Tag+1
 fixed(1)=state.Tag
 Run=fixed(1)
End Function`), llvm.Options{})
	text += `
@test.fail.after = internal global i64 -1
define i32 @main() {
entry:
 %e=alloca %vba.error
 %out=alloca i32
 br label %loop
loop:
 %stage=phi i64 [0,%entry],[%next,%success]
 store i32 123,ptr %out
 store i64 %stage,ptr @test.fail.after
 %s=call i32 @vba.p1(ptr %e,ptr %out)
 %v=load i32,ptr %out
 %statusok=icmp eq i32 %s,7
 %outok=icmp eq i32 %v,123
 %live=load i64,ptr @test.live
 %clean=icmp eq i64 %live,0
 %a=and i1 %statusok,%outok
 %b=and i1 %a,%clean
 br i1 %b,label %retry,label %failure
retry:
 store i64 -1,ptr @test.fail.after
 %s2=call i32 @vba.p1(ptr %e,ptr %out)
 %v2=load i32,ptr %out
 %s3=call i32 @vba.p1(ptr %e,ptr %out)
 %v3=load i32,ptr %out
 %sok=or i32 %s2,%s3
 %c=icmp eq i32 %sok,0
 %d=icmp eq i32 %v2,42
 %e2=icmp eq i32 %v3,43
 %f=and i1 %c,%d
 %g=and i1 %f,%e2
 call void @vba.project.clear()
 br i1 %g,label %success,label %failure
success:
 %next=add i64 %stage,1
 %done=icmp eq i64 %next,6
 br i1 %done,label %finish,label %loop
failure:
 call void @vba.project.clear()
 ret i32 1
finish:
 ret i32 0
}
`
	text = trackedStrings(text)
	text = strings.Replace(text, `define ptr @test.tracked.alloc(i64 %n) noinline {
entry:
  %p`, `define ptr @test.tracked.alloc(i64 %n) noinline {
entry:
  %at = load i64, ptr @test.fail.after
  %fail = icmp eq i64 %at, 0
  %positive = icmp sgt i64 %at, 0
  %less = sub i64 %at, 1
  %next = select i1 %positive, i64 %less, i64 -1
  store i64 %next, ptr @test.fail.after
  br i1 %fail, label %failure, label %allocate
failure:
  ret ptr null
allocate:
  %p`, 1)
	native(t, text)
}

func TestNativeGlobalAndOptionalWritesSurviveFailure(t *testing.T) {
	text := emit(t, lower(t, `Private saved As String
Private count As Long
Sub Run(ByRef observed As Long,Optional ByRef temporary As String="old")
 saved="persistent":count=42
 temporary="changed"
 observed=17
 observed=1\0
End Sub
Function Check() As Long
 If saved="persistent" Then Check=count
End Function
Sub Wrapper(ByRef observed As Long)
 Run observed
End Sub`), llvm.Options{})
	text += `
define i32 @main() {
 %e=alloca %vba.error
 %n=alloca i32
 %out=alloca i32
 store i32 0,ptr %n
 %status=call i32 @vba.p3(ptr %e,ptr %n)
 %n1=load i32,ptr %n
 %check=call i32 @vba.p2(ptr %e,ptr %out)
 %v=load i32,ptr %out
 %a=icmp eq i32 %status,11
 %b=icmp eq i32 %n1,17
 %c=icmp eq i32 %check,0
 %d=icmp eq i32 %v,42
 %ab=and i1 %a,%b
 %cd=and i1 %c,%d
 %ok=and i1 %ab,%cd
 call void @vba.project.clear()
 %exit=select i1 %ok,i32 0,i32 1
 ret i32 %exit
}
`
	native(t, trackedStrings(text))
}
