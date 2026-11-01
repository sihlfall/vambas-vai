package ast

// Set shares the assignment payload and binder with Let, retaining a distinct
// kind for VBA's object-reference assignment versus value assignment.
type SetStatement = LetStatement

func (node *Node) AsSetStatement() *SetStatement { return node.AsLetStatement() }
func (f *NodeFactory) NewSetStatement(left, right *Node) *Node {
	n := f.NewLetStatement(left, right)
	n.Kind = KindSetStatement
	return n
}

type WithStatement struct {
	StatementBase
	Expression *Expression // Evaluated once, in the enclosing With context.
	Statement  *Statement
}

func (node *Node) AsWithStatement() *WithStatement { return node.data.(*WithStatement) }
func (n *WithStatement) DeeplyEquals(other *Node) bool {
	o := other.AsWithStatement()
	return n.Expression.DeeplyEquals(o.Expression) && n.Statement.DeeplyEquals(o.Statement)
}
func (f *NodeFactory) NewWithStatement(expression, statement *Node) *Node {
	n := f.withStatementPool.New()
	n.Expression, n.Statement = expression, statement
	return f.newNode(KindWithStatement, n)
}

type WhileStatement struct {
	StatementBase
	Expression *Expression
	Statement  *Statement
}

func (n *Node) AsWhileStatement() *WhileStatement { return n.data.(*WhileStatement) }
func (n *WhileStatement) DeeplyEquals(other *Node) bool {
	o := other.AsWhileStatement()
	return n.Expression.DeeplyEquals(o.Expression) && n.Statement.DeeplyEquals(o.Statement)
}
func (f *NodeFactory) NewWhileStatement(expression, statement *Node) *Node {
	n := f.whileStatementPool.New()
	n.Expression, n.Statement = expression, statement
	return f.newNode(KindWhileStatement, n)
}

// ConditionKind is WHILE, UNTIL, or NONE for an unconditional Do. ConditionAtEnd
// distinguishes a post-test loop without rewriting Until into a synthetic Not.
type DoStatement struct {
	StatementBase
	Expression     *Expression
	Statement      *Statement
	ConditionKind  Kind
	ConditionAtEnd bool
}

func (n *Node) AsDoStatement() *DoStatement { return n.data.(*DoStatement) }
func (n *DoStatement) DeeplyEquals(other *Node) bool {
	o := other.AsDoStatement()
	return n.ConditionKind == o.ConditionKind && n.ConditionAtEnd == o.ConditionAtEnd && n.Expression.DeeplyEquals(o.Expression) && n.Statement.DeeplyEquals(o.Statement)
}
func (f *NodeFactory) NewDoStatement(expression, statement *Node, conditionKind Kind, atEnd bool) *Node {
	n := f.doStatementPool.New()
	n.Expression, n.Statement, n.ConditionKind, n.ConditionAtEnd = expression, statement, conditionKind, atEnd
	return f.newNode(KindDoStatement, n)
}

type ForStatement struct {
	StatementBase
	Initializer  *Node // LetStatement containing the counter and initial value.
	Limit        *Expression
	Step         *Expression // nil preserves the omitted default step.
	Statement    *Statement
	NextVariable *Expression // Optional name on Next; not a new declaration.
}

func (n *Node) AsForStatement() *ForStatement { return n.data.(*ForStatement) }
func (n *ForStatement) DeeplyEquals(other *Node) bool {
	o := other.AsForStatement()
	return n.Initializer.DeeplyEquals(o.Initializer) && n.Limit.DeeplyEquals(o.Limit) && n.Step.DeeplyEquals(o.Step) && n.Statement.DeeplyEquals(o.Statement) && n.NextVariable.DeeplyEquals(o.NextVariable)
}
func (f *NodeFactory) NewForStatement(initializer, limit, step, statement, next *Node) *Node {
	n := f.forStatementPool.New()
	n.Initializer, n.Limit, n.Step, n.Statement, n.NextVariable = initializer, limit, step, statement, next
	return f.newNode(KindForStatement, n)
}

type ForEachStatement struct {
	StatementBase
	Initializer  *Expression // Existing control variable; not block-scoped.
	Expression   *Expression // Collection, evaluated on entry.
	Statement    *Statement
	NextVariable *Expression
}

func (n *Node) AsForEachStatement() *ForEachStatement { return n.data.(*ForEachStatement) }
func (n *ForEachStatement) DeeplyEquals(other *Node) bool {
	o := other.AsForEachStatement()
	return n.Initializer.DeeplyEquals(o.Initializer) && n.Expression.DeeplyEquals(o.Expression) && n.Statement.DeeplyEquals(o.Statement) && n.NextVariable.DeeplyEquals(o.NextVariable)
}
func (f *NodeFactory) NewForEachStatement(initializer, expression, statement, next *Node) *Node {
	n := f.forEachStatementPool.New()
	n.Initializer, n.Expression, n.Statement, n.NextVariable = initializer, expression, statement, next
	return f.newNode(KindForEachStatement, n)
}

// OnGotoStatement preserves computed dispatch separately from unconditional GoTo.
// Each destination is a GotoStatement whose Target is resolved by the binder.
type OnGotoStatement struct {
	StatementBase
	Expression   *Expression
	Destinations *NodeList
}

func (n *Node) AsOnGotoStatement() *OnGotoStatement { return n.data.(*OnGotoStatement) }
func (n *OnGotoStatement) DeeplyEquals(other *Node) bool {
	o := other.AsOnGotoStatement()
	return n.Expression.DeeplyEquals(o.Expression) && Equals(n.Destinations, o.Destinations)
}
func (f *NodeFactory) NewOnGotoStatement(expression *Node, destinations *NodeList) *Node {
	return f.newNode(KindOnGotoStatement, &OnGotoStatement{Expression: expression, Destinations: destinations})
}

type SelectStatement struct {
	StatementBase
	Expression *Expression
	CaseBlock  *Node
}

func (n *Node) AsSelectStatement() *SelectStatement { return n.data.(*SelectStatement) }
func (n *SelectStatement) DeeplyEquals(other *Node) bool {
	o := other.AsSelectStatement()
	return n.Expression.DeeplyEquals(o.Expression) && n.CaseBlock.DeeplyEquals(o.CaseBlock)
}
func (f *NodeFactory) NewSelectStatement(expression, block *Node) *Node {
	n := f.selectStatementPool.New()
	n.Expression, n.CaseBlock = expression, block
	return f.newNode(KindSelectStatement, n)
}

type CaseBlock struct {
	NodeBase
	Clauses *NodeList
}

func (n *Node) AsCaseBlock() *CaseBlock { return n.data.(*CaseBlock) }
func (n *CaseBlock) DeeplyEquals(other *Node) bool {
	return Equals(n.Clauses, other.AsCaseBlock().Clauses)
}
func (f *NodeFactory) NewCaseBlock(clauses *NodeList) *Node {
	n := f.caseBlockPool.New()
	n.Clauses = clauses
	return f.newNode(KindCaseBlock, n)
}

// DefaultClause shares the payload, with nil Expressions for Case Else.
type CaseClause struct {
	NodeBase
	Expressions *NodeList // Values, ranges, or relational clauses, in source order.
	Statement   *Statement
}

func (n *Node) AsCaseClause() *CaseClause { return n.data.(*CaseClause) }
func (n *CaseClause) DeeplyEquals(other *Node) bool {
	o := other.AsCaseClause()
	return Equals(n.Expressions, o.Expressions) && n.Statement.DeeplyEquals(o.Statement)
}
func (f *NodeFactory) NewCaseClause(expressions *NodeList, statement *Node) *Node {
	n := f.caseClausePool.New()
	n.Expressions, n.Statement = expressions, statement
	kind := KindCaseClause
	if expressions == nil {
		kind = KindDefaultClause
	}
	return f.newNode(kind, n)
}

type CaseRangeClause struct {
	NodeBase
	LowerBound, UpperBound *Expression
}

func (n *Node) AsCaseRangeClause() *CaseRangeClause { return n.data.(*CaseRangeClause) }
func (n *CaseRangeClause) DeeplyEquals(other *Node) bool {
	o := other.AsCaseRangeClause()
	return n.LowerBound.DeeplyEquals(o.LowerBound) && n.UpperBound.DeeplyEquals(o.UpperBound)
}
func (f *NodeFactory) NewCaseRangeClause(lower, upper *Node) *Node {
	n := f.caseRangeClausePool.New()
	n.LowerBound, n.UpperBound = lower, upper
	return f.newNode(KindCaseRangeClause, n)
}

type CaseComparisonClause struct {
	NodeBase
	IsKeyword     *TokenNode // Optional Is in Case Is >= expression.
	OperatorToken *TokenNode
	Expression    *Expression
}

func (n *Node) AsCaseComparisonClause() *CaseComparisonClause { return n.data.(*CaseComparisonClause) }
func (n *CaseComparisonClause) DeeplyEquals(other *Node) bool {
	o := other.AsCaseComparisonClause()
	return n.IsKeyword.DeeplyEquals(o.IsKeyword) && n.OperatorToken.DeeplyEquals(o.OperatorToken) && n.Expression.DeeplyEquals(o.Expression)
}
func (f *NodeFactory) NewCaseComparisonClause(is, operator, expression *Node) *Node {
	n := f.caseComparisonClausePool.New()
	n.IsKeyword, n.OperatorToken, n.Expression = is, operator, expression
	return f.newNode(KindCaseComparisonClause, n)
}

type ExitStatement struct {
	StatementBase
	BlockKind Kind  // DO, FOR, FUNCTION, SUB, or PROPERTY.
	Target    *Node // Binding metadata: enclosing loop or procedure.
}

func (n *Node) AsExitStatement() *ExitStatement { return n.data.(*ExitStatement) }
func (n *ExitStatement) DeeplyEquals(other *Node) bool {
	return n.BlockKind == other.AsExitStatement().BlockKind
}
func (f *NodeFactory) NewExitStatement(kind Kind) *Node {
	n := f.exitStatementPool.New()
	n.BlockKind = kind
	return f.newNode(KindExitStatement, n)
}

// Error policies remain syntax; the binder resolves only label targets.
type OnErrorStatement struct {
	StatementBase
	Mode   int
	Target *Node
}

func (n *Node) AsOnErrorStatement() *OnErrorStatement { return n.data.(*OnErrorStatement) }
func (n *OnErrorStatement) DeeplyEquals(o *Node) bool {
	x := o.AsOnErrorStatement()
	return n.Mode == x.Mode && n.Target.DeeplyEquals(x.Target)
}
func (f *NodeFactory) NewOnErrorStatement(mode int, target *Node) *Node {
	return f.newNode(KindOnErrorStatement, &OnErrorStatement{Mode: mode, Target: target})
}

type ResumeStatement struct {
	StatementBase
	Mode   int
	Target *Node
}

func (n *Node) AsResumeStatement() *ResumeStatement { return n.data.(*ResumeStatement) }
func (n *ResumeStatement) DeeplyEquals(o *Node) bool {
	x := o.AsResumeStatement()
	return n.Mode == x.Mode && n.Target.DeeplyEquals(x.Target)
}
func (f *NodeFactory) NewResumeStatement(mode int, target *Node) *Node {
	return f.newNode(KindResumeStatement, &ResumeStatement{Mode: mode, Target: target})
}

type ErrorStatement struct {
	StatementBase
	Expression *Node
}

func (n *Node) AsErrorStatement() *ErrorStatement { return n.data.(*ErrorStatement) }
func (n *ErrorStatement) DeeplyEquals(o *Node) bool {
	return n.Expression.DeeplyEquals(o.AsErrorStatement().Expression)
}
func (f *NodeFactory) NewErrorStatement(expression *Node) *Node {
	return f.newNode(KindErrorStatement, &ErrorStatement{Expression: expression})
}

// GoSub shares label syntax with GoTo, but has a resumption-list continuation.
func (n *Node) AsGosubStatement() *GotoStatement     { return n.AsGotoStatement() }
func (n *Node) AsOnGosubStatement() *OnGotoStatement { return n.AsOnGotoStatement() }
func (f *NodeFactory) NewGosubStatement(lineNumber, label *Node) *Node {
	node := f.NewGotoStatement(lineNumber, label)
	node.Kind = KindGosubStatement
	return node
}
func (f *NodeFactory) NewGosubReturnStatement() *Node { return f.NewToken(KindGosubReturnStatement) }
func (f *NodeFactory) NewOnGosubStatement(expression *Node, destinations *NodeList) *Node {
	node := f.NewOnGotoStatement(expression, destinations)
	node.Kind = KindOnGosubStatement
	return node
}
