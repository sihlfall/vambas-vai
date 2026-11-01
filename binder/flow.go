package binder

import "sihlfall.ch/vambas-vai/ast"

func (b *Binder) newFlowNode(flags ast.FlowFlags) *ast.FlowNode {
	node := b.flowNodePool.New()
	node.Flags = flags
	b.flowNodes = append(b.flowNodes, node)
	return node
}

func setFlowNodeReferenced(node *ast.FlowNode) {
	if node.Flags&ast.FlowFlagsReferenced == 0 {
		node.Flags |= ast.FlowFlagsReferenced
	} else {
		node.Flags |= ast.FlowFlagsShared
	}
}

func (b *Binder) newFlowNodeEx(flags ast.FlowFlags, node *ast.Node, antecedent *ast.FlowNode) *ast.FlowNode {
	flow := b.newFlowNode(flags)
	flow.Node, flow.Antecedent = node, antecedent
	setFlowNodeReferenced(antecedent)
	return flow
}

func (b *Binder) addAntecedent(label *ast.FlowLabel, antecedent *ast.FlowNode) {
	if antecedent.Flags&ast.FlowFlagsUnreachable != 0 {
		return
	}
	var last *ast.FlowList
	for list := label.Antecedents; list != nil; list = list.Next {
		if list.Flow == antecedent {
			return
		}
		last = list
	}
	entry := b.flowListPool.New()
	entry.Flow = antecedent
	if last == nil {
		label.Antecedents = entry
	} else {
		last.Next = entry
	}
	setFlowNodeReferenced(antecedent)
}

func (b *Binder) finishFlowLabel(label *ast.FlowLabel) *ast.FlowNode {
	if label.Antecedents == nil {
		return b.unreachableFlow
	}
	if label.Antecedents.Next == nil {
		return label.Antecedents.Flow
	}
	return label
}

func constantBoolean(node *ast.Node) (bool, bool) {
	if node == nil {
		return false, false
	}
	switch node.Kind {
	case ast.KindTRUE:
		return true, true
	case ast.KindFALSE:
		return false, true
	case ast.KindParenthesizedExpression:
		return constantBoolean(node.AsParenthesizedExpression().Inner)
	case ast.KindPrefixUnaryExpression:
		expr := node.AsPrefixUnaryExpression()
		if expr.Operator == ast.KindNOT {
			if value, known := constantBoolean(expr.Operand); known {
				return !value, true
			}
		}
	}
	return false, false
}

func (b *Binder) createFlowCondition(flags ast.FlowFlags, expression *ast.Node) *ast.FlowNode {
	if b.currentFlow == b.unreachableFlow {
		return b.unreachableFlow
	}
	if value, known := constantBoolean(expression); known {
		if value == (flags == ast.FlowFlagsTrueCondition) {
			return b.currentFlow
		}
		return b.unreachableFlow
	}
	return b.newFlowNodeEx(flags, expression, b.currentFlow)
}

func (b *Binder) bindIfStatement(node *ast.Node) {
	stmt := node.AsIfStatement()
	b.bind(stmt.Expression)
	thenFlow := b.createFlowCondition(ast.FlowFlagsTrueCondition, stmt.Expression)
	elseFlow := b.createFlowCondition(ast.FlowFlagsFalseCondition, stmt.Expression)
	postIf := b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.currentFlow = thenFlow
	b.bind(stmt.ThenStatement)
	b.addAntecedent(postIf, b.currentFlow)
	b.currentFlow = elseFlow
	b.bind(stmt.ElseStatement)
	b.addAntecedent(postIf, b.currentFlow)
	b.currentFlow = b.finishFlowLabel(postIf)
}

// VBA GoTo permits arbitrary forward and backward edges. Compute reachability
// after the entire procedure graph exists, so a disconnected cycle is not treated
// as live and a later jump can make an earlier label reachable.
func (b *Binder) finishFunctionFlow(function *ast.Node, start *ast.FlowNode) {
	successors := make(map[*ast.FlowNode][]*ast.FlowNode)
	for _, flow := range b.flowNodes {
		if flow.Antecedent != nil {
			successors[flow.Antecedent] = append(successors[flow.Antecedent], flow)
		}
		for list := flow.Antecedents; list != nil; list = list.Next {
			successors[list.Flow] = append(successors[list.Flow], flow)
		}
	}
	reachable := map[*ast.FlowNode]bool{start: true}
	queue := []*ast.FlowNode{start}
	for i := 0; i < len(queue); i++ {
		for _, next := range successors[queue[i]] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}
	for _, flow := range b.flowNodes {
		if !reachable[flow] {
			flow.Flags |= ast.FlowFlagsUnreachable
		}
	}
	var mark func(*ast.Node) bool
	mark = func(node *ast.Node) bool {
		if node.FlowNode != nil && node.FlowNode.Flags&ast.FlowFlagsUnreachable != 0 {
			node.Flags |= ast.NodeFlagsUnreachable
		}
		node.ForEachChild(mark)
		return false
	}
	if body := function.AsFunctionDeclaration().Body; body != nil {
		mark(body)
	}
}
