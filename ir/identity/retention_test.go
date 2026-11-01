package identity_test

import (
	"slices"
	"testing"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/ir/identity"
)

// retained reports whether p's summary keeps every borrowed input and ambient
// storage: no caller-visible storage identity may change during a call.
func retained(analysis *identity.Analysis, p *ir.Procedure) bool {
	m := analysis.Summary(p)
	labels := make([]int, len(m.Names()))
	for i := range labels {
		labels[i] = i + 1
	}
	return slices.Equal(m.Shadow(labels), labels)
}

func TestBorrowedRetention(t *testing.T) {
	tests := []struct {
		name, body string
		stable     bool
	}{
		{"read bounds and elements", `Function Work(ByRef a() As Long) As Long
 Dim i As Long
 For i = LBound(a) To UBound(a)
  Work = Work + a(i)
 Next
End Function`, true},
		{"shared element writes", `Sub Work(ByRef a() As Long, ByRef b() As Long)
 a(0) = 2
 b(0) = a(0) + 1
End Sub`, true},
		{"element passed byref", `Sub Work(ByRef a() As Long)
 Increment a(0)
End Sub
Sub Increment(ByRef n As Long)
 n = n + 1
End Sub`, true},
		{"redim", `Sub Work(ByRef a() As Long)
 ReDim a(0 To 4)
End Sub`, false},
		{"preserve", `Sub Work(ByRef a() As Long)
 ReDim Preserve a(0 To 4)
End Sub`, false},
		{"erase", `Sub Work(ByRef a() As Long)
 Erase a
End Sub`, false},
		{"replace", `Sub Work(ByRef a() As Long, ByRef b() As Long)
 a = b
End Sub`, false},
		{"resize other possible alias", `Sub Work(ByRef a() As Long, ByRef b() As Long)
 a(0) = 2
 ReDim b(4)
End Sub`, false},
		// Flow-sensitive: no label makes the ReDim reachable.
		{"unreachable resize", `Sub Work(ByRef a() As Long)
 Exit Sub
 ReDim a(4)
End Sub`, true},
		{"handler resize", `Sub Work(ByRef a() As Long)
 On Error GoTo handler
 a(0) = 1
 Exit Sub
handler:
 ReDim a(2)
 Resume Next
End Sub`, false},
		{"callee resize", `Sub Work(ByRef a() As Long)
 Grow a
End Sub
Sub Grow(ByRef target() As Long)
 ReDim target(4)
End Sub`, false},
		{"private scratch resize", `Sub Work(ByRef a() As Long)
 Dim scratch() As Long
 ReDim scratch(4)
 a(0) = scratch(0)
End Sub`, true},
		{"callee private scratch resize", `Sub Work(ByRef a() As Long)
 Dim scratch() As Long
 Grow scratch
 a(0) = 3
End Sub
Sub Grow(ByRef target() As Long)
 ReDim target(4)
End Sub`, true},
		{"deep copy resize", `Sub Work(ByRef a() As Long)
 Dim copy() As Long
 copy = a
 ReDim copy(4)
End Sub`, true},
		{"parenthesized copy resize", `Sub Work(ByRef a() As Long)
 Grow (a)
End Sub
Sub Grow(ByRef target() As Long)
 ReDim target(4)
End Sub`, true},
		{"global possible alias", `Public shared() As Long
Sub Work(ByRef a() As Long)
 ReDim shared(4)
End Sub`, false},
		{"callee global possible alias", `Public shared() As Long
Sub Work(ByRef a() As Long)
 GrowGlobal
End Sub
Sub GrowGlobal()
 ReDim shared(4)
End Sub`, false},
		{"variant alias replacement", `Sub Work(ByRef a() As Long, ByRef aliasValue As Variant)
 aliasValue = Empty
End Sub`, false},
		{"array forwarded as variant", `Sub Work(ByRef a() As Long)
 ClearValue a
End Sub
Sub ClearValue(ByRef value As Variant)
 value = Empty
End Sub`, false},
		{"readonly variant view", `Sub Work(ByRef a() As Long)
 Dim tag As Long
 tag = TagOf(a)
End Sub
Function TagOf(ByRef value As Variant) As Long
 TagOf = VarType(value)
End Function`, true},
		{"possible default callback", `Sub Work(ByRef a() As Long, ByVal value As Variant)
 Dim n As Long
 n = CLng(value)
End Sub`, false},
		{"err raise default callback", `Sub Work(ByRef a() As Long, ByVal value As Variant)
 Err.Raise value
End Sub`, false},
		{"err property default callback", `Sub Work(ByRef a() As Long, ByVal value As Variant)
 Err.Number = value
End Sub`, false},
		{"borrowed record field", `Private Type State
 values() As Long
End Type
Sub Work(ByRef a() As Long, ByRef state As State)
 ReDim state.values(4)
End Sub`, false},
		{"owned record field", `Private Type State
 values() As Long
End Type
Sub Work(ByRef a() As Long)
 Dim state As State
 ReDim state.values(4)
End Sub`, true},
		{"erased callback", `Sub Work(ByRef a() As Long, ByVal obj As Object)
 obj.Touch
End Sub`, false},
		{"recursive pure", `Sub Work(ByRef a() As Long, ByVal n As Long)
 If n > 0 Then Work a, n - 1
 a(0) = 1
End Sub`, true},
		{"mutually recursive pure", `Sub Work(ByRef a() As Long, ByVal n As Long)
 If n > 0 Then Other a, n - 1
End Sub
Sub Other(ByRef b() As Long, ByVal n As Long)
 If n > 0 Then Work b, n - 1
 b(0) = 1
End Sub`, true},
		{"mutually recursive resize", `Sub Work(ByRef a() As Long, ByVal n As Long)
 If n > 0 Then Other a, n - 1
End Sub
Sub Other(ByRef b() As Long, ByVal n As Long)
 If n > 0 Then Work b, n - 1
 ReDim b(4)
End Sub`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := lower(t, "Main.bas", test.body)
			analysis := identity.Analyze(p)
			work := p.Procedures[0]
			if retained(analysis, work) != test.stable {
				t.Fatalf("retained=%t, want %t\n%s", !test.stable, test.stable, analysis.Format())
			}
		})
	}
}

func TestRetentionInterfaceAndCleanup(t *testing.T) {
	for _, mutate := range []bool{false, true} {
		t.Run(map[bool]string{false: "stable interface", true: "mutating interface"}[mutate], func(t *testing.T) {
			body := "a(0) = 1"
			if mutate {
				body = "ReDim a(4)"
			}
			p := lower(t, "Main.bas", `Sub Work(ByRef a() As Long, ByVal worker As IWorker)
 worker.Touch a
End Sub`, "IWorker.cls", `Public Sub Touch(ByRef a() As Long)
End Sub`, "Worker.cls", `Implements IWorker
Private Sub IWorker_Touch(ByRef a() As Long)
 `+body+`
End Sub`)
			analysis := identity.Analyze(p)
			// Work, the interface contract, and its implementation.
			for _, procedure := range p.Procedures {
				if retained(analysis, procedure) == mutate {
					t.Fatal(analysis.Format())
				}
			}
		})
	}
	p := lower(t, "Main.bas", `Public shared() As Long
Sub Work(ByRef a() As Long)
 Dim value As Variant
 a(0) = 1
End Sub`, "Owner.cls", `Private Sub Class_Terminate()
 ReDim Main.shared(4)
End Sub`)
	if retained(identity.Analyze(p), p.Procedures[0]) {
		t.Fatal("implicit termination callback can invalidate a global alias")
	}
}

// A call that never returns still reports what it invalidated: its caller
// is suspended inside it while the effects happen. Today the ReDim's error edge
// also reaches the exit; this guards a tighter error model.
func TestSummariesCoverNonReturningCalls(t *testing.T) {
	p := lower(t, "Main.bas", `Sub Spin(ByRef a() As Long)
    On Error Resume Next
    Do
        ReDim a(1)
    Loop
End Sub`)
	if retained(identity.Analyze(p), p.Procedures[0]) {
		t.Fatal("a resize before a non-returning loop must disqualify")
	}
}
