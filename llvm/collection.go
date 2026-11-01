package llvm

import (
	"fmt"
	"sihlfall.ch/vambas-vai/ir"
)

func (f *function) collectionInstruction(in ir.Instruction, args []operand) operand {
	for i, arg := range args[1:] {
		kind := 0
		if arg.typ.Scalar == ir.Object {
			kind = 1
		}
		for j, entry := range []string{"ptr " + f.module.cstring(in.Names[i]), fmt.Sprintf("i32 %d", kind), "ptr " + f.memory[in.Args[i+1]]} {
			f.line("%%collection.arg%d.%d.%d = getelementptr %%vba.test.arg, ptr %%test.args%d, i32 %d, i32 %d", in.ID, i, j, in.ID, i, j)
			f.line("store %s, ptr %%collection.arg%d.%d.%d", entry, in.ID, i, j)
		}
	}
	output := "null"
	mode := 0
	if in.Type.Scalar != ir.Void {
		output = fmt.Sprintf("%%call%d", in.ID)
		mode = 12
		if in.Type.Scalar == ir.Object {
			mode = 9
		}
		if in.Type.Scalar == ir.Long {
			mode = 3
		}
	}
	f.line("%s = call i32 @vba_collection_invoke(ptr %s, ptr %s, i32 %d, i32 %d, ptr %%test.args%d, i32 %d)", valueName(in.ID), output, args[0].text, in.Integer, len(args)-1, in.ID, mode)
	f.pending(in)
	return operand{text: valueName(in.ID), typ: in.Type}
}
