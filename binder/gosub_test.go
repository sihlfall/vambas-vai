package binder

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/parser"
	"testing"
)

func TestGosubUsesLocalLabelsAndConservativeReturnFlow(t *testing.T) {
	f := parser.ParseSourceFile("Sub Run()\n GoSub Work\n Dim n As Long\n n = 1\n Exit Sub\nWork:\n Return\nEnd Sub\nSub Other()\n GoSub Work\nEnd Sub\n").AsSourceFile()
	BindSourceFile(f)
	if len(f.BindDiagnostics) != 1 || f.BindDiagnostics[0].Code != ast.BindingUndefinedLabel {
		t.Fatal(f.BindDiagnostics)
	}
	fn := f.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	body := fn.Body.AsBlock().Statements.Nodes
	if body[0].AsGosubStatement().Target != fn.Labels.Lookup("Work") {
		t.Fatal("GoSub escaped the label namespace")
	}
	if body[2].Flags&ast.NodeFlagsUnreachable != 0 {
		t.Fatal("continuation was marked unreachable")
	}
	if fn.ReturnFlowNode == nil {
		t.Fatal("procedure exit flow lost")
	}
}
func TestModuleGosubReturnDoesNotPanic(t *testing.T) {
	for _, text := range []string{"Return\n", "GoSub Missing\n", "On 1 GoSub Missing\n"} {
		f := parser.ParseSourceFile(text).AsSourceFile()
		BindSourceFile(f)
		if len(f.BindDiagnostics) == 0 && len(f.ParseDiagnostics) == 0 {
			t.Fatal("module jump accepted", text)
		}
	}
}
