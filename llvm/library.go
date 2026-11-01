package llvm

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

// Native names and trailing ABI selectors belong to the backend. Declarations,
// prepared operand counts, result types, and effects come from the IR contract.
type libraryABI struct {
	name     string
	trailing []int32
}

var libraryABIs = map[stdlib.MemberID]libraryABI{
	stdlib.CDate:              {name: "vba_convert_explicit", trailing: []int32{7}},
	stdlib.CCur:               {name: "vba_convert_explicit", trailing: []int32{6}},
	stdlib.CVDate:             {name: "vba_cvdate"},
	stdlib.CDec:               {name: "vba_cdec"},
	stdlib.CByte:              {name: "vba_convert_explicit", trailing: []int32{17}},
	stdlib.CInt:               {name: "vba_convert_explicit", trailing: []int32{2}},
	stdlib.CLng:               {name: "vba_convert_explicit", trailing: []int32{3}},
	stdlib.CBool:              {name: "vba_convert_explicit", trailing: []int32{11}},
	stdlib.CStr:               {name: "vba_convert_explicit", trailing: []int32{8}},
	stdlib.CSng:               {name: "vba_convert_explicit", trailing: []int32{4}},
	stdlib.CDbl:               {name: "vba_convert_explicit", trailing: []int32{5}},
	stdlib.CVErr:              {name: "vba_cverr"},
	stdlib.IsError:            {name: "vba_iserror"},
	stdlib.AscW:               {name: "vba_ascw"},
	stdlib.StrComp:            {name: "vba_strcomp"},
	stdlib.InStr:              {name: "vba_instr"},
	stdlib.Left:               {name: "vba_left", trailing: []int32{0}},
	stdlib.LeftString:         {name: "vba_left", trailing: []int32{1}},
	stdlib.StringRepeat:       {name: "vba_string_repeat", trailing: []int32{0}},
	stdlib.StringRepeatString: {name: "vba_string_repeat", trailing: []int32{1}},
	stdlib.Split:              {name: "vba_split"},
	stdlib.IIf:                {name: "vba_iif"},
}

func (abi libraryABI) declaration(arity int) string {
	types := []string{"ptr"} // Fresh output storage.
	for i := 0; i < arity; i++ {
		types = append(types, "ptr")
	}
	for range abi.trailing {
		types = append(types, "i32")
	}
	return fmt.Sprintf("declare i32 @%s(%s)", abi.name, strings.Join(types, ", "))
}

func (f *function) libraryInstruction(in ir.Instruction) {
	abi := libraryABIs[in.Library] // Emit checks availability before generating output.
	f.module.intrinsics[abi.name] = abi.declaration(len(in.Args))
	params := []string{fmt.Sprintf("ptr %%call%d", in.ID)}
	for _, id := range in.Args {
		params = append(params, "ptr "+f.memory[id])
	}
	for _, value := range abi.trailing {
		params = append(params, fmt.Sprintf("i32 %d", value))
	}
	f.line("%s = call i32 @%s(%s)", valueName(in.ID), abi.name, strings.Join(params, ", "))
	f.pending(in)
}
