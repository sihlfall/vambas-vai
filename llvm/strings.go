package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

const nullString = "{ ptr null, ptr null, i32 -1, i16 0 }"

func zero(t ir.Type) string {
	if t.Scalar == ir.Object || t.Scalar == ir.Receiver || t.Scalar == ir.GosubAddress {
		return "null"
	}
	if ir.Floating(t) || t.Scalar == ir.Date {
		return "0.000000e+00"
	}
	if t.Scalar == ir.String {
		return nullString
	}
	if t.Scalar == ir.Variant || t.Scalar == ir.Enumerator || t.Scalar == ir.Record {
		return "zeroinitializer"
	}
	if ir.Array(t) {
		return "null"
	}
	if ir.Element(t) {
		return "zeroinitializer"
	}
	return "0"
}

func (m *module) stringLiteral(units []uint16) string {
	m.strings = true
	if len(units) == 0 {
		return "zeroinitializer"
	}
	if len(units) == 1 {
		return fmt.Sprintf("{ ptr null, ptr null, i32 1, i16 %d }", units[0])
	}
	name := fmt.Sprintf("@vba.literal.%d", len(m.literals))
	values := make([]string, 0, len(units)+1)
	for _, unit := range units {
		values = append(values, fmt.Sprintf("i16 %d", unit))
	}
	values = append(values, "i16 0")
	m.literals = append(m.literals, fmt.Sprintf("%s = private unnamed_addr constant [%d x i16] [%s], align 2", name, len(values), strings.Join(values, ", ")))
	return fmt.Sprintf("{ ptr %s, ptr null, i32 %d, i16 0 }", name, len(units))
}

func (f *function) cleanupValues(ids []ir.ValueID) {
	for i := len(ids) - 1; i >= 0; i-- {
		if f.arrayTokens[ids[i]].unlocked {
			continue
		}
		f.clear(f.memory[ids[i]], f.valueTypes[ids[i]])
	}
}

func (f *function) clear(pointer string, t ir.Type) {
	if t.Scalar == ir.Object {
		f.line("call void @vba_object_clear(ptr %s)", pointer)
		return
	}
	if t.Scalar == ir.Receiver {
		f.line("store ptr null, ptr %s, align 1", pointer)
		return
	}
	if t.Scalar == ir.Record {
		f.line("call void @vba_record_clear(ptr %s, ptr @vba.record.type%d)", pointer, t.Record)
		return
	}
	callee := "@vba.str.clear"
	if t.Scalar == ir.Variant {
		callee = "@vba_variant_clear"
	}
	if ir.Array(t) {
		callee = "@vba_array_clear"
	}
	if ir.Element(t) {
		callee = "@vba_element_clear"
		if f.scalarElementMemory[pointer] {
			f.module.scalarElementClear = true
			callee = "@vba.element.scalar.clear"
		}
	}
	if t.Scalar == ir.Enumerator {
		callee = "@vba_enum_clear"
	}
	f.line("call void %s(ptr %s)", callee, pointer)
}

func (f *function) cleanupSlots() {
	if ir.UsesGosub(f.procedure) {
		f.line("call void @vba_gosub_clear(ptr %%gosub.stack)")
	}
	// Enumerators borrow (or own) array storage and must release their locks
	// before any activation-owned array/Variant descriptor can be destroyed.
	for i := len(f.procedure.Slots) - 1; i >= 0; i-- {
		s := f.procedure.Slots[i]
		if s.Type.Scalar == ir.Enumerator || ir.Element(s.Type) {
			f.clear(f.slots[s.ID], s.Type)
		}
	}
	for i := len(f.procedure.Slots) - 1; i >= 0; i-- {
		s := f.procedure.Slots[i]
		if ir.Managed(s.Type) && !s.ByRef && s.Kind != ir.GlobalAlias && s.Type.Scalar != ir.Enumerator && !ir.Element(s.Type) {
			f.clear(f.slots[s.ID], s.Type)
		}
	}
}

func (f *function) cleanupTemporarySlots(region *ir.Statement) {
	for _, s := range f.procedure.Slots {
		if s.Kind == ir.Temporary && ir.Managed(s.Type) && s.Source.Range.ContainedBy(region.Source.Range) {
			f.clear(f.slots[s.ID], s.Type)
		}
	}
}
