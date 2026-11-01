package ast

// SkipParentheses follows typescript-go's helper for semantic tree consumers.
func SkipParentheses(node *Node) *Node {
	for node != nil && node.Kind == KindParenthesizedExpression {
		node = node.AsParenthesizedExpression().Inner
	}
	return node
}

// ForEachChild visits syntax children in source order and stops when visit returns
// true. Parent links, symbols, and flow graphs are not syntax children.
func (node *Node) ForEachChild(visit func(*Node) bool) bool {
	if node == nil {
		return false
	}
	child := func(n *Node) bool { return n != nil && visit(n) }
	list := func(nodes *NodeList) bool {
		if nodes != nil {
			for _, n := range nodes.Nodes {
				if child(n) {
					return true
				}
			}
		}
		return false
	}
	switch node.Kind {
	case KindImplementsStatement:
		return child(node.AsImplementsStatement().Interface)
	case KindSourceFile:
		n := node.AsSourceFile()
		return list(n.Statements) || child(n.EndOfFileToken)
	case KindAttributeStatement:
		n := node.AsAttributeStatement()
		return child(n.Name) || list(n.Values)
	case KindFunctionDeclaration, KindSubDeclaration, KindGetAccessor, KindLetAccessor, KindSetAccessor:
		n := node.AsFunctionDeclaration()
		return child(n.Name()) || list(n.Parameters) || child(n.Type) || child(n.Body)
	case KindVariableStatement:
		return child(node.AsVariableStatement().DeclarationList)
	case KindTypeDeclaration, KindEnumDeclaration:
		n := node.AsTypeDeclaration()
		return child(n.Name) || list(n.Members)
	case KindEnumMember:
		n := node.AsEnumMember()
		return child(n.Name) || child(n.Initializer)
	case KindReDimStatement:
		n := node.AsReDimStatement()
		return child(n.PreserveKeyword) || list(n.Declarations)
	case KindEraseStatement:
		return list(node.AsEraseStatement().Expressions)
	case KindReDimDeclaration:
		n := node.AsReDimDeclaration()
		return child(n.Expression) || list(n.Dimensions) || child(n.Type)
	case KindVariableDeclarationList:
		return list(node.AsVariableDeclarationList().Declarations)
	case KindVariableDeclaration, KindFieldDeclaration:
		n := node.AsVariableDeclaration()
		return child(n.Name) || list(n.Dimensions) || child(n.Type) || child(n.Initializer)
	case KindParameter:
		n := node.AsParameterDeclaration()
		return child(n.Name) || list(n.Dimensions) || child(n.Type) || child(n.Initializer)
	case KindTypeReference:
		n := node.AsTypeReference()
		return child(n.NewKeyword) || child(n.TypeName) || child(n.Length)
	case KindQualifiedName:
		n := node.AsQualifiedName()
		return child(n.Left) || child(n.Right)
	case KindArrayDimension:
		n := node.AsArrayDimension()
		return child(n.LowerBound) || child(n.UpperBound)
	case KindBlock:
		return list(node.AsBlock().Statements)
	case KindIfStatement:
		n := node.AsIfStatement()
		return child(n.Expression) || child(n.ThenStatement) || child(n.ElseStatement)
	case KindLetStatement, KindSetStatement:
		n := node.AsLetStatement()
		return child(n.LExpression) || child(n.RExpression)
	case KindMidAssignmentStatement:
		n := node.AsMidAssignmentStatement()
		return child(n.Mode) || child(n.Target) || child(n.Start) || child(n.Length) || child(n.Expression)
	case KindGotoStatement, KindGosubStatement:
		n := node.AsGotoStatement()
		return child(n.LineNumber) || child(n.Label)
	case KindOnErrorStatement:
		return child(node.AsOnErrorStatement().Target)
	case KindResumeStatement:
		return child(node.AsResumeStatement().Target)
	case KindErrorStatement:
		return child(node.AsErrorStatement().Expression)
	case KindOnGotoStatement, KindOnGosubStatement:
		n := node.AsOnGotoStatement()
		return child(n.Expression) || list(n.Destinations)
	case KindLabeledStatement:
		n := node.AsLabeledStatement()
		return child(n.LineNumber) || child(n.Label) || child(n.Statement)
	case KindBinaryExpression:
		n := node.AsBinaryExpression()
		return child(n.Left) || child(n.OperatorToken) || child(n.Right)
	case KindPrefixUnaryExpression:
		return child(node.AsPrefixUnaryExpression().Operand)
	case KindParenthesizedExpression:
		return child(node.AsParenthesizedExpression().Inner)
	case KindPropertyAccessExpression:
		n := node.AsPropertyAccessExpression()
		return child(n.Expression) || child(n.OperatorToken) || child(n.Name)
	case KindCallExpression:
		n := node.AsCallExpression()
		return child(n.Expression) || list(n.Arguments)
	case KindCallStatement:
		n := node.AsCallStatement()
		return child(n.CallKeyword) || child(n.Expression)
	case KindNamedArgument:
		n := node.AsNamedArgument()
		return child(n.Name) || child(n.Expression)
	case KindWhileStatement:
		n := node.AsWhileStatement()
		return child(n.Expression) || child(n.Statement)
	case KindDoStatement:
		n := node.AsDoStatement()
		if n.ConditionAtEnd {
			return child(n.Statement) || child(n.Expression)
		}
		return child(n.Expression) || child(n.Statement)
	case KindForStatement:
		n := node.AsForStatement()
		return child(n.Initializer) || child(n.Limit) || child(n.Step) || child(n.Statement) || child(n.NextVariable)
	case KindForEachStatement:
		n := node.AsForEachStatement()
		return child(n.Initializer) || child(n.Expression) || child(n.Statement) || child(n.NextVariable)
	case KindSelectStatement:
		n := node.AsSelectStatement()
		return child(n.Expression) || child(n.CaseBlock)
	case KindCaseBlock:
		return list(node.AsCaseBlock().Clauses)
	case KindCaseClause, KindDefaultClause:
		n := node.AsCaseClause()
		return list(n.Expressions) || child(n.Statement)
	case KindCaseRangeClause:
		n := node.AsCaseRangeClause()
		return child(n.LowerBound) || child(n.UpperBound)
	case KindCaseComparisonClause:
		n := node.AsCaseComparisonClause()
		return child(n.IsKeyword) || child(n.OperatorToken) || child(n.Expression)
	case KindNewExpression:
		return child(node.AsNewExpression().Expression)
	case KindWithStatement:
		n := node.AsWithStatement()
		return child(n.Expression) || child(n.Statement)
	}
	return false
}
