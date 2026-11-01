package llvm_test

import (
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
	"strings"
	"testing"
)

func TestRegistrationUsesResolvedPrivateProcedurePointers(t *testing.T) {
	p := lower(t, "Private Sub Hidden()\nEnd Sub\n")
	text, ds := llvm.EmitTests(p, llvm.TestRegistration{Suites: []llvm.TestSuite{{ID: "test.module", Tests: []llvm.TestEntry{{ID: "test.module.hidden", Procedure: 1}}}}})
	if len(ds) > 0 || !strings.Contains(text, "%vba.test.entry {ptr @vba.test.cstr.0, ptr @vba.p1, i32 0}") || !strings.Contains(text, "define internal i32 @vba.p1") {
		t.Fatal(ds, text)
	}
}
func TestInvalidTestRegistrationIsDiagnosed(t *testing.T) {
	p := lower(t, "Function HasResult() As Long\nEnd Function\n")
	for _, program := range []*ir.Program{nil, {Procedures: []*ir.Procedure{nil}}, p} {
		text, ds := llvm.EmitTests(program, llvm.TestRegistration{Suites: []llvm.TestSuite{{Tests: []llvm.TestEntry{{ID: "invalid", Procedure: 1}}}}})
		if text != "" || len(ds) == 0 {
			t.Fatal("invalid test program accepted", text, ds)
		}
	}
}
