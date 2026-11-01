package binder

import "sihlfall.ch/vambas-vai/ast"

func (b *Binder) gosubContinuation(flow *ast.FlowNode) {
	// Stack-sensitive return matching belongs to execution. Analysis conservatively
	// merges return effects with the call/failure continuation, like call flow.
	continuation := b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.addAntecedent(continuation, flow)
	b.gosubContinuations = append(b.gosubContinuations, continuation)
	b.currentFlow = continuation
}
func (b *Binder) bindGosubStatement(node *ast.Node) {
	flow := b.currentFlow
	b.bindGotoStatement(node)
	if b.container == nil || !ast.IsProcedureDeclaration(b.container) {
		return
	}
	b.gosubContinuation(flow)
}
