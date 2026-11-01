package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

func classPointer(id ir.ClassID) string {
	if id == 0 {
		return "null"
	}
	return fmt.Sprintf("@vba.class.type%d", id)
}

func (m *module) invocationParameters(out *strings.Builder, name string, parameters []ir.InvocationParameter) {
	var values []string
	for _, p := range parameters {
		flags := 0
		if p.ByRef {
			flags |= 1
		}
		if p.Optional {
			flags |= 2
		}
		if p.ParamArray {
			flags |= 4
		}
		fallback := "null"
		if p.Default != 0 {
			fallback = procedureName(p.Default)
		}
		class, builtin := p.Class, p.Builtin
		if p.Type.Scalar == ir.ArrayObject {
			class, builtin = p.Type.Class, p.Type.Builtin
		}
		values = append(values, fmt.Sprintf("%%vba.invoke.parameter {ptr %s,i32 %d,i32 %d,ptr %s,i32 %d,ptr %s}", m.cstring(strings.ToLower(p.Name)), flags, variantTag(p.Type), classPointer(class), builtin, fallback))
	}
	fmt.Fprintf(out, "%s = private constant [%d x %%vba.invoke.parameter] [%s]\n", name, len(values), strings.Join(values, ","))
}

func (m *module) invocationMetadata(program *ir.Program) string {
	if len(program.Classes) == 0 {
		return ""
	}
	var out strings.Builder
	for _, c := range program.Classes {
		var members []string
		for _, id := range c.Members {
			p := program.Procedures[id-1]
			sig := p.Invocation
			if sig == nil {
				continue
			}
			parameters := fmt.Sprintf("@vba.invoke.parameters%d", id)
			m.invocationParameters(&out, parameters, sig.Parameters)
			isDefault := 0
			if sig.Default {
				isDefault = 1
			}
			flags := sig.Kind
			if flags == ir.InvokeMethod && p.ReturnType.Scalar == ir.Void {
				flags |= ir.InvokeDiscard
			}
			members = append(members, fmt.Sprintf("%%vba.invoke.member {ptr %s,i32 %d,i32 %d,i32 %d,ptr %s,ptr @vba.invoke.p%d}", m.cstring(strings.ToLower(sig.Name)), flags, isDefault, len(sig.Parameters), parameters, id))
			fmt.Fprintf(&out, "define internal i32 @vba.invoke.p%d(ptr %%error,ptr %%out,ptr %%object,ptr %%bindings) {\nentry:\n", id)
			unsupported := p.ReturnType.Scalar == ir.Record
			for _, a := range sig.Parameters {
				if a.Type.Scalar == ir.Record {
					unsupported = true
				}
			}
			if unsupported {
				out.WriteString(" ret i32 13\n}\n")
				continue
			}
			arguments := []string{"ptr %error"}
			if p.ReturnType.Scalar != ir.Void {
				fmt.Fprintf(&out, " %%result = alloca %s\n store %s %s,ptr %%result\n", scalar(p.ReturnType), scalar(p.ReturnType), zero(p.ReturnType))
				arguments = append(arguments, "ptr %result")
			}
			arguments = append(arguments, "ptr %object")
			for i, slotID := range p.Parameters[1:] {
				slot := p.Slots[slotID-1]
				fmt.Fprintf(&out, " %%binding%d = getelementptr %%vba.binding,ptr %%bindings,i32 %d,i32 2\n %%address%d = load ptr,ptr %%binding%d\n", i, i, i, i)
				if slot.ByRef {
					arguments = append(arguments, fmt.Sprintf("ptr %%address%d", i))
				} else {
					fmt.Fprintf(&out, " %%arg%d = load %s,ptr %%address%d,align 1\n", i, scalar(slot.Type), i)
					arguments = append(arguments, fmt.Sprintf("%s %%arg%d", scalar(slot.Type), i))
				}
			}
			fmt.Fprintf(&out, " %%status = call i32 %s(%s)\n", procedureName(id), strings.Join(arguments, ","))
			if p.ReturnType.Scalar == ir.Void {
				out.WriteString(" ret i32 %status\n}\n")
				continue
			}
			out.WriteString(" %ok = icmp eq i32 %status,0\n br i1 %ok,label %box,label %failure\nbox:\n")
			if p.ReturnType.Scalar == ir.ArrayObject {
				fmt.Fprintf(&out, " %%array.result = load ptr,ptr %%result\n %%boxed = call i32 @vba_object_array_box(ptr %%out,ptr %%array.result,ptr %s)\n", objectArrayPointer(p.ReturnType))
			} else {
				fmt.Fprintf(&out, " %%boxed = call i32 @vba_native_box(ptr %%out,ptr %%result,i32 %d)\n", variantTag(p.ReturnType))
			}
			fmt.Fprintf(&out, " call void @vba_native_clear(ptr %%result,i32 %d)\n ret i32 %%boxed\nfailure:\n call void @vba_native_clear(ptr %%result,i32 %d)\n ret i32 %%status\n}\n", variantTag(p.ReturnType), variantTag(p.ReturnType))
		}
		for i, field := range c.Fields {
			if !field.Public {
				continue
			}
			for _, kind := range []ir.InvocationKind{ir.InvokeGet, ir.InvokeLet, ir.InvokeSet} {
				if kind == ir.InvokeLet && field.Type.Scalar == ir.Object || kind == ir.InvokeSet && field.Type.Scalar != ir.Object && field.Type.Scalar != ir.Variant && field.Type.Scalar != ir.ArrayObject {
					continue
				}
				name := fmt.Sprintf("@vba.field%d.%d.%d", c.ID, i, kind)
				parameters := []ir.InvocationParameter{{Name: "indices", Type: ir.Type{Scalar: ir.ArrayVariant}, ParamArray: true, ByRef: true}}
				if kind != ir.InvokeGet {
					parameters = append(parameters, ir.InvocationParameter{Name: "value", Type: ir.Type{Scalar: ir.Variant}})
				}
				m.invocationParameters(&out, name+".parameters", parameters)
				members = append(members, fmt.Sprintf("%%vba.invoke.member {ptr %s,i32 %d,i32 0,i32 %d,ptr %s.parameters,ptr %s}", m.cstring(strings.ToLower(field.Name)), kind, len(parameters), name, name))
				fmt.Fprintf(&out, "define internal i32 %s(ptr %%error,ptr %%out,ptr %%object,ptr %%bindings) {\n %%field = getelementptr %%vba.class%d,ptr %%object,i32 0,i32 3,i32 %d\n %%index.slot = getelementptr %%vba.binding,ptr %%bindings,i32 0,i32 2\n %%index.address = load ptr,ptr %%index.slot\n %%indices = load ptr,ptr %%index.address\n", name, c.ID, i)
				rhs := "null"
				if kind != ir.InvokeGet {
					out.WriteString(" %rhs.slot = getelementptr %vba.binding,ptr %bindings,i32 1,i32 2\n %rhs = load ptr,ptr %rhs.slot\n")
					rhs = "%rhs"
				}
				class, builtin := field.Class, field.Builtin
				if field.Type.Scalar == ir.ArrayObject {
					class, builtin = field.Type.Class, field.Type.Builtin
				}
				fmt.Fprintf(&out, " %%status = call i32 @vba_field_invoke(ptr %%error,ptr %%out,ptr %%field,i32 %d,ptr %s,i32 %d,i32 %d,ptr %%indices,ptr %s,ptr %s,i32 %d)\n ret i32 %%status\n}\n", variantTag(field.Type), classPointer(class), builtin, kind, rhs, classPointer(field.AutoClass), field.AutoNew)
			}
		}
		fmt.Fprintf(&out, "@vba.class.members%d = private constant [%d x %%vba.invoke.member] [%s]\n", c.ID, len(members), strings.Join(members, ","))
		fmt.Fprintf(&out, "define internal i32 @vba.class.invoke%d(ptr %%error,ptr %%out,ptr %%object,ptr %%name,i32 %%flags,i32 %%count,ptr %%args) {\n %%status = call i32 @vba_invoke_members(ptr %%error,ptr %%out,ptr %%object,ptr %%name,i32 %%flags,i32 %%count,ptr %%args,i32 %d,ptr @vba.class.members%d)\n ret i32 %%status\n}\n", c.ID, len(members), c.ID)
	}
	return out.String()
}

func (f *function) objectInvokeInstruction(in ir.Instruction, args []operand) operand {
	for i, arg := range args[1:] {
		pointer, byref := f.memory[in.Args[i+1]], 0
		if arg.typ.Form == ir.Reference {
			pointer, byref = arg.text, 1
		}
		f.line("%%invoke.arg%d.%d = getelementptr %%vba.invoke.arg,ptr %%invoke.args%d,i32 %d", in.ID, i, in.ID, i)
		f.line("call void @vba_invoke_argument(ptr %%invoke.arg%d.%d,ptr %s,ptr %s,i32 %d)", in.ID, i, f.module.cstring(strings.ToLower(in.Names[i])), pointer, byref)
	}
	f.line("%s = call i32 @vba_object_invoke(ptr %%error,ptr %%call%d,ptr %s,ptr %s,i32 %d,i32 %d,ptr %%invoke.args%d)", valueName(in.ID), in.ID, f.memory[in.Args[0]], f.module.cstring(strings.ToLower(in.Member)), in.Invocation, len(args)-1, in.ID)
	f.pending(in)
	return operand{text: valueName(in.ID), typ: in.Type}
}
