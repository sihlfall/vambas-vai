package identity

import "sort"

// solve computes the state after every node between boundary and end, in
// reverse post-order until nothing changes: the least fixed point of the shadow
// transfers. States are relative to the boundary, which keeps initial even when
// a loop reaches it again. Paths that do not start at the boundary contribute
// nothing. A nil end solves every node reachable from the boundary.
//
// The graph and queries follow typescript-go's flow nodes, but evaluation is
// forward. Its demand-driven loop evaluation re-iterates inner loops for every
// provisional outer approximation, which grows exponentially with nesting here.
func (a *Analysis) solve(g *Graph, boundary *FlowNode, initial State, end *FlowNode, emulated func(*FlowNode) bool) map[*FlowNode]State {
	region := g.order
	if end != nil {
		// Only nodes on some path from the boundary to end affect end.
		backward := make(map[*FlowNode]bool)
		for stack := []*FlowNode{end}; len(stack) > 0; {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if backward[n] || n.order < 0 {
				continue
			}
			backward[n] = true
			if n == boundary {
				continue
			}
			if n.Antecedent != nil {
				stack = append(stack, n.Antecedent)
			}
			stack = append(stack, n.Antecedents...)
		}
		region = nil
		seen := map[*FlowNode]bool{boundary: true}
		for stack := []*FlowNode{boundary}; len(stack) > 0; {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			region = append(region, n)
			for _, next := range n.successors {
				if backward[next] && !seen[next] {
					seen[next] = true
					stack = append(stack, next)
				}
			}
		}
		sort.Slice(region, func(i, j int) bool { return region[i].order < region[j].order })
	}
	states := make(map[*FlowNode]State, len(region))
	states[boundary] = initial
	for changed := true; changed; {
		changed = false
		for _, n := range region {
			if n == boundary {
				continue
			}
			var in State
			if n.Flags&FlowLabel != 0 {
				for _, antecedent := range n.Antecedents {
					in = join(in, states[antecedent])
				}
			} else if n.Antecedent != nil {
				in = states[n.Antecedent]
			}
			if out := a.apply(n, in, emulated); !out.equal(states[n]) {
				states[n], changed = out, true
			}
		}
	}
	return states
}

// activation joins every state an activation can reach: each region's end and
// the exit. Summaries cover effects during the call, not only on return, so a
// call that never returns still reports what it may have invalidated. Every
// node lies on a straight line to its region's end or to the exit.
func activation(g *Graph, states map[*FlowNode]State) State {
	s := states[g.Exit]
	for _, end := range g.ends {
		s = join(s, states[end])
	}
	return s
}

func (a *Analysis) apply(flow *FlowNode, s State, emulated func(*FlowNode) bool) State {
	if !s.Reachable() || flow.Flags&(FlowShadow|FlowCall|FlowRelease) == 0 {
		return s
	}
	s = s.clone()
	switch {
	case flow.Flags&FlowShadow != 0:
		for _, e := range flow.effects {
			switch e.kind {
			case invalidate:
				s.invalidate(e.location)
			case observe:
				if emulated == nil || !emulated(flow) {
					s.observe(e.location)
				}
			case bind:
				s.bind(location(e.location), e.target)
			}
		}
	case flow.Flags&FlowCall != 0:
		var summary *Summary
		if flow.callee >= 0 {
			summary = a.effective(flow.callee)
		}
		summary.apply(s, flow.actuals)
	case flow.Flags&FlowRelease != 0:
		a.termination.apply(s, nil)
	}
	return s
}
