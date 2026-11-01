package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

// Dispatch preserves the checked native signature, including ByRef aliases and
// ownership. The receiver is the original concrete object, with no view wrapper.
func interfaceDispatch(program *ir.Program) string {
	var out strings.Builder
	members := make(map[ir.ProcedureID]bool)
	for _, class := range program.Classes {
		for _, iface := range class.Interfaces {
			for _, method := range iface.Methods {
				members[method.Declaration] = true
			}
		}
	}
	for _, p := range program.Procedures {
		if !members[p.ID] {
			continue
		}
		parameters, arguments := []string{"ptr %error"}, []string{"ptr %error"}
		if p.ReturnType.Scalar != ir.Void {
			parameters = append(parameters, "ptr %out")
			arguments = append(arguments, "ptr %out")
		}
		for i, id := range p.Parameters {
			slot := p.Slots[id-1]
			t := scalar(slot.Type)
			if slot.ByRef {
				t = "ptr"
			}
			arg := fmt.Sprintf("%s %%arg%d", t, i)
			parameters, arguments = append(parameters, arg), append(arguments, arg)
		}
		fmt.Fprintf(&out, "define internal i32 @vba.interface.p%d(%s) {\nentry:\n %%method.slot = alloca ptr\n %%status = call i32 @vba_interface_resolve(ptr %%method.slot, ptr %%arg0, ptr @vba.class.type%d, i32 %d, ptr %s)\n %%ok = icmp eq i32 %%status, 0\n br i1 %%ok, label %%dispatch, label %%failure\nfailure:\n ret i32 %%status\ndispatch:\n %%method = load ptr, ptr %%method.slot\n %%result = call i32 %%method(%s)\n ret i32 %%result\n}\n", p.ID, strings.Join(parameters, ", "), p.Class, p.ID, procedureName(p.ID), strings.Join(arguments, ", "))
	}
	return out.String()
}
