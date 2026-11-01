package identity

import "sihlfall.ch/vambas-vai/ir"

// FlowFlags and the graph layout follow ast/flow.go (and so typescript-go):
// each node names its antecedent, and labels join several. The graph is
// derived from verified IR, not from binder flow nodes, because only the IR
// has error, Resume, GoSub, and cleanup edges.
type FlowFlags uint16

const (
	FlowStart       FlowFlags = 1 << iota // Procedure entry
	FlowBranchLabel                       // Non-looping junction
	FlowLoopLabel                         // Junction with a back edge
	FlowShadow                            // Invalidate, observe, or bind tracked storage
	FlowCall                              // Apply a callee summary to its actuals
	FlowRelease                           // Owned values may run Class_Terminate
	FlowAcquire                           // Element lock acquired; a lifetime boundary
	FlowLabel       = FlowBranchLabel | FlowLoopLabel
)

type effectKind uint8

const (
	invalidate effectKind = iota
	observe
	bind // Location becomes a reference into Target's group.
)

type effect struct {
	kind     effectKind
	location root
	target   root
}

type FlowNode struct {
	Flags       FlowFlags
	Antecedent  *FlowNode   // Antecedent for all but labels
	Antecedents []*FlowNode // Antecedents of a label
	Statement   ir.StatementID
	Instruction int // Index into the statement's instructions; -1 at region level.
	effects     []effect
	callee      int // Procedure slice index; -1 is an unsummarized callee.
	actuals     []root
	order       int         // Reverse post-order position from Start; -1 when unreachable.
	successors  []*FlowNode // Reachable successors.
}

// Edge is a region-level control transfer. Failure is the index of the failing
// instruction, or -1 when control leaves the end of From, after its cleanup.
type Edge struct {
	From, To ir.StatementID
	Failure  int
}

// Graph is one procedure's identity flow graph.
type Graph struct {
	Procedure *ir.Procedure
	Start     *FlowNode
	Exit      *FlowNode     // Normal and error exits, after activation cleanup.
	Edges     []Edge        // Every region-to-region transfer, including dynamic ones.
	after     [][]*FlowNode // Flow after each instruction of each statement.
	entries   []*FlowNode   // Region entry labels.
	ends      []*FlowNode   // Flow at each region's end, after its value cleanup.
	acquired  []map[ir.ValueID]*FlowNode
	roots     []map[ir.ValueID]root
	locations int
	order     []*FlowNode // Nodes reachable from Start, in reverse post-order.
}

// After returns the flow following instruction index of statement.
func (g *Graph) After(statement ir.StatementID, index int) *FlowNode {
	return g.after[statement-1][index]
}

// End returns the flow at a region's end, after its owned values are released.
func (g *Graph) End(statement ir.StatementID) *FlowNode { return g.ends[statement-1] }

// Acquisition returns the boundary node of an element token's lock lifetime.
func (g *Graph) Acquisition(statement ir.StatementID, token ir.ValueID) *FlowNode {
	return g.acquired[statement-1][token]
}

type binder struct {
	program     *ir.Program
	graph       *Graph
	procedure   *ir.Procedure
	current     *FlowNode
	entries     []*FlowNode
	errors      *FlowNode // Handler dispatch; nil when no policy can catch.
	failures    []Edge    // Failure points that reach handler dispatch.
	resumes     []*FlowNode
	resumeFrom  []ir.StatementID
	returns     []*FlowNode // GoSub Return sites.
	returnFrom  []ir.StatementID
	terminators bool // Releases can reenter through Class_Terminate.
	roots       map[ir.ValueID]root
	types       map[ir.ValueID]ir.Type
}

func bindProcedure(program *ir.Program, p *ir.Procedure) *Graph {
	b := &binder{program: program, procedure: p}
	for _, class := range program.Classes {
		b.terminators = b.terminators || class.Terminate != 0
	}
	n := len(p.Statements)
	b.graph = &Graph{Procedure: p, after: make([][]*FlowNode, n), ends: make([]*FlowNode, n), acquired: make([]map[ir.ValueID]*FlowNode, n), roots: make([]map[ir.ValueID]root, n), locations: len(p.Slots) + 1}
	b.graph.Start = &FlowNode{Flags: FlowStart, Instruction: -1}
	b.graph.Exit = &FlowNode{Flags: FlowBranchLabel, Instruction: -1}
	b.entries = make([]*FlowNode, n)
	b.graph.entries = b.entries
	for i, s := range p.Statements {
		b.entries[i] = &FlowNode{Flags: FlowBranchLabel, Statement: s.ID, Instruction: -1}
	}
	for _, s := range p.Statements {
		if s.ErrorMode != 0 || s.End == ir.Resume {
			b.errors = &FlowNode{Flags: FlowBranchLabel, Instruction: -1}
			break
		}
	}
	if n == 0 {
		b.addAntecedent(b.graph.Exit, b.graph.Start)
	} else {
		b.addAntecedent(b.entries[0], b.graph.Start)
	}
	for _, s := range p.Statements {
		b.bindStatement(s)
	}
	b.finishDynamicEdges()
	b.markLoops()
	return b.graph
}

func (b *binder) addAntecedent(label, antecedent *FlowNode) {
	for _, a := range label.Antecedents {
		if a == antecedent {
			return
		}
	}
	label.Antecedents = append(label.Antecedents, antecedent)
}

func (b *binder) newFlowNode(flags FlowFlags, s *ir.Statement, index int) *FlowNode {
	node := &FlowNode{Flags: flags, Antecedent: b.current, Statement: s.ID, Instruction: index, callee: -1}
	b.current = node
	return node
}

func (b *binder) shadow(s *ir.Statement, index int, effects ...effect) {
	if len(effects) > 0 {
		node := b.newFlowNode(FlowShadow, s, index)
		node.effects = effects
	}
}

// release models owned values going away. Freed storage was owned by the
// released value, so nothing else can still reference it; only Class_Terminate
// can reach other storage, and only ambient storage.
func (b *binder) release(s *ir.Statement, index int) {
	if b.terminators {
		b.newFlowNode(FlowRelease, s, index)
	}
}

func (b *binder) slotRoot(id ir.SlotID) root {
	if id == 0 || int(id) > len(b.procedure.Slots) {
		return unknownRoot
	}
	if b.procedure.Slots[id-1].Kind == ir.GlobalAlias {
		return root(ambient)
	}
	return root(id)
}

// root resolves a value's storage provenance. A reference or token without
// provenance may alias anything; other values are independent snapshots.
func (b *binder) root(id ir.ValueID) root {
	if r, ok := b.roots[id]; ok {
		return r
	}
	t := b.types[id]
	if t.Form == ir.Reference || ir.Element(t) || t.Scalar == ir.Receiver {
		return unknownRoot
	}
	return freshRoot
}

// shapeBearing storage can own a descriptor that a replacement may free.
func shapeBearing(t ir.Type) bool {
	return ir.Array(t) || t.Scalar == ir.Variant || t.Scalar == ir.Record || t.Scalar == ir.Receiver
}

// objectBearing values may release an object reference when replaced.
func objectBearing(t ir.Type) bool {
	switch t.Scalar {
	case ir.Object, ir.Variant, ir.Record, ir.ArrayObject, ir.ArrayVariant, ir.ArrayRecord,
		ir.ElementObject, ir.ElementVariant, ir.ElementRecord, ir.Enumerator:
		return true
	}
	return false
}

func replace(r root) []effect {
	return []effect{{kind: invalidate, location: r}, {kind: observe, location: r}}
}

// reentry models code that can run arbitrary public members: construction,
// erased dispatch, default members, and external calls. Such code reaches
// ambient storage, and this activation only through the references it is given.
func (b *binder) reentry(in ir.Instruction) []effect {
	effects := replace(root(ambient))
	for _, id := range in.Args {
		if t := b.types[id]; t.Form == ir.Reference {
			effects = append(effects, replace(b.root(id))...)
		}
	}
	return effects
}

func (b *binder) bindStatement(s *ir.Statement) {
	i := s.ID - 1
	b.current = b.entries[i]
	b.roots, b.types = make(map[ir.ValueID]root), make(map[ir.ValueID]ir.Type)
	b.graph.roots[i] = b.roots
	b.graph.acquired[i] = make(map[ir.ValueID]*FlowNode)
	b.graph.after[i] = make([]*FlowNode, len(s.Instructions))
	for index, in := range s.Instructions {
		if in.Op == ir.OpTry && in.OnError == ir.Propagate {
			b.fail(s, index)
		}
		b.types[in.ID] = in.Type
		b.bindInstruction(s, index, in)
		b.graph.after[i][index] = b.current
	}
	b.release(s, -1) // Owned values and argument temporaries.
	b.graph.ends[i] = b.current
	end := b.current
	if ir.ErrorPolicyEffects(s)&ir.MayRaise != 0 || s.End == ir.Return {
		b.fail(s, -1) // Resume without an active error, or a fallible result load.
	}
	jump := func(target ir.StatementID) {
		if target != 0 && int(target) <= len(b.entries) {
			b.current = end
			b.edge(s, b.procedure.Statements[target-1].Scope, s.Scope)
			b.addAntecedent(b.entries[target-1], b.current)
			b.graph.Edges = append(b.graph.Edges, Edge{From: s.ID, To: target, Failure: -1})
		}
	}
	switch s.End {
	case ir.Continue:
		jump(s.Next)
	case ir.Jump, ir.Gosub:
		jump(s.Target)
	case ir.Branch:
		jump(s.Target)
		jump(s.FalseTarget)
	case ir.Switch:
		jump(s.Target)
		for _, c := range s.Cases {
			jump(c.Target)
		}
	case ir.Return:
		b.current = end
		b.release(s, -1) // Activation-owned storage.
		b.addAntecedent(b.graph.Exit, b.current)
	case ir.Resume:
		if s.ResumeMode == 2 {
			jump(s.Target)
		} else {
			b.resumes = append(b.resumes, end) // Retry or Next of any failed region.
			b.resumeFrom = append(b.resumeFrom, s.ID)
		}
	case ir.GosubReturn:
		b.returns = append(b.returns, end)
		b.returnFrom = append(b.returnFrom, s.ID)
	}
}

// edge releases resource scopes left on the way to target's scope.
func (b *binder) edge(s *ir.Statement, to, from ir.ScopeID) {
	ancestors := map[ir.ScopeID]bool{0: true}
	for id := to; id != 0 && int(id) <= len(b.procedure.Scopes); id = b.procedure.Scopes[id-1].Parent {
		ancestors[id] = true
	}
	for id := from; !ancestors[id] && int(id) <= len(b.procedure.Scopes); id = b.procedure.Scopes[id-1].Parent {
		if len(b.procedure.Scopes[id-1].Cleanup) > 0 {
			b.release(s, -1)
			return
		}
	}
}

// fail routes an error leaving the current flow. The failed region's owned
// values are released first; the activation policy may then propagate, resume
// at the region's Next, or enter any handler.
func (b *binder) fail(s *ir.Statement, index int) {
	saved := b.current
	b.release(s, index) // Region owners, and activation owners on propagation.
	if b.errors == nil {
		b.addAntecedent(b.graph.Exit, b.current)
	} else {
		b.addAntecedent(b.errors, b.current)
		next := s.Next
		if next == 0 {
			next = b.procedure.Statements[len(b.procedure.Statements)-1].ID
		}
		b.addAntecedent(b.entries[next-1], b.current)
		failure := Edge{From: s.ID, To: next, Failure: index}
		b.graph.Edges = append(b.graph.Edges, failure)
		b.failures = append(b.failures, failure)
	}
	b.current = saved
}

func (b *binder) finishDynamicEdges() {
	p := b.procedure
	if b.errors != nil {
		b.current = b.errors
		b.release(&ir.Statement{}, -1)
		b.addAntecedent(b.graph.Exit, b.current)
		for _, s := range p.Statements {
			if s.ErrorMode == 2 && s.Handler != 0 && int(s.Handler) <= len(b.entries) {
				b.current = b.errors
				b.release(s, -1) // Leaving resource scopes is conservatively a release.
				b.addAntecedent(b.entries[s.Handler-1], b.current)
				for _, f := range b.failures {
					b.graph.Edges = append(b.graph.Edges, Edge{From: f.From, To: s.Handler, Failure: f.Failure})
				}
			}
		}
	}
	for i, resume := range b.resumes {
		for j, entry := range b.entries {
			b.current = resume
			b.release(&ir.Statement{}, -1)
			b.addAntecedent(entry, b.current)
			b.graph.Edges = append(b.graph.Edges, Edge{From: b.resumeFrom[i], To: p.Statements[j].ID, Failure: -1})
		}
	}
	for i, ret := range b.returns {
		for _, s := range p.Statements {
			if s.End == ir.Gosub && s.Next != 0 && int(s.Next) <= len(b.entries) {
				b.current = ret
				b.release(s, -1)
				b.addAntecedent(b.entries[s.Next-1], b.current)
				b.graph.Edges = append(b.graph.Edges, Edge{From: b.returnFrom[i], To: s.Next, Failure: -1})
			}
		}
	}
}

// markLoops flags labels entered by a depth-first back edge from Start. Every
// reachable cycle, including irreducible GoTo cycles, contains such a label.
// Unreachable antecedents are dropped: they contribute no state, and a dead
// cycle would otherwise have no loop label to stop backward evaluation.
func (b *binder) markLoops() {
	successors := make(map[*FlowNode][]*FlowNode)
	var nodes []*FlowNode
	seen := make(map[*FlowNode]bool)
	var collect func(*FlowNode)
	collect = func(n *FlowNode) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		nodes = append(nodes, n)
		if n.Antecedent != nil {
			successors[n.Antecedent] = append(successors[n.Antecedent], n)
			collect(n.Antecedent)
		}
		for _, a := range n.Antecedents {
			successors[a] = append(successors[a], n)
			collect(a)
		}
	}
	collect(b.graph.Exit)
	for _, e := range b.entries {
		collect(e)
	}
	const (
		white = iota
		grey
		black
	)
	color := make(map[*FlowNode]int)
	type frame struct {
		node *FlowNode
		next int
	}
	var post []*FlowNode
	stack := []frame{{node: b.graph.Start}}
	color[b.graph.Start] = grey
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next == len(successors[top.node]) {
			color[top.node] = black
			post = append(post, top.node)
			stack = stack[:len(stack)-1]
			continue
		}
		next := successors[top.node][top.next]
		top.next++
		switch color[next] {
		case white:
			color[next] = grey
			stack = append(stack, frame{node: next})
		case grey:
			if next.Flags&FlowLabel != 0 {
				next.Flags = next.Flags&^FlowBranchLabel | FlowLoopLabel
			}
		}
	}
	for _, n := range nodes {
		n.order = -1
	}
	for i := range post {
		n := post[len(post)-1-i]
		n.order = i
		n.successors = successors[n]
		b.graph.order = append(b.graph.order, n)
	}
	for _, n := range nodes {
		live := n.Antecedents[:0]
		for _, a := range n.Antecedents {
			if color[a] != white {
				live = append(live, a)
			}
		}
		n.Antecedents = live
	}
}
