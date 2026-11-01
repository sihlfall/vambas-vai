package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

// Native offsets are LLVM constant expressions, never guessed by the semantic
// lowerer. Runtime metadata describes ownership, not public VBA/COM layouts.
func recordMetadata(records []ir.RecordDefinition) string {
	return shapeMetadata(records, "record", true)
}

func shapeMetadata(records []ir.RecordDefinition, prefix string, declarations bool) string {
	if len(records) == 0 {
		return ""
	}
	var out strings.Builder
	if declarations {
		out.WriteString("%vba.record.field = type { i32, i32, i64, i64, ptr, i32, ptr }\n%vba.record.type = type { i64, i32, ptr }\n")
	}
	for _, r := range records {
		var fields []string
		for _, f := range r.Fields {
			fields = append(fields, scalar(f.Type))
		}
		fmt.Fprintf(&out, "%%vba.%s%d = type { %s }\n", prefix, r.ID, strings.Join(fields, ", "))
	}
	size := func(t string) string {
		return fmt.Sprintf("ptrtoint (ptr getelementptr (%s, ptr null, i32 1) to i64)", t)
	}
	for _, r := range records {
		var fields []string
		for i, f := range r.Fields {
			kind, tag, nested, bounds := 0, 0, "null", "null"
			switch {
			case f.Type.Scalar == ir.Object:
				kind = 5
			case f.Type.Scalar == ir.Record:
				kind = 4
				nested = fmt.Sprintf("@vba.record.type%d", f.Type.Record)
			case f.Type.Scalar == ir.String:
				kind, tag = 1, 8
			case f.Type.Scalar == ir.Variant:
				kind, tag = 2, 12
			case ir.Array(f.Type):
				kind, tag = 3, variantTag(ir.ElementType(f.Type))
				if f.Type.Scalar == ir.ArrayRecord {
					nested = fmt.Sprintf("@vba.record.type%d", f.Type.Record)
				}
				if f.Type.Scalar == ir.ArrayObject {
					nested = objectArrayPointer(f.Type)
				}
			default:
				tag = variantTag(f.Type)
			}
			if len(f.Bounds) > 0 {
				bounds = fmt.Sprintf("@vba.%s.bounds%d.%d", prefix, r.ID, i)
				var values []string
				for _, b := range f.Bounds {
					values = append(values, fmt.Sprintf("i32 %d", b.Lower), fmt.Sprintf("i32 %d", b.Upper))
				}
				fmt.Fprintf(&out, "%s = private constant [%d x i32] [%s]\n", bounds, len(values), strings.Join(values, ", "))
			}
			offset := fmt.Sprintf("ptrtoint (ptr getelementptr (%%vba.%s%d, ptr null, i32 0, i32 %d) to i64)", prefix, r.ID, i)
			fields = append(fields, fmt.Sprintf("%%vba.record.field { i32 %d, i32 %d, i64 %s, i64 %s, ptr %s, i32 %d, ptr %s }", kind, tag, offset, size(scalar(f.Type)), nested, len(f.Bounds), bounds))
		}
		fmt.Fprintf(&out, "@vba.%s.fields%d = private constant [%d x %%vba.record.field] [%s]\n", prefix, r.ID, len(fields), strings.Join(fields, ", "))
		fmt.Fprintf(&out, "@vba.%s.type%d = private constant %%vba.record.type { i64 %s, i32 %d, ptr @vba.%s.fields%d }\n", prefix, r.ID, size(fmt.Sprintf("%%vba.%s%d", prefix, r.ID)), len(fields), prefix, r.ID)
	}
	return out.String()
}
