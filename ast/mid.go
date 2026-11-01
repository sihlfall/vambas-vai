package ast

// Mid assignment is a statement special form, not assignment to a function call.
// Mode retains Mid versus Mid$; a nil Length preserves the omitted argument.
type MidAssignmentStatement struct {
	StatementBase
	Mode, Target, Start, Length, Expression *Node
}

func (n *Node) AsMidAssignmentStatement() *MidAssignmentStatement {
	return n.data.(*MidAssignmentStatement)
}
func (n *MidAssignmentStatement) DeeplyEquals(other *Node) bool {
	o := other.AsMidAssignmentStatement()
	return n.Mode.DeeplyEquals(o.Mode) && n.Target.DeeplyEquals(o.Target) && n.Start.DeeplyEquals(o.Start) && n.Length.DeeplyEquals(o.Length) && n.Expression.DeeplyEquals(o.Expression)
}
func (f *NodeFactory) NewMidAssignmentStatement(mode, target, start, length, expression *Node) *Node {
	n := f.midAssignmentPool.New()
	n.Mode, n.Target, n.Start, n.Length, n.Expression = mode, target, start, length, expression
	return f.newNode(KindMidAssignmentStatement, n)
}
