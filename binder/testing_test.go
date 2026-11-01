package binder

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/parser"
	"testing"
)

func TestStandaloneVirtualLibraryTypes(t *testing.T) {
	file := parser.ParseSourceFile("Option Explicit\nPrivate a As New Rubberduck.AssertClass\nSub Example()\n a.IsTrue True\nEnd Sub\n").AsSourceFile()
	BindSourceFile(file)
	if len(file.BindDiagnostics) > 0 {
		t.Fatal(file.BindDiagnostics)
	}
	symbol := file.Locals.Lookup("a")
	name := symbol.ValueDeclaration.AsVariableDeclaration().Type.AsTypeReference().TypeName
	if name.Symbol == nil || name.Symbol.Intrinsic != ast.IntrinsicAssertClass {
		t.Fatal("standalone library type not resolved")
	}
}
