package parser_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func TestModuleAttributesPreserveSyntaxAndMetadata(t *testing.T) {
	file := parseDeclarations(t, `Attribute VB_Name = "Utilities"
Attribute VB_PredeclaredId = True
Attribute VB_Description = "A ""quoted"" description", 1
Option Private Module
Option Explicit
Sub Work()
Attribute Work.VB_Description = "description"
End Sub`)
	if file.ModuleName != "Utilities" || !file.PredeclaredID || file.Statements.Nodes[3].Kind != ast.KindOptionPrivateModuleStatement {
		t.Fatal("module metadata was lost")
	}
	attr := file.Statements.Nodes[2].AsAttributeStatement()
	if len(attr.Values.Nodes) != 2 || attr.Values.Nodes[0].AsStringLiteral().Text != `A "quoted" description` {
		t.Fatal("attribute values were not preserved")
	}
	binder.BindSourceFile(file)
	if !file.OptionPrivateModule || len(file.BindDiagnostics) != 0 || len(file.Locals) != 1 || len(file.FunctionDeclarations.Nodes[0].Locals) != 0 {
		t.Fatal("attribute metadata was executed as value syntax", file.BindDiagnostics)
	}
	other := parser.ParseSourceFile(file.Text)
	if !file.AsNode().DeeplyEquals(other) {
		t.Fatal("module attributes do not participate in AST equality")
	}
}

func TestMalformedModuleMetadataRecovers(t *testing.T) {
	for _, damaged := range []string{
		"Attribute VB_Name = True\n", "Attribute VB_Name = \"\"\n",
		"Attribute VB_PredeclaredId = 1\n", "Attribute VB_Name = \"First\", 2\n",
		"Attribute VB_Name =\n", "Attribute =\n", "Attribute VB_Name \"First\"\n",
		"Option Private\n", "Option Private Module\nOption Private Module\n",
		"Sub Bad()\nOption Private Module\nEnd Sub\n",
	} {
		t.Run(damaged, func(t *testing.T) {
			file := parser.ParseSourceFile(damaged + "Sub Good()\nEnd Sub").AsSourceFile()
			if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) == 0 || file.FunctionDeclarations.Nodes[len(file.FunctionDeclarations.Nodes)-1].AsFunctionDeclaration().Name().AsIdentifier().Text != "Good" {
				t.Fatal("malformed module metadata did not recover to the next procedure")
			}
			binder.BindSourceFile(file)
		})
	}
}

func TestLastModuleAttributeWins(t *testing.T) {
	file := parseDeclarations(t, `Attribute VB_Name = "First"
Attribute VB_Name = "Last"
Attribute VB_PredeclaredId = True
Attribute VB_PredeclaredId = False`)
	if file.ModuleName != "Last" || file.PredeclaredID {
		t.Fatal("the last attribute did not determine the module metadata")
	}
}
