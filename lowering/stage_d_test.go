package lowering_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestStageDConversionsUseVerifiedCatalogCalls(t *testing.T) {
	p := lower(t, module(`Sub Probe()
 Dim b As Byte, value As Variant, flag As Boolean, n As Long, text As String
 b=VBA.Conversion.CByte(Expression:=255)
 value=VBA.CVErr(Expression:=2001)
 flag=VBA.Information.IsError(Arg:=value)
 n=VBA.CLng(value)
 text=VBA.CStr(value)
 Dim day As Date, money As Currency
 day=VBA.CDate(Expression:="2024-02-29")
 money=VBA.CCur(Expression:="1.2345")
 value=VBA.CVDate(Expression:=1.25)
 value=VBA.CDec(Expression:="0.1")
End Sub`))
	want := map[stdlib.MemberID]ir.Scalar{
		stdlib.CByte: ir.Byte, stdlib.CVErr: ir.Variant, stdlib.IsError: ir.Boolean,
		stdlib.CLng: ir.Long, stdlib.CStr: ir.String,
		stdlib.CDate: ir.Date, stdlib.CCur: ir.Currency, stdlib.CVDate: ir.Variant, stdlib.CDec: ir.Variant,
	}
	for _, statement := range p.Procedures[0].Statements {
		for _, in := range statement.Instructions {
			if in.Op != ir.OpLibraryCall {
				continue
			}
			scalar, ok := want[in.Library]
			if !ok || in.Type != (ir.Type{Scalar: scalar, Form: ir.Result}) || len(in.Args) != 1 {
				t.Fatal("conversion lost its catalog/result contract", ir.Format(p))
			}
			delete(want, in.Library)
		}
	}
	if len(want) != 0 {
		t.Fatal("conversion bypassed its explicit helper", want)
	}
}
