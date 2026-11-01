package ast

type ReDimStatement struct {
	StatementBase
	PreserveKeyword *Node
	Declarations    *NodeList
}

func (n *Node) AsReDimStatement() *ReDimStatement { return n.data.(*ReDimStatement) }
func (n *ReDimStatement) DeeplyEquals(other *Node) bool {
	o := other.AsReDimStatement()
	return n.PreserveKeyword.DeeplyEquals(o.PreserveKeyword) && Equals(n.Declarations, o.Declarations)
}
func (f *NodeFactory) NewReDimStatement(preserve *Node, declarations *NodeList) *Node {
	n := f.reDimStatementPool.New()
	n.PreserveKeyword, n.Declarations = preserve, declarations
	return f.newNode(KindReDimStatement, n)
}

// The final parentheses contain allocation bounds, not call/index arguments.
// Expression may itself contain ordinary calls/indexes and a With receiver.
type ReDimDeclaration struct {
	NodeBase
	Expression *Node
	Dimensions *NodeList
	Type       *Node
}

func (n *Node) AsReDimDeclaration() *ReDimDeclaration { return n.data.(*ReDimDeclaration) }
func (n *ReDimDeclaration) DeeplyEquals(other *Node) bool {
	o := other.AsReDimDeclaration()
	return n.Expression.DeeplyEquals(o.Expression) && Equals(n.Dimensions, o.Dimensions) && n.Type.DeeplyEquals(o.Type)
}
func (f *NodeFactory) NewReDimDeclaration(expression *Node, dimensions *NodeList, typeNode *Node) *Node {
	n := f.reDimDeclarationPool.New()
	n.Expression, n.Dimensions, n.Type = expression, dimensions, typeNode
	return f.newNode(KindReDimDeclaration, n)
}
