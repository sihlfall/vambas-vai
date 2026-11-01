package llvm

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ir"
)

func (f *function) integerDivision(in ir.Instruction, args []operand) {
	// Widening permits MIN/-1 for remainder and avoids signed overflow in LLVM.
	// Select a safe divisor before sdiv/srem: even a failing path cannot produce
	// poison, and Try preserves the source error before any later side effect.
	id := in.ID
	t := scalar(in.Type)
	extend := "sext"
	if in.Type.Scalar == ir.Byte {
		extend = "zext"
	}
	f.line("%%div.left%d = %s %s %s to i64", id, extend, t, args[0].text)
	f.line("%%div.right%d = %s %s %s to i64", id, extend, t, args[1].text)
	f.line("%%div.zero%d = icmp eq i64 %%div.right%d, 0", id, id)
	f.line("%%div.safe%d = select i1 %%div.zero%d, i64 1, i64 %%div.right%d", id, id, id)
	op := "sdiv"
	if in.Op == ir.OpModulo {
		op = "srem"
	}
	f.line("%%div.wide%d = %s i64 %%div.left%d, %%div.safe%d", id, op, id, id)
	f.line("%%div.value%d = trunc i64 %%div.wide%d to %s", id, id, t)
	f.line("%%div.check%d = %s %s %%div.value%d to i64", id, extend, t, id)
	f.line("%%div.overflow%d = icmp ne i64 %%div.check%d, %%div.wide%d", id, id, id)
	condition := fmt.Sprintf("%%div.failed%d", id)
	status := fmt.Sprintf("%%div.status%d", id)
	f.line("%s = or i1 %%div.zero%d, %%div.overflow%d", condition, id, id)
	f.line("%s = select i1 %%div.zero%d, i32 11, i32 6", status, id)
	f.results[id] = result{condition: condition, status: status, value: fmt.Sprintf("%%div.value%d", id), direct: true, source: in.Source, recordError: true}
}
