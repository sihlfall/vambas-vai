package parser_test

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
	"testing"
)

func TestOptionBaseAndEraseSyntax(t *testing.T) {
	source := "Option Base 1\nSub Run()\n Dim a() As Long, v As Variant\n Erase a(), v(0)\nEnd Sub\n"
	file := parseDeclarations(t, source)
	if file.Statements.Nodes[0].AsOptionBaseStatement().Base != 1 {
		t.Fatal("Option Base value was lost")
	}
	erase := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[1].AsEraseStatement()
	if len(erase.Expressions.Nodes) != 2 || erase.Expressions.Nodes[0].Kind != ast.KindCallExpression {
		t.Fatal("Erase lost its designators")
	}
	if !file.AsNode().DeeplyEquals(parser.ParseSourceFile(source)) {
		t.Fatal("syntax equality changed")
	}
	binder.BindSourceFile(file)
	if file.OptionBase != 1 {
		t.Fatal("default bound was not bound")
	}
	validateRanges(t, file)
}

func TestArrayOptionAndEraseRecovery(t *testing.T) {
	for _, source := range []string{
		"Option Base 2\nSub Good()\nEnd Sub\n",
		"Option Base\nSub Good()\nEnd Sub\n",
		"Option Base 1\nOption Base 0\nSub Good()\nEnd Sub\n",
		"Sub Bad()\nOption Base 1\nEnd Sub\nSub Good()\nEnd Sub\n",
		"Sub Bad()\nErase\nEnd Sub\nSub Good()\nEnd Sub\n",
		"Sub Bad()\nErase a,\nEnd Sub\nSub Good()\nEnd Sub\n",
	} {
		file := parser.ParseSourceFile(source).AsSourceFile()
		if len(file.ParseDiagnostics) == 0 {
			t.Fatal("invalid array syntax accepted", source)
		}
		if len(file.FunctionDeclarations.Nodes) == 0 {
			t.Fatal("following procedure was lost")
		}
		validateRanges(t, file)
	}
}
