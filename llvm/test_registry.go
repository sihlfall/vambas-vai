package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

// TestRegistration is framework-neutral. Procedure IDs are resolved by the
// adapter before emission; private procedures need no special source wrappers.
type TestRegistration struct{ Suites []TestSuite }
type TestSuite struct {
	ID                                 string
	Initialize, Cleanup, Before, After ir.ProcedureID
	Tests                              []TestEntry
}
type TestEntry struct {
	ID        string
	Procedure ir.ProcedureID
	Ignored   bool
}

func EmitTests(program *ir.Program, registry TestRegistration) (string, []ir.Diagnostic) {
	ds := ir.Verify(program)
	if len(ds) > 0 {
		return "", ds
	}
	check := func(id ir.ProcedureID, optional bool) {
		if id == 0 && optional {
			return
		}
		if id == 0 || int(id) > len(program.Procedures) {
			ds = append(ds, ir.Diagnostic{Message: "test registration references a missing procedure"})
			return
		}
		p := program.Procedures[id-1]
		if len(p.Parameters) != 0 || p.ReturnType != (ir.Type{Scalar: ir.Void}) {
			ds = append(ds, ir.Diagnostic{Source: p.Source, Message: "test entry must be a parameterless Sub"})
		}
	}
	for _, s := range registry.Suites {
		for _, id := range []ir.ProcedureID{s.Initialize, s.Cleanup, s.Before, s.After} {
			check(id, true)
		}
		for _, t := range s.Tests {
			check(t.Procedure, false)
		}
	}
	if len(ds) > 0 {
		return "", ds
	}
	text, ds := Emit(program, Options{})
	if len(ds) > 0 {
		return "", ds
	}
	var out strings.Builder
	out.WriteString(text)
	out.WriteString("\n%vba.test.entry = type { ptr, ptr, i32 }\n%vba.test.suite = type { ptr, ptr, ptr, ptr, ptr, i32, ptr }\ndeclare i32 @vba_test_main(i32, ptr, i32, ptr)\n")
	literals := &module{cstringPrefix: "@vba.test.cstr."}
	ref := func(id ir.ProcedureID) string {
		if id == 0 {
			return "null"
		}
		return procedureName(id)
	}
	suites := []string{}
	for i, s := range registry.Suites {
		entries := []string{}
		for _, t := range s.Tests {
			ignored := 0
			if t.Ignored {
				ignored = 1
			}
			entries = append(entries, fmt.Sprintf("%%vba.test.entry {ptr %s, ptr %s, i32 %d}", literals.cstring(t.ID), ref(t.Procedure), ignored))
		}
		fmt.Fprintf(&out, "@vba.tests.%d = private constant [%d x %%vba.test.entry] [%s]\n", i, len(entries), strings.Join(entries, ", "))
		suites = append(suites, fmt.Sprintf("%%vba.test.suite {ptr %s, ptr %s, ptr %s, ptr %s, ptr %s, i32 %d, ptr @vba.tests.%d}", literals.cstring(s.ID), ref(s.Initialize), ref(s.Cleanup), ref(s.Before), ref(s.After), len(entries), i))
	}
	// Use a disjoint constant prefix from ordinary procedure literals.
	reg := fmt.Sprintf("@vba.test.suites = private constant [%d x %%vba.test.suite] [%s]\n", len(suites), strings.Join(suites, ", "))
	out.WriteString(reg)
	for _, l := range literals.literals {
		out.WriteString(l)
		out.WriteByte('\n')
	}
	out.WriteString("define i32 @main(i32 %argc, ptr %argv) {\nentry:\n")
	fmt.Fprintf(&out, "  %%status = call i32 @vba_test_main(i32 %%argc, ptr %%argv, i32 %d, ptr @vba.test.suites)\n", len(suites))
	if len(program.Globals) > 0 || ir.UsesErrors(program) {
		out.WriteString("  call void @vba.project.clear()\n")
	}
	out.WriteString("  ret i32 %status\n}\n")
	return out.String(), nil
}
