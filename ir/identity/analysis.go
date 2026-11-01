// Package identity implements identity retention analysis over semantic IR.
// It proves that storage identities survive execution of some code, and that
// storage roots are independent. The backend uses it to omit element locks.
// See README.md for the model.
package identity

import (
	"fmt"
	"slices"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
)

// Summary is a procedure's shadow on its borrowed formals and ambient storage.
// Index 0 is ambient storage; index i+1 is formal i. It is computed for
// independent inputs and applied to the caller's partition, so it is
// parameterized by incoming dependencies without enumerating them: aliased
// actuals share a caller group, and invalidation is group-wide.
type Summary struct {
	names    []string
	borrowed []bool // ByRef/ParamArray formals; ByVal formals are private copies.
	changed  []bool
	observed []bool
	groups   []int // Canonical partition over the indices at exit.
}

func emptySummary(p *ir.Procedure) *Summary {
	n := 1
	m := &Summary{names: []string{"ambient"}, borrowed: []bool{false}}
	if p != nil {
		n += len(p.Parameters)
		for _, id := range p.Parameters {
			s := p.Slots[id-1]
			m.names = append(m.names, s.Name)
			m.borrowed = append(m.borrowed, s.ByRef || s.ParamArray)
		}
	}
	m.changed, m.observed, m.groups = make([]bool, n), make([]bool, n), make([]int, n)
	for i := range m.groups {
		m.groups[i] = i
	}
	return m
}

func summarize(p *ir.Procedure, exit State) *Summary {
	m := emptySummary(p)
	if !exit.Reachable() {
		return m
	}
	locations := []location{ambient}
	locations = append(locations, p.Parameters...)
	for j, l := range locations {
		m.changed[j], m.observed[j] = exit.changed[l], exit.observed[l]
		for k := range j {
			if exit.groups[locations[k]] == exit.groups[l] {
				m.groups[j] = m.groups[k]
				break
			}
		}
	}
	return m
}

func (m *Summary) equal(n *Summary) bool {
	return slices.Equal(m.changed, n.changed) && slices.Equal(m.observed, n.observed) && slices.Equal(m.groups, n.groups)
}

// joinInto adds n's effects to m, mapping formals by position.
func (m *Summary) joinInto(n *Summary) {
	for j := range min(len(m.changed), len(n.changed)) {
		m.changed[j] = m.changed[j] || n.changed[j]
		m.observed[j] = m.observed[j] || n.observed[j]
		for k := range j {
			if n.groups[k] == n.groups[j] && m.groups[k] != m.groups[j] {
				keep, drop := min(m.groups[k], m.groups[j]), max(m.groups[k], m.groups[j])
				for i, g := range m.groups {
					if g == drop {
						m.groups[i] = keep
					}
				}
			}
		}
	}
}

func (m *Summary) clone() *Summary {
	return &Summary{names: m.names, borrowed: m.borrowed, changed: slices.Clone(m.changed), observed: slices.Clone(m.observed), groups: slices.Clone(m.groups)}
}

// apply runs the shadow at a call site. A nil summary is an unsummarized
// callee: it may invalidate or observe any tracked storage.
func (m *Summary) apply(s State, actuals []root) {
	if m == nil {
		s.invalidate(unknownRoot)
		s.observe(unknownRoot)
		return
	}
	target := func(j int) root {
		switch {
		case j == 0:
			return root(ambient)
		case !m.borrowed[j]:
			return freshRoot
		case j-1 < len(actuals):
			return actuals[j-1]
		}
		return unknownRoot
	}
	for j := range m.groups {
		for k := range j {
			if m.groups[k] == m.groups[j] {
				s.connect(target(k), target(j))
				break
			}
		}
	}
	for j := range m.changed {
		if m.changed[j] {
			s.invalidate(target(j))
		}
		if m.observed[j] {
			s.observe(target(j))
		}
	}
}

// Names lists the summary's tracked inputs: borrowed formals, then ambient.
func (m *Summary) Names() []string {
	var names []string
	for j := 1; j < len(m.names); j++ {
		if m.borrowed[j] {
			names = append(names, m.names[j])
		}
	}
	return append(names, m.names[0])
}

func (m *Summary) indices() []int {
	var indices []int
	for j := 1; j < len(m.names); j++ {
		if m.borrowed[j] {
			indices = append(indices, j)
		}
	}
	return append(indices, 0)
}

// Shadow presents the summary as an integer-label transformer over Names().
// Equal positive input labels may depend on the same storage; label 0 is
// unknown and may depend on anything. An unchanged positive output label
// certifies retention. Replacement labels are opaque.
func (m *Summary) Shadow(labels []int) []int {
	indices := m.indices()
	s := newState(len(m.names))
	next := 0
	unknown := false
	for i, label := range labels {
		next = max(next, label)
		unknown = unknown || label == 0
		for k := range i {
			if label > 0 && labels[k] == label {
				s.union(location(indices[k]), location(indices[i]))
			}
		}
	}
	if unknown {
		for j := range s.groups {
			s.union(0, location(j))
		}
	}
	actuals := make([]root, len(m.names)-1)
	for j := range actuals {
		actuals[j] = root(j + 1)
	}
	m.apply(s, actuals)
	output := make([]int, len(labels))
	replaced := make(map[location]int)
	for i, j := range indices {
		g := s.groups[j]
		keep := labels[i] > 0 && !s.changed[j]
		for k, other := range indices {
			if s.groups[other] == g && (labels[k] != labels[i] || s.changed[other]) {
				keep = false
			}
		}
		switch {
		case labels[i] == 0:
			output[i] = 0
		case keep:
			output[i] = labels[i]
		default:
			if replaced[g] == 0 {
				next++
				replaced[g] = next
			}
			output[i] = replaced[g]
		}
	}
	return output
}

// Observes lists the inputs whose lock bookkeeping the procedure may observe.
func (m *Summary) Observes() []string {
	var names []string
	for i, j := range m.indices() {
		if m.observed[j] {
			names = append(names, m.Names()[i])
		}
	}
	return names
}

// Analysis holds identity flow graphs and converged summaries for a program.
// It does not mutate the IR; recompute it after transforming the program.
type Analysis struct {
	Program         *ir.Program
	graphs          []*Graph
	summaries       []*Summary
	effectives      []*Summary
	implementations [][]int
	termination     *Summary
	absolute        []map[*FlowNode]State // States from procedure entry, independent inputs.
}

// Analyze builds identity flow graphs for structurally valid IR and computes
// procedure summaries to a least fixed point over the call graph, including
// recursion, interface implementations, and Class_Terminate.
func Analyze(program *ir.Program) *Analysis {
	a := &Analysis{Program: program}
	a.implementations = make([][]int, len(program.Procedures))
	for _, class := range program.Classes {
		for _, iface := range class.Interfaces {
			for _, method := range iface.Methods {
				d, impl := int(method.Declaration)-1, int(method.Implementation)-1
				if d >= 0 && d < len(program.Procedures) && impl >= 0 && impl < len(program.Procedures) {
					a.implementations[d] = append(a.implementations[d], impl)
				}
			}
		}
	}
	for _, p := range program.Procedures {
		a.graphs = append(a.graphs, bindProcedure(program, p))
		a.summaries = append(a.summaries, emptySummary(p))
	}
	for changed := true; changed; {
		changed = false
		a.prepare()
		a.absolute = make([]map[*FlowNode]State, len(a.graphs))
		for i, g := range a.graphs {
			a.absolute[i] = a.solve(g, g.Start, newState(g.locations), nil, nil)
			if s := summarize(g.Procedure, activation(g, a.absolute[i])); !s.equal(a.summaries[i]) {
				a.summaries[i] = s
				changed = true
			}
		}
	}
	return a
}

// prepare snapshots effective summaries for one round of the fixed point.
// Interface contracts include every implementation, mapped by position.
func (a *Analysis) prepare() {
	a.effectives = make([]*Summary, len(a.summaries))
	for i, s := range a.summaries {
		e := s.clone()
		for _, impl := range a.implementations[i] {
			e.joinInto(a.summaries[impl])
		}
		a.effectives[i] = e
	}
	// Class_Terminate has only its private receiver, so a release can reach
	// ambient storage alone.
	a.termination = emptySummary(nil)
	for _, class := range a.Program.Classes {
		if t := int(class.Terminate) - 1; t >= 0 && t < len(a.summaries) {
			a.termination.joinInto(a.effectives[t])
		}
	}
}

func (a *Analysis) effective(i int) *Summary { return a.effectives[i] }

// Summary returns the converged summary of a procedure, including its
// interface implementations.
func (a *Analysis) Summary(p *ir.Procedure) *Summary {
	for i, q := range a.Program.Procedures {
		if q == p {
			return a.effectives[i]
		}
	}
	return nil
}

func (a *Analysis) procedureIndex(p *ir.Procedure) int {
	for i, q := range a.Program.Procedures {
		if q == p {
			return i
		}
	}
	return -1
}

// Graph returns a procedure's identity flow graph.
func (a *Analysis) Graph(p *ir.Procedure) *Graph {
	if i := a.procedureIndex(p); i >= 0 {
		return a.graphs[i]
	}
	return nil
}

// StateAt returns the state after flow relative to procedure entry, where all
// borrowed formals and ambient storage are independent. This is the state a
// summary is computed from.
func (a *Analysis) StateAt(p *ir.Procedure, flow *FlowNode) State {
	i := a.procedureIndex(p)
	if i < 0 {
		return State{}
	}
	return a.absolute[i][flow]
}

// Lifetime evaluates a reference lifetime: state at end relative to a new
// boundary. Inside a procedure body the caller is unknown, so borrowed formals
// and ambient storage may all depend on each other at entry.
func (a *Analysis) Lifetime(p *ir.Procedure, boundary, end *FlowNode) State {
	return a.lifetime(p, boundary, end, nil)
}

func (a *Analysis) lifetime(p *ir.Procedure, boundary, end *FlowNode, emulated func(*FlowNode) bool) State {
	i := a.procedureIndex(p)
	if i < 0 || boundary == nil || end == nil {
		return State{}
	}
	at := a.absolute[i][boundary]
	if !at.Reachable() {
		return State{}
	}
	initial := newState(len(at.groups))
	copy(initial.groups, at.groups)
	for _, id := range p.Parameters {
		if s := p.Slots[id-1]; s.ByRef || s.ParamArray {
			initial.union(ambient, id)
		}
	}
	return a.solve(a.graphs[i], boundary, initial, end, emulated)[end]
}

// LockRemovable reports whether the element lock acquired by token, the Try
// of an OpArrayIndex in statement, can be omitted. Its lifetime runs to its
// first explicit release or to the region's cleanup. The lock must not be
// needed for identity retention, and no operation in its lifetime may observe
// the lock count. Potential invalidations count even where the lock would
// have prevented them with error 10.
func (a *Analysis) LockRemovable(p *ir.Procedure, s *ir.Statement, token ir.ValueID) bool {
	return a.LockRemovableExcept(p, s, token, nil)
}

// LockRemovableExcept is LockRemovable, ignoring lock-count observations by
// the instructions of s for which emulated returns true. The consumer must
// reproduce those instructions' saturation checks as if the lock were held.
func (a *Analysis) LockRemovableExcept(p *ir.Procedure, s *ir.Statement, token ir.ValueID, emulated func(index int) bool) bool {
	g := a.Graph(p)
	if g == nil || s.ID == 0 || int(s.ID) > len(g.after) {
		return false
	}
	boundary := g.Acquisition(s.ID, token)
	r, ok := g.roots[s.ID-1][token]
	if boundary == nil || !ok || r < 0 {
		return false
	}
	end := g.End(s.ID)
	for i := boundary.Instruction + 1; i < len(s.Instructions); i++ {
		if in := s.Instructions[i]; in.Op == ir.OpReleaseElement && len(in.Args) == 1 && in.Args[0] == token {
			end = g.After(s.ID, i)
			break
		}
	}
	var filter func(*FlowNode) bool
	if emulated != nil {
		filter = func(n *FlowNode) bool {
			return n.Statement == s.ID && n.Instruction >= 0 && n.Flags&FlowShadow != 0 && emulated(n.Instruction)
		}
	}
	state := a.lifetime(p, boundary, end, filter)
	return state.Reachable() && state.Retained(location(r)) && !state.Observed(location(r))
}

func (g *Graph) slotLocation(slot ir.SlotID) (location, bool) {
	p := g.Procedure
	if slot == 0 || int(slot) > len(p.Slots) {
		return 0, false
	}
	if p.Slots[slot-1].Kind == ir.GlobalAlias {
		return ambient, true
	}
	return slot, true
}

// Invalidates reports whether instruction index of s may invalidate the
// storage identity of slot, including through aliases, callees, and releases.
// Inside a procedure body, borrowed and ambient storage may all alias.
func (a *Analysis) Invalidates(p *ir.Procedure, s *ir.Statement, index int, slot ir.SlotID) bool {
	g := a.Graph(p)
	if g == nil || s.ID == 0 || int(s.ID) > len(g.after) || index < 0 || index >= len(s.Instructions) {
		return true
	}
	l, ok := g.slotLocation(slot)
	if !ok {
		return true
	}
	end, boundary := g.after[s.ID-1][index], g.entries[s.ID-1]
	if index > 0 {
		boundary = g.after[s.ID-1][index-1]
	}
	if end == boundary {
		return false
	}
	state := a.Lifetime(p, boundary, end)
	return state.Reachable() && !state.Retained(l)
}

// ReleaseInvalidates reports whether releasing owned values (at region ends,
// scope exits, and failures) may invalidate slot through Class_Terminate.
// Terminate code reaches ambient storage only, which borrowed slots may alias.
func (a *Analysis) ReleaseInvalidates(p *ir.Procedure, slot ir.SlotID) bool {
	if !a.termination.changed[ambient] {
		return false
	}
	if slot == 0 || int(slot) > len(p.Slots) {
		return true
	}
	s := p.Slots[slot-1]
	return s.Kind == ir.GlobalAlias || s.ByRef || s.ParamArray
}

// Format lists each procedure's summary for independent inputs, in the
// integer-label notation of README.md.
func (a *Analysis) Format() string {
	var out strings.Builder
	for i, p := range a.Program.Procedures {
		m := a.effectives[i]
		names := m.Names()
		labels := make([]int, len(names))
		for j := range labels {
			labels[j] = j + 1
		}
		output := m.Shadow(labels)
		fmt.Fprintf(&out, "%s%s = %s", p.Name, formatLabels(names, labels), formatLabels(names, output))
		if observes := m.Observes(); len(observes) > 0 {
			fmt.Fprintf(&out, " observes %s", strings.Join(observes, ", "))
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func formatLabels(names []string, labels []int) string {
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = fmt.Sprintf("%s: %d", name, labels[i])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
