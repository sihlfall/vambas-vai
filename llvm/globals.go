package llvm

import (
	"fmt"
	"strconv"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

// Globals own values for the project's lifetime, independently of activations.
// Initialization has no user callbacks; the initial runtime profile is single
// threaded. Hosts may explicitly clear/reset a quiescent project. Native main
// performs this on both success and failure.
func projectGlobals(globals []ir.Global, errors bool) string {
	if len(globals) == 0 && !errors {
		return ""
	}
	var out strings.Builder
	for _, g := range globals {
		fmt.Fprintf(&out, "@vba.g%d = internal global %s %s\n", g.ID, scalar(g.Type), zero(g.Type))
		if len(g.Bounds) > 0 {
			var bounds []string
			for _, b := range g.Bounds {
				bounds = append(bounds, fmt.Sprintf("i32 %d", b.Lower), fmt.Sprintf("i32 %d", b.Upper))
			}
			fmt.Fprintf(&out, "@vba.g%d.bounds = private constant [%d x i32] [%s]\n", g.ID, len(bounds), strings.Join(bounds, ", "))
		}
	}
	initialize := ir.GlobalInitializationEffects(globals) != 0
	if initialize {
		out.WriteString("@vba.project.ready = internal global i1 false\n")
	}
	out.WriteString("define internal void @vba.project.clear() {\nentry:\n")
	if errors {
		out.WriteString("  call void @vba_err_clear()\n")
	}
	for i := len(globals) - 1; i >= 0; i-- {
		g := globals[i]
		if ir.Managed(g.Type) {
			callee := "vba.str.clear"
			if ir.Array(g.Type) {
				callee = "vba_array_clear"
			}
			if g.Type.Scalar == ir.Object {
				callee = "vba_object_clear"
			}
			if g.Type.Scalar == ir.Variant {
				callee = "vba_variant_clear"
			}
			if g.Type.Scalar == ir.Record {
				fmt.Fprintf(&out, "  call void @vba_record_clear(ptr @vba.g%d, ptr @vba.record.type%d)\n", g.ID, g.Type.Record)
			} else {
				fmt.Fprintf(&out, "  call void @%s(ptr @vba.g%d)\n", callee, g.ID)
			}
		}
		fmt.Fprintf(&out, "  store %s %s, ptr @vba.g%d, align 1\n", scalar(g.Type), zero(g.Type), g.ID)
	}
	if initialize {
		out.WriteString("  store i1 false, ptr @vba.project.ready\n")
	}
	out.WriteString("  ret void\n}\n")
	if !initialize {
		return out.String()
	}
	out.WriteString("define internal i32 @vba.project.ensure(ptr %error) {\nentry:\n  %ready = load i1, ptr @vba.project.ready\n  br i1 %ready, label %success, label %initialize\ninitialize:\n")
	for _, g := range globals {
		if g.Type.Scalar != ir.Record && len(g.Bounds) == 0 {
			continue
		}
		label := fmt.Sprintf("g%d", g.ID)
		if g.Type.Scalar == ir.Record {
			fmt.Fprintf(&out, "  %%status.%s = call i32 @vba_record_init(ptr @vba.g%d, ptr @vba.record.type%d)\n", label, g.ID, g.Type.Record)
		} else if g.Type.Scalar == ir.ArrayObject {
			fmt.Fprintf(&out, "  %%status.%s = call i32 @vba_object_array_redim(ptr @vba.g%d, i32 4096, ptr %s, i32 2, i32 %d, ptr @vba.g%d.bounds)\n", label, g.ID, objectArrayPointer(g.Type), len(g.Bounds), g.ID)
		} else if g.Type.Scalar == ir.ArrayRecord {
			fmt.Fprintf(&out, "  %%status.%s = call i32 @vba_record_array_redim(ptr @vba.g%d, ptr @vba.record.type%d, i32 2, i32 %d, ptr @vba.g%d.bounds)\n", label, g.ID, g.Type.Record, len(g.Bounds), g.ID)
		} else {
			fmt.Fprintf(&out, "  %%status.%s = call i32 @vba_array_redim(ptr @vba.g%d, i32 %d, i32 %d, i32 2, i32 %d, ptr @vba.g%d.bounds)\n", label, g.ID, arrayStorageTag(g.Type), variantTag(ir.ElementType(g.Type)), len(g.Bounds), g.ID)
		}
		fmt.Fprintf(&out, "  %%failed.%s = icmp ne i32 %%status.%s, 0\n  br i1 %%failed.%s, label %%%s.failure, label %%%s.success\n%s.failure:\n", label, label, label, label, label, label)
		for i, value := range []string{"%status." + label, "0", "0", strconv.Itoa(g.Source.Range.Pos())} {
			fmt.Fprintf(&out, "  %%error.%s.%d = getelementptr %%vba.error, ptr %%error, i32 0, i32 %d\n  store i32 %s, ptr %%error.%s.%d, align 1\n", label, i, i, value, label, i)
		}
		out.WriteString("  call void @vba.project.clear()\n")
		if errors {
			fmt.Fprintf(&out, "  call void @vba_err_capture(i32 %%status.%s, ptr null)\n", label)
		}
		fmt.Fprintf(&out, "  ret i32 %%status.%s\n%s.success:\n", label, label)
	}
	out.WriteString("  store i1 true, ptr @vba.project.ready\n  br label %success\nsuccess:\n  ret i32 0\n}\n")
	return out.String()
}
