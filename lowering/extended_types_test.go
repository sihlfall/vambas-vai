package lowering_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
	"strings"
	"testing"
)

func TestCurrencyLiteralHasExactScaledIR(t *testing.T) {
	p := lower(t, module(`Function Value() As Currency
 Value=-922337203685477.5808@
End Function`))
	found := false
	for _, statement := range p.Procedures[0].Statements {
		for _, in := range statement.Instructions {
			if in.Op == ir.OpConstant && in.Type.Scalar == ir.Currency {
				found = true
				if in.Integer != -9223372036854775808 {
					t.Fatal("Currency literal lost precision", ir.Format(p))
				}
			}
			if in.Op == ir.OpStringLiteral || in.Op == ir.OpBox {
				t.Fatal("Currency literal used runtime coercion", ir.Format(p))
			}
		}
	}
	if !found {
		t.Fatal("missing scaled literal", ir.Format(p))
	}
}

func TestExtendedTypeDeferredConstantProfiles(t *testing.T) {
	for _, source := range []string{
		"Const Value As Currency=1.2345@\n",
		"Sub Run(Optional value As Currency=1.2345@)\nEnd Sub",
		"Sub Run(Optional value As Date=1)\nEnd Sub",
	} {
		p, diagnostics := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(source)}}))
		if p != nil || len(diagnostics) == 0 {
			t.Fatal("deferred constant profile silently accepted", source)
		}
		found := false
		for _, d := range diagnostics {
			found = found || strings.Contains(d.Message, "Date/Currency")
		}
		if !found {
			t.Fatal("missing profile diagnostic", diagnostics)
		}
	}
}
