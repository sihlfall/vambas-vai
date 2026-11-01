package compiler_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/compiler"
)

func TestReDimFlowPreservesRuntimeBoundEvaluation(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByVal n As Long)
 Dim values() As Long, other() As Long
 ReDim values(Grow(n)), other(n)
End Sub
Function Grow(ByVal n As Long) As Long
 Grow = n + 1
End Function
`})
	noErrors(t, p)
	end := p.Module("Main").FunctionDeclarations.Nodes[0].AsSubDeclaration().EndFlowNode
	if end.Flags&ast.FlowFlagsArrayMutation == 0 || end.Node.Kind != ast.KindReDimDeclaration || end.Antecedent.Flags&ast.FlowFlagsArrayMutation == 0 || end.Antecedent.Antecedent.Flags&ast.FlowFlagsCall == 0 || end.Antecedent.Antecedent.Node.Kind != ast.KindCallExpression {
		t.Fatal("ReDim did not bind bound calls before the two mutations", end)
	}
	if end.Antecedent.Antecedent.Antecedent.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("allocation bounds introduced a spurious call")
	}
}
