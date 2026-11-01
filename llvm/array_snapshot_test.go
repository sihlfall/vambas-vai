package llvm_test

import (
	"regexp"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
)

// Each check returns 0, or the number of its first failed condition. Every
// one would observe a stale snapshot if a reload were missing.
const snapshotChecks = `Option Explicit

Private Type Holder
    v() As Long
End Type

Public g() As Long

Private Sub GrowGlobal(ByVal n As Long)
    ReDim Preserve g(1 To n)
End Sub

Private Sub Alias(a() As Long, b() As Long)
    Dim x As Long
    x = a(0)
    ReDim Preserve b(0 To 5)
    a(5) = x + 7
End Sub

Private Sub Both(ByRef x As Long, ByRef a() As Long)
    ReDim a(0 To 1)
End Sub

Private Sub GrowThenFail(a() As Long)
    ReDim a(1 To 8)
    Err.Raise 5
End Sub

' The failed call's error edge carries a stale snapshot into the handler.
Private Function FailedCallee() As Long
    Dim a() As Long
    ReDim a(1 To 2)
    On Error GoTo Handler
    GrowThenFail a
    FailedCallee = 1
    Exit Function
Handler:
    If UBound(a) <> 8 Then FailedCallee = 2: Exit Function
    a(8) = 3
    If a(8) <> 3 Then FailedCallee = 3
End Function

Private Function LoopResize() As Long
    Dim a() As Long, i As Long, s As Long
    ReDim a(1 To 2)
    For i = 1 To 5
        ReDim Preserve a(1 To i + 2)
        a(i + 2) = i
        s = s + UBound(a) + a(i + 2)
    Next
    If s <> 40 Then LoopResize = 1
End Function

Private Function GlobalCallee() As Long
    Dim i As Long, s As Long
    ReDim g(1 To 1)
    For i = 2 To 6
        GrowGlobal i
        g(i) = i
        s = s + g(UBound(g))
    Next
    If s <> 20 Then GlobalCallee = 1
End Function

Private Function Aliases() As Long
    Dim v() As Long
    ReDim v(0 To 0)
    v(0) = 35
    Alias v, v
    If UBound(v) <> 5 Then Aliases = 1: Exit Function
    If v(5) <> 42 Then Aliases = 2
End Function

Private Function HandlerResize() As Long
    Dim a() As Long, i As Long, s As Long
    On Error GoTo Grow
    ReDim a(1 To 3)
    For i = 1 To 6
        a(i) = i
        s = s + a(i) + UBound(a)
    Next
    If s <> 48 Then HandlerResize = 1
    Exit Function
Grow:
    ReDim Preserve a(1 To 6)
    Resume
End Function

Private Function GosubResize() As Long
    Dim a() As Long, i As Long, s As Long
    ReDim a(1 To 1)
    For i = 1 To 4
        If i > UBound(a) Then GoSub Grow
        a(i) = i * 10
        s = s + a(i)
    Next
    If s <> 100 Or UBound(a) <> 4 Then GosubResize = 1
    Exit Function
Grow:
    ReDim Preserve a(1 To i)
    Return
End Function

Private Function RecordField() As Long
    Dim r As Holder
    ReDim r.v(2)
    r.v(2) = 5
    ReDim Preserve r.v(10)
    r.v(10) = r.v(2) + 1
    If r.v(10) <> 6 Or UBound(r.v) <> 10 Then RecordField = 1
End Function

Private Function RankChange() As Long
    Dim m() As Long, x As Long
    ReDim m(3)
    m(3) = 1
    ReDim m(1 To 2, 1 To 3)
    m(2, 3) = 5
    If UBound(m, 2) <> 3 Or LBound(m, 1) <> 1 Or m(2, 3) <> 5 Then RankChange = 1: Exit Function
    On Error Resume Next
    x = m(1)
    If Err.Number <> 9 Then RankChange = 2: Exit Function
    Err.Clear
    Erase m
    x = m(1, 1)
    If Err.Number <> 9 Then RankChange = 3
End Function

Private Function WideBound() As Long
    Dim c() As Long
    ReDim c(1 To 2, 3 To 4, 5 To 6)
    If UBound(c, 3) <> 6 Or LBound(c, 1) <> 1 Or UBound(c, 2) <> 4 Then WideBound = 1
End Function

Private Function Terminate() As Long
    Dim h As Holder2
    ReDim g(1 To 2)
    Set h = New Holder2
    g(2) = UBound(g)
    Set h = Nothing
    If UBound(g) <> 20 Then Terminate = 1: Exit Function
    g(20) = g(2)
    If g(20) <> 2 Then Terminate = 2
End Function

Private Function VirtualLocks() As Long
    Dim p() As Long, i As Long
    ReDim p(0 To 10)
    For i = 1 To 10
        p(i) = p(i - 1) + i
    Next
    If p(10) <> 55 Then VirtualLocks = 1
End Function

Private Function RealLockInLoop() As Long
    Dim a() As Long, i As Long, failures As Long
    ReDim a(0 To 3)
    On Error Resume Next
    For i = 0 To 3
        a(i) = i
        Both a(i), a
        If Err.Number = 10 Then failures = failures + 1
        Err.Clear
        a(i) = a(i) + 1
    Next
    If failures <> 4 Or UBound(a) <> 3 Or a(3) <> 4 Then RealLockInLoop = 1
End Function

Public Function Run() As Long
    Run = LoopResize(): If Run <> 0 Then Run = Run + 100: Exit Function
    Run = GlobalCallee(): If Run <> 0 Then Run = Run + 200: Exit Function
    Run = Aliases(): If Run <> 0 Then Run = Run + 300: Exit Function
    Run = HandlerResize(): If Run <> 0 Then Run = Run + 400: Exit Function
    Run = GosubResize(): If Run <> 0 Then Run = Run + 500: Exit Function
    Run = RecordField(): If Run <> 0 Then Run = Run + 600: Exit Function
    Run = RankChange(): If Run <> 0 Then Run = Run + 700: Exit Function
    Run = WideBound(): If Run <> 0 Then Run = Run + 800: Exit Function
    Run = Terminate(): If Run <> 0 Then Run = Run + 900: Exit Function
    Run = VirtualLocks(): If Run <> 0 Then Run = Run + 1000: Exit Function
    Run = RealLockInLoop(): If Run <> 0 Then Run = Run + 1100: Exit Function
    Run = FailedCallee(): If Run <> 0 Then Run = Run + 1200: Exit Function
End Function
`

func TestNativeArraySnapshotRefresh(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: snapshotChecks},
		compiler.SourceFileInput{FileName: "Holder2.cls", Text: `Private Sub Class_Terminate()
    ReDim Preserve Main.g(1 To 20)
End Sub
`})
	var entry ir.ProcedureID
	for _, procedure := range p.Procedures {
		if procedure.Name == "Main.Run" {
			entry = procedure.ID
		}
	}
	text := emit(t, p, llvm.Options{Entry: entry})
	// Every scalar access in the checks goes through a snapshot.
	if strings.Contains(text, "call i32 @vba_array_index(") {
		t.Fatal("a typed scalar access kept the runtime path")
	}
	native(t, text)
}

// Prefix sums need no runtime lock calls: the LHS lock is virtual, and the
// nested read checks saturation against the snapshot count plus one.
func TestVirtualLockPlan(t *testing.T) {
	text := emit(t, lower(t, `Sub Prefix(ByRef p() As Long)
 Dim i As Long
 For i = 1 To UBound(p)
  p(i) = p(i - 1) + i
 Next
End Sub`), llvm.Options{})
	plans := regexp.MustCompile(`@vba\.array\.snapshot\.index\.1\.1\(ptr [^,]+, i64 4, i1 (true|false), i32 (\d)`).FindAllStringSubmatch(text, -1)
	var got []string
	for _, plan := range plans {
		got = append(got, plan[1]+"/"+plan[2])
	}
	if strings.Join(got, " ") != "false/0 false/1" {
		t.Fatalf("lock plan %v\n%s", got, text)
	}
}
