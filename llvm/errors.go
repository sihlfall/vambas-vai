package llvm

import (
	"fmt"
	"strconv"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/ir"
)

func (f *function) errorInstruction(in ir.Instruction, args []operand) {
	name := valueName(in.ID)
	switch in.Op {
	case ir.OpErrClear:
		f.line("call void @vba_err_clear()")
	case ir.OpErrGet:
		f.line("%s = call i32 @vba_err_get(ptr %%call%d, i32 %s)", name, in.ID, args[0].text)
		f.pending(in)
	case ir.OpErrSet:
		f.line("%s = call i32 @vba_err_set(i32 %s, ptr %s)", name, args[0].text, f.memory[in.Args[1]])
		f.pending(in)
	case ir.OpErrRaise:
		var parameters []string
		for _, id := range in.Args {
			parameters = append(parameters, "ptr "+f.memory[id])
		}
		parameters = append(parameters, fmt.Sprintf("ptr @vba.err.source%d", f.procedure.ID))
		f.line("%s = call i32 @vba_err_raise(%s)", name, strings.Join(parameters, ", "))
		f.pending(in)
		r := f.results[in.ID]
		r.preserveErr = true
		f.results[in.ID] = r
	case ir.OpWithCheck:
		f.line("%%with.ptr%d = load ptr, ptr %s", in.ID, f.slots[in.Slot])
		condition := fmt.Sprintf("%%with.missing%d", in.ID)
		f.line("%s = icmp eq ptr %%with.ptr%d, null", condition, in.ID)
		f.results[in.ID] = result{condition: condition, status: "91", recordError: true, source: in.Source}
	}
}
func (f *function) errorPolicy(s *ir.Statement) {
	f.line("call void @vba_err_clear()")
	switch s.ErrorMode {
	case 1:
		f.line("store i32 0, ptr %%handler")
	case 2:
		f.line("store i32 %d, ptr %%handler", s.Handler)
	case 3:
		f.line("store i32 -1, ptr %%handler")
	case 4:
		f.line("store i1 false, ptr %%handler.active")
	}
}

// recordError writes the private error provenance record in one store. The
// procedure, statement, and offset are constants; only the status may vary.
func (f *function) recordError(name, status string, procedure ir.ProcedureID, statement ir.StatementID, offset int) {
	if _, err := strconv.Atoi(status); err == nil {
		f.line("store %%vba.error { i32 %s, i32 %d, i32 %d, i32 %d }, ptr %%error, align 1", status, procedure, statement, offset)
		return
	}
	f.line("%%%s = insertvalue %%vba.error { i32 0, i32 %d, i32 %d, i32 %d }, i32 %s, 0", name, procedure, statement, offset, status)
	f.line("store %%vba.error %%%s, ptr %%error, align 1", name)
}

// A failureTail is a shared block entered with a status from several
// failure sites; its status phi is added once every site is known.
type failureTail struct {
	block  *block
	inputs []string
}

func (t *failureTail) enter(f *function, status string) {
	t.inputs = append(t.inputs, fmt.Sprintf("[ %s, %%%s ]", status, f.current.label))
	f.current.jump(t.block.label)
}

// fail continues at region s's shared failure tail. A failure's provenance and
// Err capture stay at its site; the region's cleanup and handler routing do
// not depend on the failing instruction, so each region emits them once.
// Cleanup clears all of the region's values; unacquired ones are still zero.
func (f *function) fail(s *ir.Statement, status string) {
	if f.failureTails == nil {
		f.failureTails = make(map[ir.StatementID]*failureTail)
	}
	tail := f.failureTails[s.ID]
	if tail == nil {
		tail = &failureTail{block: f.newBlock(fmt.Sprintf("fail.s%d", s.ID))}
		f.failureTails[s.ID] = tail
		site := f.current
		f.current = tail.block
		if f.module.errors {
			f.routeFailure(s, "%"+tail.block.label+".status", tail.block.label)
		} else {
			f.cleanupValues(s.Cleanup)
			f.propagate("%" + tail.block.label + ".status")
		}
		f.current = site
	}
	tail.enter(f, status)
}

// propagate leaves the activation with status through its shared unwind block,
// which releases activation owners once for every propagating failure.
func (f *function) propagate(status string) {
	if f.unwind == nil {
		f.unwind = &failureTail{block: f.newBlock("unwind")}
		site := f.current
		f.current = f.unwind.block
		f.cleanupSlots()
		f.current.end = "ret i32 %unwind.status"
		f.current = site
	}
	f.unwind.enter(f, status)
}

func (f *function) finishFailureTails() {
	tails := []*failureTail{f.unwind}
	for _, tail := range f.failureTails {
		tails = append(tails, tail)
	}
	for _, tail := range tails {
		if tail != nil {
			phi := fmt.Sprintf("%%%s.status = phi i32 %s", tail.block.label, strings.Join(tail.inputs, ", "))
			tail.block.lines = append([]string{phi}, tail.block.lines...)
		}
	}
}

func (f *function) routeFailure(s *ir.Statement, status, label string) {
	f.cleanupValues(s.Cleanup)
	f.cleanupTemporarySlots(s)
	f.line("%%%s.handler = load i32, ptr %%handler", label)
	f.line("%%%s.active = load i1, ptr %%handler.active", label)
	f.line("%%%s.enabled = icmp ne i32 %%%s.handler, 0", label, label)
	f.line("%%%s.inactive = xor i1 %%%s.active, true", label, label)
	f.line("%%%s.handle = and i1 %%%s.enabled, %%%s.inactive", label, label, label)
	f.current.branch("%"+label+".handle", label+".dispatch", label+".propagate")
	f.current = f.newBlock(label + ".propagate")
	f.propagate(status)
	f.current = f.newBlock(label + ".dispatch")
	f.line("%%%s.inline = icmp eq i32 %%%s.handler, -1", label, label)
	f.current.branch("%"+label+".inline", label+".next", label+".goto")
	f.current = f.newBlock(label + ".next")
	next := s.Next
	if next == 0 {
		next = f.procedure.Statements[len(f.procedure.Statements)-1].ID
	}
	current := f.current
	destination := f.edge(s, next, label+".next")
	current.jump(destination)
	f.current = f.newBlock(label + ".goto")
	f.line("store i1 true, ptr %%handler.active")
	f.line("store i32 %d, ptr %%resume.retry", s.Retry)
	f.line("store i32 %d, ptr %%resume.next", next)
	var cases []switchCase
	seen := map[ir.StatementID]bool{}
	for _, region := range f.procedure.Statements {
		if region.ErrorMode == 2 && !seen[region.Handler] {
			seen[region.Handler] = true
			cases = append(cases, switchCase{strconv.Itoa(int(region.Handler)), fmt.Sprintf("s%d", region.Handler)})
		}
	}
	f.current.switchOn("i32", "%"+label+".handler", label+".propagate", cases)
}
func (f *function) resumeStatement(s *ir.Statement) {
	label := fmt.Sprintf("resume%d", s.ID)
	f.line("%%%s.active = load i1, ptr %%handler.active", label)
	f.current.branch("%"+label+".active", label+".valid", label+".invalid")
	f.current = f.newBlock(label + ".invalid")
	f.line("call void @vba_err_capture(i32 20, ptr @vba.err.source%d)", f.procedure.ID)
	f.recordError(label+".err", "20", f.procedure.ID, s.ID, s.Source.Range.Pos())
	f.fail(s, "20")
	f.current = f.newBlock(label + ".valid")
	f.line("store i1 false, ptr %%handler.active")
	f.line("call void @vba_err_clear()")
	selector := fmt.Sprintf("%%%s.target", label)
	if s.ResumeMode == 2 {
		f.line("%s = add i32 0, %d", selector, s.Target)
	} else {
		slot := "%resume.retry"
		if s.ResumeMode == 1 {
			slot = "%resume.next"
		}
		f.line("%s = load i32, ptr %s", selector, slot)
	}
	dispatch := f.current
	var cases []switchCase
	for _, target := range f.procedure.Statements {
		name := fmt.Sprintf("%s.to%d", label, target.ID)
		f.current = f.newBlock(name)
		ancestors := map[ir.ScopeID]bool{0: true}
		for scope := target.Scope; scope != 0; scope = f.procedure.Scopes[scope-1].Parent {
			ancestors[scope] = true
		}
		for i := len(f.procedure.Scopes) - 1; i >= 0; i-- {
			scope := f.procedure.Scopes[i]
			if !ancestors[scope.ID] {
				for j := len(scope.Cleanup) - 1; j >= 0; j-- {
					id := scope.Cleanup[j]
					f.clear(f.slots[id], f.procedure.Slots[id-1].Type)
				}
			}
		}
		f.current.jump(fmt.Sprintf("s%d", target.ID))
		cases = append(cases, switchCase{strconv.Itoa(int(target.ID)), name})
	}
	dispatch.switchOn("i32", selector, fmt.Sprintf("s%d", f.procedure.Statements[len(f.procedure.Statements)-1].ID), cases)
}
func (f *function) errorReturn(s *ir.Statement) {
	if s.Source.Node != nil && s.Source.Node.Kind == ast.KindExitStatement {
		f.line("call void @vba_err_clear()")
		return
	}
	label := fmt.Sprintf("return.err%d", s.ID)
	f.line("%%%s.active = load i1, ptr %%handler.active", label)
	f.current.branch("%"+label+".active", label+".clear", label+".done")
	f.current = f.newBlock(label + ".clear")
	f.line("call void @vba_err_clear()")
	f.current.jump(label + ".done")
	f.current = f.newBlock(label + ".done")
}
