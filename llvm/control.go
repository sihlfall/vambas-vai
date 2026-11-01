package llvm

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ir"
)

// Edge cleanup is local to the generated CFG. It never mutates binder flows.
func (f *function) edge(from *ir.Statement, target ir.StatementID, suffix string) string {
	to := f.procedure.Statements[target-1]
	ancestors := map[ir.ScopeID]bool{0: true}
	for id := to.Scope; id != 0; id = f.procedure.Scopes[id-1].Parent {
		ancestors[id] = true
	}
	var slots []ir.SlotID
	for id := from.Scope; !ancestors[id]; id = f.procedure.Scopes[id-1].Parent {
		cleanup := f.procedure.Scopes[id-1].Cleanup
		for i := len(cleanup) - 1; i >= 0; i-- {
			slots = append(slots, cleanup[i])
		}
	}
	if len(slots) == 0 {
		return fmt.Sprintf("s%d", target)
	}
	label := fmt.Sprintf("edge%d.%s", from.ID, suffix)
	f.current = f.newBlock(label)
	for _, id := range slots {
		f.clear(f.slots[id], f.procedure.Slots[id-1].Type)
	}
	f.current.jump(fmt.Sprintf("s%d", target))
	return label
}

func (f *function) controlInstruction(in ir.Instruction, args []operand) operand {
	name := valueName(in.ID)
	value := operand{text: name, typ: in.Type}
	switch in.Op {
	case ir.OpCondition:
		if ir.Floating(args[0].typ) {
			f.floatCondition(in, args[0])
		} else if args[0].typ.Scalar == ir.Variant {
			f.line("%s = call i32 @vba_condition(ptr %%call%d, ptr %s)", name, in.ID, f.memory[in.Args[0]])
			f.pending(in)
		} else {
			f.line("%%truth%d = icmp ne %s %s, 0", in.ID, scalar(args[0].typ), args[0].text)
			f.line("%s = sext i1 %%truth%d to i16", name, in.ID)
		}
	case ir.OpCheckInitialized:
		condition := fmt.Sprintf("%%uninitialized%d", in.ID)
		f.line("%s = icmp eq i16 %s, 0", condition, args[0].text)
		f.results[in.ID] = result{condition: condition, status: "92", source: in.Source, recordError: true}
	case ir.OpCheckDispatch:
		condition := fmt.Sprintf("%%invalid.dispatch%d", in.ID)
		f.line("%s = icmp ugt i16 %s, 255", condition, args[0].text)
		f.results[in.ID] = result{condition: condition, status: "5", source: in.Source, recordError: true}
	case ir.OpForValidate:
		f.line("%s = call i32 @vba_for_validate(ptr %s, ptr %s, ptr %s)", name, f.memory[in.Args[0]], f.memory[in.Args[1]], f.memory[in.Args[2]])
		f.pending(in)
	case ir.OpForTest:
		f.line("%s = call i32 @vba_for_test(ptr %%call%d, ptr %s, ptr %s, ptr %s)", name, in.ID, f.memory[in.Args[0]], f.memory[in.Args[1]], f.memory[in.Args[2]])
		f.pending(in)
	case ir.OpEnumStart:
		if args[0].typ.Scalar == ir.Object {
			f.line("%s = call i32 @vba_collection_enum_start(ptr %s, ptr %s)", name, f.slots[in.Slot], args[0].text)
			f.pending(in)
			break
		}
		f.line("%s = call i32 @vba_enum_start(ptr %s, ptr %s, i32 %d, i32 %s)", name, f.slots[in.Slot], args[0].text, arrayStorageTag(args[0].typ), args[1].text)
		f.pending(in)
	case ir.OpEnumObjectNext:
		f.line("%s = call i32 @vba_collection_enum_object_next(ptr %%call%d, ptr %s, ptr %s, i32 %s)", name, in.ID, f.slots[in.Slot], args[0].text, args[1].text)
		f.pending(in)
	case ir.OpEnumNext:
		f.line("%s = call i32 @vba_enum_next(ptr %%call%d, ptr %s, ptr %s)", name, in.ID, f.slots[in.Slot], args[0].text)
		f.pending(in)
	}
	return value
}
