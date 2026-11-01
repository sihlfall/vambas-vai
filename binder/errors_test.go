package binder_test

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/stdlib"
	"testing"
)

func TestErrorTargetsAndAmbientObject(t *testing.T) {
	f := controlFile(t, `On Error GoTo handler
Err.Raise 6
handler:
Resume done
done:
F=Err.Number
Err.Clear`)
	body := statements(f, 0)
	target := body[0].AsOnErrorStatement().Target.AsGotoStatement().Target
	fn := f.FunctionDeclarations.Nodes[0]
	if target != fn.AsFunctionDeclaration().Labels.Lookup("handler") {
		t.Fatal("handler label not resolved")
	}
	raised := body[1].AsCallStatement().Expression.AsCallExpression().Expression.Symbol
	if raised == nil || raised.Library != stdlib.ErrRaise || raised.Intrinsic != ast.IntrinsicNone {
		t.Fatal("Err.Raise identity lost")
	}
	resume := body[3].AsResumeStatement().Target.AsGotoStatement().Target
	if resume != fn.AsFunctionDeclaration().Labels.Lookup("done") {
		t.Fatal("Resume label not resolved")
	}
}
