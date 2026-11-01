package binder

import "sihlfall.ch/vambas-vai/ast"

type exitTarget struct {
	kind ast.Kind
	node *ast.Node
	flow *ast.FlowNode
}

func (b *Binder) bindWithStatement(node *ast.Node) {
	statement := node.AsWithStatement()
	// A nested header may itself use the outer With's receiver. The body uses
	// the captured result; accesses link back to this node instead of rebinding
	// the header expression or introducing a synthetic local variable.
	b.bind(statement.Expression)
	save := b.withTarget
	b.withTarget = node
	b.bind(statement.Statement)
	b.withTarget = save
}

func (b *Binder) bindLoopBody(body, loop *ast.Node, kind ast.Kind, exit *ast.FlowNode) {
	b.exitTargets = append(b.exitTargets, exitTarget{kind, loop, exit})
	b.bind(body)
	b.exitTargets = b.exitTargets[:len(b.exitTargets)-1]
}

func (b *Binder) loopLabel(node *ast.Node) *ast.FlowNode {
	label := b.newFlowNode(ast.FlowFlagsLoopLabel)
	label.Node = node
	b.addAntecedent(label, b.currentFlow)
	return label // Keep even disconnected labels: GoTo can enter the body later.
}

func (b *Binder) bindWhileStatement(node *ast.Node) {
	statement := node.AsWhileStatement()
	header := b.loopLabel(node)
	exit := b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.currentFlow = header
	b.bind(statement.Expression)
	bodyFlow := b.createFlowCondition(ast.FlowFlagsTrueCondition, statement.Expression)
	b.addAntecedent(exit, b.createFlowCondition(ast.FlowFlagsFalseCondition, statement.Expression))
	b.currentFlow = bodyFlow
	// VBA has no Exit While, and a While does not intercept Exit Do/For.
	b.bind(statement.Statement)
	b.addAntecedent(header, b.currentFlow)
	b.currentFlow = b.finishFlowLabel(exit)
}

func (b *Binder) bindDoStatement(node *ast.Node) {
	statement := node.AsDoStatement()
	header := b.loopLabel(node)
	exit := b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.currentFlow = header
	bodyFlag, exitFlag := ast.FlowFlagsTrueCondition, ast.FlowFlagsFalseCondition
	if statement.ConditionKind == ast.KindUNTIL {
		bodyFlag, exitFlag = exitFlag, bodyFlag
	}
	if statement.Expression != nil && !statement.ConditionAtEnd {
		b.bind(statement.Expression)
		bodyFlow := b.createFlowCondition(bodyFlag, statement.Expression)
		b.addAntecedent(exit, b.createFlowCondition(exitFlag, statement.Expression))
		b.currentFlow = bodyFlow
	}
	b.bindLoopBody(statement.Statement, node, ast.KindDO, exit)
	if statement.Expression != nil && statement.ConditionAtEnd {
		b.bind(statement.Expression)
		b.addAntecedent(header, b.createFlowCondition(bodyFlag, statement.Expression))
		b.addAntecedent(exit, b.createFlowCondition(exitFlag, statement.Expression))
	} else {
		b.addAntecedent(header, b.currentFlow)
	}
	b.currentFlow = b.finishFlowLabel(exit)
}

func (b *Binder) bindNextVariable(next, variable *ast.Node) {
	if next == nil {
		return
	}
	next.FlowNode = b.currentFlow
	if next.Kind == ast.KindIdentifier && variable.Kind == ast.KindIdentifier {
		if ast.SymbolNameKey(next.AsIdentifier().Text) == ast.SymbolNameKey(variable.AsIdentifier().Text) {
			next.Symbol = variable.Symbol
			return
		}
	} else if next.DeeplyEquals(variable) {
		return
	}
	b.report(ast.BindingInvalidLoopVariable, next, variable, "Next variable does not match its For control variable")
}

func (b *Binder) bindForStatement(node *ast.Node) {
	var variable, body, next *ast.Node
	each := node.Kind == ast.KindForEachStatement
	if each {
		statement := node.AsForEachStatement()
		variable, body, next = statement.Initializer, statement.Statement, statement.NextVariable
		b.bind(statement.Expression)
	} else {
		statement := node.AsForStatement()
		variable, body, next = statement.Initializer.AsLetStatement().LExpression, statement.Statement, statement.NextVariable
		b.bind(statement.Initializer)
		// VBA captures limit and step once, unlike TypeScript's for condition
		// and incrementor expressions which are evaluated on every iteration.
		b.bind(statement.Limit)
		b.bind(statement.Step)
	}
	header := b.loopLabel(node)
	exit := b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.currentFlow = header
	b.addAntecedent(exit, b.newFlowNodeEx(ast.FlowFlagsFalseCondition, node, header))
	b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsTrueCondition, node, header)
	if each {
		b.bind(variable)
		b.bindAssignmentTargetFlow(variable)
	}
	b.bindLoopBody(body, node, ast.KindFOR, exit)
	b.bindNextVariable(next, variable)
	if !each && b.currentFlow != b.unreachableFlow {
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsAssignment, variable, b.currentFlow)
	}
	b.addAntecedent(header, b.currentFlow)
	b.currentFlow = b.finishFlowLabel(exit)
}

func (b *Binder) bindExitStatement(node *ast.Node) {
	statement := node.AsExitStatement()
	for i := len(b.exitTargets) - 1; i >= 0; i-- {
		target := b.exitTargets[i]
		if target.kind == statement.BlockKind {
			statement.Target = target.node
			b.addAntecedent(target.flow, b.currentFlow)
			b.currentFlow = b.unreachableFlow
			return
		}
	}
	if statement.BlockKind == ast.KindFUNCTION && b.container.Kind == ast.KindFunctionDeclaration ||
		statement.BlockKind == ast.KindSUB && b.container.Kind == ast.KindSubDeclaration ||
		statement.BlockKind == ast.KindPROPERTY && ast.IsPropertyAccessor(b.container) {
		statement.Target = b.container
		b.addAntecedent(b.returnTarget, b.currentFlow)
		b.currentFlow = b.unreachableFlow
		return
	}
	if statement.BlockKind != ast.KindNONE {
		b.report(ast.BindingInvalidExit, node, nil, "Exit does not have a matching enclosing loop or procedure")
	}
}

func (b *Binder) bindSelectStatement(node *ast.Node) {
	statement := node.AsSelectStatement()
	b.bind(statement.Expression)
	statement.CaseBlock.FlowNode = b.currentFlow
	noMatch := b.currentFlow
	exit := b.newFlowNode(ast.FlowFlagsBranchLabel)
	for _, clauseNode := range statement.CaseBlock.AsCaseBlock().Clauses.Nodes {
		clause := clauseNode.AsCaseClause()
		clauseNode.FlowNode = noMatch
		b.currentFlow = noMatch
		if clauseNode.Kind == ast.KindDefaultClause {
			noMatch = b.unreachableFlow
		} else {
			matched := b.newFlowNode(ast.FlowFlagsBranchLabel)
			for _, expression := range clause.Expressions.Nodes {
				b.bind(expression)
				if b.currentFlow != b.unreachableFlow {
					b.addAntecedent(matched, b.newFlowNodeEx(ast.FlowFlagsSwitchClause|ast.FlowFlagsTrueCondition, expression, b.currentFlow))
					b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsSwitchClause|ast.FlowFlagsFalseCondition, expression, b.currentFlow)
				}
			}
			noMatch = b.currentFlow
			b.currentFlow = b.finishFlowLabel(matched)
		}
		b.bind(clause.Statement)
		b.addAntecedent(exit, b.currentFlow) // VBA cases never fall through.
	}
	b.addAntecedent(exit, noMatch)
	b.currentFlow = b.finishFlowLabel(exit)
}
