package parser_test

import (
	"sihlfall.ch/vambas-vai/ast"
	"testing"
)

func TestErrorPolicyAndResumeSyntax(t *testing.T) {
	f := parseDeclarations(t, `Sub S()
 On Error GoTo handler
 On Error Resume Next
 On Error GoTo 0
 On Error GoTo -1
 Resume
 Resume 0
 Resume Next
 Resume handler
 Error 11
handler:
End Sub`)
	statements := f.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	if len(statements) != 10 {
		t.Fatal("lost error statements", len(statements))
	}
	for i, mode := range []int{2, 3, 1, 4} {
		if statements[i].Kind != ast.KindOnErrorStatement || statements[i].AsOnErrorStatement().Mode != mode {
			t.Fatal("lost error policy", i)
		}
	}
	target := statements[0].AsOnErrorStatement().Target.AsGotoStatement()
	if target.Label.AsIdentifier().Text != "handler" {
		t.Fatal("lost handler syntax")
	}
	for i, mode := range []int{0, 0, 1, 2} {
		if statements[i+4].Kind != ast.KindResumeStatement || statements[i+4].AsResumeStatement().Mode != mode {
			t.Fatal("lost Resume form", i)
		}
	}
	if statements[8].Kind != ast.KindErrorStatement || statements[8].AsErrorStatement().Expression.AsNumericLiteral().Text != "11" {
		t.Fatal("lost Error expression")
	}
}
