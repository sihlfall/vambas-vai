package lowering_test

import (
	"sihlfall.ch/vambas-vai/ir"
	"strings"
	"testing"
)

func TestArrayReadAndByRefLifetimes(t *testing.T) {
	p := lower(t, module(`Sub Run(ByRef a() As Long,ByRef n As Long)
 n=a(0)
 Change a(0)
End Sub
Sub Change(ByRef value As Long)
 value=value+1
End Sub`))
	ss := p.Procedures[0].Statements
	count := func(s *ir.Statement, op ir.Op) int {
		n := 0
		for _, in := range s.Instructions {
			if in.Op == op {
				n++
			}
		}
		return n
	}
	if count(ss[0], ir.OpArrayIndex) != 1 || count(ss[0], ir.OpReleaseElement) != 1 || count(ss[1], ir.OpArrayIndex) != 1 || count(ss[1], ir.OpReleaseElement) != 0 {
		t.Fatal(ir.Format(p))
	}
	// Move release before the reference read. Even with a token still present in
	// the cleanup list, that read would use memory whose array may be destroyed.
	code := ss[0].Instructions
	read, release := 0, 0
	for i, in := range code {
		if in.Op == ir.OpReadReference {
			read = i
		}
		if in.Op == ir.OpReleaseElement {
			release = i
		}
	}
	value := code[release]
	copy(code[read+1:release+1], code[read:release])
	code[read] = value
	var messages strings.Builder
	for _, d := range ir.Verify(p) {
		messages.WriteString(d.Message)
	}
	if !strings.Contains(messages.String(), "use after element reference release") {
		t.Fatal(messages.String())
	}
}
