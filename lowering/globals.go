package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/ir"
)

func (l *lowerer) declareGlobals(statement *ast.Node) {
	s := statement.AsVariableStatement()
	for _, n := range nodes(s.DeclarationList.AsVariableDeclarationList().Declarations) {
		d := n.AsVariableDeclaration()
		if fixedString(d.Type) || d.Initializer != nil {
			l.unsupported(n, "fixed-length or explicitly initialized globals")
			continue
		}
		g := ir.Global{ID: ir.GlobalID(len(l.program.Globals) + 1), Name: ast.SourceFileOf(n).ModuleName + "." + d.Name.AsIdentifier().Text, Type: l.scalar(n, l.checker.GetTypeOfSymbol(n.Symbol), false), Source: source(n), AutoNew: l.autoNew(n), AutoClass: l.autoClass(n)}
		for _, bound := range l.checker.GetArrayBounds(n) {
			g.Bounds = append(g.Bounds, ir.ArrayBound{Lower: bound.Lower, Upper: bound.Upper})
		}
		l.globals[n.Symbol] = g.ID
		l.program.Globals = append(l.program.Globals, g)
	}
}
