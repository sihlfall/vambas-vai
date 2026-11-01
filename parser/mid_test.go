package parser_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/parser"
)

func TestMidStatementSyntax(t *testing.T) {
	file := parseDeclarations(t, "Sub Run()\n Mid$(text, 2, 3) = Mid(text, 1, 3)\n Mid (text, 1) = \"x\"\n text = Mid$(text, 2)\nEnd Sub\n")
	validateRanges(t, file)
	body := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	first := body[0].AsMidAssignmentStatement()
	if first.Mode.AsIdentifier().Text != "Mid$" || first.Length == nil || first.Expression.Kind != ast.KindCallExpression {
		t.Fatal("Mid$ assignment lost source distinctions")
	}
	if body[1].AsMidAssignmentStatement().Length != nil || body[2].Kind != ast.KindLetStatement {
		t.Fatal("Mid function and statement were conflated")
	}
	if !file.AsNode().DeeplyEquals(parser.ParseSourceFile(file.Text)) {
		t.Fatal("Mid syntax is not structurally comparable")
	}
}

func TestMidAssignmentRecovery(t *testing.T) {
	for _, statement := range []string{"Mid$(s,1,) = \"x\"", "Mid$(s,1,2,3) = \"x\"", "Mid$(s,1) ="} {
		file := parser.ParseSourceFile("Sub Run()\n" + statement + "\nEnd Sub\nSub Later()\nEnd Sub\n").AsSourceFile()
		if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) != 2 {
			t.Fatal("Mid recovery lost diagnostics or the following procedure", statement)
		}
	}
}
