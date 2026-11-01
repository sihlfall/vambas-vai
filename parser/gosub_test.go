package parser

import (
	"sihlfall.ch/vambas-vai/ast"
	"testing"
)

func TestGosubSyntaxAndRanges(t *testing.T) {
	f := ParseSourceFile("Sub Run()\n GoSub Work: Return\n On 2 GoSub Work, 100\nGo Sub Work\n On 1 Go Sub 100\nWork: Return\n100 Return\nEnd Sub\n").AsSourceFile()
	if len(f.ParseDiagnostics) > 0 {
		t.Fatal(f.ParseDiagnostics)
	}
	body := f.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	if body[0].Kind != ast.KindGosubStatement || body[0].AsGotoStatement().Label.AsIdentifier().Text != "Work" || body[1].Kind != ast.KindGosubReturnStatement || body[2].Kind != ast.KindOnGosubStatement || len(body[2].AsOnGotoStatement().Destinations.Nodes) != 2 {
		t.Fatal(body)
	}
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Pos() < 0 || n.End() > len(f.Text) || n.End() < n.Pos() {
			t.Fatal(n.Loc)
		}
		return n.ForEachChild(visit)
	}
	visit(f.AsNode())
}
