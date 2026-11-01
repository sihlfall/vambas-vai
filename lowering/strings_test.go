package lowering_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func TestStringOwnershipAndIntrinsics(t *testing.T) {
	p := lower(t, module(`Option Explicit
Private Const Prefix As String = "Hel" & "lo"
Function Greeting(ByVal name As String) As String
    Dim copy As String
    copy = Prefix & name
    Greeting = copy
End Function
Function Count() As Long
    Dim s As String
    s = Greeting(vbNullString) & vbNullChar & ChrW(CharCode:=55357) & ChrW(56832)
    Count = Len(s) + VBA.LenB(s)
End Function
`))
	if p.Procedures[0].ReturnType.Scalar != ir.String || p.Procedures[0].Effects&ir.ManagesLifetime == 0 {
		t.Fatal("missing String type or lifetime effects")
	}
	for _, f := range p.Procedures {
		for _, s := range f.Statements {
			for _, in := range s.Instructions {
				if in.Type == (ir.Type{Scalar: ir.String}) && in.Op == ir.OpLoad && in.Ownership != ir.Owned {
					t.Fatal("mutable String load lost its value snapshot")
				}
			}
		}
	}
	// Cleanup is verified rather than trusted as an optional backend convention.
	f := p.Procedures[0]
	for _, s := range f.Statements {
		if len(s.Cleanup) > 0 {
			s.Cleanup = nil
			if len(ir.Verify(p)) == 0 {
				t.Fatal("String ownership leaked through verification")
			}
			break
		}
	}
}

func TestUnsupportedStringFormsAreRejected(t *testing.T) {
	for _, text := range []string{
		"Sub F()\n Dim s As String * 3\n s = \"abcdef\"\nEnd Sub\n",
		"Function F() As Long\n F = Len(1)\nEnd Function\n",
		"Sub F(ByRef s As String)\n MidB$(s,1,1) = \"x\"\nEnd Sub\n",
		"Function F(ByVal s As String) As Boolean\n F = s Like \"x*\"\nEnd Function\n",
		"Function F(ByVal s As String) As Long\n F = s\nEnd Function\n",
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(text)}})
		semantic, diagnostics := lowering.Lower(p)
		if semantic != nil || len(diagnostics) == 0 {
			t.Fatalf("unsupported String behavior silently compiled: %s", text)
		}
	}
}

func TestStringIntrinsicShadowing(t *testing.T) {
	p := lower(t, module("Function Len(ByVal expression As String) As Long\n Len = 42\nEnd Function\nFunction F() As Long\n F = Len(\"abc\")\nEnd Function\n"))
	call := false
	for _, s := range p.Procedures[1].Statements {
		for _, in := range s.Instructions {
			if in.Op == ir.OpLen {
				t.Fatal("user Len declaration was replaced by intrinsic")
			}
			if in.Op == ir.OpCall && in.Callee == 1 {
				call = true
			}
		}
	}
	if !call {
		t.Fatal("user Len declaration was not called")
	}
}

func TestMidIntrinsicShadowing(t *testing.T) {
	p := lower(t, module("Function Mid(ByVal s As String, ByVal start As Long) As String\n Mid=s\nEnd Function\nFunction F() As String\n F=Mid$(\"abc\",1)\nEnd Function\n"))
	found := false
	for _, s := range p.Procedures[1].Statements {
		for _, in := range s.Instructions {
			if in.Op == ir.OpMid {
				t.Fatal("user Mid was replaced by intrinsic")
			}
			if in.Op == ir.OpCall && in.Callee == 1 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("user Mid was not called")
	}
}
