package identity_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/ir/identity"
	"sihlfall.ch/vambas-vai/lowering"
)

func lower(t *testing.T, files ...string) *ir.Program {
	t.Helper()
	var sources []compiler.SourceFileInput
	for i := 0; i < len(files); i += 2 {
		sources = append(sources, compiler.SourceFileInput{FileName: files[i], Text: files[i+1]})
	}
	program, diagnostics := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: sources}))
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	return program
}

func procedure(t *testing.T, program *ir.Program, name string) *ir.Procedure {
	t.Helper()
	for _, p := range program.Procedures {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("missing procedure %s", name)
	return nil
}

func summaries(t *testing.T, program *ir.Program) map[string]string {
	t.Helper()
	lines := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(identity.Analyze(program).Format()), "\n") {
		lines[line[:strings.IndexByte(line, '{')]] = line
	}
	return lines
}

func TestSummaries(t *testing.T) {
	tests := []struct {
		name, source string
		want         []string
	}{
		{"element write preserves, Erase invalidates", `Sub F(a() As Long, b() As Long)
    a(0) = 42
    Erase b
End Sub`, []string{"Main.F{a: 1, b: 2, ambient: 3} = {a: 1, b: 4, ambient: 3} observes a, b"}},
		{"branch merge", `Sub F(a() As Long, b() As Long, ByVal c As Boolean)
    If c Then
        b(0) = 1
    Else
        Erase a
    End If
End Sub`, []string{"Main.F{a: 1, b: 2, ambient: 3} = {a: 4, b: 2, ambient: 3} observes a, b"}},
		{"loop", `Sub F(a() As Long, b() As Long)
    Dim i As Long
    For i = 0 To 3
        If i = 2 Then ReDim b(5)
        a(i) = i
    Next
End Sub`, []string{"Main.F{a: 1, b: 2, ambient: 3} = {a: 1, b: 4, ambient: 3} observes a, b"}},
		{"calls translate formals and ambient", `Public g() As Long
Sub F(a() As Long, b() As Long)
    Erase b
End Sub
Sub Grow()
    ReDim g(10)
End Sub
Sub Calls(x() As Long, y() As Long)
    Dim local() As Long
    F y, x
    F x, local
    Grow
End Sub`, []string{
			"Main.Grow{ambient: 1} = {ambient: 2} observes ambient",
			"Main.Calls{x: 1, y: 2, ambient: 3} = {x: 4, y: 2, ambient: 5} observes x, ambient",
		}},
		{"recursion", `Sub R(a() As Long, b() As Long, ByVal n As Long)
    If n > 0 Then R b, a, n - 1 Else Erase a
End Sub`, []string{"Main.R{a: 1, b: 2, ambient: 3} = {a: 4, b: 5, ambient: 3} observes a, b"}},
		{"element ByRef argument", `Sub Touch(ByRef x As Long)
    x = x + 1
End Sub
Sub Both(ByRef x As Long, ByRef a() As Long)
    ReDim a(5)
End Sub
Sub Use(a() As Long, b() As Long)
    Touch a(0)
    Both b(0), a
End Sub`, []string{
			"Main.Touch{x: 1, ambient: 2} = {x: 1, ambient: 2}",
			"Main.Use{a: 1, b: 2, ambient: 3} = {a: 4, b: 2, ambient: 3} observes a, b",
		}},
		{"With binds without invalidating", `Type P
    v() As Long
    n As Long
End Type
Sub Fields(r As P)
    With r
        .n = 1
    End With
End Sub
Sub Shape(r As P)
    With r
        ReDim .v(3)
    End With
End Sub`, []string{
			"Main.Fields{r: 1, ambient: 2} = {r: 1, ambient: 2}",
			"Main.Shape{r: 1, ambient: 2} = {r: 3, ambient: 2} observes r",
		}},
		{"error handler and Resume edges", `Sub F(a() As Long, b() As Long)
    On Error GoTo Handler
    a(0) = 1
    Exit Sub
Handler:
    Erase b
    Resume Next
End Sub`, []string{"Main.F{a: 1, b: 2, ambient: 3} = {a: 1, b: 4, ambient: 3} observes a, b"}},
		{"GoSub return edges", `Sub F(a() As Long, b() As Long)
    GoSub Shrink
    a(0) = 1
    Exit Sub
Shrink:
    Erase b
    Return
End Sub`, []string{"Main.F{a: 1, b: 2, ambient: 3} = {a: 1, b: 4, ambient: 3} observes a, b"}},
		{"dead code and dead cycles", `Sub F(a() As Long, b() As Long, ByVal c As Boolean)
    If c Then GoTo Second
First:
    a(0) = 1
Second:
    If c Then GoTo First
    Exit Sub
Dead:
    Erase b
    GoTo Dead
End Sub`, []string{"Main.F{a: 1, b: 2, ambient: 3} = {a: 1, b: 2, ambient: 3} observes a"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines := summaries(t, lower(t, "Main.bas", test.source))
			for _, want := range test.want {
				if got := lines[want[:strings.IndexByte(want, '{')]]; got != want {
					t.Errorf("got  %s\nwant %s", got, want)
				}
			}
		})
	}
}

func TestShadowLabels(t *testing.T) {
	program := lower(t, "Main.bas", `Sub F(a() As Long, b() As Long)
    a(0) = 42
    Erase b
End Sub
Sub Keep(a() As Long, b() As Long)
    a(0) = b(0)
End Sub`)
	analysis := identity.Analyze(program)
	f := analysis.Summary(procedure(t, program, "Main.F"))
	if names := f.Names(); !slices.Equal(names, []string{"a", "b", "ambient"}) {
		t.Fatal(names)
	}
	for _, test := range []struct{ in, want []int }{
		{[]int{1, 2, 3}, []int{1, 4, 3}}, // Independent inputs.
		{[]int{1, 1, 2}, []int{3, 3, 2}}, // Potentially dependent inputs.
		{[]int{1, 2, 1}, []int{1, 3, 1}},
		{[]int{0, 2, 3}, []int{0, 4, 4}}, // Unknown may depend on anything.
	} {
		if got := f.Shadow(test.in); !slices.Equal(got, test.want) {
			t.Errorf("F%v = %v, want %v", test.in, got, test.want)
		}
	}
	keep := analysis.Summary(procedure(t, program, "Main.Keep"))
	if got := keep.Shadow([]int{1, 1, 2}); !slices.Equal(got, []int{1, 1, 2}) {
		t.Errorf("Keep = %v", got)
	}
}

func TestInterfaceAndTerminationSummaries(t *testing.T) {
	program := lower(t,
		"IShape.cls", `Option Explicit
Public Sub Resize(ByRef values() As Long)
End Sub
`,
		"Grower.cls", `Option Explicit
Implements IShape
Private Sub IShape_Resize(ByRef values() As Long)
    ReDim values(4)
End Sub
`,
		"Holder.cls", `Option Explicit
Private Sub Class_Terminate()
    Erase Main.shared
End Sub
`,
		"Main.bas", `Option Explicit
Public shared() As Long
Public Sub Dispatch(ByVal s As IShape, ByRef a() As Long, ByRef b() As Long)
    s.Resize b
    a(0) = 1
End Sub
Public Sub Drop(ByRef a() As Long)
    Dim h As Holder
    Set h = New Holder
    Set h = Nothing
    a(0) = 1
End Sub
`)
	lines := summaries(t, program)
	for _, want := range []string{
		"Main.Dispatch{a: 1, b: 2, ambient: 3} = {a: 1, b: 4, ambient: 5} observes a, b, ambient",
		"Main.Drop{a: 1, ambient: 2} = {a: 1, ambient: 3} observes a, ambient",
	} {
		if got := lines[want[:strings.IndexByte(want, '{')]]; got != want {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	}
}

// locks reports LockRemovable for each element token of p, in order.
func locks(analysis *identity.Analysis, p *ir.Procedure) []bool {
	var removable []bool
	for _, s := range p.Statements {
		for _, in := range s.Instructions {
			if in.Op == ir.OpTry && ir.Element(in.Type) {
				removable = append(removable, analysis.LockRemovable(p, s, in.ID))
			}
		}
	}
	return removable
}

func TestElementLockLifetimes(t *testing.T) {
	tests := []struct {
		name, body string
		want       []bool
	}{
		{"immediate read", "x = a(1)", []bool{true}},
		{"scalar write", "a(1) = x + 1", []bool{true}},
		// The LHS lock is live while nested indexing reads the lock count.
		{"nested indexing", "a(1) = a(2) + b(0)", []bool{false, true, true}},
		{"independent array", "a(1) = b(2) + 1", []bool{true, true}},
		{"harmless call", "Touch a(1)", []bool{true}},
		{"call resizes the locked array", "Both a(1), a", []bool{false}},
		{"call resizes another array", "Both a(1), b", []bool{true}},
		{"ambient effects cannot reach locals", "Touch a(1): Grow", []bool{true}},
		{"function result through a call", "a(1) = Twice(b(2))", []bool{true, true}},
		{"call observes the locked array", "a(1) = Peek(a)", []bool{false}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := lower(t, "Main.bas", `Public g() As Long
Sub Touch(ByRef x As Long)
    x = x + 1
End Sub
Sub Both(ByRef x As Long, ByRef v() As Long)
    ReDim v(5)
End Sub
Sub Grow()
    ReDim g(5)
End Sub
Function Twice(ByVal x As Long) As Long
    Twice = 2 * x
End Function
Function Peek(v() As Long) As Long
    Peek = v(0)
End Function
Sub Run()
    Dim a(3) As Long, b(3) As Long, x As Long
    `+test.body+`
End Sub`)
			got := locks(identity.Analyze(program), procedure(t, program, "Main.Run"))
			if !slices.Equal(got, test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestBorrowedLocksIncludeUnknownCallers(t *testing.T) {
	program := lower(t, "Main.bas", `Public g() As Long
Sub Grow()
    ReDim g(5)
End Sub
Sub Local(a() As Long)
    Dim own(3) As Long
    own(1) = a(1)
    a(2) = 7
End Sub
Sub Reenter(a() As Long)
    Dim own(3) As Long
    own(1) = Twice(1)
    a(2) = Twice(2): Grow
End Sub
Function Twice(ByVal x As Long) As Long
    Twice = 2 * x
    Grow
End Function`)
	analysis := identity.Analyze(program)
	if got := locks(analysis, procedure(t, program, "Main.Local")); !slices.Equal(got, []bool{true, true, true}) {
		t.Errorf("Local %v", got)
	}
	// A borrowed parameter may alias g, so ambient effects revoke its lock.
	if got := locks(analysis, procedure(t, program, "Main.Reenter")); !slices.Equal(got, []bool{true, false}) {
		t.Errorf("Reenter %v", got)
	}
}

func TestLifetimeAcrossLoops(t *testing.T) {
	for _, test := range []struct {
		name, body string
		retained   bool
	}{
		{"element writes", "ps(i).n = j", true},
		{"conditional resize in a nested loop", "If i = j Then ReDim Preserve ps(3)", false},
		{"field array resize", "ReDim ps(i).v(j)", false},
		{"other array", "ReDim qs(j)", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			program := lower(t, "Main.bas", `Type P
    v() As Long
    n As Long
End Type
Sub Run()
    Dim ps() As P, qs() As P, i As Long, j As Long
    ReDim ps(2)
    With ps(1)
        For i = 0 To 2
            Do While j < 3
                `+test.body+`
                j = j + 1
            Loop
        Next
        .n = 1
    End With
End Sub`)
			analysis := identity.Analyze(program)
			p := procedure(t, program, "Main.Run")
			g := analysis.Graph(p)
			var boundary *identity.FlowNode
			var base ir.SlotID
			var last *ir.Statement
			for _, s := range p.Statements {
				for _, in := range s.Instructions {
					if in.Op == ir.OpKeepElement {
						boundary = g.Acquisition(s.ID, in.Args[0])
						base = s.Instructions[0].Slot
					}
					if in.Op == ir.OpWithReference {
						last = s
					}
				}
			}
			if boundary == nil || last == nil || base == 0 {
				t.Fatal("missing With lifetime")
			}
			state := analysis.Lifetime(p, boundary, g.End(last.ID))
			if !state.Reachable() || state.Retained(base) != test.retained {
				t.Fatalf("retained=%v, want %v", state.Retained(base), test.retained)
			}
			if !state.Dependent(base, s(p, "$with")) {
				t.Fatal("a With receiver depends on its target")
			}
		})
	}
}

func s(p *ir.Procedure, name string) ir.SlotID {
	for _, slot := range p.Slots {
		if slot.Name == name {
			return slot.ID
		}
	}
	return 0
}

func TestInvalidatesAndEdges(t *testing.T) {
	program := lower(t, "Main.bas", `Public g() As Long
Sub Grow()
    ReDim g(5)
End Sub
Sub Work(a() As Long, b() As Long)
    Dim c() As Long
    a(0) = 1
    ReDim b(2)
    c(0) = 1
    Grow
    ReDim c(3)
End Sub`)
	analysis := identity.Analyze(program)
	p := procedure(t, program, "Main.Work")
	slot := func(name string) ir.SlotID { return s(p, name) }
	// invalidating lists the statements with an instruction that may
	// invalidate the given slot, in source order.
	invalidating := func(id ir.SlotID) []ir.StatementID {
		var got []ir.StatementID
		for _, st := range p.Statements {
			for i := range st.Instructions {
				if analysis.Invalidates(p, st, i, id) {
					got = append(got, st.ID)
					break
				}
			}
		}
		return got
	}
	resizes := map[string]ir.StatementID{}
	for _, st := range p.Statements {
		for _, in := range st.Instructions {
			if in.Op == ir.OpArrayResize {
				resizes[p.Slots[st.Instructions[0].Slot-1].Name] = st.ID
			}
			if in.Op == ir.OpCall {
				resizes["call"] = st.ID
			}
		}
	}
	// Borrowed a may alias b and g; the owned c is independent of both.
	if got, want := invalidating(slot("a")), []ir.StatementID{resizes["b"], resizes["call"]}; !slices.Equal(got, want) {
		t.Errorf("a: %v, want %v", got, want)
	}
	if got, want := invalidating(slot("c")), []ir.StatementID{resizes["c"]}; !slices.Equal(got, want) {
		t.Errorf("c: %v, want %v", got, want)
	}
	for _, e := range analysis.Graph(p).Edges {
		if e.Failure >= 0 {
			t.Errorf("no error policy, so failures only exit: %+v", e)
		}
	}
}

func TestEmulatedLockObservers(t *testing.T) {
	program := lower(t, "Main.bas", `Sub Prefix(p() As Long, q() As Long)
    p(1) = p(0) + 1
    p(2) = q(0) + 1
End Sub`)
	analysis := identity.Analyze(program)
	p := procedure(t, program, "Main.Prefix")
	var removable, emulated []bool
	for _, st := range p.Statements {
		var indexes []int
		for i, in := range st.Instructions {
			if in.Op == ir.OpArrayIndex {
				indexes = append(indexes, i)
			}
		}
		if len(indexes) != 2 {
			continue
		}
		lhs := st.Instructions[indexes[0]+1].ID
		removable = append(removable, analysis.LockRemovable(p, st, lhs))
		emulated = append(emulated, analysis.LockRemovableExcept(p, st, lhs, func(i int) bool { return i == indexes[1] }))
	}
	// Both LHS locks are observed by the nested read: q may alias p. Only a
	// consumer that reproduces the nested read's saturation check (same
	// array, so only for p(0)) may treat the first lock as virtual.
	if !slices.Equal(removable, []bool{false, false}) || !slices.Equal(emulated, []bool{true, true}) {
		t.Fatalf("removable %v, emulated %v", removable, emulated)
	}
}

// Nested loops under an error policy with Resume form one large cycle. A
// demand-driven evaluation that re-iterates inner loops for each provisional
// outer approximation grows exponentially here; the forward solver does not.
func TestDeepLoopNestsStayFast(t *testing.T) {
	var body strings.Builder
	body.WriteString("Public g() As Long\nSub Deep(a() As Long)\n    Dim i0 As Long, i1 As Long, i2 As Long, i3 As Long, i4 As Long, i5 As Long, i6 As Long, i7 As Long, i8 As Long, i9 As Long, i10 As Long, i11 As Long\n    On Error GoTo Handler\n")
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&body, "    For i%d = 0 To 1\n        a(i%d) = a(i%d) + 1\n        If i%d = 1 Then ReDim g(i%d)\n", i, i, i, i, i)
	}
	for i := 11; i >= 0; i-- {
		fmt.Fprintf(&body, "    Next i%d\n", i)
	}
	body.WriteString("    Exit Sub\nHandler:\n    Resume Next\nEnd Sub\n")
	program := lower(t, "Main.bas", body.String())
	done := make(chan string, 1)
	go func() { done <- identity.Analyze(program).Format() }()
	select {
	case got := <-done:
		if !strings.Contains(got, "Main.Deep{a: 1, ambient: 2} = {a: 1, ambient: 3}") {
			t.Fatal(got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("analysis of nested loops did not finish")
	}
}
