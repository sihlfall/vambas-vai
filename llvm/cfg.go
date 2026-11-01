package llvm

import (
	"fmt"
	"strings"
)

// Keep executable edges alongside terminator text. SSA must include the actual
// error, cleanup, and indirect-return edges, not just source-level flow nodes.
func (b *block) jump(target string) {
	b.end = "br label %" + target
	b.successors = []string{target}
}

func (b *block) branch(condition, yes, no string) {
	b.end = fmt.Sprintf("br i1 %s, label %%%s, label %%%s", condition, yes, no)
	b.successors = []string{yes, no}
}

type switchCase struct {
	value  string
	target string
}

func (b *block) switchOn(typ, value, fallback string, cases []switchCase) {
	b.end = fmt.Sprintf("switch %s %s, label %%%s [", typ, value, fallback)
	b.successors = []string{fallback}
	for _, c := range cases {
		b.end += fmt.Sprintf("\n    %s %s, label %%%s", typ, c.value, c.target)
		b.successors = append(b.successors, c.target)
	}
	b.end += "\n  ]"
}

func (b *block) indirectBranch(value string, targets []string) {
	labels := make([]string, len(targets))
	for i, target := range targets {
		labels[i] = "label %" + target
	}
	b.end = fmt.Sprintf("indirectbr ptr %s, [%s]", value, strings.Join(labels, ", "))
	b.successors = append([]string(nil), targets...)
}
