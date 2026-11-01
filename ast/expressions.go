package ast

// PropertyAccessExpression follows typescript-go's expression/name shape. The
// operator is retained because VBA distinguishes dot from dictionary (!) access.
type PropertyAccessExpression struct {
	NodeBase
	Expression    *Expression
	OperatorToken *TokenNode
	Name          *IdentifierNode
	WithTarget    *Node // Enclosing With for an omitted Expression; binding only.
}

func (node *Node) AsPropertyAccessExpression() *PropertyAccessExpression {
	return node.data.(*PropertyAccessExpression)
}
func (n *PropertyAccessExpression) DeeplyEquals(other *Node) bool {
	o := other.AsPropertyAccessExpression()
	return n.Expression.DeeplyEquals(o.Expression) && n.OperatorToken.DeeplyEquals(o.OperatorToken) && n.Name.DeeplyEquals(o.Name)
}
func (f *NodeFactory) NewPropertyAccessExpression(expression, operator, name *Node) *Node {
	data := f.propertyAccessExpressionPool.New()
	data.Expression, data.OperatorToken, data.Name = expression, operator, name
	return f.newNode(KindPropertyAccessExpression, data)
}

// CallExpression also represents indexing and default-member application: VBA
// uses the same syntax for all three, so their resolution belongs to later passes.
type CallExpression struct {
	NodeBase
	Expression     *Expression
	Arguments      *NodeList
	HasParentheses bool // False for an unparenthesized statement argument list.
}

func (node *Node) AsCallExpression() *CallExpression { return node.data.(*CallExpression) }
func (n *CallExpression) DeeplyEquals(other *Node) bool {
	o := other.AsCallExpression()
	return n.HasParentheses == o.HasParentheses && n.Expression.DeeplyEquals(o.Expression) && Equals(n.Arguments, o.Arguments)
}
func (f *NodeFactory) NewCallExpression(expression *Node, arguments *NodeList, parentheses bool) *Node {
	data := f.callExpressionPool.New()
	data.Expression, data.Arguments, data.HasParentheses = expression, arguments, parentheses
	return f.newNode(KindCallExpression, data)
}

// CallStatement preserves the explicit Call keyword and invocation in statement
// context, including a bare parameterless invocation.
type CallStatement struct {
	StatementBase
	CallKeyword *TokenNode
	Expression  *Expression
}

func (node *Node) AsCallStatement() *CallStatement { return node.data.(*CallStatement) }
func (n *CallStatement) DeeplyEquals(other *Node) bool {
	o := other.AsCallStatement()
	return n.CallKeyword.DeeplyEquals(o.CallKeyword) && n.Expression.DeeplyEquals(o.Expression)
}
func (f *NodeFactory) NewCallStatement(keyword, expression *Node) *Node {
	data := f.callStatementPool.New()
	data.CallKeyword, data.Expression = keyword, expression
	return f.newNode(KindCallStatement, data)
}

type NamedArgument struct {
	NodeBase
	Name       *IdentifierNode
	Expression *Expression
}

func (node *Node) AsNamedArgument() *NamedArgument { return node.data.(*NamedArgument) }
func (n *NamedArgument) DeeplyEquals(other *Node) bool {
	o := other.AsNamedArgument()
	return n.Name.DeeplyEquals(o.Name) && n.Expression.DeeplyEquals(o.Expression)
}
func (f *NodeFactory) NewNamedArgument(name, expression *Node) *Node {
	data := f.namedArgumentPool.New()
	data.Name, data.Expression = name, expression
	return f.newNode(KindNamedArgument, data)
}

// OmittedExpression is a deliberate empty argument slot, not a recovery node.
type OmittedExpression struct{ NodeBase }

func (n *OmittedExpression) DeeplyEquals(other *Node) bool {
	return other.Kind == KindOmittedExpression
}
func (f *NodeFactory) NewOmittedExpression() *Node {
	return f.newNode(KindOmittedExpression, f.omittedExpressionPool.New())
}

// NewExpression follows the dedicated typescript-go expression shape, but its
// operand is VBA type-name syntax and it has no constructor argument list.
type NewExpression struct {
	NodeBase
	Expression *Node // Identifier or QualifiedName, resolved in type space later.
}

func (node *Node) AsNewExpression() *NewExpression { return node.data.(*NewExpression) }
func (n *NewExpression) DeeplyEquals(other *Node) bool {
	return n.Expression.DeeplyEquals(other.AsNewExpression().Expression)
}
func (f *NodeFactory) NewNewExpression(expression *Node) *Node {
	n := f.newExpressionPool.New()
	n.Expression = expression
	return f.newNode(KindNewExpression, n)
}
