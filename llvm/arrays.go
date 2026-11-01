package llvm

import (
	"fmt"
	"sihlfall.ch/vambas-vai/ir"
	"strings"
)

func objectArrayPointer(t ir.Type) string {
	if t.Class != 0 {
		return fmt.Sprintf("@vba.objectarray.class%d", t.Class)
	}
	return fmt.Sprintf("@vba.objectarray.builtin%d", t.Builtin)
}
func objectArrayMetadata(classes []ir.ClassDefinition) string {
	var out strings.Builder
	out.WriteString("%vba.objectarray.type = type { ptr, i32 }\n")
	for i := 0; i <= 4; i++ {
		fmt.Fprintf(&out, "@vba.objectarray.builtin%d = private constant %%vba.objectarray.type { ptr null, i32 %d }\n", i, i)
	}
	for _, c := range classes {
		fmt.Fprintf(&out, "@vba.objectarray.class%d = private constant %%vba.objectarray.type { ptr @vba.class.type%d, i32 0 }\n", c.ID, c.ID)
	}
	return out.String()
}

func arrayStorageTag(t ir.Type) int {
	if ir.Array(t) {
		return variantTag(ir.ElementType(t))
	}
	return 0
}

func (f *function) arrayInstruction(in ir.Instruction, args []operand) operand {
	name := valueName(in.ID)
	value := operand{text: name, typ: in.Type}
	switch in.Op {
	case ir.OpParamArrayPack:
		for i, arg := range args[1:] {
			pointer, byref := f.memory[in.Args[i+1]], 0
			if arg.typ.Form == ir.Reference {
				pointer, byref = arg.text, 1
			}
			for field, entry := range []string{"ptr " + pointer, fmt.Sprintf("i32 %d", byref)} {
				f.line("%%paramarray.field%d.%d.%d = getelementptr %%vba.paramarg, ptr %%paramarray.args%d, i32 %d, i32 %d", in.ID, i, field, in.ID, i, field)
				f.line("store %s, ptr %%paramarray.field%d.%d.%d", entry, in.ID, i, field)
			}
		}
		f.line("%s = call i32 @vba_paramarray_pack(ptr %s, i32 %d, ptr %%paramarray.args%d)", name, args[0].text, len(args)-1, in.ID)
		f.pending(in)
	case ir.OpReleaseElement:
		if !f.arrayTokens[in.Args[0]].unlocked {
			f.clear(f.memory[in.Args[0]], args[0].typ)
		}
		value.text = "0"
	case ir.OpArrayResize, ir.OpArrayIndex:
		if in.Op == ir.OpArrayIndex && f.inlineAccess(in.ID) {
			f.inlineArrayIndex(in, args)
			break
		}
		start := 1
		if in.Op == ir.OpArrayResize {
			start = 3
		}
		for i, arg := range args[start:] {
			f.line("%%array.arg%d.%d = getelementptr i32, ptr %%array.args%d, i32 %d", in.ID, i, in.ID, i)
			f.line("store i32 %s, ptr %%array.arg%d.%d, align 4", arg.text, in.ID, i)
		}
		if in.Op == ir.OpArrayResize {
			if args[0].typ.Scalar == ir.ArrayObject || in.Class != 0 || in.Integer != 0 {
				t := args[0].typ
				if t.Scalar == ir.Variant {
					t = ir.Type{Scalar: ir.ArrayObject, Class: in.Class, Builtin: uint8(in.Integer)}
				}
				f.line("%s = call i32 @vba_object_array_redim(ptr %s, i32 %d, ptr %s, i32 %s, i32 %d, ptr %%array.args%d)", name, args[0].text, arrayStorageTag(args[0].typ), objectArrayPointer(t), args[2].text, (len(args)-3)/2, in.ID)
			} else if args[0].typ.Scalar == ir.ArrayRecord {
				f.line("%s = call i32 @vba_record_array_redim(ptr %s, ptr @vba.record.type%d, i32 %s, i32 %d, ptr %%array.args%d)", name, args[0].text, args[0].typ.Record, args[2].text, (len(args)-3)/2, in.ID)
			} else {
				f.line("%s = call i32 @vba_array_redim(ptr %s, i32 %d, i32 %s, i32 %s, i32 %d, ptr %%array.args%d)", name, args[0].text, arrayStorageTag(args[0].typ), args[1].text, args[2].text, (len(args)-3)/2, in.ID)
			}
		} else {
			if args[0].typ.Scalar == ir.Variant && args[0].typ.Form == ir.Value {
				f.line("%s = call i32 @vba_capture_index(ptr %%call%d,ptr %s,i32 %d,ptr %%array.args%d)", name, in.ID, f.memory[in.Args[0]], len(args)-1, in.ID)
			} else if args[0].typ.Scalar == ir.ArrayRecord {
				f.line("%s = call i32 @vba_record_array_index(ptr %%call%d, ptr %s, ptr @vba.record.type%d, i32 %d, ptr %%array.args%d)", name, in.ID, args[0].text, args[0].typ.Record, len(args)-1, in.ID)
			} else {
				f.line("%s = call i32 @vba_array_index(ptr %%call%d, ptr %s, i32 %d, i32 %d, i32 %d, ptr %%array.args%d)", name, in.ID, args[0].text, arrayStorageTag(args[0].typ), variantTag(ir.ElementType(in.Type)), len(args)-1, in.ID)
			}
		}
		f.pending(in)
	case ir.OpElementRef:
		if pointer := f.elementPointers[in.Args[0]]; pointer != "" {
			value.text = pointer
		} else {
			f.line("%s = call ptr @vba_element_address(ptr %s, i32 %d)", name, f.memory[in.Args[0]], variantTag(in.Type))
		}
	case ir.OpArrayErase:
		if args[0].typ.Scalar == ir.ArrayRecord {
			f.line("%s = call i32 @vba_record_array_erase(ptr %s, ptr @vba.record.type%d)", name, args[0].text, args[0].typ.Record)
		} else {
			f.line("%s = call i32 @vba_array_erase(ptr %s, i32 %d)", name, args[0].text, arrayStorageTag(args[0].typ))
		}
		f.pending(in)
	case ir.OpArrayBound:
		if f.inlineAccess(in.ID) {
			f.inlineArrayBound(in, args)
			break
		}
		f.line("%s = call i32 @vba_array_bound(ptr %%call%d, ptr %s, i32 %d, i32 %s, i32 %s)", name, in.ID, args[0].text, arrayStorageTag(args[0].typ), args[1].text, args[2].text)
		f.pending(in)
	case ir.OpReadReference:
		if in.Type.Scalar == ir.Object {
			f.line("%s = call i32 @vba_object_load(ptr %%call%d, ptr %s, i32 0)", name, in.ID, args[0].text)
		} else if in.Type.Scalar == ir.Record {
			f.line("%s = call i32 @vba_record_load(ptr %%call%d, ptr %s, ptr @vba.record.type%d)", name, in.ID, args[0].text, in.Type.Record)
		} else if ir.Array(in.Type) {
			f.line("%%read%d = load ptr, ptr %s, align 1", in.ID, args[0].text)
			f.line("%s = call i32 @vba_array_copy(ptr %%call%d, ptr %%read%d)", name, in.ID, in.ID)
		} else if in.Type.Scalar == ir.String || in.Type.Scalar == ir.Variant {
			callee := "@vba_string_load"
			if in.Type.Scalar == ir.Variant {
				callee = "@vba_variant_load"
			}
			f.line("%s = call i32 %s(ptr %%call%d, ptr %s)", name, callee, in.ID, args[0].text)
		} else {
			f.line("%%read%d = load %s, ptr %s, align 1", in.ID, scalar(in.Type), args[0].text)
			f.line("store %s %%read%d, ptr %%call%d, align 1", scalar(in.Type), in.ID, in.ID)
			f.line("%s = add i32 0, 0", name)
		}
		f.pending(in)
	case ir.OpWriteReference:
		if args[1].typ.Scalar == ir.Object {
			f.line("%s = call i32 @vba_object_store(ptr %s, ptr %s)", name, args[0].text, args[1].text)
		} else if args[1].typ.Scalar == ir.Record {
			f.line("%s = call i32 @vba_record_store(ptr %s, ptr %s, ptr @vba.record.type%d)", name, args[0].text, f.memory[in.Args[1]], args[1].typ.Record)
		} else if ir.Array(args[1].typ) {
			f.line("%s = call i32 @vba_array_store(ptr %s, ptr %s)", name, args[0].text, args[1].text)
		} else if ir.Managed(args[1].typ) {
			callee := "@vba_string_store"
			if args[1].typ.Scalar == ir.Variant {
				callee = "@vba_variant_store"
			}
			f.line("%s = call i32 %s(ptr %s, ptr %s)", name, callee, args[0].text, f.memory[in.Args[1]])
		} else {
			f.line("store %s %s, ptr %s, align 1", scalar(args[1].typ), args[1].text, args[0].text)
			f.line("%s = add i32 0, 0", name)
		}
		f.pending(in)
	default:
		panic(fmt.Sprintf("unverified array operation %s", in.Op))
	}
	return value
}
