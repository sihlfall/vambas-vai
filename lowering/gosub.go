package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/ir"
)

func (b *procedureBuilder) gosubStatement(node *ast.Node) {
	target := node.AsGotoStatement().Target
	if target == nil || b.labels[target.ValueDeclaration] == nil {
		b.unsupported(node, "unresolved GoSub target")
		return
	}
	next := b.block(node)
	b.emitGosub(node, b.labels[target.ValueDeclaration], next)
	b.activate(next)
}
func (b *procedureBuilder) emitGosub(node *ast.Node, target, next *ir.Statement) {
	b.operation(node, ir.OpGosubPush, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime)
	b.statement.End = ir.Gosub
	b.link(b.statement, target, 1)
	b.link(b.statement, next, 0)
}
