package llvm_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
)

func expectString(t *testing.T, p *ir.Program, entry ir.ProcedureID, want []uint16, transform func(string) string) {
	t.Helper()
	var harness strings.Builder
	fmt.Fprintf(&harness, `define i32 @main() {
entry:
  %%error = alloca %%vba.error
  %%output = alloca %%vba.string
  store %%vba.string zeroinitializer, ptr %%output
  %%status = call i32 @vba.p%d(ptr %%error, ptr %%output)
  %%s = load %%vba.string, ptr %%output
  %%length = call i32 @vba.str.len(%%vba.string %%s)
  %%ok.status = icmp eq i32 %%status, 0
  %%ok.length = icmp eq i32 %%length, %d
  %%ok.initial = and i1 %%ok.status, %%ok.length
  br i1 %%ok.initial, label %%contents, label %%failure
failure:
  ret i32 1
contents:
`, entry, len(want))
	previous := "true"
	for i, unit := range want {
		fmt.Fprintf(&harness, "  %%u%d = call i16 @vba.str.unit(%%vba.string %%s, i32 %d)\n  %%c%d = icmp eq i16 %%u%d, %d\n  %%all%d = and i1 %s, %%c%d\n", i, i, i, i, unit, i, previous, i)
		previous = fmt.Sprintf("%%all%d", i)
	}
	fmt.Fprintf(&harness, "  call void @vba.str.clear(ptr %%output)\n  %%exit = select i1 %s, i32 0, i32 1\n  ret i32 %%exit\n}\n", previous)
	text := emit(t, p, llvm.Options{}) + harness.String()
	if transform != nil {
		text = transform(text)
	}
	native(t, trackedStrings(text))
}

func TestNativeMidReads(t *testing.T) {
	for _, test := range []struct{ expression, want string }{
		{`Mid("abcdef",2,3)`, "bcd"}, {`Mid$("abcdef",2)`, "bcdef"},
		{`Mid$("abc",4)`, ""}, {`Mid$("abc",2,2147483647)`, "bc"},
		{`Mid("abc",2,0)`, ""}, {`Mid$(vbNullString,1)`, ""},
		{`VBA.Mid$(Length:=2,String:="abcdef",Start:=3)`, "cd"},
	} {
		t.Run(test.expression, func(t *testing.T) {
			p := lower(t, "Function Run() As String\n Run = "+test.expression+"\nEnd Function\n")
			expectString(t, p, 1, utf16.Encode([]rune(test.want)), nil)
		})
	}
	p := lower(t, "Function Run() As String\n Run = Mid$(ChrW(55357) & ChrW(56832),2,1)\nEnd Function\n")
	expectString(t, p, 1, []uint16{56832}, nil)
}

func TestNativeMidMutations(t *testing.T) {
	for _, test := range []struct{ body, want string }{
		{`s = "abcdef": Mid$(s,2,2) = "XY": Run = s`, "aXYdef"},
		{`s = "abcdef": Mid(s,2,4) = "X": Run = s`, "aXcdef"},
		{`s = "abcdef": Mid$(s,5) = "WXYZ": Run = s`, "abcdWX"},
		{`s = "abcdef": Mid$(s,2,0) = "x": Run = s`, "abcdef"},
		{`s = "abc": Mid$(s,2,1) = vbNullChar: Run = s`, "a\x00c"},
		{`s = "a": Mid$(s,1) = "XY": Run = s`, "X"},
		{`s = "abcde" & ChrW(102): Mid$(s,2,4) = Mid$(s,1,4): Run = s`, "aabcdf"},
		{`s = "abcde" & ChrW(102): Mid$(s,2) = s: Run = s`, "aabcde"},
		{`s = "abc" & ChrW(100): copy = s: Mid$(s,2,2) = "XY": Run = s & copy`, "aXYdabcd"},
		{`s = "abcde" & ChrW(102): copy = Mid$(s,2,3): Mid$(s,2,1) = "X": Run = s & copy`, "aXcdefbcd"},
	} {
		t.Run(test.body, func(t *testing.T) {
			p := lower(t, "Function Run() As String\n Dim s As String, copy As String\n "+test.body+"\nEnd Function\n")
			expectString(t, p, 1, utf16.Encode([]rune(test.want)), nil)
		})
	}
	p := lower(t, "Function Run() As String\n Dim s As String\n s=\"ab\"\n Mid$(s,1,1)=ChrW(55357)\n Run=s\nEnd Function\n")
	expectString(t, p, 1, []uint16{55357, 98}, nil)
}
