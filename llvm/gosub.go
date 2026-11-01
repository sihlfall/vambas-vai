package llvm

import (
	"fmt"
	"strconv"

	"sihlfall.ch/vambas-vai/ir"
)

func (f *function) gosubInstruction(s *ir.Statement, in ir.Instruction) {
	if in.Op == ir.OpGosubPush {
		f.line("%s = call i32 @vba_gosub_push_ptr(ptr %%gosub.stack, ptr blockaddress(%s, %%gosub.cont%d))", valueName(in.ID), procedureName(f.procedure.ID), s.ID)
	} else {
		f.line("%s = call i32 @vba_gosub_pop_ptr(ptr %%call%d, ptr %%gosub.stack)", valueName(in.ID), in.ID)
	}
	f.pending(in)
}

// Every pushed address names a continuation trampoline in this function. The
// indirectbr destination list is complete, including forward/recursive GoSubs.
// GoSub preserves suspended caller scopes; Return cleans only scopes it leaves.
func (f *function) gosubDispatcher() {
	var calls []*ir.Statement
	var scopes []ir.ScopeID
	seen := map[ir.ScopeID]bool{}
	for _, s := range f.procedure.Statements {
		if s.End == ir.Gosub {
			calls = append(calls, s)
		}
		if s.End == ir.GosubReturn && !seen[s.Scope] {
			seen[s.Scope] = true
			scopes = append(scopes, s.Scope)
		}
	}
	f.current = f.newBlock("gosub_return")
	if len(calls) == 0 {
		f.current.end = "unreachable"
		return
	} // Pop on an empty stack always raises error 3.
	f.line("%%gosub.destination = load ptr, ptr %%gosub.target")
	labels := []string{}
	for _, call := range calls {
		labels = append(labels, fmt.Sprintf("gosub.cont%d", call.ID))
	}
	f.current.indirectBranch("%gosub.destination", labels)
	for _, call := range calls {
		block := f.newBlock(fmt.Sprintf("gosub.cont%d", call.ID))
		f.current = block
		f.line("%%gosub.scope%d = load i32, ptr %%gosub.return.scope", call.ID)
		cases := []switchCase{}
		for _, scope := range scopes {
			from := &ir.Statement{ID: call.ID, Scope: scope}
			target := f.edge(from, call.Next, fmt.Sprintf("gosub.scope%d", scope))
			cases = append(cases, switchCase{strconv.Itoa(int(scope)), target})
		}
		block.switchOn("i32", fmt.Sprintf("%%gosub.scope%d", call.ID), "gosub.invalid", cases)
	}
	f.current = f.newBlock("gosub.invalid")
	f.current.end = "unreachable"
}
