package parser_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/parser"
)

func TestReDimSyntaxAndReceiverApplications(t *testing.T) {
	text := `Sub Run()
ReDim Preserve obj.Arrays(i).Buffer(0 To n, 1 To Grow(n)) As Long, other(10)
With obj
 ReDim .Buffer(0 To n)
End With
ReDim typed&(n)
ReDim Make(index:=i).Buffer(0 To n)
End Sub
`
	file := parseDeclarations(t, text)
	statements := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().Body.AsBlock().Statements.Nodes
	r := statements[0].AsReDimStatement()
	if r.PreserveKeyword == nil || len(r.Declarations.Nodes) != 2 {
		t.Fatal(r)
	}
	d := r.Declarations.Nodes[0].AsReDimDeclaration()
	if d.Expression.Kind != ast.KindPropertyAccessExpression || len(d.Dimensions.Nodes) != 2 || d.Type == nil || d.Dimensions.Nodes[1].AsArrayDimension().UpperBound.Kind != ast.KindCallExpression {
		t.Fatal(d)
	}
	if d.Expression.AsPropertyAccessExpression().Expression.Kind != ast.KindCallExpression {
		t.Fatal("receiver indexing was lost")
	}
	if statements[2].AsReDimStatement().Declarations.Nodes[0].AsReDimDeclaration().Expression.AsIdentifier().TypeHint != ast.KindAMPERSAND {
		t.Fatal("typed ReDim name lost")
	}
	if statements[3].AsReDimStatement().Declarations.Nodes[0].AsReDimDeclaration().Expression.AsPropertyAccessExpression().Expression.AsCallExpression().Arguments.Nodes[0].Kind != ast.KindNamedArgument {
		t.Fatal("named receiver argument was lost")
	}
	if !file.AsNode().DeeplyEquals(parser.ParseSourceFile(text)) {
		t.Fatal("ReDim syntax equality failed")
	}
}

func TestReDimRecovery(t *testing.T) {
	for _, statement := range []string{"ReDim", "ReDim Preserve arr(", "ReDim arr(1 To)", "ReDim arr(1),", "ReDim arr As Long", "ReDim arr()", "ReDim arr(index:=1)"} {
		text := "Sub Broken()\n" + statement + "\nEnd Sub\nSub Good()\nEnd Sub\n"
		file := parser.ParseSourceFile(text).AsSourceFile()
		if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) != 2 {
			t.Fatal(statement, file.ParseDiagnostics)
		}
		validateRanges(t, file)
	}
}
