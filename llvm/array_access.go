package llvm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/ir/identity"
)

// An unlocked token has no element memory or cleanup. Its lock was either
// proven unnecessary or is virtual: accesses that would observe it add it to
// their saturation check instead.
type arrayTokenPlan struct {
	unlocked bool
}

// An arraySnapshot caches one typed array's descriptor pointer, data pointer,
// rank, lock count, and leading bounds in a private alloca. SROA promotes it to
// registers, so element stores cannot clobber it and LLVM can hoist and fold the
// checks. Identity retention analysis decides where it must be reloaded.
type arraySnapshot struct {
	slot   ir.SlotID
	fields []snapshotField // UDT field chain from the slot to the array.
	rank   int             // Stored bounds; at least every index rank through it.
	name   string
}

type snapshotField struct {
	record string
	index  uint32
}

type arrayAccessPlan struct {
	snapshot int
	depth    int // Virtual locks on the same array live at this access.
}

type snapshotRefresh struct {
	snapshot int
	full     bool // Reload the shape, not only the lock count.
}

type snapshotPoint struct {
	statement ir.StatementID
	index     int
}

func nativeArrayElementSize(t ir.Type) int {
	switch t.Scalar {
	case ir.Byte:
		return 1
	case ir.Boolean, ir.Integer:
		return 2
	case ir.Long, ir.Single:
		return 4
	case ir.Double, ir.Date, ir.Currency:
		return 8
	}
	return 0
}

// Identity retention analysis proves snapshot validity and unnecessary element
// locks. It is computed only for programs with inline candidates.
func (m *module) identityAnalysis() *identity.Analysis {
	if m.identity == nil {
		m.identity = identity.Analyze(m.program)
	}
	return m.identity
}

type inlineAccess struct {
	index    int // OpArrayIndex position; its Try follows immediately.
	snapshot int
	token    ir.ValueID
	end      int // First explicit release of the token, or the region's end.
}

// planArrayAccess selects typed scalar array accesses for direct access through
// snapshots, assigns each element lock a mode, and places snapshot reloads.
// The typed storage invariant holds for every root: ByRef arrays bind only to
// storage of exactly their declared type.
func (f *function) planArrayAccess() {
	f.arrayAccess = make(map[ir.ValueID]arrayAccessPlan)
	f.arrayTokens = make(map[ir.ValueID]arrayTokenPlan)
	f.indexTokens = make(map[ir.ValueID]ir.ValueID)
	f.elementPointers = make(map[ir.ValueID]string)
	f.scalarElementMemory = make(map[string]bool)
	f.entryRefresh = make(map[ir.StatementID][]snapshotRefresh)
	f.afterRefresh = make(map[snapshotPoint][]snapshotRefresh)
	keys := make(map[string]int)
	accesses := make(map[ir.StatementID][]inlineAccess)
	for _, s := range f.procedure.Statements {
		paths := make(map[ir.ValueID]arraySnapshot)
		types := make(map[ir.ValueID]ir.Type)
		for i, in := range s.Instructions {
			types[in.ID] = in.Type
			switch in.Op {
			case ir.OpAddress:
				paths[in.ID] = arraySnapshot{slot: in.Slot}
			case ir.OpFieldRef:
				if base, ok := paths[in.Args[0]]; ok && types[in.Args[0]].Form == ir.Reference {
					base.fields = append(append([]snapshotField(nil), base.fields...), snapshotField{scalar(types[in.Args[0]]), in.Field})
					paths[in.ID] = base
				}
			}
			if in.Op != ir.OpArrayBound && in.Op != ir.OpArrayIndex {
				continue
			}
			path, ok := paths[in.Args[0]]
			if t := types[in.Args[0]]; !ok || !ir.Array(t) || t.Form != ir.Reference || path.slot == 0 || f.procedure.Slots[path.slot-1].ParamArray {
				continue
			}
			rank := 1
			if in.Op == ir.OpArrayIndex {
				if nativeArrayElementSize(ir.ElementType(in.Type)) == 0 {
					continue // Managed elements retain their ownership/proxy runtime.
				}
				rank = len(in.Args) - 1
			}
			key := strconv.Itoa(int(path.slot))
			for _, field := range path.fields {
				key += "." + strconv.Itoa(int(field.index))
			}
			k, ok := keys[key]
			if !ok {
				k = len(f.arraySnapshots)
				keys[key] = k
				path.name = fmt.Sprintf("%%snapshot%d", k)
				f.arraySnapshots = append(f.arraySnapshots, &path)
			}
			f.arraySnapshots[k].rank = max(f.arraySnapshots[k].rank, rank)
			f.arrayAccess[in.ID] = arrayAccessPlan{snapshot: k}
			if in.Op == ir.OpArrayIndex {
				token := s.Instructions[i+1].ID // Verified immediate Try.
				f.indexTokens[in.ID] = token
				accesses[s.ID] = append(accesses[s.ID], inlineAccess{index: i, snapshot: k, token: token, end: len(s.Instructions)})
			}
		}
	}
	if len(f.arraySnapshots) == 0 {
		return
	}
	for _, s := range f.procedure.Statements {
		f.planArrayLocks(s, accesses[s.ID])
	}
	f.planSnapshotRefresh(accesses)
	for _, snapshot := range f.arraySnapshots {
		f.module.snapshotRanks[snapshot.rank] = true
		f.line("%s = alloca %%vba.array.snapshot.%d", snapshot.name, snapshot.rank)
		f.line("store %%vba.array.snapshot.%d zeroinitializer, ptr %s", snapshot.rank, snapshot.name)
	}
}

// planArrayLocks assigns each inline token a lock mode. A lock is omitted when
// identity analysis proves it unobservable. It is virtual when its only
// observers are inline accesses to the same array, which then check saturation
// against the snapshot's lock count plus the virtual locks live at them. The
// array's identity must still be retained, so no ReDim can see a missing lock.
func (f *function) planArrayLocks(s *ir.Statement, accesses []inlineAccess) {
	if len(accesses) == 0 {
		return
	}
	analysis := f.module.identityAnalysis()
	// An unlocked token may only yield its cell pointer and be released,
	// never be moved into scope storage.
	references := make(map[ir.ValueID]bool)
	for _, in := range s.Instructions {
		if in.Op == ir.OpTry && ir.Element(in.Type) {
			references[in.ID] = true
		}
	}
	for _, in := range s.Instructions {
		if in.Op != ir.OpElementRef && in.Op != ir.OpReleaseElement {
			for _, id := range in.Args {
				delete(references, id)
			}
		}
	}
	snapshotAt := make(map[int]int)
	for i := range accesses {
		a := &accesses[i]
		snapshotAt[a.index] = a.snapshot
		for j := a.index + 2; j < len(s.Instructions); j++ {
			if in := s.Instructions[j]; in.Op == ir.OpReleaseElement && in.Args[0] == a.token {
				a.end = j
				break
			}
		}
	}
	const (
		real = iota
		unlocked
		virtual
	)
	modes := make([]int, len(accesses))
	for i, a := range accesses {
		switch {
		case !references[a.token]:
		case analysis.LockRemovable(f.procedure, s, a.token):
			modes[i] = unlocked
		case analysis.LockRemovableExcept(f.procedure, s, a.token, func(j int) bool {
			k, ok := snapshotAt[j]
			return ok && k == a.snapshot
		}):
			modes[i] = virtual
		}
	}
	// A real acquisition would see the true lock count, without the virtual
	// locks around it, so a virtual lock cannot enclose one.
	for changed := true; changed; {
		changed = false
		for i, t := range accesses {
			for j, x := range accesses {
				if modes[i] == virtual && modes[j] == real && x.snapshot == t.snapshot && x.index > t.index+1 && x.index < t.end {
					modes[i], changed = real, true
				}
			}
		}
	}
	for i, x := range accesses {
		depth := 0
		for j, t := range accesses {
			if modes[j] == virtual && t.snapshot == x.snapshot && x.index > t.index+1 && x.index < t.end {
				depth++
			}
		}
		f.arrayAccess[s.Instructions[x.index].ID] = arrayAccessPlan{snapshot: x.snapshot, depth: depth}
		f.arrayTokens[x.token] = arrayTokenPlan{unlocked: modes[i] != real}
	}
}

type freshness struct{ shape, locks bool }

var fresh = freshness{true, true}

func (a freshness) and(b freshness) freshness {
	return freshness{a.shape && b.shape, a.locks && b.locks}
}

type regionFreshness struct {
	end      freshness
	failures map[int]freshness
	entry    *snapshotRefresh
	after    map[int]snapshotRefresh
}

// planSnapshotRefresh places reloads so every inline access sees a current
// snapshot. The prologue loads every snapshot. After an instruction that may
// invalidate the array or change a lock count, the snapshot is reloaded on the
// success path. A region reloads on entry when some incoming edge, including
// error, Resume, and GoSub edges, may carry a stale snapshot. This is a must-
// analysis over the region graph, so loops without such events reload nothing.
func (f *function) planSnapshotRefresh(accesses map[ir.StatementID][]inlineAccess) {
	p := f.procedure
	if len(p.Statements) == 0 {
		return
	}
	analysis := f.module.identityAnalysis()
	edges := analysis.Graph(p).Edges
	scopes := false
	for _, scope := range p.Scopes {
		scopes = scopes || len(scope.Cleanup) > 0
	}
	// Real lock acquisitions and releases change the count; region cleanup
	// releases on every exit, and leaving a resource scope releases its locks.
	lockEvents := make(map[snapshotPoint]bool)
	endLockStale := make(map[ir.StatementID]bool)
	for _, s := range p.Statements {
		locks := false
		for i, in := range s.Instructions {
			event := in.Op == ir.OpTry && ir.Element(in.Type) && !f.arrayTokens[in.ID].unlocked ||
				in.Op == ir.OpReleaseElement && !f.arrayTokens[in.Args[0]].unlocked ||
				in.Op == ir.OpEnumStart || in.Op == ir.OpInvocationCapture
			lockEvents[snapshotPoint{s.ID, i}] = event
			locks = locks || event
		}
		endLockStale[s.ID] = locks || scopes && (s.Scope != 0 || s.End == ir.Resume)
	}
	successors := make(map[ir.StatementID][]ir.StatementID)
	predecessors := make(map[ir.StatementID][]identity.Edge)
	for _, e := range edges {
		successors[e.From] = append(successors[e.From], e.To)
		predecessors[e.To] = append(predecessors[e.To], e)
	}
	first := p.Statements[0].ID
	reachable := map[ir.StatementID]bool{first: true}
	for queue := []ir.StatementID{first}; len(queue) > 0; queue = queue[1:] {
		for _, next := range successors[queue[0]] {
			if !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}
	for k, snapshot := range f.arraySnapshots {
		released := analysis.ReleaseInvalidates(p, snapshot.slot)
		uses := make(map[ir.StatementID]bool)
		for id, list := range accesses {
			for _, a := range list {
				uses[id] = uses[id] || a.snapshot == k
			}
		}
		for _, s := range p.Statements {
			for _, in := range s.Instructions {
				if plan, ok := f.arrayAccess[in.ID]; ok && plan.snapshot == k {
					uses[s.ID] = true
				}
			}
		}
		invalidates := make(map[snapshotPoint]bool)
		for _, s := range p.Statements {
			for i := range s.Instructions {
				invalidates[snapshotPoint{s.ID, i}] = analysis.Invalidates(p, s, i, snapshot.slot)
			}
		}
		transfer := func(s *ir.Statement, in freshness) regionFreshness {
			r := regionFreshness{failures: make(map[int]freshness), after: make(map[int]snapshotRefresh)}
			st := in
			if uses[s.ID] && st != fresh {
				r.entry = &snapshotRefresh{snapshot: k, full: !st.shape}
				st = fresh
			}
			pending := false
			for i, ins := range s.Instructions {
				point := snapshotPoint{s.ID, i}
				if ins.Op == ir.OpTry {
					failed := st
					failed.locks = failed.locks && !endLockStale[s.ID]
					failed.shape = failed.shape && !released
					r.failures[i] = failed
				}
				if invalidates[point] {
					st, pending = freshness{}, true
				}
				if lockEvents[point] {
					st.locks, pending = false, true
				}
				if pending && ins.Type.Form != ir.Result { // Reload after the Try.
					r.after[i] = snapshotRefresh{snapshot: k, full: !st.shape}
					st, pending = fresh, false
				}
			}
			r.end = st
			r.end.locks = r.end.locks && !endLockStale[s.ID]
			r.end.shape = r.end.shape && !released
			return r
		}
		in := make(map[ir.StatementID]freshness)
		for _, s := range p.Statements {
			in[s.ID] = fresh
			if !reachable[s.ID] {
				in[s.ID] = freshness{} // Never trust edges the analysis did not see.
			}
		}
		for changed := true; changed; {
			changed = false
			out := make(map[ir.StatementID]regionFreshness)
			for _, s := range p.Statements {
				out[s.ID] = transfer(s, in[s.ID])
			}
			for _, s := range p.Statements {
				if !reachable[s.ID] {
					continue
				}
				state := fresh // The prologue loads every snapshot before the first region.
				for _, e := range predecessors[s.ID] {
					if e.Failure < 0 {
						state = state.and(out[e.From].end)
					} else if failed, ok := out[e.From].failures[e.Failure]; ok {
						state = state.and(failed)
					} else {
						state = freshness{}
					}
				}
				if state != in[s.ID] {
					in[s.ID], changed = state, true
				}
			}
		}
		for _, s := range p.Statements {
			r := transfer(s, in[s.ID])
			if r.entry != nil {
				f.entryRefresh[s.ID] = append(f.entryRefresh[s.ID], *r.entry)
			}
			for i, refresh := range r.after {
				point := snapshotPoint{s.ID, i}
				f.afterRefresh[point] = append(f.afterRefresh[point], refresh)
			}
		}
	}
}

func (f *function) refreshSnapshot(r snapshotRefresh) {
	snapshot := f.arraySnapshots[r.snapshot]
	if !r.full {
		f.module.snapshotLocks[snapshot.rank] = true
		f.line("call void @vba.array.snapshot.locks.%d(ptr %s)", snapshot.rank, snapshot.name)
		return
	}
	storage := f.slots[snapshot.slot]
	for _, field := range snapshot.fields {
		f.snapshotFields++
		name := fmt.Sprintf("%%snapshot.field%d", f.snapshotFields)
		f.line("%s = getelementptr %s, ptr %s, i32 0, i32 %d", name, field.record, storage, field.index)
		storage = name
	}
	f.line("call void @vba.array.snapshot.%d(ptr %s, ptr %s)", snapshot.rank, storage, snapshot.name)
}

func (f *function) inlineAccess(id ir.ValueID) bool {
	_, ok := f.arrayAccess[id]
	return ok
}

func (f *function) refreshAllSnapshots() {
	for k := range f.arraySnapshots {
		f.refreshSnapshot(snapshotRefresh{snapshot: k, full: true})
	}
}

func (f *function) inlineArrayBound(in ir.Instruction, args []operand) {
	snapshot := f.arraySnapshots[f.arrayAccess[in.ID].snapshot]
	f.module.snapshotBounds[snapshot.rank] = true
	f.line("%%array.bound.result%d = call { i32, i32 } @vba.array.snapshot.bound.%d(ptr %s, i32 %s, i32 %s)", in.ID, snapshot.rank, snapshot.name, args[1].text, args[2].text)
	f.line("%s = extractvalue { i32, i32 } %%array.bound.result%d, 0", valueName(in.ID), in.ID)
	f.line("%%array.bound.value%d = extractvalue { i32, i32 } %%array.bound.result%d, 1", in.ID, in.ID)
	f.line("%%array.bound.failed%d = icmp ne i32 %s, 0", in.ID, valueName(in.ID))
	f.results[in.ID] = result{condition: fmt.Sprintf("%%array.bound.failed%d", in.ID), status: valueName(in.ID), value: fmt.Sprintf("%%array.bound.value%d", in.ID), direct: true, recordError: true, source: in.Source}
}

func (f *function) inlineArrayIndex(in ir.Instruction, args []operand) {
	plan := f.arrayAccess[in.ID]
	snapshot := f.arraySnapshots[plan.snapshot]
	rank := len(args) - 1
	f.module.snapshotIndexes[[2]int{rank, snapshot.rank}] = true
	token := f.indexTokens[in.ID]
	unlocked := f.arrayTokens[token].unlocked
	parameters := []string{"ptr " + snapshot.name, fmt.Sprintf("i64 %d", nativeArrayElementSize(ir.ElementType(in.Type))), fmt.Sprintf("i1 %t", !unlocked), fmt.Sprintf("i32 %d", plan.depth)}
	for _, arg := range args[1:] {
		parameters = append(parameters, "i32 "+arg.text)
	}
	f.line("%%array.index.result%d = call { i32, ptr } @vba.array.snapshot.index.%d.%d(%s)", in.ID, rank, snapshot.rank, strings.Join(parameters, ", "))
	f.line("%s = extractvalue { i32, ptr } %%array.index.result%d, 0", valueName(in.ID), in.ID)
	pointer := fmt.Sprintf("%%array.data%d", in.ID)
	f.line("%s = extractvalue { i32, ptr } %%array.index.result%d, 1", pointer, in.ID)
	f.elementPointers[token] = pointer
	f.line("%%array.failed%d = icmp ne i32 %s, 0", in.ID, valueName(in.ID))
	r := result{condition: fmt.Sprintf("%%array.failed%d", in.ID), status: valueName(in.ID), recordError: true, source: in.Source}
	if unlocked {
		r.direct, r.value = true, "zeroinitializer"
	} else {
		// This token is only used through a statically typed scalar reference:
		// it needs the original descriptor lock and cell pointer, not a Variant
		// view or String proxy. It remains compatible with ordinary cleanup.
		f.line("%%array.desc%d = load ptr, ptr %s", in.ID, snapshot.name)
		f.line("%%array.locked%d = select i1 %%array.failed%d, ptr null, ptr %%array.desc%d", in.ID, in.ID, in.ID)
		f.line("%%array.token%d = insertvalue %%vba.element zeroinitializer, ptr %%array.locked%d, 0", in.ID, in.ID)
		f.line("%%array.cell%d = insertvalue %%vba.element %%array.token%d, ptr %s, 1", in.ID, in.ID, pointer)
		f.line("store %%vba.element %%array.cell%d, ptr %%call%d, align 1", in.ID, in.ID)
		r.callSlot = fmt.Sprintf("%%call%d", in.ID)
	}
	f.results[in.ID] = r
}

func sortedRanks(set map[int]bool) []int {
	ranks := make([]int, 0, len(set))
	for rank := range set {
		ranks = append(ranks, rank)
	}
	sort.Ints(ranks)
	return ranks
}

func (m *module) arrayAccessHelpers() string {
	var out strings.Builder
	if len(m.snapshotRanks) > 0 {
		out.WriteString("%vba.array.bound = type { i32, i32 }\n%vba.array = type { i16, i16, i32, i32, ptr, [0 x %vba.array.bound] }\n")
	}
	// Snapshot fields: descriptor, data, rank, lock count, leading bounds in
	// descriptor order (the last subscript's bound first).
	for _, m := range sortedRanks(m.snapshotRanks) {
		fmt.Fprintf(&out, "%%vba.array.snapshot.%d = type { ptr, ptr, i32, i32, [%d x %%vba.array.bound] }\n", m, m)
		fmt.Fprintf(&out, "define internal void @vba.array.snapshot.%d(ptr %%storage, ptr %%snapshot) alwaysinline nounwind {\nentry:\n", m)
		fmt.Fprintf(&out, "  %%array = load ptr, ptr %%storage, align 1\n  store ptr %%array, ptr %%snapshot\n  %%rank.slot = getelementptr %%vba.array.snapshot.%d, ptr %%snapshot, i32 0, i32 2\n", m)
		out.WriteString("  %null = icmp eq ptr %array, null\n  br i1 %null, label %empty, label %fill\nempty:\n  store i32 0, ptr %rank.slot\n  ret void\nfill:\n")
		out.WriteString("  %rank.ptr = getelementptr %vba.array, ptr %array, i32 0, i32 0\n  %rank16 = load i16, ptr %rank.ptr, align 1\n  %rank = zext i16 %rank16 to i32\n  store i32 %rank, ptr %rank.slot\n")
		fmt.Fprintf(&out, "  %%data.ptr = getelementptr %%vba.array, ptr %%array, i32 0, i32 4\n  %%data = load ptr, ptr %%data.ptr, align 1\n  %%data.slot = getelementptr %%vba.array.snapshot.%d, ptr %%snapshot, i32 0, i32 1\n  store ptr %%data, ptr %%data.slot\n", m)
		fmt.Fprintf(&out, "  %%locks.ptr = getelementptr %%vba.array, ptr %%array, i32 0, i32 3\n  %%locks = load i32, ptr %%locks.ptr, align 1\n  %%locks.slot = getelementptr %%vba.array.snapshot.%d, ptr %%snapshot, i32 0, i32 3\n  store i32 %%locks, ptr %%locks.slot\n  br label %%dim0\n", m)
		for k := 0; k < m; k++ {
			fmt.Fprintf(&out, "dim%d:\n  %%has%d = icmp ugt i32 %%rank, %d\n  br i1 %%has%d, label %%load%d, label %%done\nload%d:\n", k, k, k, k, k, k)
			for field, name := range []string{"count", "lower"} {
				fmt.Fprintf(&out, "  %%%s.ptr%d = getelementptr %%vba.array, ptr %%array, i32 0, i32 5, i64 %d, i32 %d\n  %%%s%d = load i32, ptr %%%s.ptr%d, align 1\n", name, k, k, field, name, k, name, k)
				fmt.Fprintf(&out, "  %%%s.slot%d = getelementptr %%vba.array.snapshot.%d, ptr %%snapshot, i32 0, i32 4, i64 %d, i32 %d\n  store i32 %%%s%d, ptr %%%s.slot%d\n", name, k, m, k, field, name, k, name, k)
			}
			next := fmt.Sprintf("dim%d", k+1)
			if k+1 == m {
				next = "done"
			}
			fmt.Fprintf(&out, "  br label %%%s\n", next)
		}
		out.WriteString("done:\n  ret void\n}\n")
	}
	for _, m := range sortedRanks(m.snapshotLocks) {
		fmt.Fprintf(&out, "define internal void @vba.array.snapshot.locks.%d(ptr %%snapshot) alwaysinline nounwind {\nentry:\n", m)
		out.WriteString("  %array = load ptr, ptr %snapshot\n  %null = icmp eq ptr %array, null\n  br i1 %null, label %done, label %load\nload:\n")
		fmt.Fprintf(&out, "  %%locks.ptr = getelementptr %%vba.array, ptr %%array, i32 0, i32 3\n  %%locks = load i32, ptr %%locks.ptr, align 1\n  %%locks.slot = getelementptr %%vba.array.snapshot.%d, ptr %%snapshot, i32 0, i32 3\n  store i32 %%locks, ptr %%locks.slot\n  br label %%done\ndone:\n  ret void\n}\n", m)
	}
	indexes := make([][2]int, 0, len(m.snapshotIndexes))
	for key := range m.snapshotIndexes {
		indexes = append(indexes, key)
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i][0] < indexes[j][0] || indexes[i][0] == indexes[j][0] && indexes[i][1] < indexes[j][1]
	})
	for _, key := range indexes {
		out.WriteString(snapshotIndexHelper(key[0], key[1]))
	}
	for _, m := range sortedRanks(m.snapshotBounds) {
		out.WriteString(snapshotBoundHelper(m))
	}
	if m.scalarElementClear {
		out.WriteString(`define internal void @vba.element.scalar.clear(ptr %element) alwaysinline nounwind {
entry:
  %array.ptr = getelementptr %vba.element, ptr %element, i32 0, i32 0
  %array = load ptr, ptr %array.ptr, align 1
  %empty = icmp eq ptr %array, null
  br i1 %empty, label %done, label %unlock
unlock:
  call void @vba_array_unlock(ptr %array)
  store %vba.element zeroinitializer, ptr %element, align 1
  br label %done
done:
  ret void
}
`)
	}
	return out.String()
}

// snapshotIndexHelper checks allocation, rank, subscripts, and lock saturation
// against a snapshot of at least rank stored bounds. With %lock it acquires the
// descriptor's real lock; otherwise the access would have seen the snapshot's
// count plus %depth virtual locks.
func snapshotIndexHelper(rank, stored int) string {
	var out strings.Builder
	snapshot := fmt.Sprintf("%%vba.array.snapshot.%d", stored)
	parameters := []string{"ptr %snapshot", "i64 %size", "i1 %lock", "i32 %depth"}
	for i := 0; i < rank; i++ {
		parameters = append(parameters, fmt.Sprintf("i32 %%i%d", i))
	}
	fmt.Fprintf(&out, "define internal { i32, ptr } @vba.array.snapshot.index.%d.%d(%s) alwaysinline nounwind {\nentry:\n", rank, stored, strings.Join(parameters, ", "))
	out.WriteString("  %array = load ptr, ptr %snapshot\n  %null = icmp eq ptr %array, null\n  br i1 %null, label %bounds.error, label %rank.check\nrank.check:\n")
	fmt.Fprintf(&out, "  %%rank.slot = getelementptr %s, ptr %%snapshot, i32 0, i32 2\n  %%rank = load i32, ptr %%rank.slot\n  %%rank.ok = icmp eq i32 %%rank, %d\n  br i1 %%rank.ok, label %%dim0, label %%bounds.error\n", snapshot, rank)
	offset, stride := "0", "1"
	for i := 0; i < rank; i++ {
		fmt.Fprintf(&out, "dim%d:\n  %%count.ptr%d = getelementptr %s, ptr %%snapshot, i32 0, i32 4, i64 %d, i32 0\n  %%lower.ptr%d = getelementptr %s, ptr %%snapshot, i32 0, i32 4, i64 %d, i32 1\n", i, i, snapshot, rank-i-1, i, snapshot, rank-i-1)
		fmt.Fprintf(&out, "  %%count%d = load i32, ptr %%count.ptr%d\n  %%lower%d = load i32, ptr %%lower.ptr%d\n  %%count.wide%d = zext i32 %%count%d to i64\n  %%lower.wide%d = sext i32 %%lower%d to i64\n  %%index.wide%d = sext i32 %%i%d to i64\n  %%delta%d = sub i64 %%index.wide%d, %%lower.wide%d\n  %%valid%d = icmp ult i64 %%delta%d, %%count.wide%d\n", i, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i)
		fmt.Fprintf(&out, "  %%part%d = mul i64 %%delta%d, %s\n  %%offset%d = add i64 %s, %%part%d\n  %%stride%d = mul i64 %s, %%count.wide%d\n", i, i, stride, i, offset, i, i, stride, i)
		next := fmt.Sprintf("dim%d", i+1)
		if i+1 == rank {
			next = "lock.select"
		}
		fmt.Fprintf(&out, "  br i1 %%valid%d, label %%%s, label %%bounds.error\n", i, next)
		offset, stride = fmt.Sprintf("%%offset%d", i), fmt.Sprintf("%%stride%d", i)
	}
	out.WriteString("lock.select:\n  br i1 %lock, label %acquire, label %check.counter\nacquire:\n  %status = call i32 @vba_array_lock(ptr %array)\n  %locked = icmp eq i32 %status, 0\n  br i1 %locked, label %success, label %lock.failed\nlock.failed:\n  %failure = insertvalue { i32, ptr } zeroinitializer, i32 %status, 0\n  ret { i32, ptr } %failure\ncheck.counter:\n")
	fmt.Fprintf(&out, "  %%locks.slot = getelementptr %s, ptr %%snapshot, i32 0, i32 3\n  %%locks = load i32, ptr %%locks.slot\n  %%held = add i32 %%locks, %%depth\n  %%full = icmp eq i32 %%held, -1\n  br i1 %%full, label %%lock.error, label %%success\nsuccess:\n", snapshot)
	fmt.Fprintf(&out, "  %%data.slot = getelementptr %s, ptr %%snapshot, i32 0, i32 1\n  %%data = load ptr, ptr %%data.slot\n  %%bytes = mul i64 %s, %%size\n", snapshot, offset)
	out.WriteString("  %cell = getelementptr i8, ptr %data, i64 %bytes\n  %result = insertvalue { i32, ptr } zeroinitializer, ptr %cell, 1\n  ret { i32, ptr } %result\nbounds.error:\n  ret { i32, ptr } { i32 9, ptr null }\nlock.error:\n  ret { i32, ptr } { i32 10, ptr null }\n}\n")
	return out.String()
}

// snapshotBoundHelper answers LBound/UBound from the stored bounds, selecting
// them by constant indices so they stay promotable. A dimension beyond the
// stored bounds reads the (current) descriptor.
func snapshotBoundHelper(stored int) string {
	var out strings.Builder
	snapshot := fmt.Sprintf("%%vba.array.snapshot.%d", stored)
	fmt.Fprintf(&out, "define internal { i32, i32 } @vba.array.snapshot.bound.%d(ptr %%snapshot, i32 %%dimension, i32 %%upper) alwaysinline nounwind {\nentry:\n", stored)
	out.WriteString("  %array = load ptr, ptr %snapshot\n  %null = icmp eq ptr %array, null\n  br i1 %null, label %bounds.error, label %dimension.check\ndimension.check:\n")
	fmt.Fprintf(&out, "  %%rank.slot = getelementptr %s, ptr %%snapshot, i32 0, i32 2\n  %%rank = load i32, ptr %%rank.slot\n", snapshot)
	out.WriteString("  %positive = icmp sgt i32 %dimension, 0\n  %within = icmp ule i32 %dimension, %rank\n  %valid = and i1 %positive, %within\n  br i1 %valid, label %value, label %bounds.error\nvalue:\n  %index = sub i32 %rank, %dimension\n")
	fmt.Fprintf(&out, "  %%cached = icmp ult i32 %%index, %d\n  br i1 %%cached, label %%stored, label %%descriptor\nstored:\n", stored)
	count, lower := "0", "0"
	for k := 0; k < stored; k++ {
		fmt.Fprintf(&out, "  %%count.ptr%d = getelementptr %s, ptr %%snapshot, i32 0, i32 4, i64 %d, i32 0\n  %%count%d = load i32, ptr %%count.ptr%d\n", k, snapshot, k, k, k)
		fmt.Fprintf(&out, "  %%lower.ptr%d = getelementptr %s, ptr %%snapshot, i32 0, i32 4, i64 %d, i32 1\n  %%lower%d = load i32, ptr %%lower.ptr%d\n", k, snapshot, k, k, k)
		fmt.Fprintf(&out, "  %%is%d = icmp eq i32 %%index, %d\n  %%count.select%d = select i1 %%is%d, i32 %%count%d, i32 %s\n  %%lower.select%d = select i1 %%is%d, i32 %%lower%d, i32 %s\n", k, k, k, k, k, count, k, k, k, lower)
		count, lower = fmt.Sprintf("%%count.select%d", k), fmt.Sprintf("%%lower.select%d", k)
	}
	out.WriteString("  br label %compute\ndescriptor:\n  %index.wide = zext i32 %index to i64\n  %count.desc.ptr = getelementptr %vba.array, ptr %array, i32 0, i32 5, i64 %index.wide, i32 0\n  %lower.desc.ptr = getelementptr %vba.array, ptr %array, i32 0, i32 5, i64 %index.wide, i32 1\n  %count.desc = load i32, ptr %count.desc.ptr, align 1\n  %lower.desc = load i32, ptr %lower.desc.ptr, align 1\n  br label %compute\ncompute:\n")
	fmt.Fprintf(&out, "  %%count = phi i32 [ %s, %%stored ], [ %%count.desc, %%descriptor ]\n  %%lower = phi i32 [ %s, %%stored ], [ %%lower.desc, %%descriptor ]\n", count, lower)
	out.WriteString(`  %count.wide = zext i32 %count to i64
  %lower.wide = sext i32 %lower to i64
  %last = sub i64 %count.wide, 1
  %upper.value = add i64 %lower.wide, %last
  %want.upper = icmp ne i32 %upper, 0
  %chosen = select i1 %want.upper, i64 %upper.value, i64 %lower.wide
  %low = icmp slt i64 %chosen, -2147483648
  %high = icmp sgt i64 %chosen, 2147483647
  %overflow = or i1 %low, %high
  br i1 %overflow, label %overflow.error, label %success
success:
  %narrow = trunc i64 %chosen to i32
  %result = insertvalue { i32, i32 } zeroinitializer, i32 %narrow, 1
  ret { i32, i32 } %result
bounds.error:
  ret { i32, i32 } { i32 9, i32 0 }
overflow.error:
  ret { i32, i32 } { i32 6, i32 0 }
}
`)
	return out.String()
}
