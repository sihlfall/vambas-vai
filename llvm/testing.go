package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

func (m *module) cstring(text string) string {
	prefix := m.cstringPrefix
	if prefix == "" {
		prefix = "@vba.cstr."
	}
	name := fmt.Sprintf("%s%d", prefix, len(m.literals))
	var b strings.Builder
	for _, v := range []byte(text) {
		fmt.Fprintf(&b, "\\%02X", v)
	}
	m.literals = append(m.literals, fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\\00\"", name, len(text)+1, b.String()))
	return name
}
func (f *function) testInstruction(s *ir.Statement, in ir.Instruction, args []operand) operand {
	name := valueName(in.ID)
	value := operand{text: name, typ: in.Type}
	switch in.Op {
	case ir.OpObjectNull:
		value.text = "null"
	case ir.OpObjectEqual:
		f.line("%%identity%d = icmp eq ptr %s, %s", in.ID, args[0].text, args[1].text)
		f.line("%s = sext i1 %%identity%d to i16", name, in.ID)
	case ir.OpObjectCheck:
		f.line("%s = call i32 @vba_object_check(ptr %s, i32 %s)", name, args[0].text, args[1].text)
		f.pending(in)
	case ir.OpObjectNew:
		f.line("%s = call i32 @vba_object_new(ptr %%call%d, i32 %s)", name, in.ID, args[0].text)
		f.pending(in)
	case ir.OpCreateObject:
		f.line("%s = call i32 @vba_create_object(ptr %%call%d, ptr %s)", name, in.ID, f.memory[in.Args[0]])
		f.pending(in)
	case ir.OpTestAssert:
		for i, a := range args[1:] {
			kind := 0
			if a.typ.Scalar == ir.Object {
				kind = 1
			}
			pointer := f.memory[in.Args[i+1]]
			for j, field := range []string{fmt.Sprintf("ptr %s", f.module.cstring(in.Names[i])), fmt.Sprintf("i32 %d", kind), "ptr " + pointer} {
				f.line("%%test.field%d.%d.%d = getelementptr %%vba.test.arg, ptr %%test.args%d, i32 %d, i32 %d", in.ID, i, j, in.ID, i, j)
				f.line("store %s, ptr %%test.field%d.%d.%d", field, in.ID, i, j)
			}
		}
		f.line("%s = call i32 @vba_test_assert(ptr %s, i32 %d, i32 %d, ptr %%test.args%d, i32 %d, i32 %d, i32 %d)", name, args[0].text, in.Integer, len(args)-1, in.ID, f.procedure.ID, s.ID, in.Source.Range.Pos())
		f.pending(in)
	}
	return value
}
