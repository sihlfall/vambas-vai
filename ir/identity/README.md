# Identity retention analysis

`identity.Analyze(program)` accompanies verified semantic IR. It determines:

1. whether a storage identity is guaranteed to survive execution of some code;
2. whether two storage roots are provably independent: invalidating one cannot
   invalidate the other.

It does not change the IR and stores no claims in it. Consumers recompute it
from the program they lower, so transformations cannot leave stale results.
The LLVM backend uses it to keep
[array descriptor snapshots](../../llvm/ARRAYS.md#shape-snapshots) current and to
omit or virtualize element locks. The language server does not use it.

## Shadow states

A statement, region, or procedure is an abstract state transformer
`Ĉ : Σ → Σ`. For presentation, a state maps each tracked root to an integer label:

- roots with the same positive label may depend on the same storage;
- roots with different positive labels are provably independent;
- label 0 is unknown and gives no independence or retention guarantee.

Across a piece of code, an unchanged positive label certifies retention; a
replacement label withdraws it. Labels are opaque and only meaningful within
the analysis that produced them.

```text
Sub F(a() As Long, b() As Long)
    a(0) = 42
    Erase b
End Sub

F̂({a: 1, b: 2, ambient: 3}) = {a: 1, b: 4, ambient: 3}   independent inputs
F̂({a: 1, b: 1, ambient: 2}) = {a: 3, b: 3, ambient: 2}   dependent inputs
```

`Analysis.Format` prints every procedure this way; `Summary.Shadow` applies a
summary to any labelling.

A label is a dependency group. Every reference into an array belongs to the
array's group: element tokens, field and Variant views, captured With receivers,
enumerators, and ByRef actuals. Arrays owned through Variant or UDT fields share
their container's group. This is coarse: replacing an inner array reports the
whole container as possibly changed. Such false positives keep unnecessary locks
but never justify an unsafe removal.

## Tracked roots

The tracked roots are a procedure's slots, plus `ambient`: module globals, class
fields, and storage reachable outside this activation. Slot aliases of globals
and class field references resolve to `ambient`. A reference or token without
recorded provenance resolves to an unknown root, which affects every location.
Owned value snapshots are fresh and untracked.

Code that runs outside this activation's instructions (construction, erased
dispatch, default members, external calls, `Class_Terminate`) can only reach
ambient storage and the references it is explicitly given. VBA cannot retain a
reference to an activation's locals beyond the region that passes it.

## Shadow operations

| Operation | Effect |
|---|---|
| Numeric element reads/writes, value copies, String content | Preserve |
| `ReDim`, dynamic `Erase`, replacing an array, Variant, or UDT, ParamArray rebinding | Invalidate and observe the target's group |
| Lock acquisition (`OpArrayIndex`, enumeration, invocation capture) | Observe the array's group |
| With capture, kept element, enumerator, ParamArray reference | Bind the slot into the target's group |
| Calls | Apply the callee summary to the actuals |
| Releasing an object-bearing value | Apply the `Class_Terminate` summaries |
| Reentrant dynamic operations | Invalidate and observe ambient and reference arguments |
| Unsummarized operations | Invalidate and observe every location |

*Observe* is separate from retention. Lock counts are observable: error 10 when
a lock is saturated. A proof about identity lifetime says nothing about lock
bookkeeping, so an operation that reads the lock count records an observation.

*Bind* is an assignment of a reference to a slot. It merges the slot into the
target's group and withdraws the slot's own certificate, but acquiring a
reference does not invalidate the target. *Connect* merges two independent
groups and withdraws both certificates; summaries use it when a callee leaves
two formals dependent.

## Flow graph and evaluation

The graph follows the typescript-go binder's flow nodes (`ast/flow.go`). Each
`FlowNode` names its antecedent; labels join several. The graph is derived from
verified IR rather than from binder flow nodes, because only the IR has the
executable edges:

- region terminators (Continue, Jump, Branch, Switch, Return, GoSub);
- every `OpTry` failure, after region cleanup, to procedure exit or, when the
  procedure has an error policy, to every handler and to the failed region's
  Next;
- Resume to every region, and GoSub Return to every resumption site;
- region and scope cleanup, as release nodes.

Labels entered by a depth-first back edge become loop labels. Antecedents not
reachable from Start are dropped; dead GoTo cycles contribute nothing.

Evaluation runs forward, unlike typescript-go's demand-driven
`getFlowTypeOfReference`. Iterating inner loop labels for every provisional
outer approximation grows exponentially with loop nesting, and a procedure with
Resume is one large cycle. Instead, the solver visits nodes in reverse
post-order until nothing changes: the least fixed point. A whole procedure is
solved once from Start. A lifetime query solves only the nodes on paths from
its boundary to its end, usually a short straight line. The vba-regex test
project (540 procedures, about 96,000 instructions) analyzes in under half a
second.

The internal state is finite:

```go
type State struct {
	groups   []location // Partition by canonical member.
	changed  []bool     // Lost the boundary's retention guarantee.
	observed []bool     // Lock bookkeeping may have been observed.
}
```

A branch merge closes the union of both partitions and unions both sets.
Sequential composition is function composition. Transfers only merge groups and
add members, so the iteration terminates. Integer labels are generated only for
presentation, after convergence: unaffected groups keep their input labels.

## Summaries

A procedure's summary covers its borrowed (ByRef and ParamArray) formals and
ambient storage. It is computed with independent inputs and applied to the
caller's partition, which makes it parameterized by the incoming dependencies.
Invalidating a formal invalidates its actual's whole caller group, including
aliases that the callee could not distinguish. Different parameter names never
imply independence.

A summary joins every state the activation can reach, not only its exit. A
call that never returns has still had its effects while its caller waited.

Summaries reach a least fixed point over the call graph, including recursion.
An interface member includes every implementation, with formals mapped by
position. Release nodes apply the joined `Class_Terminate` summaries.

## Lock elision

An element token's lifetime runs from its acquiring `OpTry` to its first
`OpReleaseElement`, or to the region's cleanup. This establishes a new analysis
boundary. Inside a procedure body the caller is unknown, so borrowed formals and
ambient storage start in one group. The lock can be omitted when, on every path
through the lifetime, the token's root keeps its certificate and its lock count
is not observed.

Invalidations count even where the lock itself would have prevented them with
error 10. The analysis never uses the lock as evidence that the lock is
unnecessary.

`LockRemovable` answers for region-local lifetimes. `LockRemovableExcept`
ignores observations by instructions whose saturation checks the consumer
reproduces itself. The backend uses it for virtual locks, emulating nested
accesses to the same array. `Lifetime` evaluates any boundary, such as a With
element kept across loops, for future consumers.

## Snapshots

`Invalidates` reports whether one instruction may invalidate a slot's storage,
through aliases, callees, and releases. `ReleaseInvalidates` covers region and
scope cleanup. `Graph.Edges` lists every region-to-region transfer, including
failures (with the failing instruction), handler dispatch, Resume, and GoSub
Return. With these, the backend runs a must-analysis that places descriptor
snapshot reloads only where a snapshot may be stale.

## Limits

- Unknown indices are not refined; an array and all its elements and owned
  contents form one root.
- A bound slot stays merged with its previous target's group.
- Object reachability is not tracked; all object state is ambient.
- Error edges are conservative: every failure may reach every handler.
