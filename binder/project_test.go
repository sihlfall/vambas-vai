package binder_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func TestProjectBindingRejectsSharedOrAlreadyBoundTrees(t *testing.T) {
	fresh := parser.ParseSourceFile("Sub S()\nEnd Sub").AsSourceFile()
	bound := parser.ParseSourceFile("").AsSourceFile()
	binder.BindSourceFile(bound)
	for _, files := range [][]*ast.SourceFile{{fresh, bound}, {fresh, fresh}, {fresh, nil}} {
		if _, err := binder.BindProject("Project", files); err == nil || fresh.IsBound() || fresh.Symbol != nil {
			t.Fatal("invalid project inputs changed a fresh tree or were accepted")
		}
	}
}
