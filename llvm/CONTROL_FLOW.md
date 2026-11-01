# Control flow in the native compiler

The native path now supports:

- Single-line and multiline If/ElseIf/Else.
- Select Case with values, ranges, relational alternatives, comma lists, and Else.
- Computed `On expression GoTo` with named/numeric destination lists.
- For/Next with Integer, Long, Single, Double, or numeric Variant counters; captured bounds and
  Step, descending loops, Step zero, and nested `Next inner, outer` syntax.
- While/Wend, pre-test and post-test Do While/Until, and unconditional Do/Loop.
- Procedure-local named and numeric labels, forward/backward GoTo, and jumps
  into ordinary branches and numeric For bodies.
- Exit For, Exit Do, Exit Function, and Exit Sub, using the binder's resolved
  target rather than guessing from the nearest enclosing syntax node.
- For Each over supported arrays, including multidimensional arrays, arrays of
  Variant, Variant-held arrays, and value-returned arrays.

The AST and binder keep their existing typescript-go-style separation: syntax
records the original constructs and binding resolves label and Exit targets.
The checker validates conditions and control-variable types. Lowering creates
its own executable regions; it does not reinterpret binder FlowNodes as code.
Declarations nested inside branches or loops still have procedure scope. Fixed
arrays are initialized at procedure entry, including when GoTo skips a Dim.

## Execution and errors

For captures start, limit, and Step once in source order before initializing its
counter. Omitted Step is 1. Limits and Step live in private loop-state slots;
assigning their source variables later does not change the captured values.
Next reads the current counter, so body assignments and ByRef calls can change
the next iteration. A nonnegative Step tests counter <= limit; a negative Step
tests counter >= limit. Zero Step does not advance the counter and can therefore
run indefinitely until an explicit exit or jump.

Integer/Long increments use checked arithmetic and checked narrowing. Overflow
at Next raises error 6, including after the last body execution, and leaves the
failing assignment uncommitted. Numeric Variant counters retain their subtype
and use the existing overflow promotion rules. Normal completion leaves the
counter at its final value; Exit For skips the increment.

An initialized flag belongs to each numeric For in each procedure activation.
GoTo into a body skips initialization. Reaching Next without earlier successful
initialization raises error 92, even if the body explicitly assigned the counter.
Previously initialized numeric loop state remains available for later GoTo
reentry during the same activation. These flags and captures are not global.

While and pre-test Do can execute zero times. Post-test Do executes its body
before testing. Until reverses branch destinations without synthesizing a new
source expression. Conditions coerce numeric zero to False and nonzero to True;
Null is False in a condition. Explicit `CBool(Null)` still raises error 94.
Boolean And/Or expressions continue to evaluate both operands.

Variant relational comparisons and binary String comparisons support conditions
without discarding Null or the declared-operand distinctions used for numeric
versus String coercion. Existing Linux C-locale conversion limitations still
apply. Text/locale comparison modes, Like, mixed statically numeric/String
comparisons, and Variant logical operators remain outside this subset.

## Enumeration and cleanup

[Collection enumeration](COLLECTION.md) now uses the same resource scopes,
with retained object ownership and independent cursors rather than array locks.
Variant counters receive value snapshots; Object/class counters retain object
items. The remaining paragraphs in this section describe array enumeration.

For Each captures its collection once and iterates array data in column-major
order. Each element is copied into a Variant control variable; changing that
variable does not modify the cell. Changes to later source cells remain visible.
An empty/unallocated typed array executes no body and preserves the control
variable. Normal completion retains the last assigned element value.

Variable collections borrow and lock the original SAFEARRAY. Value expressions
give the enumerator an owned array snapshot. ReDim, dynamic Erase, and whole-array
replacement cannot invalidate a live enumeration; they raise error 10. Fixed
Erase can reset cells without replacing their storage. String and Variant cell
copies follow the existing ownership rules and remain fallible.

The enumerator has a private 24-byte runtime state, not an exported VBA type.
Resource scopes identify which enumerator slots belong to each loop. Edges out
of a scope release its locks, including normal completion, Exit, and outward
GoTo. Return and error propagation release every active enumerator before arrays
or Variants owning their storage are destroyed. Temporary expression values and
array-element references are cleaned on every region exit and loop backedge.
Repeated iterations reuse activation-local slots without accumulating owners.

GoTo **into** a For Each resource scope from outside is currently diagnosed as
unsupported. Jumps within an active enumeration and jumps out of it are supported.
Supporting arbitrary entry/reentry needs a specified lifetime for the retained
collection after the original scope was left; this compiler rejects that case.

## Select Case and computed GoTo

Select Case evaluates and captures its selector once. Each alternative then uses
that value even if a case expression changes the original variable through ByRef,
retags a Variant, or modifies a String with Mid assignment. Managed captures own
independent snapshots and are released before the chosen body (or on no match).
Errors release activation-owned captures through the normal propagation path.

Alternatives run in source order, stop at the first match, and never fall through
to the next body. A range evaluates both comparisons, as VBA's eager And does;
a false lower comparison does not skip the upper expression. Reversed ranges
do not match. Null in the selector goes directly to Case Else/end without
evaluating alternatives. Null comparison results do not match. String/Variant
comparisons retain the existing declared-type coercion and binary comparison
profile; mixed statically numeric/String comparisons remain unsupported.

Consecutive integral literal/enum equality alternatives lower to semantic Switch,
including `Case Is = value`. Duplicate values retain their first destination.
Grouping stops at every dynamic expression, range, or non-equality predicate;
none of their side effects or errors can move across a dispatch. Other
alternatives emit ordinary comparisons and branches. There is no density
threshold, range expansion, jump-table construction, or target heuristic in our
lowerer or emitter. LLVM receives `switch` and may choose a code jump table,
comparison tree, bit tests, data lookup, or eliminate dispatch entirely. General
arithmetic and comparison simplification also remain LLVM's job.

On GoTo evaluates its index once, Let-coerces it to Integer (including rounding),
then checks 0..255. Negative or greater-than-255 Integer values raise error 5;
coercion failures keep their own errors (for example overflow 6 or Null 94).
Index 1 selects the first destination. Zero and indices beyond the list length
continue at the following statement. Duplicate destinations and mixed numeric/
named labels are supported. Targets belong to the current procedure; destinations
beyond position 255 are bound but cannot be selected by a valid index.

On GoTo uses the same switch edges and scope cleanup as other branches. Outward
jumps release enumeration locks; fallthrough keeps active scopes. As with
ordinary GoTo, entering a For Each scope from outside is diagnosed. This feature
is distinct from On Error GoTo; error handlers have their own
[activation policies](ERROR_HANDLING.md). [GoSub/Return](GOSUB.md) now use pointer-based resumption dispatch.

## IR and LLVM

Regions have dense procedure-local IDs. Their terminators are Continue, Jump,
Branch, Switch, or Return. A Branch names a Boolean value defined in that same region
and two destinations. A Switch names an Integer/Long value, unique integral
value/target pairs, and a default destination. Values cannot cross regions except through slots;
our backend constructs SSA values and phis directly for suitable scalar slots; see
the [SSA output contract](README.md#direct-ssa-output). Loop-state slots persist
across regions, while argument temporaries and owned snapshots have shorter
lifetimes. Resource scopes have acyclic parent links and explicit cleanup slots.

The verifier checks targets, branch types, region-local dominance, resource scope
ownership, and the existing Result/try/cleanup contracts. The debug formatter
shows branches, loop slots, and scopes. LLVM emits ordinary blocks and branches,
inserting cleanup on edges leaving resource scopes. Retry/Next recovery metadata
is kept separately from explicit jump/branch destinations. It now drives
[On Error and Resume](ERROR_HANDLING.md), preserving original provenance.

## Try it

From the repository root:

```sh
go run ./cmd/vambas-compile -root examples/control-flow -entry Main.Main -o /tmp/control.ll
clang-20 -O2 /tmp/control.ll runtime/*.c -lm -o /tmp/control
/tmp/control
echo $? # 42
```

Pure Integer/Long/Boolean control flow needs no C runtime. Variant conditions,
binary String comparisons, numeric Variant For, and array For Each link the
existing runtime, including separately compiled `runtime/control.c`. On Windows
link OleAut32 instead of `m`. See [Variant](VARIANT.md) and [arrays](ARRAYS.md)
for the supported 64-bit memory layouts and linking instructions.

The [dispatch example](../examples/dispatch/Main.bas) needs no C runtime:

```sh
go run ./cmd/vambas-compile -root examples/dispatch -entry Main.Main -o /tmp/dispatch.ll
clang-20 -O2 /tmp/dispatch.ll -o /tmp/dispatch
/tmp/dispatch
echo $? # 42
```

Tests run native LLVM at O0/O2, original/optimized bitcode, an independent scalar
IR oracle, malformed-graph verification, and runtime allocation fault injection.
Coverage includes captured evaluation order, counter mutation, negative/zero
steps, skipped and entered bodies, final counter values, error 92/overflow,
recursive branches, nested resource cleanup, and returned array lifetimes.
Dispatch tests also check selector mutation, first-match/eager-range semantics,
index coercion/errors, and actual LLVM-selected code jump tables on x86-64.
Windows and VBA/VB6-host differential execution remain outstanding.

Current limits also include general COM enumerators,
nonnumeric/string-valued For bounds or Step, and loop control storage
other than local/parameter/result variables. Single and Double execution,
floating loops, and their checked conversion rules are described in
[floating-point support](FLOATING_POINT.md).

References: [array For Each semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/b132463a-fd25-4143-8fc7-a443930e0651),
[Select Case](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/select-case-statement),
[On GoTo](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/ongosub-ongoto-statements),
[Do semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/61d886e0-1768-4032-8bbb-dd3eca7977df),
[Null in loop conditions](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/doloop-statement),
[relational coercions](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/comparison-operators),
and [the VBA language specification](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/d5418146-0bd2-45eb-9c7a-fd9502722c74).
