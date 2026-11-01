package identity

import (
	"slices"

	"sihlfall.ch/vambas-vai/ir"
)

// A location is a tracked storage root. Location 0 is ambient storage: globals,
// object fields, and caller storage reachable outside this activation. Other
// locations are procedure SlotIDs; references, element tokens, and captured
// receivers belong to the group of the storage they were derived from.
type location = ir.SlotID

const ambient location = 0

// A root is a location, or one of two values that do not name storage.
type root int64

const (
	unknownRoot root = -1 // May alias any tracked storage.
	freshRoot   root = -2 // An independent value snapshot; no tracked storage.
)

// State is the finite fixed-point representation of the shadow state. groups
// is a partition by canonical (least) member; changed and observed are relative
// to the analysis boundary. A nil groups slice is the unreachable state.
type State struct {
	groups   []location
	changed  []bool // Lost its boundary retention guarantee.
	observed []bool // Its group's lock bookkeeping may have been observed.
}

func newState(size int) State {
	s := State{groups: make([]location, size), changed: make([]bool, size), observed: make([]bool, size)}
	for i := range s.groups {
		s.groups[i] = location(i)
	}
	return s
}

func (s State) Reachable() bool { return s.groups != nil }

// Retained reports whether l's boundary identity survived on every path.
func (s State) Retained(l location) bool { return !s.Reachable() || !s.changed[l] }

// Observed reports whether l's lock bookkeeping may have been observed.
func (s State) Observed(l location) bool { return s.Reachable() && s.observed[l] }

// Dependent reports whether invalidating one location may invalidate the other.
func (s State) Dependent(a, b location) bool {
	return s.Reachable() && s.groups[a] == s.groups[b]
}

func (s State) clone() State {
	if !s.Reachable() {
		return s
	}
	return State{groups: slices.Clone(s.groups), changed: slices.Clone(s.changed), observed: slices.Clone(s.observed)}
}

func (s State) equal(t State) bool {
	return slices.Equal(s.groups, t.groups) && slices.Equal(s.changed, t.changed) && slices.Equal(s.observed, t.observed)
}

// join is the branch merge: equivalence closure of both partitions, and the
// union of both changed and observed sets.
func join(a, b State) State {
	if !a.Reachable() {
		return b
	}
	if !b.Reachable() {
		return a
	}
	s := a.clone()
	for i, g := range b.groups {
		s.union(location(i), g)
	}
	for i := range s.changed {
		s.changed[i] = s.changed[i] || b.changed[i]
		s.observed[i] = s.observed[i] || b.observed[i]
	}
	return s
}

// union merges two groups in place, keeping representatives canonical.
func (s State) union(a, b location) {
	ga, gb := s.groups[a], s.groups[b]
	if ga == gb {
		return
	}
	keep, drop := min(ga, gb), max(ga, gb)
	for i, g := range s.groups {
		if g == drop {
			s.groups[i] = keep
		}
	}
}

func (s State) mark(set []bool, l location) {
	g := s.groups[l]
	for i, m := range s.groups {
		if m == g {
			set[i] = true
		}
	}
}

// The transfers below mutate a cloned state. An unknown root affects every
// location; a fresh root affects none.

func (s State) invalidate(r root) {
	switch {
	case r == unknownRoot:
		for i := range s.changed {
			s.changed[i] = true
		}
	case r >= 0:
		s.mark(s.changed, location(r))
	}
}

func (s State) observe(r root) {
	switch {
	case r == unknownRoot:
		for i := range s.observed {
			s.observed[i] = true
		}
	case r >= 0:
		s.mark(s.observed, location(r))
	}
}

// bind makes slot hold a reference into target's group. The slot's previous
// identity is gone; target's storage is not invalidated by acquiring a
// reference to it. The old group stays merged, which only loses independence.
func (s State) bind(slot location, target root) {
	s.changed[slot] = true
	switch {
	case target == unknownRoot:
		for i := range s.groups {
			s.union(slot, location(i))
		}
		s.invalidate(unknownRoot)
	case target >= 0:
		s.union(slot, location(target))
	}
}

// connect makes two previously independent groups potentially dependent. The
// merged group conservatively withdraws both groups' retention certificates.
func (s State) connect(a, b root) {
	if a == freshRoot || b == freshRoot {
		return
	}
	if a == unknownRoot || b == unknownRoot {
		s.invalidate(unknownRoot)
		return
	}
	s.union(location(a), location(b))
	s.mark(s.changed, location(a))
}
