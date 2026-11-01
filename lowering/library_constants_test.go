package lowering_test

import (
	"slices"
	"testing"

	"sihlfall.ch/vambas-vai/ir"
)

func TestCatalogStringConstantValuesAndRepresentation(t *testing.T) {
	for _, test := range []struct {
		expression string
		op         ir.Op
		units      []uint16
	}{
		{"VBA.vbNullString", ir.OpNullString, nil},
		{"VBA.vbNullChar", ir.OpStringLiteral, []uint16{0}},
		{"VBA.vbCrLf", ir.OpStringLiteral, []uint16{13, 10}},
		{"VBA.Constants.vbNewLine", ir.OpStringLiteral, []uint16{13, 10}},
		{"VBA.vbCr", ir.OpStringLiteral, []uint16{13}},
		{"VBA.vbLf", ir.OpStringLiteral, []uint16{10}},
		{"VBA.vbTab", ir.OpStringLiteral, []uint16{9}},
		{"VBA.vbBack", ir.OpStringLiteral, []uint16{8}},
		{"VBA.vbFormFeed", ir.OpStringLiteral, []uint16{12}},
		{"VBA.vbVerticalTab", ir.OpStringLiteral, []uint16{11}},
		{"vbNullString", ir.OpStringLiteral, []uint16{'l', 'o', 'c', 'a', 'l'}},
		{"Alias", ir.OpNullString, nil},
		{"Joined", ir.OpStringLiteral, nil},
	} {
		t.Run(test.expression, func(t *testing.T) {
			p := lower(t, module(`Option Explicit
Private Const vbNullString As String = "local"
Private Const Alias As String = VBA.vbNullString
Private Const Joined As String = VBA.vbNullString & ""
Function Probe() As String
 Probe = `+test.expression+`
End Function
`))
			count := 0
			for _, statement := range p.Procedures[0].Statements {
				for _, in := range statement.Instructions {
					if in.Op == ir.OpStringLiteral || in.Op == ir.OpNullString {
						count++
						if in.Op != test.op || !slices.Equal(in.Units, test.units) {
							t.Fatal("constant lost its UTF-16 contents or null-pointer distinction", ir.Format(p))
						}
					}
				}
			}
			if count != 1 {
				t.Fatal("expected one materialized string constant", ir.Format(p))
			}
		})
	}
}

func TestCatalogObjectErrorLowering(t *testing.T) {
	p := lower(t, module("Function Probe() As Long\n Probe = VBA.vbObjectError\nEnd Function\n"))
	for _, statement := range p.Procedures[0].Statements {
		for _, in := range statement.Instructions {
			if in.Op == ir.OpConstant && in.Type == (ir.Type{Scalar: ir.Long}) && in.Integer == -2147221504 {
				return
			}
		}
	}
	t.Fatal("vbObjectError did not materialize its signed Long value", ir.Format(p))
}
