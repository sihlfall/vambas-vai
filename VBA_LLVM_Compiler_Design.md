# VBA-to-LLVM Compiler: Architecture and Semantic IR

**Status:** Design draft v0.1  
**Date:** 2026-10-25  
**Primary objective:** Compile and execute VBA code and its tests natively on Linux, without Office, Windows, Wine, or COM emulation.  
**Secondary objective:** Generate Windows binaries that interoperate with existing VB6/VBA-compatible components through their **public COM interfaces**, including the limited callback patterns supported by classic Visual Basic.

> **Scope of this document.** This is an architectural proposal, not a claim that the current parser implements all of VBA or that every subtle behavior described here has been experimentally validated. Sections labeled **Decision** record choices made in the design discussion; sections labeled **To verify** deliberately defer language-specific facts to the specification and differential tests.

## 1. Motivation, scope, and compatibility contract

This project is not an attempt to modernize VB6 or to extend VBA with new language features. It is an attempt to compile *existing* VBA programs correctly, with particular emphasis on running unit tests on Linux. Behavioral fidelity has priority over convenience, elegance, and performance when those goals conflict.

### Goals

1. Respect the selected VBA language specification for all implemented features, including error behavior, evaluation order, coercions, and observable values.
2. Run VBA procedures and tests as native Linux programs against user-supplied replacements for external dependencies.
3. Preserve compatible Automation data representations and provide Windows COM ABI adapters for public interfaces to existing VB6 DLLs and similar components.
4. Support classic callback mechanisms within the scope of VB6/VBA, including `AddressOf` and COM event sinks where implemented.
5. Make failing computations explicit in semantic IR using `Result<T>`; compile VBA error policy into procedure-local control flow rather than invoking a generic runtime error dispatcher.
6. Optimize away `Result` tagging and branches whenever semantic analysis can prove them unnecessary.

### Non-goals

- Implementing Office, Excel, Access, or Windows COM on Linux.
- Calling real Windows VB6 DLLs from the Linux test runner.
- Reproducing the *private* ABI or internal stack frames of the VB6 compiler.
- Assuming that any extension to VBA is desirable, or silently adopting Visual Basic .NET semantics.
- Supporting every VBA/VB6 syntax form from the first release.

### Compatibility profiles

**Language profile.** Define a versioned conformance target for VBA, informed primarily by [MS-VBAL] and the VBA language reference. Document divergences from a particular Office/VB6 host instead of conflating them.

**Linux-native profile.** Use the same language semantics and semantic type information; resolve external interfaces to test implementations. No COM runtime, activation, registry, apartments, or `IDispatch` server is required.

**Windows interoperability profile.** Build external entry points and call sites conforming to the applicable COM interface definitions and target architecture. Existing public COM interfaces—not arbitrary VB6 internal routines—are the compatibility boundary. Initially, 32-bit x86 is the most directly relevant profile for VB6 DLL interoperability; support for other architectures is a separate target decision.

**Decision:** Platform-specific service implementations may differ; VBA-visible semantics must not change merely because a Linux test implementation is substituted for a Windows COM component.

## 2. Starting point: existing Go syntax tree and binder

The current parser follows the `typescript-go` style. It produces a `KindSourceFile` tree with a common `*ast.Node` header, `NodeList`s, source ranges, syntax flags, and kind-specific payloads. The binder enriches **the same tree** with symbols, scopes, diagnostics, and antecedent-based flow nodes. The design must use this AST as input, not replace it.

Relevant existing guarantees and representations from the accompanying AST reference:

- `SourceFile.Statements` is the source-order module view; `FunctionDeclarations` is a compatibility view, so consumers must not traverse both as independent trees.
- `Node.Symbol` connects declarations and references; `Node.Locals` holds source-file or procedure-level value declarations. Blocks do not create VBA variable scopes.
- Function declarations have a distinct `ResultSymbol`, assigned when code writes the function's name. That symbol is not the same as the procedure symbol.
- Parameters preserve explicit `ByVal`, `ByRef`, `Optional`, and `ParamArray` syntax. An absent annotation or omitted bound is *not* replaced with invented syntax.
- Array designators preserve the distinction between `nil` dimensions and a present but empty `()` list.
- Declaration type-character suffixes are represented by `Identifier.TypeHint` and ignored for normalized symbol lookup.
- The binder creates a backward-linked `FlowNode` graph with branches, junctions, and labels. This is a **binding-flow representation**, not yet a lowered executable CFG.
- `KindLetStatement`, `KindIfStatement`, `KindGotoStatement`, `KindLabeledStatement`, literals, binary/unary expressions, and relevant declaration kinds are documented; numerous VBA constructs remain unimplemented or only scaffolded.
- An initial [semantic checker](checker/README.md) now supplies declared scalar/class/array types, procedure signatures, constant type inference and cycle checking, scalar diagnostics, and [call/default-member/storage plans](checker/CALLS.md) in checker-owned tables. `KindTypeReference` remains source syntax. Dynamic/external resolution, semantic IR, effects, and executable lowering are still future work. The source can be recovered from parse errors and unsupported statements may be skipped.

### Input contract for compilation

**Decision:** The compiler must reject an input containing parse or bind errors that compromise correctness, unresolved required semantics, or unsupported executable constructs. Successful parser recovery is not proof that the recovered AST has sufficient information for code generation. Source ranges remain attached to diagnostics and IR provenance.

### Proposed next semantic components

```text
Parse (existing) -> Bind (existing) -> Check / Resolve Types (new)
  -> Typed Semantic IR (new) -> Effect Analysis (new)
  -> Lowered CFG (new) -> LLVM IR (new)
```

The type checker should resolve qualified type names, imports/reference metadata, calls, lvalues, default properties, coercions, constants, and parameter passing. No checked type should be stored by mistaking `Node.Symbol` for a type; use an explicit semantic mapping or checker-owned representation.

## 3. Architecture and boundaries

```text
                       VBA source and reference metadata
                                    |
                         Parser + existing binder
                                    |
                        Type checker / resolver
                                    |
                         Typed Semantic IR
                    values, statements, Result<T>,
                      effects, error policies
                                    |
                    Fallibility / semantic optimization
                                    |
                   Explicit CFG + ownership lowering
                                    |
                            LLVM IR / native
                                    |
                         Platform service adapters
                          /                    \
             Linux test substitutes      Windows COM ABI
```

### Major boundaries

- **AST** describes user-written source faithfully and carries binding metadata.
- **Semantic IR** states what each operation *means*: its type, potential errors, storage effects, and containing resumable statement.
- **Lowered IR / CFG** states how those semantics execute: branches, temporaries, error paths, cleanup, and calls.
- **Runtime** implements intrinsic operations that cannot reasonably be expressed as simple machine instructions. It does *not* interpret `On Error` policies or pick statement continuations.
- **External adapter** provides environment-specific services without modifying language semantics.

**Decision:** The semantic IR is LLVM-independent and could be verified or interpreted without LLVM. LLVM is the final code generator, not the definition of VBA semantics.

## 4. Semantic types, effects, and Result values

A VBA procedure has a **language return type** `T` (or no result for `Sub`) and a separate **effect signature**. In particular:

```text
procedure Calculate(Long, Long) -> Long
    effects: { MayRaise }

procedure GetAnswer() -> Long
    effects: { }                 ; proven not to raise
```

Within semantic IR, fallible computations produce first-class values:

```text
Result<T> = Ok(T) | Error(VbaError)

%r = checked_add %a, %b : Result<Long>
%v = try %r                 ; follow enclosing statement's error edge
```

The syntax is illustrative, not a committed serialization format. `try` means **propagate into the current semantic error continuation**, not necessarily return from the procedure. It must not silently erase errors.

**Decision:** `Result<T>` is a **semantic type**. It does not prescribe a heap object, tagged runtime allocation, LLVM struct, or public ABI return type.

### Effect metadata

An operation's effects may include:

- `MayRaise` — produces a VBA runtime error rather than a normal value.
- `WritesStorage` — modifies an lvalue, object, or other observable memory.
- `CallsExternal` — invokes a service whose behavior must not be treated as pure.
- `ReadsAmbientErr` / `WritesAmbientErr` — observes or updates `Err` state.
- `ManagesLifetime` — acquires/releases String/Variant ownership, with cleanup on exits.

Other effects may be introduced when justified; effects must not be used as a substitute for correct evaluation-order semantics. A procedure-level `MayRaise` summary is derived from operations and callees, not guessed from source syntax alone.

### Static types versus Variant

A `Variant` is a VBA/Automation value type with observable tags and coercion rules. The checker may deduce a narrower static type for an expression, but optimization must preserve `VarType`, `IsNull`, `IsEmpty`, `CVErr`, default-property evaluation, overflow, and all other observable behavior. TypeScript-like inference is therefore a tool for selecting operations and proving fallibility, not permission to redefine dynamic typing.

Example:

```vb
Dim a As Long, b As Long
Dim v As Variant
v = a + b
```

The checker may select checked `Long` arithmetic and then the specified conversion into a `Variant`, rather than route the addition through dynamic dispatch. The exact VBA promotion/overflow rules must be implemented and tested; `Variant` optimization must not change which tag is produced.

## 5. Error model: values, procedure state, and observable Err

**Three concepts must remain distinct:**

1. **`Result<T>`:** transient success or failure from one semantic computation.
2. **Procedure activation error state:** controls propagation, inline resumption, handler entry, and `Resume` destinations.
3. **The VBA `Err` object:** externally observable error information, with its own update/clear semantics.

A conceptual error value is:

```text
VbaError {
    number,
    source,
    description,
    help_file,
    help_context,
    origin / platform payload (if required)
}
```

This is a *semantic record*. Storage strategy is not fixed. Information such as COM `IErrorInfo` can be captured/translated at boundaries, and costly description construction can be deferred if doing so preserves behavior.

The public `Err` state should be modeled as part of a logical VBA execution context, *not conflated with the error policy of a single procedure activation*. Whether a particular field is actually stored, cached, or reconstructed can be decided during lowering. `Err.Clear`, `Err.Raise`, and implicit clearing rules are semantic operations and must be covered by conformance tests.

A conceptual per-activation state is:

```text
ProcedureErrorState {
    policy: Default | ResumeNext | Goto(handler),
    handler_active: Bool,
    fault_statement: Optional<StatementId>,
    saved_retry_target: Optional<BlockId>,
    saved_next_target: Optional<BlockId>
}
```

This is a **model**, not an obligatory memory layout. The source specification may impose additional state transitions; lowering should minimize physical state.

### Baseline error policies

| State/policy | Intended behavior |
|---|---|
| Default | Propagate `Error` to the caller; caller applies its own current policy. |
| Resume Next | Continue at the next executable statement in this procedure's normal execution order. |
| GoTo handler | Save fault identity and continuation, enter the named handler, and mark the handler active. |
| Active handler | A fresh error cannot be caught by the already-active handler; propagate outward. |
| Unhandled host entry | Report a host-visible failure according to the selected execution profile. |

Microsoft's formal VBA specification also describes `Retry`, `Ignore`, and `Terminate` policies in its abstract runtime semantics. These should **not** be mistaken for freely available `On Error` statements; their precise role in the formal model needs to be accounted for before declaring full specification conformance. See [MS-VBAL §5.4.4](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/47b18690-3175-44d9-9de1-31629f0aacc7).

### Cross-procedure errors

A callee does not inherit its caller's active `On Error Resume Next` mode. If an unhandled error escapes the callee, the caller processes it at **the statement containing the call**, not at the callee's final failing operation. A caller's `Resume Next` continues after that statement. Procedures compiled by this compiler exchange errors through the private internal ABI; published COM methods translate at the ABI boundary.

### Resume operations

- `Resume` retries the saved fault statement (or the appropriate caller-side call statement when the failure originated below it).
- `Resume Next` continues at the saved next statement.
- `Resume label` transfers to the named label in the same procedure.
- `Resume` without an appropriate active error is itself an error.

These operations require stored or SSA-represented **statement identities/continuations**. They do not require a generic runtime continuation dispatcher.

**To verify:** Exact `Err` clearing rules for every `On Error`, `Resume`, procedure exit, and cross-procedure transition; formal policy transitions in corner cases; host-specific discrepancies. The documented VBA `Err` object and error-handling references are the baseline, but should not be supplemented with guesses.

## 6. Statements are recovery regions

A physical line, an AST statement, and a **semantic recovery region** are different concepts.

For example:

```vb
On Error Resume Next
x = F(): y = 42
```

The two assignments on one physical line have separate executable recovery points. On a failure in `F()`, the first assignment is abandoned while the next executable statement may run. A line number alone cannot identify the continuation.

Each lowered executable statement/region should carry:

```text
StatementRegion {
    id: StatementId,
    source_node: AstNodeRef,
    entry: BlockId,
    normal_successor: BlockId,
    error_successor: BlockId,
    retry_target: BlockId,
    next_target: BlockId
}
```

Targets can coincide or be derived by CFG construction. `StatementId`s should be stable within a compilation and connect diagnostics, handlers, and test traces to source. They are not VBA-visible integers and should not be inferred from byte offsets alone.

### Example: side effects, then failure

```vb
Public Sub Probe()
    Dim hits As Long
    Dim result As Long
    result = 10
    On Error Resume Next
    result = BumpThenFail(hits)  ' S1
    result = result + 1         ' S2
End Sub

Private Function BumpThenFail(ByRef hits As Long) As Long
    hits = hits + 1
    Err.Raise 5
End Function
```

The intended observations after S2 are `hits = 1`, `result = 11`, assuming no additional effects. The increment of `hits` survives the failure; the assignment to `result` in S1 does not occur.

Illustrative semantic IR:

```text
S1.entry:
    %r = invoke BumpThenFail(byref hits) : Result<Long>
    match %r:
        Ok(%value) -> S1.commit(%value)
        Error(%err) -> S1.error(%err)

S1.commit(%value):
    store result, %value
    jump S2.entry

S1.error(%err):
    update_err_state %err
    ; Current effective policy is ResumeNext.
    jump S2.entry

S2.entry:
    %r2 = checked_add (load result), 1 : Result<Long>
    ...
```

**Crucial invariant:** If an operation fails while computing an assignment, the final store is skipped, but completed earlier side effects are **not** rolled back. If evaluating the lvalue itself can have side effects or fail, its order relative to RHS evaluation must follow VBA semantics, not arbitrary compiler convenience.

### Composite statements: explicit research item

The recovery region is *not* assumed to be every `*ast.Node` with a statement kind. Single-line and multiline `If`, loops, `With`, `For Each`, property calls, and colon-separated statements may have distinct execution-order and continuation rules.

**To verify:** The precise next-statement/retry target when a condition, loop bound, enumerator acquisition, lvalue, or intermediate argument raises. Until verified, these constructions cannot silently use an approximate rule like “jump after the entire AST parent.” The compiler should encode an explicit semantic region decomposition, backed by a conformance test for each form.

## 7. Lowering Result<T> into straight-line code and CFG edges

**Decision:** There is **no general runtime error dispatcher**. The compiler generates local branches whose destinations follow the procedure's effective policy.

Source:

```vb
On Error Resume Next
x = F() + G()
y = 42
```

Illustrative lowering:

```text
S1:
    %f = call F() : Result<Long>
    %fv = unwrap %f or jump S1.fail

    %g = call G() : Result<Long>
    %gv = unwrap %g or jump S1.fail

    %sum = checked_add %fv, %gv : Result<Long>
    %v = unwrap %sum or jump S1.fail

    store x, %v
    jump S2

S1.fail(%error):
    record_error %error
    jump S2

S2:
    store y, 42
```

**Note:** The actual evaluation order of operands and argument expressions must be determined by the VBA spec and preserved. The above order is illustrative, not itself an authority for evaluating `F()` before `G()`.

### Dynamic handler state without a runtime dispatcher

A procedure may execute `On Error Resume Next` conditionally. The compiler cannot always replace the policy with a compile-time constant:

```vb
If enabled Then On Error Resume Next
x = MightFail()
```

On the **error edge only**, generated code can branch on a procedure-local policy value, or better, use SSA/control-flow information to avoid the state variable when its value is known. An `On Error GoTo Handler` branch can store a compact fault/continuation ID. `Resume` and `Resume Next` then lower to a compiler-generated local `switch` or equivalent branches over those IDs.

This is **finite procedure-local control flow**, not an interpreter for arbitrary runtime errors. No central dispatcher knows statement successors. LLVM is free to simplify these branches.

### Internal ABI

The calling convention between compiled procedures is private. A fallible procedure may be lowered as `{ status, value, error }`, a status return plus output slot, split blocks, or another efficient target-specific convention. Infallible internal procedures may return bare `T`. Private choices must never leak into a published COM method signature.

Errors must not unwind across foreign ABI frames. Callback/COM adapters explicitly catch/translate the compiler's error result according to the boundary contract.

## 8. Optimization contract

`On Error Resume Next` is expected to be uncommon. Its presence must not tax unrelated procedures. More generally, fallibility is not synonymous with an installed error handler: a normal arithmetic or conversion operation may still raise and must propagate the error correctly.

### Compiler-specific passes

1. **Semantic resolution:** choose exact checked operations and conversions, assign statement regions and errors.
2. **Effect inference:** compute `MayRaise` summaries for operations/procedures, conservatively across unavailable and recursive callees.
3. **Infallibility proofs:** eliminate failure alternatives only when justified by types, constants, range facts, or known callees.
4. **Specialization / inlining:** remove internal `Result` handling at call sites for proven-infallible callees.
5. **Result lowering:** translate monadic values into success and failure CFG edges; avoid heap allocation or mandatory tag materialization.
6. **CFG simplification:** prune unreachable error paths, policy checks, dead error-state slots, and unnecessary continuation tables.
7. **LLVM optimization:** constant folding, branch simplification, inlining, scalar replacement, etc.

### Non-negotiable safety rules

- Never eliminate a check merely because the *caller* has no `On Error` statement. An error may be observed by an ancestor caller.
- Never assume native integer overflow can wrap if VBA requires an overflow error.
- Never fold a `Variant` operation if that changes observable subtype, coercion, error, or side effects.
- Never reorder operations across `MayRaise`, `WritesStorage`, or ambient `Err` dependencies without a proof of equivalence.
- Never erase a side effect that occurs before a later error in the same statement.

### Success criteria

For a proven-infallible procedure, optimized native code should contain **no** `Result` storage, tag checks, error-policy slots, saved resume points, or error-specific branches. For a fallible procedure with no handler, the normal path should contain only the error checks still semantically required, leading to a compact error return. When an error handler exists, its additional logic should be isolated to relevant failure paths as much as possible.

The optimizer's correctness argument is behavioral equivalence to the typed semantic IR, not bitwise equivalence to a proposed `Result<T>` runtime layout.

### String optimization freedom

**Decision:** VBA/VB6 compatibility permits optimizations beyond those performed
by the reference compiler. Preserve language-visible values, types, mutations,
aliasing, evaluation order, and required errors while allowing a more efficient
internal representation and fewer allocations.

- Fold constant string expressions, including concatenations such as
  `"ab" & "cd"`, before runtime allocation. Fold operations on those constants
  when their complete semantics are known, for example `Len("ab" & "cd")`.
- Represent a transient one-character String as a UTF-16 code unit when its uses
  permit it. Keep its semantic type String, including Variant subtype and coercion
  behavior. Preserve surrogate and embedded-NUL values without treating the unit
  as a Unicode scalar value or a string terminator.
- Materialize string storage when required by assignment/storage semantics,
  ByRef aliasing, supported pointer observations such as StrPtr, or a foreign/COM
  boundary. Respect that boundary's lifetime and ownership contract. Temporary
  representations may remain specialized across private calls when proven safe.
- Preserve comparison mode, locale-dependent behavior, argument side effects,
  bounds checks, and null-versus-empty distinctions wherever observable. A
  character shortcut requires the same proof as any other string optimization.
- Resource-exhaustion checks apply to allocations remaining after legal
  allocation elision; exact allocation counts and failure thresholds are
  implementation dependent. Value-dependent VBA errors and failures from
  remaining allocations retain their error continuations and cleanup obligations.

Keep these optimizations in semantic IR passes or subsequent representation
lowering, leaving the source AST and shared checker intact. String IR must expose
values, effects, and ownership sufficiently to prove these transformations; a
String expression need not imply an immediately allocated BSTR.

## 9. Value and memory compatibility

`VARIANT`, `BSTR`, `SAFEARRAY`, `VT_BYREF`, and COM interface references are **not invented compiler types** at public Windows boundaries. Their representations, ownership, clearing, copying, and reference counting follow the applicable Automation contracts.

- `VARIANT` is a tagged union defined by Automation; its representation and tag values are ABI requirements where exchanged across COM.
- `BSTR` length/ownership semantics must be preserved; it is not interchangeable with a null-terminated C `char*`.
- `SAFEARRAY` has dimension/bounds/ownership semantics; dynamic and fixed array declaration distinctions in the AST matter.
- By-reference arguments need alias-correct storage and lifetime behavior, including by-reference `Variant` values and COM out parameters.
- Reference-counted object values and temporaries require deterministic release at VBA-defined points. Error edges must run any mandated cleanup.
- `Empty`, `Null`, `Nothing`, and `Error`/`CVErr` values must remain semantically distinct from a *raised* runtime error represented by `Result<T>.Error`.

**Important distinction:** A Linux-native implementation can preserve VBA's value semantics and use compatible Automation layouts where required by the chosen internal design. It does **not** thereby need to implement COM activation, vtables, registration, apartments, or `IDispatch`. Exact ABI conformance is enforced on Windows COM-facing adapters, with independent validation per target architecture.

## 10. External interfaces: Windows COM and Linux substitutes

### Common metadata model

The checker consumes a platform-independent interface description derived, where available, from type libraries or explicit declarations:

```text
InterfaceId
MemberId / name
Kind: method | property-get | property-put | property-putref
Parameter types, order, names, optional/default/byref flags
Return type
Calling / marshaling attributes required by the public ABI
```

The exact schema remains open. The key is that the compiler can **type-check code without loading the implementation**.

### Windows adapter

- Emit appropriate vtable or `IDispatch` calls based on the resolved interface and member semantics.
- Translate COM failures (`HRESULT` and associated error information, where applicable) into semantic `VbaError` values.
- Export public COM methods using their specified signatures and transform internal `Result<T>` into the correct boundary convention.
- Handle reference counting, ownership, and callback reentrancy according to public COM contracts.

### Linux substitute adapter

- Resolve the same compile-time interface identities to test implementations through a deterministic mapping supplied by the test environment.
- Invoke replacement methods **directly** using a private ABI or a minimal test adapter; COM emulation is not required.
- Return the same semantic values and errors as the corresponding interface contract specifies.
- Permit both early-bound and supported late-bound code through a compiler-defined member-invocation abstraction; it need not be a real `IDispatch` server.

The adapter is not allowed to redefine the VBA error, string, Variant, or ByRef semantics.

### Callbacks

Two callback categories must be modeled separately:

1. **`AddressOf` function pointers:** classic VBA restricts this to procedures in standard modules; Windows `AddressOf` callbacks use `__stdcall` in the documented use case. A boundary thunk converts callback arguments and translates/contains errors; it must not unwind through foreign frames.
2. **COM events / interface callbacks:** implement the necessary published interfaces and event sink behavior on Windows, while permitting direct synchronous callback invocation in Linux substitutes. Lifetime and reentrancy are first-class requirements.

The callback surface should be deliberately restricted to what the chosen VB6/VBA compatibility profile supports, not generalized into new closure semantics.

## 11. Runtime responsibilities and failure boundaries

The runtime supplies verified primitives where LLVM instructions alone cannot encode the required semantics:

- VBA numeric coercions, checked arithmetic, comparisons, and conversion error construction.
- `Variant` operations and `VarType` behavior.
- String, array, object, and reference-management primitives.
- `Err` state operations, `Err.Raise`, and conversions from external errors.
- Startup/teardown of a logical execution context and host-facing unhandled-error reporting.

**It must not** receive “execute this VBA statement and decide where to go next” requests. The compiler owns recovery regions and produces the CFG. This allows platform adapters to remain replaceable and enables LLVM to see ordinary branches and values.

### Lifetime and partial evaluation

A failed expression can leave completed external side effects in place. However, temporary values created during partially completed evaluation must be cleaned up in accordance with VBA/Automation ownership rules. The semantic IR should encode the lifetime obligations; lowering inserts cleanup on success and error edges. This is a separate correctness dimension from preserving assignment commit points.

## 12. IR invariants and verification

A verifier for typed semantic IR should enforce at least:

1. Every expression has a resolved semantic type or an explicit error diagnostic.
2. Every operation declaring `MayRaise` has an explicit continuation for its `Error` alternative.
3. A `try` / `unwrap` can only consume `Result<T>` and is always owned by an identified recovery region.
4. Every recoverable statement has valid retry/next targets appropriate to its source-language construct.
5. Each possible procedure exit commits the correct function-result semantics and required cleanup.
6. `ByRef` aliasing and ownership annotations remain valid across calls and failure edges.
7. No public COM entry or call exposes a private `Result<T>` ABI.
8. No recovered placeholder or unsupported AST form silently produces executable native code.
9. Every optimization preserves required side effects, coercions, observable `Err` state, and error continuations.

The verifier should make miscompilation a detectable compiler failure instead of letting an underspecified AST construct fall into a generic codegen path.

### Current implementation

The initial scalar foundation is implemented in `ir` and `lowering`; see the
[typed IR contract and supported subset](ir/README.md). `lowering.Lower` consumes
the existing checker and returns a complete verified IR or diagnostics, rejecting
recovered and unsupported source. Integer/Long/Single/Double/Boolean procedures have typed
storage, ByRef aliases and value temporaries, checked operations, `Result<T>` and
explicit propagation, statement IDs with retry/next metadata, function result
slots, direct calls, and conservative recursive effect summaries.

The initial encoding is structured and sequential: each fallible operation is
followed by a try whose success continues the instruction sequence and whose
failure propagates. The `llvm` backend now lowers those continuations to local
success/error blocks, emits textual LLVM, and uses a private scalar status/output
calling convention with error-origin metadata. `cmd/vambas-compile` loads project
snapshots and emits modules with an optional native entry wrapper. It has no
generic runtime dispatcher. See the [backend contract and native example](llvm/README.md).

Variable-length String values now extend this pipeline with explicit owned value
snapshots, region cleanup, mutable slot assignment, ByVal/ByRef behavior, and
String returns. Embedded LLVM runtime helpers implement UTF-16 concatenation and
inline one-character values. Typed Len/LenB and ChrW, String constants, and null
String state are supported. Tests check mutation/aliasing, original contents
across argument mutation, live-buffer cleanup, and allocation failures. LLVM
eliminates allocation paths for constant concatenation/length and character cases.
The [String implementation contract](llvm/STRINGS.md) documents the private
representation and the currently supported subset.

Mid/Mid$ reads and Mid assignment now extend the typed String subset. Assignment
has distinct syntax and checker storage/coercion plans, explicit checked IR
operations, and runtime detachment of shared/literal buffers before writes.
Unshared managed buffers are reused; overlapping views retain their old values.
Tests verify clipping/bounds, argument effects, allocation failure, and cleanup.
See the [Mid contract](llvm/MID.md), including current Variant/Null limitations.

Variant storage, parameters/results, managed snapshots, boxing/coercions,
Empty/Null, numeric promotion, and Optional Missing arguments are now implemented.
The memory container matches 64-bit Automation VARIANT, with real BSTR payloads.
Typed ByRef Variant views preserve the original storage type and write through
immediately, including String aliases that materialize BSTR storage. Windows
uses OleAut32; Linux provides a narrower portable scalar coercion profile.
See the [Variant ABI, runtime linking, and remaining subset limits](llvm/VARIANT.md).

Arrays now use SAFEARRAY descriptors with fixed/dynamic bounds, column-major
data, owned copies, and BSTR/Variant element cleanup. Whole-array ByRef aliases
share descriptor slots; element references hold checked descriptor locks across
calls and release immediately after ordinary value reads. ReDim/Preserve, Erase,
Option Base, bounds/type queries, and array returns are executable. Allocation
failure and invalid Preserve bounds leave pending destinations unchanged. See the
[array ABI, lifetime contract, example, and subset limits](llvm/ARRAYS.md).

If/ElseIf/Else, Select Case, numeric For/Next, array For Each, While/Wend, Do/Loop,
labels/GoTo, computed On GoTo, and resolved Exit statements lower to explicit
region branches and semantic switches. Integral dispatch emits LLVM switch;
LLVM chooses jump tables, bit tests, comparison trees, or other forms. For bounds
and Step are captured once; hidden activation-local state supports Next and error
92 for uninitialized entry. Enumeration scopes preserve live array identity and
release descriptor locks on exits, outward jumps, returns, and errors. See the
[control-flow implementation and remaining limits](llvm/CONTROL_FLOW.md).

Single and Double now extend typed storage, calls, native arithmetic/division,
comparisons, conversions, loops, and SAFEARRAY/Variant aliases. IEEE precision,
checked errors, ties-to-even rounding, and Double division's failure-side store
remain explicit rather than enabling LLVM fast-math. See the
[floating-point contract and example](llvm/FLOATING_POINT.md).

Nominal UDT shapes now lower to typed field references, deep value copies,
function returns, and ByRef aliases, including nested records and managed array/
String/Variant fields. LLVM chooses aggregate offsets; runtime metadata handles
ownership and atomic replacement. This is a compiler-private UDT layout, not a
public COM/Declare ABI. Enums retain Long storage throughout. See the
[UDT/enum implementation and remaining limits](llvm/UDTS_ENUMS.md).

GoSub/Return, On GoSub, On Error/Resume, the complete
VbaError/Err state, other String content-writing forms, fixed-length Strings, other
ownership-bearing values beyond String/Variant/arrays/UDTs, and public/COM ABIs remain future
work. Tests use both the scalar semantic oracle and assembled/verified LLVM,
JIT execution, and native Clang builds at O0/O2. VBA/VB6 host conformance testing
remains outstanding.

## 13. Conformance strategy and acceptance tests

Use the written VBA language specification as the reference for intended semantics. Use small programs executed in a real reference VBA environment to probe ambiguous cases; when VB6 and VBA behavior differs, capture the results and select or document the intended profile. Record not just final return values, but `Err` fields, mutations, call order, object lifetimes, and which statement executed next.

### Minimum semantic test matrix

| Area | Cases to test |
|---|---|
| Error-free path | Constant functions lower without any error machinery; typed arithmetic uses correct overflow rules. |
| Basic `Resume Next` | Failing RHS does not commit assignment; the next executable statement runs; earlier side effects persist. |
| Multiple operations | Failure in the first, middle, or last potentially failing expression within one statement. |
| Colon-separated statements | Each assignment on the same physical line has the specified continuation. |
| Compound control flow | Failing `If` condition, loop bound, `For Each` acquisition/body, nested single-line `If`, and `With`. |
| `On Error GoTo` | Handler entry, fault identity, active-handler restriction, `Resume`, `Resume Next`, `Resume label`. |
| Dynamic policy | Conditionally executed `On Error` statements and branches/jumps across policy changes. |
| Cross-procedure failure | Callee with default policy and caller with inline handler or labeled handler; deep call chains. |
| `Err` state | Raising, clearing, successive errors, procedure exits, handlers, and reading all relevant fields. |
| ByRef and partial effects | Aliases, lvalue evaluation, side effects before error, error during a call or assignment. |
| Variant | Runtime subtype, `Null`, `Empty`, `CVErr`, conversions, overflow, comparison, array and string ownership. |
| String optimization | Constant concatenation/folding and allocation-free character temporaries preserve values, String/Variant types, comparison modes, aliases, boundary materialization, and cleanup on surviving error paths. |
| Boundary behavior | COM success/failure translation, by-reference arguments, callbacks, reentrancy, and reference counts. |

### Two complementary suites

- **Language conformance suite:** Runs against reference VBA/VB6 and the Linux-native compiler; compares semantic observations. Does not require COM emulation. External dependencies use deterministic test doubles where possible.
- **Windows interoperability suite:** Runs against real COM objects and selected existing VB6 DLLs. Verifies public ABI, marshaling, object lifetimes, events, and error information at the boundary; these cannot be established solely by Linux tests.

**Definition of done for a feature:** It parses without recovery artifacts, binds, checks types, produces verified semantic IR, passes its cross-platform semantic tests, and—if externally visible—passes relevant Windows ABI tests. “It generates runnable LLVM” is not sufficient.

## 14. Phased implementation direction (not a build commitment)

**Phase A — Semantic foundation:** Resolve built-in scalar types, storage categories, function result variables, `ByRef`, and basic expressions. Introduce typed semantic IR with `Result<T>`, `MayRaise`, statement IDs, and a verifier. Compile simple procedures with no external dependencies.

**Phase B — Errors as language semantics:** Implement checked operations, propagation, `Err`, `Err.Raise`, `On Error Resume Next`, and then labeled handlers and `Resume`. Verify against reference behavior; extend parser and binder as necessary. No runtime dispatch interpreter.

**Phase C — Runtime values:** Implement `Variant`, strings, arrays, conversions, storage/cleanup, and exact dynamic observable semantics. Introduce fallibility analysis and remove unnecessary error representation on straightforward paths.

**Phase D — Test-time interfaces:** Resolve external interface metadata and load user-provided Linux substitutes. Support essential test-facing object operations and callbacks without COM infrastructure.

**Phase E — Windows COM compatibility:** Add public COM adapters and Windows-specific interoperability tests, including existing VB6 DLLs, callback entry points, late binding, and relevant class/event features.

Ordering can change with the actual VBA tests being ported. The accepted subset must always be explicitly reported; no claimed “full VBA” support from incomplete grammar coverage.

## 15. Open design questions and validation items

These are deliberately unresolved rather than silently decided:

1. **Exact VBA dialect:** Which revision of [MS-VBAL] is normative, and which host-specific/VB6 deviations are in scope?
2. **Statement recovery decomposition:** How do failed composite conditions, loop mechanics, and property/lvalue evaluation identify the precise retry/next statement?
3. **Error object lifecycle:** Precisely when do `On Error`, exits, `Resume`, propagated calls, and callback boundaries clear or modify `Err`?
4. **Formal error policies:** How do `Retry` and `Ignore` from [MS-VBAL §5.4.4] map to syntax/runtime behavior and future IR validation?
5. **Evaluation order:** Which expression and argument evaluation orders are specified versus merely observed in a given host?
6. **Object and default-property semantics:** What minimum object model suffices for actual VBA test programs, and which behaviors require host contracts?
7. **External metadata:** Which import/manifest/type-library format should feed the checker on Linux?
8. **Native ABI details:** How should target-specific return and error channels be represented without unnecessary copies while preserving `ByRef` aliasing?
9. **Execution context:** How are ambient `Err` state and callbacks scoped across reentrant calls, threads, and potential async host behavior?
10. **First Windows interoperability target:** Which architecture, COM interface styles, and reference VB6 components form the acceptance baseline?

## 16. Design decisions summary

| Decision | Status |
|---|---|
| Conformance to VBA semantics ahead of language improvement | Accepted |
| Native Linux test execution with replacement external implementations | Accepted |
| No COM emulator or real VB6 DLL execution on Linux | Accepted |
| Windows binary compatibility through public COM interfaces | Accepted |
| Existing `typescript-go`-style AST/binder retained | Accepted |
| Separate semantic typing and LLVM-independent typed IR | Implemented for the initial scalar subset |
| `Result<T>` as a first-class semantic IR type | Accepted |
| Procedure `MayRaise` effect distinct from VBA return type | Accepted |
| Compiler-generated error branches; no generic error dispatcher | Accepted |
| Optimize error representation away when provably unnecessary | Accepted |
| Statement-level retry and next continuations | Accepted in principle; exact decomposition to verify |
| Internal call ABI can differ from public COM ABI | Accepted |
| COM event and `AddressOf` callbacks constrained to classic language surface | Accepted in principle; details phased |
| Concrete semantic IR encoding / Go packages / runtime ABI | Initial `ir`/`lowering` and scalar `llvm` backend implemented; private scalar ABI only |

## 17. References

**Project input**

- Supplied **AST reference** (`README.md`): current Go AST node kinds, symbol/binder behavior, scope rules, recovery, source ranges, and antecedent-based flow graph. All descriptions of the *current implementation* in §2 are derived from that document.

**External normative or explanatory references**

- Microsoft Open Specifications, **[MS-VBAL], §5.4.4 Error Handling Statements**: https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/47b18690-3175-44d9-9de1-31629f0aacc7
- Microsoft Learn, **On Error statement (VBA)**: https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/on-error-statement
- Microsoft Learn, **Resume statement (VBA)**: https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/resume-statement
- Microsoft Learn, **Err object (VBA)**: https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/err-object
- Microsoft Learn, **VARIANT structure (`oaidl.h`)**: https://learn.microsoft.com/en-us/windows/win32/api/oaidl/ns-oaidl-variant
- Microsoft Learn, **Automation Variant manipulation and ownership**: https://learn.microsoft.com/en-us/previous-versions/windows/desktop/automat/variant-manipulation-functions
- Microsoft Learn, **String manipulation functions (`BSTR`)**: https://learn.microsoft.com/en-us/previous-versions/windows/desktop/automat/string-manipulation-functions
- Microsoft Learn, **AddressOf operator (VBA)**: https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/addressof-operator

---

**Guiding rule:** Preserve VBA's observable behavior in a typed, verifiable semantic IR. Represent potential failures explicitly, but make error handling a local compiler-generated control-flow concern whose overhead disappears wherever the semantics prove it unnecessary. Treat public COM compatibility as a boundary contract and Linux dependency substitution as a platform integration mechanism—not as alternate language implementations.
