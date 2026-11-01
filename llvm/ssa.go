package llvm

import (
	"fmt"
	"regexp"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

// Only native scalars whose storage identity is unobservable become SSA values.
// In particular OpAddress excludes ByRef actuals and error-side write targets.
// Managed values stay in storage: their tags, ownership and coercion effects are
// observable. No scalar is boxed to construct a phi.
func scalarSSASlots(p *ir.Procedure) map[ir.SlotID]bool {
	eligible := make(map[ir.SlotID]bool)
	used := make(map[ir.ValueID]bool)
	for _, s := range p.Statements {
		used[s.Condition], used[s.Selector], used[s.ReturnAddress] = true, true, true
		for _, id := range s.Cleanup {
			used[id] = true
		}
		for _, in := range s.Instructions {
			for _, id := range in.Args {
				used[id] = true
			}
		}
	}
	for _, s := range p.Slots {
		if s.ByRef || s.Kind == ir.GlobalAlias || s.Type.Form != ir.Value {
			continue
		}
		switch s.Type.Scalar {
		case ir.Boolean, ir.Byte, ir.Integer, ir.Long, ir.Single, ir.Double, ir.Date, ir.Currency:
			eligible[s.ID] = true
		}
	}
	for _, s := range p.Statements {
		for _, in := range s.Instructions {
			// Double assignments reserve an error-side destination even when
			// their expression never uses it. An unused address has no identity
			// requirement and emits no LLVM instruction.
			if in.Op == ir.OpAddress && !used[in.ID] {
				continue
			}
			if in.Slot != 0 && in.Op != ir.OpLoad && in.Op != ir.OpStore {
				delete(eligible, in.Slot)
			}
		}
	}
	return eligible
}

type slotPhi struct {
	name   string
	slot   ir.SlotID
	block  *block
	inputs []string // One per incoming edge, including duplicate switch edges.
}

func (f *function) writeScalar(slot ir.SlotID, value string) {
	if f.current.definitions == nil {
		f.current.definitions = make(map[ir.SlotID]string)
	}
	f.current.definitions[slot] = value
}

func (f *function) loadSlot(slot ir.SlotID, name string) string {
	if f.ssaSlots[slot] {
		return f.readScalar(f.current, slot)
	}
	f.line("%s = load %s, ptr %s, align 1", name, scalar(f.procedure.Slots[slot-1].Type), f.slots[slot])
	return name
}

func (f *function) storeSlot(slot ir.SlotID, value string) {
	if f.ssaSlots[slot] {
		f.writeScalar(slot, value)
		return
	}
	f.line("store %s %s, ptr %s, align 1", scalar(f.procedure.Slots[slot-1].Type), value, f.slots[slot])
}

// Demand-driven SSA construction. A read before a local definition reserves a
// block-entry phi. Its operands are filled only after all blocks/edges exist,
// so forward jumps, loop backedges, Resume and GoSub need no special ordering.
func (f *function) readScalar(b *block, slot ir.SlotID) string {
	if value, ok := b.definitions[slot]; ok {
		return value
	}
	phi := &slotPhi{name: fmt.Sprintf("%%ssa.%d", len(f.ssaPhis)), slot: slot, block: b}
	f.ssaPhis = append(f.ssaPhis, phi)
	b.phis = append(b.phis, phi)
	if b.definitions == nil {
		b.definitions = make(map[ir.SlotID]string)
	}
	b.definitions[slot] = phi.name
	return phi.name
}

func (f *function) finishSSA() map[string]string {
	if len(f.ssaPhis) == 0 {
		return nil
	}
	blocks := make(map[string]*block, len(f.blocks))
	for _, b := range f.blocks {
		blocks[b.label] = b
	}
	for _, b := range f.blocks {
		for _, target := range b.successors {
			next := blocks[target]
			if next == nil {
				panic("LLVM backend emitted an unknown block: " + target)
			}
			next.predecessors = append(next.predecessors, b)
		}
	}
	// Reading predecessor definitions may reserve additional phis. Iteration,
	// rather than recursion, also handles very long procedures and dead cycles.
	for i := 0; i < len(f.ssaPhis); i++ {
		phi := f.ssaPhis[i]
		for _, predecessor := range phi.block.predecessors {
			phi.inputs = append(phi.inputs, f.readScalar(predecessor, phi.slot))
		}
	}
	aliases := make(map[string]string)
	for changed := true; changed; {
		changed = false
		for _, phi := range f.ssaPhis {
			if _, removed := aliases[phi.name]; removed {
				continue
			}
			value, trivial := "", true
			for _, input := range phi.inputs {
				input = resolveSSA(input, aliases)
				if input == phi.name {
					continue
				}
				if value != "" && value != input {
					trivial = false
					break
				}
				value = input
			}
			if trivial {
				if value == "" {
					// No incoming definition is possible in a disconnected block
					// or self-only dead cycle. Use the language's zero value.
					value = zero(f.procedure.Slots[phi.slot-1].Type)
				}
				aliases[phi.name] = value
				changed = true
			}
		}
	}
	return aliases
}

func resolveSSA(value string, aliases map[string]string) string {
	start := value
	for aliases[value] != "" {
		value = aliases[value]
	}
	for start != value {
		next := aliases[start]
		aliases[start] = value
		start = next
	}
	return value
}

// Only compiler-reserved phi names are substituted, at token boundaries. This
// renders already-built SSA; it neither parses nor transforms LLVM instructions.
var ssaName = regexp.MustCompile(`%ssa\.[0-9]+\b`)

func resolveSSAText(text string, aliases map[string]string) string {
	if len(aliases) == 0 || strings.HasPrefix(text, ";") || !strings.Contains(text, "%ssa.") {
		return text
	}
	return ssaName.ReplaceAllStringFunc(text, func(name string) string { return resolveSSA(name, aliases) })
}

func (f *function) formatPhis(out *strings.Builder, b *block, aliases map[string]string) {
	for _, phi := range b.phis {
		if aliases[phi.name] != "" {
			continue
		}
		fmt.Fprintf(out, "  %s = phi %s ", phi.name, scalar(f.procedure.Slots[phi.slot-1].Type))
		for i, input := range phi.inputs {
			if i > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(out, "[ %s, %%%s ]", resolveSSA(input, aliases), b.predecessors[i].label)
		}
		out.WriteByte('\n')
	}
}
