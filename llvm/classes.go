package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

func classMetadata(classes []ir.ClassDefinition, declarations bool) string {
	if len(classes) == 0 {
		return ""
	}
	var out strings.Builder
	shapes := make([]ir.RecordDefinition, len(classes))
	for i, c := range classes {
		shapes[i] = ir.RecordDefinition{ID: ir.RecordID(c.ID), Fields: c.Fields}
	}
	out.WriteString(shapeMetadata(shapes, "class.fields", declarations))
	out.WriteString("%vba.object.type = type { i64, ptr, ptr, ptr, ptr, ptr, i64, ptr }\n%vba.interface = type { ptr, i64, ptr }\n%vba.interface.entry = type { i32, ptr }\n")
	for _, c := range classes {
		fmt.Fprintf(&out, "%%vba.class%d = type { i64, i32, ptr, %%vba.class.fields%d }\n", c.ID, c.ID)
		var interfaces []string
		for i, iface := range c.Interfaces {
			var methods []string
			for _, method := range iface.Methods {
				methods = append(methods, fmt.Sprintf("%%vba.interface.entry { i32 %d, ptr %s }", method.Declaration, procedureName(method.Implementation)))
			}
			fmt.Fprintf(&out, "@vba.class.interface%d.%d = private constant [%d x %%vba.interface.entry] [%s]\n", c.ID, i, len(methods), strings.Join(methods, ","))
			interfaces = append(interfaces, fmt.Sprintf("%%vba.interface { ptr @vba.class.type%d, i64 %d, ptr @vba.class.interface%d.%d }", iface.Class, len(methods), c.ID, i))
		}
		fmt.Fprintf(&out, "@vba.class.interfaces%d = private constant [%d x %%vba.interface] [%s]\n", c.ID, len(interfaces), strings.Join(interfaces, ","))
		initialize, terminate := "null", "null"
		if c.Initialize != 0 {
			initialize = procedureName(c.Initialize)
		}
		if c.Terminate != 0 {
			terminate = procedureName(c.Terminate)
		}
		fmt.Fprintf(&out, "@vba.class.type%d = private constant %%vba.object.type { i64 ptrtoint (ptr getelementptr (%%vba.class%d, ptr null, i32 1) to i64), ptr @vba.class.fields.type%d, ptr null, ptr %s, ptr %s, ptr @vba.class.invoke%d, i64 %d, ptr @vba.class.interfaces%d }\n", c.ID, c.ID, c.ID, initialize, terminate, c.ID, len(c.Interfaces), c.ID)
	}
	return out.String()
}

func (f *function) classInstruction(in ir.Instruction, args []operand) operand {
	name := valueName(in.ID)
	switch in.Op {
	case ir.OpClassNew:
		f.line("%s = call i32 @vba_class_construct(ptr %%call%d, ptr @vba.class.type%d, ptr %%error)", name, in.ID, in.Class)
		f.pending(in)
	case ir.OpObjectAutoLoad:
		if in.Class != 0 {
			f.line("%s = call i32 @vba_class_load(ptr %%call%d, ptr %s, ptr @vba.class.type%d, ptr %%error)", name, in.ID, args[0].text, in.Class)
		} else {
			f.line("%s = call i32 @vba_object_load(ptr %%call%d, ptr %s, i32 %d)", name, in.ID, args[0].text, in.Integer)
		}
		f.pending(in)
	case ir.OpClassCheck:
		f.line("%s = call i32 @vba_class_check_exact(ptr %s, ptr @vba.class.type%d)", name, args[0].text, in.Class)
		f.pending(in)
	case ir.OpInterfaceCheck, ir.OpClassCast:
		nullable := 0
		if in.Op == ir.OpClassCast {
			nullable = 1
		}
		f.line("%s = call i32 @vba_class_check(ptr %s, ptr @vba.class.type%d, i32 %d)", name, args[0].text, in.Class, nullable)
		f.pending(in)
	case ir.OpClassFieldRef:
		f.line("%s = getelementptr %%vba.class%d, ptr %s, i32 0, i32 3, i32 %d", name, in.Class, args[0].text, in.Field)
	}
	return operand{text: name, typ: in.Type}
}
