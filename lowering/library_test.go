package lowering_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ir"
)

// Catalog identity alone must not select the Variant-snapshot helper ABI:
// bounds retain array storage, Mid retains Missing, and conversions retain their
// declared result types (explicit scalar conversions now use catalog helpers).
func TestCatalogSpecializedLowering(t *testing.T) {
	p := lower(t, module(`Option Explicit
Function Probe(ByRef values() As Long, Optional absent As Variant) As Long
 Dim text As String, value As Variant, count As Long
 text=VBA.Mid$(VBA.ChrW(CharCode:=65) & "BC",Start:=2)
 value=VBA.CVar(VBA.Len(text))
 count=VBA.CInt(value)
 count=VBA.CLng(value)
 value=VBA.CBool(count)
 text=VBA.CStr(count)
 value=VBA.CSng(count)
 value=VBA.CDbl(count)
 value=VBA.VarType(value)
 value=VBA.IsEmpty(value)
 value=VBA.IsNull(value)
 value=VBA.IsMissing(absent)
 value=VBA.IsArray(values)
 Probe=VBA.LBound(ArrayName:=values)+VBA.UBound(Dimension:=1,ArrayName:=values)+VBA.LenB(text)
End Function
`))
	var bounds int
	for _, statement := range p.Procedures[0].Statements {
		for _, in := range statement.Instructions {
			if in.Op == ir.OpArrayBound {
				bounds++
			}
		}
	}
	if bounds != 2 {
		t.Fatal("catalog bounds did not retain their specialized operations", ir.Format(p))
	}
}
