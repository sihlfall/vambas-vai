package testrunner_test

import (
	"context"
	"encoding/xml"
	"os/exec"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/testdiscovery"
	"sihlfall.ch/vambas-vai/testrunner"
	"strings"
	"testing"
	"time"
)

func nativeReport(t *testing.T, opt string, sources ...compiler.SourceFileInput) testrunner.Report {
	t.Helper()
	if _, e := exec.LookPath("clang-20"); e != nil {
		t.Skip("requires LLVM 20")
	}
	p := compiler.NewProgram(compiler.ProgramOptions{ProjectName: "Specs", Sources: sources})
	m, ds := (testdiscovery.Rubberduck{}).Discover(p)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	r, e := testrunner.Run(context.Background(), p, m, testrunner.Options{Optimization: opt, Timeout: 10 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.CompileDiagnostics) > 0 {
		t.Fatal(r.CompileDiagnostics)
	}
	return r
}

func TestCollectionObjectItemsInAssertions(t *testing.T) {
	for _, opt := range []string{"O0", "O2"} {
		t.Run(opt, func(t *testing.T) {
			r := nativeReport(t, opt, compiler.SourceFileInput{FileName: "Cases.bas", Text: `
'@TestModule
Private Assert As New Rubberduck.AssertClass
'@TestMethod
Private Sub ObjectItems()
 Dim c As New Collection, child As New Collection
 c.Add child
 Assert.AreSame child, c(1)
 Assert.IsNotNothing c.Item(1)
 Dim late As Object
 Set late = Assert
 late.AreSame Actual:=c(1), Expected:=child
 With Assert
  .AreSame c(1), child
  .IsNotNothing c(1)
 End With
 With late
  .AreSame Actual:=c(1), Expected:=child
 End With
End Sub`})
			if len(r.Suites) != 1 || len(r.Suites[0].Tests) != 1 || r.Suites[0].Tests[0].Status != "passed" {
				t.Fatalf("Collection object assertions: %+v", r)
			}
		})
	}
}

func TestNativeAssertionsAndLifecycle(t *testing.T) {
	for _, opt := range []string{"O0", "O2"} {
		t.Run(opt, func(t *testing.T) {
			r := nativeReport(t, opt, compiler.SourceFileInput{FileName: "Cases.bas", Text: `Option Explicit
'@TestModule
Private strict As New Rubberduck.AssertClass
Private loose As New Rubberduck.PermissiveAssertClass
Private late As Object
Private cleanups As Long
Private sequence As Long
'@ModuleInitialize
Private Sub ModuleStart()
 Set late = CreateObject("Rubberduck.AssertClass")
End Sub
'@TestCleanup
Private Sub CleanCase()
 cleanups = cleanups + 1
End Sub
'@ModuleCleanup
Private Sub ModuleEnd()
 late.AreEqual 8&, cleanups
 Set late = Nothing
 Set strict = Nothing
End Sub
Private Function Tick() As Long
 sequence = sequence + 1
 Tick = sequence
End Function
Private Function NewAssert() As Rubberduck.AssertClass
 Set NewAssert = New Rubberduck.AssertClass
End Function
Private Sub ClearCopy(ByVal value As Object)
 Set value = Nothing
End Sub
Private Sub ClearAlias(ByRef value As Object)
 Set value = Nothing
End Sub
'@TestMethod("Passing")
Private Sub Values()
 strict.AreEqual 3&, 1& + 2&
 loose.AreEqual 3, 3&
 loose.AreEqual "3", 3&
 strict.AreNotEqual 3, 3&
 strict.AreEqual "abc", "abc"
 strict.AreNotEqual "abc", "ABC"
 strict.AreEqual ChrW(13) & ChrW(10), vbCrLf
 strict.AreEqual ChrW(10), vbLf
 strict.AreEqual Empty, Empty
 strict.AreEqual Null, Null
 strict.IsFalse False
 strict.IsTrue True
 strict.IsTrue ActualCheck()
 strict.AreEqual Actual:=Tick(), Expected:=Tick() - 1
 Dim a As Object
 Dim b As Object
 Set a = New Rubberduck.AssertClass
 Set b = a
 strict.AreSame a, b
 strict.IsTrue a Is b
 ClearCopy a
 strict.IsNotNothing a
 ClearAlias a
 strict.IsNothing a
 strict.AreNotSame a, b
 Set a = NewAssert()
 strict.AreNotSame a, b
 Dim typed As Rubberduck.AssertClass
 Set typed = CreateObject("Rubberduck.AssertClass")
 typed.Succeed
 On Error Resume Next
 Set typed = CreateObject("Rubberduck.PermissiveAssertClass")
 strict.AreEqual 13&, Err.Number
 Err.Clear
 On Error GoTo 0
 typed.Succeed
 Set b = Nothing
 ' As New recreates lazily after an explicit reset.
 Set strict = Nothing
 strict.IsTrue True
 Dim first(2 To 3) As Long
 Dim second(2 To 3) As Long
 first(2) = 42
 second(2) = 42
 strict.SequenceEquals first, second
 second(3) = 1
 strict.NotSequenceEquals first, second
 Dim narrow(2 To 3) As Integer
 narrow(2) = 42
 loose.SequenceEquals first, narrow
 Dim outer1 As Variant
 Dim outer2 As Variant
 ReDim outer1(0 To 0) As Variant
 ReDim outer2(0 To 0) As Variant
 outer1(0) = first
 outer2(0) = first
 strict.SequenceEquals outer1, outer2
 outer2(0) = second
 strict.NotSequenceEquals outer1, outer2
 late.Succeed
End Sub
Private Function ActualCheck() As Boolean
 ActualCheck = True
End Function
'@TestMethod
Private Sub CannotHideFailure()
 On Error Resume Next
 late.Fail "first failure"
 late.AreEqual 1&, 2&, "second failure"
 late.Succeed
End Sub
'@TestMethod
Private Sub RuntimeFailure()
 Error 5
End Sub
'@TestMethod
Private Sub CleanupRan()
 strict.AreEqual 3&, cleanups
 strict.AreEqual "Err number: 5", "Err number: " & 5&
End Sub
'@TestMethod
Private Sub StrictMismatch()
 strict.AreEqual 1, 1&
End Sub
'@TestMethod
Private Sub Uncertain()
 late.Inconclusive "Waiting for host support"
End Sub
'@TestMethod
Private Sub BoundsMatter()
 Dim first(0 To 1) As Long
 Dim second(1 To 2) As Long
 strict.SequenceEquals first, second
End Sub
'@TestMethod
Private Sub InvalidLateArgument()
 late.IsTrue Nonsense:=True
End Sub
'@IgnoreTest
'@TestMethod
Private Sub Ignored()
 late.Fail "Must not run"
End Sub
`})
			if !r.Failed() || len(r.Suites) != 1 || len(r.Suites[0].Issues) != 0 {
				t.Fatalf("%+v", r)
			}
			suite := r.Suites[0]
			expected := []string{"passed", "failed", "error", "passed", "failed", "inconclusive", "failed", "error", "skipped"}
			if len(suite.Tests) != len(expected) {
				t.Fatalf("%+v", suite)
			}
			for i, status := range expected {
				if suite.Tests[i].Status != status {
					t.Errorf("%s: want %s got %+v", suite.Tests[i].Test.Name, status, suite.Tests[i])
				}
			}
			if len(suite.Tests[1].Issues) != 2 || suite.Tests[1].Assertions != 3 {
				t.Fatal("handler swallowed failure", suite.Tests[1])
			}
			if suite.Tests[2].Issues[0].Number != 5 || suite.Tests[7].Issues[0].Number != 448 {
				t.Fatal("runtime errors not preserved", suite.Tests)
			}
			if suite.Tests[1].Issues[0].Source.File != "Cases.bas" || suite.Tests[1].Issues[0].Source.Line < 65 {
				t.Fatal("missing assertion source", suite.Tests[1])
			}
			data, e := r.JUnit()
			if e != nil {
				t.Fatal(e)
			}
			var doc struct{ XMLName xml.Name }
			if e = xml.Unmarshal(data, &doc); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestWorkerIsolationAndSharedModuleState(t *testing.T) {
	common := compiler.SourceFileInput{FileName: "Shared.bas", Text: "Public counter As Long\n"}
	test := func(name string) compiler.SourceFileInput {
		return compiler.SourceFileInput{FileName: name + ".bas", Text: `'@TestModule
Private a As New Rubberduck.AssertClass
'@TestMethod
Private Sub First()
 a.AreEqual 0&, Shared.counter
 Shared.counter = 41
End Sub
'@TestMethod
Private Sub Second()
 a.AreEqual 41&, Shared.counter
End Sub
`}
	}
	r := nativeReport(t, "O2", common, test("One"), test("Two"))
	if r.Failed() || len(r.Suites) != 2 {
		t.Fatal(r)
	}
}
func TestFixtureFailuresStillCleanup(t *testing.T) {
	r := nativeReport(t, "O0", compiler.SourceFileInput{FileName: "Cases.bas", Text: `'@TestModule
Private a As New Rubberduck.AssertClass
Private entered As Boolean
'@TestInitialize
Private Sub Before()
 a.Fail "setup failed"
End Sub
'@TestCleanup
Private Sub After()
 entered = True
End Sub
'@ModuleCleanup
Private Sub Finish()
 a.IsTrue entered
 a.Fail "cleanup failure must be reported separately"
End Sub
'@TestMethod
Private Sub NeverEntered()
 a.Fail "body must be skipped"
End Sub
`})
	s := r.Suites[0]
	if len(s.Tests) != 1 || s.Tests[0].Status != "failed" || len(s.Tests[0].Issues) != 1 || s.Tests[0].Issues[0].Phase != "TestInitialize" || len(s.Issues) != 1 || s.Issues[0].Phase != "ModuleCleanup" {
		t.Fatal(r)
	}
}
func TestCompilationFailureIsAReport(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Cases.bas", Text: "'@TestModule\n'@TestMethod\nPrivate Sub Future()\n Dim c As New FutureWidget\nEnd Sub\n"}}})
	m, _ := (testdiscovery.Rubberduck{}).Discover(p)
	r, e := testrunner.Run(context.Background(), p, m, testrunner.Options{})
	if e != nil || len(r.CompileDiagnostics) == 0 || !r.Failed() {
		t.Fatal(r, e)
	}
	data, e := r.JUnit()
	if e != nil || !strings.Contains(string(data), "compile_error") {
		t.Fatal(string(data), e)
	}
}
func TestModuleTimeoutIsNotAnAssertionFailure(t *testing.T) {
	if _, e := exec.LookPath("clang-20"); e != nil {
		t.Skip("requires LLVM 20")
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Hang.bas", Text: "'@TestModule\n'@TestMethod\nPrivate Sub Hang()\n Do\n Loop\nEnd Sub\n"}}})
	m, _ := (testdiscovery.Rubberduck{}).Discover(p)
	r, e := testrunner.Run(context.Background(), p, m, testrunner.Options{Timeout: 100 * time.Millisecond})
	if e != nil || len(r.CompileDiagnostics) > 0 {
		t.Fatal(r, e)
	}
	if len(r.Suites) != 1 || r.Suites[0].Tests[0].Status != "timed_out" {
		t.Fatal(r)
	}
}
