package testdiscovery_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/testdiscovery"
	"strings"
	"testing"
)

func TestResolvedPrivateTestsAndFixtures(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{ProjectName: "Demo", Sources: []compiler.SourceFileInput{{FileName: "Specs.bas", Text: `Option Explicit
'@TestModule
'@ModuleInitialize
Private Sub SetupModule()
End Sub
'@TestInitialize
Private Sub SetupCase()
End Sub
'@TestCleanup
Private Sub CleanupCase()
End Sub
'@ModuleCleanup
Private Sub CleanupModule()
End Sub
'@IgnoreTest
'@TestMethod("Text")
Private Sub Hidden()
End Sub
'@TestMethod
Public Sub Visible()
End Sub
`}}})
	m, ds := (testdiscovery.Rubberduck{}).Discover(p)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	if len(m.Suites) != 1 || len(m.Suites[0].Tests) != 2 {
		t.Fatalf("%+v", m)
	}
	s := m.Suites[0]
	if s.ModuleInitialize == nil || s.ModuleCleanup == nil || s.TestInitialize == nil || s.TestCleanup == nil {
		t.Fatal("missing fixtures")
	}
	hidden := s.Tests[0]
	if hidden.ID != "demo.specs.hidden" || hidden.Category != "Text" || !hidden.Ignored || hidden.Entry.Declaration == nil || hidden.Entry.Symbol != hidden.Entry.Declaration.Symbol || hidden.Source.Line != 17 {
		t.Fatalf("%+v", hidden)
	}
	if s.Tests[1].Category != "Uncategorized" {
		t.Fatal(s.Tests[1])
	}
}
func TestInvalidTestRoles(t *testing.T) {
	for _, body := range []string{
		"'@TestMethod\nFunction Invalid() As Long\nEnd Function\n",
		"'@TestMethod\nSub Invalid(ByVal value As Long)\nEnd Sub\n",
		"'@TestMethod(\"A\", \"B\")\nSub Invalid()\nEnd Sub\n",
		"'@TestMethod(\"unterminated)\nSub Invalid()\nEnd Sub\n",
		"'@ModuleInitialize\nSub First()\nEnd Sub\n'@ModuleInitialize\nSub Second()\nEnd Sub\n",
		"'@TestMethod\n'@TestCleanup\nSub Invalid()\nEnd Sub\n",
		"'@TestModule\nSub Invalid()\nEnd Sub\n",
		"'@TestMethod\nDim intervening As Long\nSub Invalid()\nEnd Sub\n",
	} {
		t.Run(strings.Split(body, "\n")[0], func(t *testing.T) {
			p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Specs.bas", Text: "'@TestModule\n" + body}}})
			_, ds := (testdiscovery.Rubberduck{}).Discover(p)
			if len(ds) == 0 {
				t.Fatal("invalid annotation accepted", body)
			}
		})
	}
}
func TestDiscoveryDoesNotRequireExecutableBodies(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Specs.bas", Text: "'@TestModule\n'@TestMethod\nPrivate Sub Future()\n Dim x As New FutureWidget\nEnd Sub\n"}}})
	m, ds := (testdiscovery.Rubberduck{}).Discover(p)
	if len(ds) > 0 || len(m.Suites) != 1 || len(m.Suites[0].Tests) != 1 {
		t.Fatal(m, ds)
	}
}

func TestClosestRepeatedRoleWins(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Cases.bas", Text: "'@TestModule\n'@TestMethod(\"Old\")\n' Sub OldBody()\n' End Sub\n'@TestMethod(\"Current\")\nPrivate Sub Current()\nEnd Sub\n"}}})
	m, ds := (testdiscovery.Rubberduck{}).Discover(p)
	if len(ds) > 0 || len(m.Suites[0].Tests) != 1 || m.Suites[0].Tests[0].Category != "Current" {
		t.Fatal(m, ds)
	}
}
