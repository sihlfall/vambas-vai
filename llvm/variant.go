package llvm

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ir"
)

const variantDeclarations = `
; Automation VARIANT layout for 64-bit targets. Link runtime/sources.txt C sources (OleAut32 on Windows).
%vba.gosub.stack = type { ptr, i64, i64 }
declare i32 @vba_gosub_push_ptr(ptr, ptr)
declare i32 @vba_gosub_pop_ptr(ptr, ptr)
declare void @vba_gosub_clear(ptr)
%vba.paramarg = type { ptr, i32 }
declare i32 @vba_paramarray_pack(ptr, i32, ptr)
declare i32 @vba_collection_invoke(ptr, ptr, i32, i32, ptr, i32)
declare i32 @vba_collection_enum_start(ptr, ptr)
declare i32 @vba_collection_enum_object_next(ptr, ptr, ptr, i32)
%vba.test.arg = type { ptr, i32, ptr }
%vba.invoke.arg = type {ptr, %vba.variant}
%vba.invoke.parameter = type {ptr,i32,i32,ptr,i32,ptr}
%vba.invoke.member = type {ptr,i32,i32,i32,ptr,ptr}
%vba.binding = type {%vba.variant,[3 x i64],ptr,ptr,i32,i32}
declare void @vba_invoke_argument(ptr,ptr,ptr,i32)
declare i32 @vba_object_invoke(ptr,ptr,ptr,ptr,i32,i32,ptr)
declare i32 @vba_invocation_capture(ptr,ptr)
declare i32 @vba_object_value_at(ptr,ptr,ptr)
declare i32 @vba_variant_to_at(ptr,ptr,i32,ptr)
declare ptr @vba_invocation_error_push(ptr)
declare void @vba_invocation_error_pop(ptr)
declare i32 @vba_capture_index(ptr,ptr,i32,ptr)
declare i32 @vba_invoke_members(ptr,ptr,ptr,ptr,i32,i32,ptr,i32,ptr)
declare i32 @vba_native_box(ptr,ptr,i32)
declare void @vba_native_clear(ptr,i32)
declare i32 @vba_field_invoke(ptr,ptr,ptr,i32,ptr,i32,i32,ptr,ptr,ptr,i32)
declare i32 @vba_object_check(ptr, i32)
declare i32 @vba_object_new(ptr, i32)
declare i32 @vba_class_new(ptr, ptr)
declare i32 @vba_class_construct(ptr, ptr, ptr)
declare i32 @vba_class_load(ptr, ptr, ptr, ptr)
declare i32 @vba_class_check(ptr, ptr, i32)
declare i32 @vba_class_check_exact(ptr, ptr)
declare i32 @vba_interface_resolve(ptr, ptr, ptr, i32, ptr)
declare void @vba_object_retain(ptr)
declare void @vba_object_clear(ptr)
declare i32 @vba_object_store(ptr, ptr)
declare i32 @vba_object_load(ptr, ptr, i32)
declare i32 @vba_create_object(ptr, ptr)
declare i32 @vba_test_assert(ptr, i32, i32, ptr, i32, i32, i32)
%vba.variant = type { i16, i16, i16, i16, i64, i64 }
%vba.element = type { ptr, ptr, %vba.variant, %vba.string }
%vba.enumerator = type { ptr, i64, i32, i32 }
declare void @vba_err_capture(i32, ptr)
declare void @vba_err_raise_failure(i32, ptr)
declare void @vba_err_clear()
declare i32 @vba_err_raise(ptr, ptr, ptr, ptr, ptr, ptr)
declare i32 @vba_err_get(ptr, i32)
declare i32 @vba_err_set(i32, ptr)
declare i32 @vba_variant_load(ptr, ptr)
declare i32 @vba_variant_store(ptr, ptr)
declare void @vba_variant_clear(ptr)
declare i32 @vba_variant_box_number(ptr, i32, i64)
declare i32 @vba_variant_box_float(ptr, i32, double)
declare i32 @vba_variant_box_string(ptr, ptr)
declare i32 @vba_variant_box_object(ptr, ptr)
declare i32 @vba_variant_object(ptr, ptr)
declare i32 @vba_variant_to(ptr, ptr, i32)
declare i32 @vba_variant_binary(ptr, ptr, ptr, i32)
declare i32 @vba_variant_unary(ptr, ptr, i32)
declare i32 @vba_variant_query(ptr, i32)
declare i32 @vba_variant_len(ptr, ptr, i32)
declare i32 @vba_string_load(ptr, ptr)
declare i32 @vba_string_store(ptr, ptr)
declare i32 @vba_string_pin(ptr)
declare void @vba_bstr_free(ptr)
declare ptr @vba_string_ref(ptr)
declare i32 @vba_array_copy(ptr, ptr)
declare i32 @vba_array_store(ptr, ptr)
declare void @vba_array_clear(ptr)
declare i32 @vba_array_box(ptr, ptr, i32)
declare i32 @vba_object_array_box(ptr, ptr, ptr)
declare i32 @vba_object_array_unbox(ptr, ptr, ptr)
declare i32 @vba_object_array_redim(ptr, i32, ptr, i32, i32, ptr)
declare i32 @vba_array_unbox(ptr, ptr, i32)
declare i32 @vba_array_redim(ptr, i32, i32, i32, i32, ptr)
declare i32 @vba_array_erase(ptr, i32)
declare i32 @vba_array_bound(ptr, ptr, i32, i32, i32)
declare i32 @vba_array_index(ptr, ptr, i32, i32, i32, ptr)
declare ptr @vba_element_address(ptr, i32)
declare void @vba_element_clear(ptr)
declare i32 @vba_array_lock(ptr)
declare void @vba_array_unlock(ptr)
declare i32 @vba_condition(ptr, ptr)
declare i32 @vba_variant_compare(ptr, ptr, ptr, i32, i32)
declare i32 @vba_for_validate(ptr, ptr, ptr)
declare i32 @vba_for_test(ptr, ptr, ptr, ptr)
declare i32 @vba_enum_start(ptr, ptr, i32, i32)
declare i32 @vba_enum_next(ptr, ptr, ptr)
declare void @vba_enum_clear(ptr)
declare i32 @vba_record_array_redim(ptr, ptr, i32, i32, ptr)
declare i32 @vba_record_array_erase(ptr, ptr)
declare i32 @vba_record_array_index(ptr, ptr, ptr, i32, ptr)
declare i32 @vba_record_init(ptr, ptr)
declare i32 @vba_record_load(ptr, ptr, ptr)
declare i32 @vba_record_store(ptr, ptr, ptr)
declare void @vba_record_clear(ptr, ptr)
define internal void @vba.str.foreign.drop(ptr %p) {
  call void @vba_bstr_free(ptr %p)
  ret void
}
`

func variantTag(t ir.Type) int {
	if t.Scalar == ir.Object {
		return 0x1000
	}
	if ir.Array(t) {
		return 8192 | variantTag(ir.ElementType(t))
	}
	switch t.Scalar {
	case ir.Record:
		return 36
	case ir.Boolean:
		return 11
	case ir.Byte:
		return 17
	case ir.Integer:
		return 2
	case ir.Long:
		return 3
	case ir.Single:
		return 4
	case ir.Double:
		return 5
	case ir.Date:
		return 7
	case ir.Currency:
		return 6
	case ir.String:
		return 8
	case ir.Variant:
		return 12
	}
	panic("unverified Variant tag")
}

func (f *function) pending(in ir.Instruction) {
	name := valueName(in.ID)
	condition := fmt.Sprintf("%%failed%d", in.ID)
	f.line("%s = icmp ne i32 %s, 0", condition, name)
	callSlot := ""
	if in.Type.Scalar != ir.Void {
		callSlot = fmt.Sprintf("%%call%d", in.ID)
	}
	f.results[in.ID] = result{condition: condition, status: name, callSlot: callSlot, source: in.Source, recordError: true}
}

func (f *function) variantBinary(in ir.Instruction, args []operand, operation int) {
	f.line("%s = call i32 @vba_variant_binary(ptr %%call%d, ptr %s, ptr %s, i32 %d)", valueName(in.ID), in.ID, f.memory[in.Args[0]], f.memory[in.Args[1]], operation)
	f.pending(in)
}

func (f *function) variantInstruction(in ir.Instruction, args []operand) operand {
	name := valueName(in.ID)
	value := operand{text: name, typ: in.Type}
	switch in.Op {
	case ir.OpLoadManaged:
		if in.Type.Scalar == ir.Object {
			if class := f.procedure.Slots[in.Slot-1].AutoClass; class != 0 {
				f.line("%s = call i32 @vba_class_load(ptr %%call%d, ptr %s, ptr @vba.class.type%d, ptr %%error)", name, in.ID, f.slots[in.Slot], class)
			} else {
				f.line("%s = call i32 @vba_object_load(ptr %%call%d, ptr %s, i32 %d)", name, in.ID, f.slots[in.Slot], f.procedure.Slots[in.Slot-1].AutoNew)
			}
		} else if in.Type.Scalar == ir.Record {
			f.line("%s = call i32 @vba_record_load(ptr %%call%d, ptr %s, ptr @vba.record.type%d)", name, in.ID, f.slots[in.Slot], in.Type.Record)
		} else if ir.Array(in.Type) {
			f.line("%%array.loaded%d = load ptr, ptr %s, align 1", in.ID, f.slots[in.Slot])
			f.line("%s = call i32 @vba_array_copy(ptr %%call%d, ptr %%array.loaded%d)", name, in.ID, in.ID)
		} else if in.Type.Scalar == ir.Variant || f.module.variants {
			callee := "@vba_string_load"
			if in.Type.Scalar == ir.Variant {
				callee = "@vba_variant_load"
			}
			f.line("%s = call i32 %s(ptr %%call%d, ptr %s)", name, callee, in.ID, f.slots[in.Slot])
		} else {
			f.line("%%loaded%d = load %%vba.string, ptr %s, align 1", in.ID, f.slots[in.Slot])
			f.line("call void @vba.str.retain(%%vba.string %%loaded%d)", in.ID)
			f.line("store %%vba.string %%loaded%d, ptr %%call%d, align 1", in.ID, in.ID)
			f.line("%s = add i32 0, 0", name)
		}
		f.pending(in)
	case ir.OpStoreManaged:
		if args[0].typ.Scalar == ir.Object {
			f.line("%s = call i32 @vba_object_store(ptr %s, ptr %s)", name, f.slots[in.Slot], args[0].text)
		} else if args[0].typ.Scalar == ir.Record {
			f.line("%s = call i32 @vba_record_store(ptr %s, ptr %s, ptr @vba.record.type%d)", name, f.slots[in.Slot], f.memory[in.Args[0]], args[0].typ.Record)
		} else if ir.Array(args[0].typ) {
			f.line("%s = call i32 @vba_array_store(ptr %s, ptr %s)", name, f.slots[in.Slot], args[0].text)
		} else if args[0].typ.Scalar == ir.Variant || f.module.variants {
			callee := "@vba_string_store"
			if args[0].typ.Scalar == ir.Variant {
				callee = "@vba_variant_store"
			}
			f.line("%s = call i32 %s(ptr %s, ptr %s)", name, callee, f.slots[in.Slot], f.memory[in.Args[0]])
		} else {
			f.line("call void @vba.str.assign(ptr %s, %%vba.string %s)", f.slots[in.Slot], args[0].text)
			f.line("%s = add i32 0, 0", name)
		}
		f.pending(in)
	case ir.OpBox:
		if args[0].typ.Scalar == ir.Object {
			f.line("%s = call i32 @vba_variant_box_object(ptr %%call%d, ptr %s)", name, in.ID, args[0].text)
		} else if args[0].typ.Scalar == ir.ArrayObject {
			f.line("%s = call i32 @vba_object_array_box(ptr %%call%d, ptr %s, ptr %s)", name, in.ID, args[0].text, objectArrayPointer(args[0].typ))
		} else if ir.Array(args[0].typ) {
			f.line("%s = call i32 @vba_array_box(ptr %%call%d, ptr %s, i32 %d)", name, in.ID, args[0].text, variantTag(ir.ElementType(args[0].typ)))
		} else if args[0].typ.Scalar == ir.String {
			f.line("%s = call i32 @vba_variant_box_string(ptr %%call%d, ptr %s)", name, in.ID, f.memory[in.Args[0]])
		} else if ir.Floating(args[0].typ) || args[0].typ.Scalar == ir.Date {
			input := args[0].text
			if args[0].typ.Scalar == ir.Single {
				input = fmt.Sprintf("%%float.box%d", in.ID)
				f.line("%s = fpext float %s to double", input, args[0].text)
			}
			f.line("%s = call i32 @vba_variant_box_float(ptr %%call%d, i32 %d, double %s)", name, in.ID, variantTag(args[0].typ), input)
		} else if args[0].typ.Scalar == ir.Currency {
			f.line("%s = call i32 @vba_variant_box_number(ptr %%call%d, i32 6, i64 %s)", name, in.ID, args[0].text)
		} else {
			extend := "sext"
			if args[0].typ.Scalar == ir.Byte {
				extend = "zext"
			}
			f.line("%%wide.box%d = %s %s %s to i64", in.ID, extend, scalar(args[0].typ), args[0].text)
			f.line("%s = call i32 @vba_variant_box_number(ptr %%call%d, i32 %d, i64 %%wide.box%d)", name, in.ID, variantTag(args[0].typ), in.ID)
		}
		f.pending(in)
	case ir.OpUnbox:
		if in.Type.Scalar == ir.Object {
			f.line("%s = call i32 @vba_variant_object(ptr %%call%d, ptr %s)", name, in.ID, f.memory[in.Args[0]])
		} else if in.Type.Scalar == ir.ArrayObject {
			f.line("%s = call i32 @vba_object_array_unbox(ptr %%call%d, ptr %s, ptr %s)", name, in.ID, f.memory[in.Args[0]], objectArrayPointer(in.Type))
		} else if ir.Array(in.Type) {
			f.line("%s = call i32 @vba_array_unbox(ptr %%call%d, ptr %s, i32 %d)", name, in.ID, f.memory[in.Args[0]], variantTag(ir.ElementType(in.Type)))
		} else {
			f.line("%s = call i32 @vba_variant_to_at(ptr %%call%d, ptr %s, i32 %d,ptr %%error)", name, in.ID, f.memory[in.Args[0]], variantTag(in.Type))
		}
		f.pending(in)
	case ir.OpPinString:
		f.line("%s = call i32 @vba_string_pin(ptr %s)", name, args[0].text)
		f.pending(in)
	case ir.OpVariantRef:
		tag := 0x4000 | variantTag(args[0].typ)
		address := args[0].text
		if args[0].typ.Scalar == ir.String {
			address = fmt.Sprintf("%%bstr.slot%d", in.ID)
			f.line("%s = call ptr @vba_string_ref(ptr %s)", address, args[0].text)
		}
		f.line("%%alias.address%d = ptrtoint ptr %s to i64", in.ID, address)
		f.line("%%alias%d = insertvalue %%vba.variant { i16 %d, i16 0, i16 0, i16 0, i64 0, i64 0 }, i64 %%alias.address%d, 4", in.ID, tag, in.ID)
		alias := fmt.Sprintf("%%alias%d", in.ID)
		if args[0].typ.Scalar == ir.Object || args[0].typ.Scalar == ir.ArrayObject {
			class, builtin := in.Class, in.Integer
			if args[0].typ.Scalar == ir.ArrayObject {
				class, builtin = args[0].typ.Class, int64(args[0].typ.Builtin)
			}
			if class != 0 {
				f.line("%%alias.type%d = ptrtoint ptr @vba.class.type%d to i64", in.ID, class)
				f.line("%%alias.class%d = insertvalue %%vba.variant %s,i64 %%alias.type%d,5", in.ID, alias, in.ID)
				alias = fmt.Sprintf("%%alias.class%d", in.ID)
			}
			if builtin != 0 {
				f.line("%%alias.builtin%d = insertvalue %%vba.variant %s,i16 %d,1", in.ID, alias, builtin)
				alias = fmt.Sprintf("%%alias.builtin%d", in.ID)
			}
		}
		f.line("store %%vba.variant %s, ptr %%reference%d, align 1", alias, in.ID)
		value.text = fmt.Sprintf("%%reference%d", in.ID)
	case ir.OpVarType, ir.OpIsEmpty, ir.OpIsNull, ir.OpIsMissing, ir.OpIsArray:
		op := map[ir.Op]int{ir.OpVarType: 1, ir.OpIsEmpty: 2, ir.OpIsNull: 3, ir.OpIsMissing: 4, ir.OpIsArray: 5}[in.Op]
		input := f.memory[in.Args[0]]
		if args[0].typ.Form == ir.Reference {
			input = args[0].text
		}
		f.line("%%query%d = call i32 @vba_variant_query(ptr %s, i32 %d)", in.ID, input, op)
		f.line("%s = trunc i32 %%query%d to i16", name, in.ID)
	}
	return value
}

// Ownership acquisition/return can fail outside a source statement. Statement
// zero identifies the activation boundary; the caller handles its call region.
func (f *function) activationCheck(label, status string, source ir.Source, regions ...*ir.Statement) {
	f.line("%%%s.failed = icmp ne i32 %s, 0", label, status)
	f.current.branch("%"+label+".failed", label+".failure", label+".success")
	f.current = f.newBlock(label + ".failure")
	var statement ir.StatementID
	if len(regions) > 0 {
		statement = regions[0].ID
	}
	f.recordError(label+".error", status, f.procedure.ID, statement, source.Range.Pos())
	if f.module.errors {
		f.line("call void @vba_err_capture(i32 %s, ptr @vba.err.source%d)", status, f.procedure.ID)
	}
	if f.module.errors && len(regions) > 0 {
		f.fail(regions[0], status)
	} else {
		f.propagate(status)
	}
	f.current = f.newBlock(label + ".success")
}
