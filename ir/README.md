# Typed semantic IR

`lowering.Lower(program)` translates an immutable, checked `compiler.Program` into
an LLVM-independent `ir.Program`. It returns **nil IR** if any input has parse,
binding, or semantic diagnostics, recovered syntax, unresolved required semantics,
or an unsupported declaration/operation. Every input procedure is checked, even
when unreachable. Editor recovery and permissive checker results remain useful
to the language server; they do not grant permission to compile.

The existing `typescript-go`-style Program, AST, binder, and checker remain the
shared front end. Lowering consumes checker signatures, storage plans, argument
bindings, enum values, and conversions without modifying syntax or binding data.
VBA requires the additional storage, effects, and error semantics described here.
Binder `FlowNode` links are analysis metadata and are never used as executable CFG.

[Identity retention analysis](identity/README.md) accompanies the IR without
annotating it. Shadow transformers over a TypeScript-style flow graph derived
from regions, error, Resume, and GoSub edges prove retention and independence of
storage roots, with summaries parameterized by caller aliasing. LLVM recomputes
it from the IR it lowers. It keeps array descriptor snapshots current for
direct element and bounds access, and omits or virtualizes element locks. The
descriptor ABI is unchanged.

```go
p := compiler.NewProgram(compiler.ProgramOptions{
    Sources: []compiler.SourceFileInput{{
        FileName: "Main.bas",
        Text: `Function Bump(ByRef value As Long) As Long
    value = value + 1
    Bump = value
End Function`,
    }},
})
semantic, diagnostics := lowering.Lower(p)
if len(diagnostics) != 0 {
    // Report diagnostics, including File and half-open UTF-8 byte Loc.
    return
}
fmt.Print(ir.Format(semantic))
```

Import `compiler`, `lowering`, and `ir` from `sihlfall.ch/vambas-vai/`, and `fmt`
from the standard library. [The executable example](../lowering/example_test.go)
also demonstrates the API. Lowering does not read files, invoke LLVM, or cache or
attach compiler state to the checker. Each invocation returns a fresh IR.

`llvm.Emit` now consumes this verified representation and emits local success/error
blocks and textual LLVM. See the [scalar backend and native example](../llvm/README.md).

## Data and execution contract

`Program.Procedures` preserves source order, with one-based procedure IDs.
Compiler-generated optional-default thunks follow the source procedures. A
procedure has a qualified name, language `ReturnType`, parameters, scalar storage
slots, statement regions, and an `Effects` summary. All procedures use private
semantic calling conventions; a Public VBA declaration does not introduce a
native/COM export. There is no native ABI in this IR yet.

| Structure | Meaning |
|---|---|
| `Type{Scalar, Form, Record}` | Resolved language type, nominal UDT ID when applicable, a reference, or `Result<T>` |
| `Program.Globals`, `Slot.Global` | Project-owned storage and activation-local aliases |
| `Program.Records`, `RecordDefinition.Fields` | Nominal field shapes and fixed bounds, independent of native layout |
| `Program.Classes`, `Procedure.Class`, `Procedure.Receiver` | Nominal class fields/members, lifecycle handlers, and implicit instance parameters |
| `ClassDefinition.Interfaces`, `OpInterfaceCheck`, `OpInterfaceCall` | Explicit interface contracts, receiver proofs, and dispatch with implementation effects; see [Implements](../llvm/IMPLEMENTS.md) |
| `Procedure.Invocation`, `InvocationParameter` | Public member kinds, argument contracts, and optional-default thunks |
| `Slot{Type, Kind, ByRef}` | Local, parameter, function result, or argument temporary storage |
| `Statement{ID, Retry, Next}` | A source statement/recovery region and its continuations |
| `Statement{End, Condition, Target, FalseTarget}` | Jump, conditional branch, and return destinations |
| `Statement{Selector, Cases}` | Integral switch selector and unique value/target pairs; `Target` is the default |
| `Procedure.Scopes`, `Statement.Scope` | Resource nesting and enumerator cleanup on outgoing edges |
| `Instruction{ID, Op, Type, Args}` | An ordered typed computation with explicit operands |
| `Instruction{Effects, OnError}` | Observable effects and an explicit error continuation |
| `Instruction.Ownership`, `Statement.Cleanup` | String/Variant/array ownership and element-lock cleanup on success/failure |
| `Source{FileName, Range, Node}` | UTF-8 byte range and optional immutable AST provenance |

Boolean values are 0 or -1. Integer and Long are signed 16-bit and 32-bit language
values; Single/Double are IEEE binary32/binary64. Floating constants carry exact
values in `Instruction.Float`; Single constants must already be rounded.
This does not prescribe their LLVM representation. `Result<T>` means
`Ok(T) | Error(VbaError)`; the error is an opaque semantic error record preserved
by propagation. No heap allocation, tagged struct, or native return convention is
implied. `Result<Void>` still carries a possible error.

On entry, locals, argument temporaries, and function result storage have their
language zero value. ByVal parameters receive copies; ByRef parameters alias the
original caller storage. `OpAddress` preserves exact storage type and identity.
Multiple arguments may alias the same slot. Parenthesized ByRef arguments and
other value expressions use separate temporary slots, with no copy-back. The
numeric types have no ownership or cleanup obligations. Strings carry explicit
ownership and region cleanup; see the [String contract](../llvm/STRINGS.md).
Variants also carry ownership. Managed loads/stores, boxing/coercion, and BSTR
exposure have explicit Result/try edges; raw loads/stores cannot access managed
slots. Typed Variant references to String storage require a checked pin first.

Instructions evaluate in order. Calls evaluate explicit arguments in source
order, then supply their computed values/references in parameter order. This
preserves named-argument evaluation and the checker's explicit/implicit call and
parenthesis distinctions. Namespace-qualified calls do not evaluate a receiver.

Ordinary runtime helpers use `OpLibraryCall` with a catalog member ID in
`Instruction.Library`. [Library signatures](library.go) derive the prepared
operand count, exact result type, and base effects from declarations marked
`SnapshotCall`. This initial contract accepts Variant value snapshots in formal
order, including pre-coerced Long arguments; explicit numeric/string defaults
are supplied by lowering. Specialized calls retain their storage-aware operations.
The verifier rejects unknown, specialized, or constant member IDs, references in
place of snapshots, mismatched `$` result types, and library payloads on other
operations. Result/Try, ownership, default-member callbacks, and cleanup retain
the ordinary checked-operation contracts. The old function-specific helper
opcodes are reserved and no longer accepted by verification.

Every MayRaise instruction produces a Result and is immediately followed by
`OpTry`. The try consumes that Result, binds the success value, and continues with
the next instruction. `OnError: Propagate` delivers the original error to the
activation policy, skipping pending instructions in that region. The policy
selects recovery or propagation. Assignment stores occur after all required tries:

```text
%1 = load $value                    : Long
%2 = constant 1                     : Integer
%3 = convert %2                     : Long
%4 = checked_add %1 %3              : Result<Long>
%5 = try %4                        : Long  success=next error=propagate
store $value %5                     : Void
```

Checked addition, subtraction, multiplication, negation, and Long-to-Integer
narrowing raise overflow on an out-of-range result. Integer-to-Long widening is
infallible. Logical operators use the scalar's bit width; comparisons produce
VBA Boolean values. Calls conservatively MayRaise and inherit callee effects to
a fixed point, including mutual recursion. `WritesStorage` currently includes
local and temporary writes; optimization and finer effect summaries come later.
The effect vocabulary also reserves external calls and ambient Err reads/writes.
These effects are separate from the procedure's language return type.

Value IDs are unique within a procedure and usable only within the defining
statement. Storage carries state between regions. `Retry` names the current
region; `Next` names its recovery continuation. Jump/Branch/Switch destinations describe
execution separately. Colon-separated statements have
distinct IDs even on the same physical line. A `Return` region reads the current
function result slot (or Void for Sub), returning Ok. Normal completion uses a
final synthetic region whose provenance is the procedure. Explicit Exit returns
at its source position. Unreachable regions remain represented and verified.

Try's success continues the instruction sequence, and its error propagates.
Region terminators define source control flow without allowing values to escape
their defining regions. Loop-state slots persist across iterations; enumerator
scopes release locks on outgoing edges and before activation-owned array cleanup.
Retry/Next now drive executable On Error/Resume recovery; see the
[error-policy contract](../llvm/ERROR_HANDLING.md).
Select Case captures its selector in a private slot. Dynamic alternatives retain
ordered comparisons; consecutive integral literal/enum equality alternatives use
Switch. The IR has no jump-table representation or density policy. On GoTo uses
Integer coercion, `OpCheckDispatch` (0..255, error 5), and Switch over 1-based
destination indices. Both use ordinary resource-scope cleanup on outgoing edges.

The LLVM backend implements these local success/error continuations, branches,
and scope cleanup. See the [control-flow contract](../llvm/CONTROL_FLOW.md).

## Supported source subset

- Standard modules with Function/Sub declarations, metadata, and checked enums.
- Nominal UDTs with nested/managed/scalar-array fields, whole-value copies and
  returns, typed field aliases, and matching ByRef parameters; see the
  [record contract](../llvm/UDTS_ENUMS.md).
- Integer, Long, Single, Double, and Boolean parameters/results; ByVal/ByRef,
  typed Optional/default arguments, and Optional Variant/Missing.
- ParamArray calls with Variant values/references, omissions, empty packs, and
  forwarded aliases; see the [ParamArray contract](../llvm/PARAMARRAY.md).
- Project-lifetime module globals, numeric Const declarations, Boolean/integer
  assignment conversions; see the [project contract](../llvm/PROJECT_STORAGE.md).
- Scalar Dim locals, function result variables, Let assignment, procedure Exit,
  and normal completion.
- If/ElseIf/Else, Select Case (values, ranges, comparisons, Else), For/Next,
  array For Each, While/Wend, Do/Loop, named/numeric labels, GoTo, computed
  On GoTo, and resolved loop/procedure Exit statements.
- Decimal integral/floating and hexadecimal/octal integral literals (including `%`/`&`/`!`/`#` and exponents), Boolean literals, enum constants,
  parentheses, and references to declared local/parameter/result storage.
- Unary `+`, checked `-`, `Not`; checked `+`, `-`, `*`, floating `/`, integral `\` and `Mod`; `And`, `Or`, `Xor`;
  `=`, `<>`, `<`, `<=`, `>`, `>=` with compatible supported operand types.
- Explicit, implicit, named-argument, namespace-qualified, forward, and recursive
  direct calls to supported procedures in the same program.
- Identity and numeric conversions, Variant boxing/coercions, and explicit
  `CInt`/`CLng`/`CSng`/`CDbl`/`CBool`/`CStr` conversions; see
  [floating-point precision and error-side stores](../llvm/FLOATING_POINT.md).
- Unsigned Byte storage/arrays, checked coercions, and direct String ↔ Byte()
  assignment; see the [byte conversion contract](../llvm/ARRAYS.md#string-and-byte-assignment).
- Distinct Date/Currency scalars and typed arrays/fields, checked coercions and
  calls, and exact Decimal Variant arithmetic; see the
  [Stage D contract](../llvm/DATA_TYPES.md). Decimal remains a Variant subtype.
- Variable-length Strings, assignment, String Const expressions, concatenation,
  parameters/results, and typed String `Len`/`LenB` and `ChrW`; see the
  [ownership and runtime details](../llvm/STRINGS.md).
- `AscW`, `StrComp`, `vbBinaryCompare`, `InStr`, `Left`/`Left$`, `String`/`String$`,
  `Split`, and eager `IIf`; see the [library/runtime profile](../llvm/BUILTINS.md).
- Variant storage, parameters/results, snapshots, typed ByRef views, arithmetic,
  concatenation, Empty/Null, subtype queries, and Optional Missing parameters;
  see the [Variant contract](../llvm/VARIANT.md).
- GoSub/Return and computed On GoSub with per-activation resumption storage;
  see the [GoSub contract](../llvm/GOSUB.md).
- Built-in Collection construction, members/default Item, Set/Nothing/Is,
  object parameters/results, captured With, and For Each; see the
  [Collection contract](../llvm/COLLECTION.md).
- Ordinary class construction, instance fields, implicit receivers, direct
  methods and Property Get/Let/Set, lifecycle hooks, lazy/predeclared instances,
  dynamic invocation, object Variants, identity, and managed cleanup; see the
  [class contract](../llvm/CLASSES.md).
- On Error policies, Resume, and intrinsic Err properties/methods; see the
  [error contract](../llvm/ERROR_HANDLING.md).
- Fixed/dynamic arrays of supported scalar/UDT types, indexing, ReDim/Preserve,
  Erase, bounds/type queries, array returns/copies, and whole-array/element ByRef
  aliases; see the [array contract](../llvm/ARRAYS.md).

Other constructs produce diagnostics: unsupported loop forms, fixed-length strings, String content writes beyond Mid assignment,
unsupported Variant/array operations/subtypes, UDT/UDT-array Variant boxing, deferred class features and COM,
auto-instantiated (`As New`) array elements, dynamic UDT value boundaries, procedure Static storage, unsupported Const types,
array/UDT Optional parameters, external calls,
and other operators.
These are front-end capabilities in several cases; compiler support is narrower.
No source construct is silently skipped as a code-generation fallback.

Mid uses `OpMid` for an owned substring, `OpMidCheck` before evaluating a statement's
replacement, and `OpMidAssign` for checked content mutation. All return Result/Try
values with explicit effects. See the [Mid contract](../llvm/MID.md).

## Verification and tests

`ir.Verify` checks resolved types, value identity and dominance within regions,
scalar storage and ByRef compatibility, parameter and return contracts, literal
ranges, effects, Result handling, immediate success/error continuations, region
retry/next targets, supported operations, and source provenance. It also accepts
malformed IR containers/payloads as input and diagnoses them without panicking.
`Lower` always runs it before returning IR. Consumers that transform IR must verify
again; `ir.Format` is a debug listing for verified programs, not a serialization.

Run `go test ./ir ./lowering`. Mutation tests exercise verifier rejection, and a
small independent scalar test interpreter checks observable overflow, aliases,
argument order, default result values, and assignment commit behavior. In a failed
callee, completed ByRef stores survive while the caller's pending assignment does
not commit. These tests establish the IR's contract without requiring LLVM; they
are not yet differential conformance tests against a VBA/VB6 host.

Virtual assertion objects, Set/Nothing/Is, lazy As New, and native test calls now
have verified private IR operations; see the [test runner contract](../testrunner/README.md).
These handles do not extend the Automation VARIANT ABI with private COM pointers.


Native object calls use `OpInvocationCapture` before later argument evaluation.
It retains an object or locks an array. The verifier permits these private captures
only as invocation/index receivers. `OpObjectInvoke` retains invocation kind and
argument names while consuming Variant values or constrained typed ByRef views.
`OpObjectValue` performs an erased object's default value chain. `OpObjectAutoLoad`
and slot/global/field AutoClass metadata describe lazy class construction.
Public member metadata must match the typed private signature, and default
procedures must have the expected parameterless result ABI. Lifecycle and dynamic
callback effects propagate into callers and managed cleanup operations.
