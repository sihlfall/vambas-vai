package stdlib_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/stdlib"
)

// Compare to the pinned, independently maintained source index, rather than
// deriving expected values from the catalog we are testing.
func TestConstantsCoverPinnedInventory(t *testing.T) {
	index, err := os.ReadFile("../VBA_Standard_Library_MS-VBAL_Section_6.md")
	if err != nil {
		t.Fatal(err)
	}
	header := regexp.MustCompile("^### (6\\.[^ ]*) \\[`([^`]+)`")
	row := regexp.MustCompile("^\\| \\[`([^`]+)`\\]\\([^)]*\\) \\| `([^`]+)` \\|")
	constants := make(map[string]stdlib.Constant)
	for _, c := range stdlib.Constants() {
		constants[c.Name] = c
	}
	counts := map[string]int{
		"FormShowConstants": 2, "VbAppWinStyle": 6, "VbCalendar": 2,
		"VbCallType": 4, "VbCompareMethod": 2, "VbDateTimeFormat": 5,
		"VbDayOfWeek": 8, "VbFileAttribute": 8, "VbFirstWeekOfYear": 4,
		"VbIMEStatus": 20, "VbMsgBoxResult": 7, "VbMsgBoxStyle": 20,
		"VbQueryClose": 5, "VbStrConv": 9, "VbTriState": 3, "VbVarType": 19,
		"ColorConstants": 8, "Constants": 11, "KeyCodeConstants": 99, "SystemColorConstants": 27,
	}
	texts := map[string]string{
		"Chr$(0)": "\x00", "Chr$(8)": "\b", "Chr$(9)": "\t",
		"Chr$(10)": "\n", "Chr$(11)": "\v", "Chr$(12)": "\f", "Chr$(13)": "\r",
		"Chr$(13) + Chr$(10)": "\r\n", "implementation-defined": "\r\n",
	}
	var owner, section string
	total := 0
	for _, line := range strings.Split(string(index), "\n") {
		if m := header.FindStringSubmatch(line); m != nil {
			section, owner = m[1], m[2]
		}
		m := row.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, value := m[1], m[2]
		c, ok := constants[name]
		if !ok || c.Owner != owner || c.Section != section {
			t.Fatalf("missing or misplaced inventory constant %s in %s (%s): %+v", name, owner, section, c)
		}
		want := stdlib.Literal{Kind: stdlib.NumericLiteral, Text: value}
		kind := stdlib.TypeLong
		if text, ok := texts[value]; ok {
			want = stdlib.Literal{Kind: stdlib.StringLiteral, Text: text}
			kind = stdlib.TypeString
		} else if value == "null string pointer" {
			want = stdlib.Literal{Kind: stdlib.NullStringLiteral}
			kind = stdlib.TypeString
		}
		if c.Value != want || c.Type != (stdlib.Type{Kind: kind}) {
			t.Fatalf("%s: got %+v (%+v), want %+v (%v)", name, c.Value, c.Type, want, kind)
		}
		delete(constants, name)
		counts[owner]--
		total++
	}
	if total != 269 || len(constants) != 0 || len(stdlib.Enums()) != 16 {
		t.Fatalf("inventory coverage: %d constants, %d extra catalog entries, %d enums", total, len(constants), len(stdlib.Enums()))
	}
	for owner, remaining := range counts {
		if remaining != 0 {
			t.Errorf("%s constant count differs by %d", owner, remaining)
		}
	}
}
