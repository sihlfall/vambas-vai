package lowering_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ir"
)

func TestImplicitVariantLocalsAndUntypedSignatures(t *testing.T) {
	p := lower(t, module(`Function Add(a, b)
 sum = a + b
 Add = sum
End Function
`))
	f := p.Procedures[0]
	if f.ReturnType.Scalar != ir.Variant || len(f.Slots) != 4 || f.Slots[3].Name != "sum" || f.Slots[3].Type.Scalar != ir.Variant {
		t.Fatal(ir.Format(p))
	}
}

func TestVariantViewsKeepOriginalStorageAndForwarding(t *testing.T) {
	p := lower(t, module(`Sub Run(ByRef n As Long, ByRef s As String, ByRef v As Variant)
 Change n
 Change s
 Change v
End Sub
Sub Change(ByRef v As Variant)
 v = 3
End Sub
`))
	count := func(s *ir.Statement, op ir.Op) int {
		n := 0
		for _, in := range s.Instructions {
			if in.Op == op {
				n++
			}
		}
		return n
	}
	ss := p.Procedures[0].Statements
	if count(ss[0], ir.OpVariantRef) != 1 || count(ss[0], ir.OpBox) != 0 || count(ss[1], ir.OpPinString) != 1 || count(ss[1], ir.OpVariantRef) != 1 || count(ss[2], ir.OpVariantRef) != 0 {
		t.Fatal(ir.Format(p))
	}
	// A raw managed load would bypass dereferencing/copying (and String pinning).
	for i := range p.Procedures[1].Statements[0].Instructions {
		in := &p.Procedures[1].Statements[0].Instructions[i]
		if in.Op == ir.OpStoreManaged {
			in.Op = ir.OpStore
			break
		}
	}
	var messages strings.Builder
	for _, d := range ir.Verify(p) {
		messages.WriteString(d.Message)
	}
	if !strings.Contains(messages.String(), "managed slots require") {
		t.Fatal(messages.String())
	}
}
