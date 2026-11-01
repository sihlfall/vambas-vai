# VBA standard-library integration plan and inventory

Status: stages A and D in progress. Stage D now includes Byte conversions,
Error/Decimal Variants, and initial Date/Currency execution profiles. The initial
stage B procedural-module namespaces are implemented
for the five modules represented by current declarations. The remaining stages
are a proposed implementation roadmap. Inventory baseline
annotations describe the source inspected on 26 October 2026; they are not a full
conformance certification.

This merges the integration plan with every entry in
[the MS-VBAL section 6 name index](VBA_Standard_Library_MS-VBAL_Section_6.md).
The original index remains the reference inventory. Its descriptions, values,
specification links, and version caveat are retained below.

## Objective and scope

Make most standard-library functions available to the checker and compiler while
keeping `binder/intrinsics.go` a small assembly entry point. Adding an ordinary
function should normally require a declaration descriptor, a runtime helper, an
ABI entry, and behavioral tests, without adding a dedicated opcode or branches
throughout the compiler.

Deliver broad declaration/checker coverage independently of executable coverage.
Preserve VBA lookup, conversions, argument evaluation, storage identity, ownership,
and error behavior. Diagnose a valid but unavailable operation at compilation with
its qualified name and missing capability.

The inventory covers 160 function-name entries, 13 subroutine-name entries,
7 property-name entries, 14 class members, 124 enum constants, and 145 module
constants. `$` and byte-oriented `B` spellings remain separate entries. The
inventory's total of 494 also includes its 31 enum/module/class category names.
Host object models are outside this scope. Language constructs and built-ins
outside section 6 need a separate companion list.

## Current foundations and scaling problems

- [binder/intrinsics.go](binder/intrinsics.go) and
  [binder/library.go](binder/library.go) construct
  ordinary ambient declarations and symbols, fresh per project. Retain that model.
- [ast/symbol.go](ast/symbol.go) identifies library declarations through
  `stdlib.MemberID`. The separate testing library retains the legacy
  `IntrinsicKind`, currently `uint8`; migrated VBA identities remain reserved in
  that enum and are no longer attached to declarations.
- [lowering/library.go](lowering/library.go) routes all catalog calls by member
  identity, including specialized string, Variant, array-bound, Err, and object
  creation operations. Receiver-aware Collection calls retain their specialized
  path. Runtime selectors use explicit mappings rather than intrinsic ranges.
- [checker/library.go](checker/library.go) now isolates existing `InStr`,
  `Len`/`LenB`, and `Mid`/`Mid$` policies by catalog member ID. Ordinary argument
  checking remains in [checker/calls.go](checker/calls.go).
- Ordinary helpers now use `OpLibraryCall`: [IR contracts](ir/library.go) derive
  arity, result types, and base effects from the catalog, while
  [LLVM ABI metadata](llvm/library.go) owns native names and selectors. Richer
  argument and runtime profiles remain incremental work.
- [binder/project.go](binder/project.go) now identifies ambient containers through
  their owning library root rather than a fixed name list or intrinsic ranges.
  The current procedural modules support `VBA.Strings.Left`-style qualification;
  broader enum/class metadata remains incremental work.
- Byte storage, arrays, and direct String ↔ Byte() assignment now execute; see
  the [conversion contract](llvm/ARRAYS.md#string-and-byte-assignment). `CByte`,
  `vbByte`, and Error Variant operations now execute under the
  [Stage D data-type contract](llvm/DATA_TYPES.md).
- Date and Currency now have distinct executable scalars and array/field storage;
  Decimal remains a Variant subtype. LongLong and LongPtr still need executable
  representations and an explicit target profile beyond adding declarations.

## Stage A progress

Implemented so far:

- A frontend/backend-independent [stdlib catalog](stdlib/catalog.go) with explicit
  `uint16` member IDs, language types, literal defaults, module/enum ownership,
  and specification section references. Readers receive independent descriptor
  and parameter slices.
- Migration of 29 existing functions: 13 string functions, seven conversions,
  five information functions, `IIf`, `CreateObject`, and the `LBound`/`UBound`
  language built-ins.
  All 11 previously declared library constants are also catalog-backed. This adds
  no new executable function behavior and preserves current signature and
  runtime-profile restrictions.
- `binder/intrinsics.go` now only assembles the catalog and separate testing
  library. Constant data lives in [stdlib/constants.go](stdlib/constants.go).
  The catalog distinguishes null-string values from ordinary empty strings;
  lowering recognizes `vbNullString` by catalog identity. Negative constants
  retain unary-minus syntax and their existing declared arithmetic types.
- Explicit ByRef parameter metadata, Boolean/Single/Double declaration types,
  and separate tracking for language built-ins outside the section 6 inventory.
  Omitted Mid Length retains Missing; array bounds retain a typed ByRef view and
  a ByVal Long Dimension default of 1.
- Fresh declaration/default AST nodes and function/parameter/constant symbols in
  [binder/library.go](binder/library.go), retaining existing unqualified and
  `VBA.` exports and distinct `$` forms.
- Catalog IDs are now the sole declaration identities used for checking and
  lowering the migrated functions and constants. The temporary per-member
  `catalogIntrinsic` bridge was removed from the binder; catalog symbols have
  `IntrinsicNone`. Legacy enum values remain reserved so the separate testing
  library's identities retain their numbers.
- Checker call policies now select catalog IDs in `checker/library.go`, keeping
  ordinary argument binding free of individual built-in identities. Existing
  InStr argument mapping and Len/Mid result refinements retain their original
  timing and leave selected declaration signatures intact. Regression cases
  cover shadowing user functions (including `$` calls), diagnostics, and useful
  result information on invalid calls.
- Explicit routing of noncatalog intrinsic calls, replacing range-based routing
  in the intrinsic-call entry point. Catalog membership alone does not select the
  Variant-snapshot helper ABI: specialized calls keep existing typed and
  storage-aware operations.
- A dedicated `lowering/library.go` dispatches ordinary runtime helpers and
  specialized operations by catalog ID. Len/LenB, ChrW, Mid/Mid$, conversions,
  type tests, array bounds, and `vbBinaryCompare` retain their current IR and
  behavior. Scope tests require catalog identities without duplicate legacy tags;
  checker identity assertions use the same contract.
- Nine existing helper forms now use generic `OpLibraryCall`: AscW, StrComp,
  InStr, Left/Left$, String/String$, Split, and IIf. Declarations opt into
  `SnapshotCall`; a shared IR signature derives their arity, exact result type,
  and base effects. Argument evaluation, coercions, and snapshots retain their
  existing behavior. Specialized operations remain on their existing paths.
- `llvm/library.go` supplies native helper names and ABI selectors and generates
  helper declarations. The backend rejects unavailable members before emission.
  The verifier rejects unknown/specialized/constant IDs, wrong arity and result
  types, references instead of snapshots, misplaced member payloads, missing Try,
  and incorrect effects. Catalog/ABI coverage tests and malformed-IR cases cover
  the new contract. Old per-function helper opcodes remain reserved and are
  rejected by verification; default-member and termination effects still apply.
- Catalog integrity/isolation tests, declaration-parent and symbol-ownership
  checks across ambient scopes, and checker assertions for catalog identity and
  user shadowing. Existing behavioral tests continue to cover call contracts.
- Qualified/named call-plan checks for every newly migrated function, including
  Missing versus typed defaults and array ByRef storage preservation. A combined
  lowering regression exercises the specialized catalog paths.
- Constant regressions cover UTF-16 control characters, embedded NUL, the signed
  `vbObjectError` value and enum expressions, user shadowing and `VBA.` lookup,
  null-string aliases, and ordinary empty strings produced by constant
  concatenation. Existing scope-isolation checks now cover all catalog constants.
- [Class descriptors](stdlib/classes.go) now cover Collection, its Add/Remove
  Subs and Item/Count getters, and Err's existing singleton, writable state, and
  Raise/Clear Subs. The shared builder materializes fresh members, parameters,
  parent links, qualified aliases, and Collection's default member. Nested
  descriptor slices and mutable declaration trees remain isolated across readers
  and project scopes. This preserves Err's synthetic namespace/variable model;
  it does not introduce a constructible Err type or accessor conformance changes.
- Collection and Err checking/lowering now use catalog identities exclusively.
  Collection invocation codes, Err field selectors, and virtual class codes are
  explicit mappings independent of catalog and legacy enum numbering. The final
  intrinsic-range dispatch, including the separate testing-class mapping, is
  removed. Specialized verified operations and their effects remain unchanged.
- `CreateObject` moved out of the testing declaration builder into the Interaction
  catalog, with `VBA.Interaction.CreateObject` and `Interaction.CreateObject`
  aliases. Its existing Class-only signature and restricted virtual testing-object
  runtime profile remain unchanged. Tests cover alias/signature identity, user
  shadowing, named Err arguments and Missing, and rejection of specialized object
  identities by generic library-call verification. Native regressions exercise
  Collection, qualified object creation, all five Err field selectors, default
  Number reads, Clear, and named Raise. Editor semantic tokens also recognize
  catalog identity, preserving Err's standard-library highlighting even though
  its synthetic namespace has no declaration node.

Stage A validation before the Stage D increment, on 27 October 2026: the full
`go test -timeout 20m ./...` suite passed
with `VBA_LLVM_TEST_REQUIRED=1` and LLVM 20 installed, including native execution
at `-O0` and `-O2`.

Still pending in A: richer descriptor forms (ParamArray, property setters and
broader class/accessor metadata), richer default policies, and extensions beyond
the initial Variant-snapshot library-call contract. Existing VBA object/error
families are migrated; the independent testing library intentionally retains its
own identities. Current lowering continues to emit specialized verified operations
alongside generic library calls.

## Stage B progress

- **Constants complete for the pinned inventory:** all 269 constants (124 enum
  members and 145 module constants) have catalog identities, values, declarations,
  checker support, and native materialization. No UI, filesystem, locale, or other
  service implementation is required to declare the constants describing it.
- All 16 predefined enums materialize as fresh enum declarations, usable both as
  Long-backed type annotations and as qualification scopes. Their members have
  the enum as canonical owner and share declarations with their root aliases.
- Procedural-module descriptors now materialize fresh `Strings`, `Conversion`,
  `Information`, `Interaction`, `Constants`, `ColorConstants`, `KeyCodeConstants`,
  and `SystemColorConstants` namespace symbols and export tables. Functions and
  module constants have their defining module as canonical
  symbol parent. Unqualified, `VBA.`-qualified, module-qualified, and fully
  qualified exports alias the same declaration and checker signature.
- Lookup preserves ordinary project/local shadowing and distinct `$` forms.
  Ambient containers are recognized by library-root identity; user modules with
  matching names do not become ambient containers. Missing members stay missing
  in the selected module, and module names remain namespace-only values.
- Checker tests cover qualification and case folding across the existing module
  families, named arguments, constant values, user-module shadowing, missing and
  wrong-owner members, and invalid namespace value reads. Scope-isolation tests
  include module symbols and ownership. A native fixture exercises helper and
  specialized calls through the new namespaces.
- An inventory test compares every constant's name, value, owner, and section
  against the pinned source index, with per-owner counts. Checker coverage uses
  every constant through all four lookup forms and every numeric constant in a
  user enum. Native coverage materializes every numeric constant through all four
  forms, plus every string constant, signed system colors, and combined flags.
- The current declaration profile gives numeric constants type `Long`, preserving
  32-bit signed system-color bit patterns. String constants retain their exact
  control characters and `vbNullString` retains its null-pointer identity.
  `vbNewLine` is explicitly CRLF for the Windows-oriented profile, independent of
  the compiler host OS. `vbLongLong = 20` is included as a Long type-code constant
  in the 64-bit frontend profile; executable LongLong/LongPtr support remains
  Stage D work. These declarations do not assert Windows type-library conformance.

Constant validation on 27 October 2026: all non-LLVM Go packages passed; targeted
LLVM 20 regressions passed with native tools required, including the complete
constant fixture at `-O0` and `-O2`.

The remaining procedural modules will be introduced with their declarations.
Language built-ins such as LBound/UBound retain their existing root exports.
Remaining B work concerns procedure signatures, properties/accessors, and broader
class metadata and checker policies; the constant inventory has no deferred rows.

## Stage D progress

The first increment completes Byte conversion and exposes Error Variant values;
it does not complete the other Stage D representation families.

- Catalog declarations and module/root aliases for `CByte(Expression) As Byte`,
  `CVErr(Expression) As Variant`, and `IsError(Arg) As Boolean`, plus `vbByte` and
  `vbError` constants. Existing scope-isolation, ownership, and ABI integrity
  checks cover the new declarations; named calls and user shadowing are tested.
- `CByte` uses the existing unsigned 8-bit storage/Variant/array representation,
  ties-to-even rounding, and checked coercion. Overflow, Null, and invalid input
  preserve the destination and retain runtime errors 6, 94, and 13 respectively.
  Boolean True converts to Byte 255 across explicit, implicit and ByRef writes.
- Error values use Automation `VT_ERROR` and a signed Long code, independently of
  the runtime status/Err channel. `CVErr` preserves an existing Error (including
  Missing); other inputs are coerced to Long and must be in 0..65535, otherwise
  error 5 is raised after successful coercion. `IsError` tests the value subtype.
- Explicit Byte/Integer/Long/Boolean/Single/Double/String conversions now use
  generic verified catalog calls and a shared native conversion helper. They
  extract an Error's code, with `CStr` producing `"Error <code>"`. Implicit
  assignment and typed ByRef stores continue to reject Error with error 13.
  Ordinary conversion profiles and source evaluation order are preserved.
- Arithmetic/concatenation rejects Error before Null propagation. Relational
  operations compare two standard Error codes numerically and reject mixed
  Error/non-Error operands. Comparisons involving implementation-defined Error
  codes (such as Missing) deterministically raise error 13 in this profile.
- Native tests cover Byte boundaries, returns and aliases; Error snapshots,
  arrays, Collection items, ByVal/ByRef, ParamArray and Missing; explicit/implicit
  coercion differences; comparison and failure order; and output preservation.
  C ABI tests check byte-sized writes, borrowed Error views, nonfinite inputs,
  allocation failures and cleanup for `CStr(Error)` at `-O0` and `-O2`.

The [data-type contract](llvm/DATA_TYPES.md) records exact behavior and sources.
Validation on 27 October 2026: all Go packages outside the LLVM suite passed;
39 selected LLVM checks passed with `VBA_LLVM_TEST_REQUIRED=1`, covering the new
Stage D fixtures and existing Byte/Variant, library, coercion and runtime ABI
regressions. The corrected Boolean-to-Byte cases and allocation-failure matrix
were also rerun successfully through original/optimized LLVM and native `-O0`/`-O2`.
The second increment adds Date, Currency, and Decimal:

- `CDate`/`CVDate`, `CCur`, `CDec`, and the `vbDate`/`vbCurrency`/`vbDecimal`
  constants are catalog-backed. Existing qualification, signature, shadowing,
  descriptor isolation and generic IR/ABI coverage checks include these entries.
- Date is an eight-byte Automation `DATE` serial; Currency is a signed eight-byte
  `CY` integer scaled by 10,000. Typed locals/globals, UDT/class fields, calls and
  returns, ByRef Variant views and eight-byte typed SAFEARRAY cells preserve
  identity and storage. Currency `@` literals use exact compiler-side decimal
  evaluation, including the signed minimum, without a Double round-trip.
- Decimal uses the existing Variant's Windows-compatible 16-byte DECIMAL overlay:
  96 coefficient bits, scale 0..28, and sign 0/0x80. Standalone/by-reference
  DECIMAL reserved words are zero; owned Variants overlay that word with tag 14.
  Windows builds assert these layouts against SDK Date/Currency/Decimal types.
- Currency coercion/addition/subtraction/multiplication and Decimal coercion,
  addition/subtraction/multiplication/division/comparison use integer arithmetic.
  Wider intermediates, ties-to-even scale reduction and checked commits preserve
  exact values and destination contents on errors. Decimal arithmetic has an
  independent 160-case arbitrary-precision rational reference check at `-O0`/`-O2`.
- Date numeric/ISO conversion validates range and calendar dates; arithmetic,
  comparisons and conversions preserve its tag and stored serial. The current
  text profile accepts ISO date/time and numeric input and produces ISO text at
  whole-second resolution. Locale-specific formats, calendar library functions,
  date literals, explicit Date/Currency optional defaults and Const evaluation
  remain deferred. Omitted typed optional parameters retain zero defaults.
- Native fixtures cover exact boundaries, scalar/array/field storage, static and
  dynamic calls, snapshots/aliases and partial-write behavior. C ABI checks cover
  field offsets, native versus Variant DECIMAL reserved words, borrowed views,
  checked writes and allocation failures for Date/Currency/Decimal string output.

See the [data-type contract](llvm/DATA_TYPES.md) for supported operations and
restrictions. Windows memory layouts are the target; execution against Windows
and VBA/VB6 remains unvalidated. LongLong and LongPtr are still pending, and the
native Variant target remains explicitly 64-bit. Numeric For-counter expansion
and broader locale/formatting behavior remain incremental work.

Second-increment validation on 27 October 2026: all packages outside LLVM passed,
and 43 selected LLVM checks passed with required LLVM tools. New native fixtures
cover Date/Currency/Decimal storage, arithmetic, calls and failures; an independent
160-case Decimal rational reference and C ABI/allocation-failure checks execute at
`-O0` and `-O2`. Full Windows runtime/locale conformance is not asserted.

## Architecture

### One declarative catalog, split by VBA module

Start with typed Go descriptors rather than a new source language or generator:

```text
stdlib/
  catalog.go             stable IDs and descriptor types
  constants.go           previously assigned constant identities and values
  constant_ids.go        additional stable constant identities
  constants_inventory.go remaining inventory constant values
  enums.go               enum definitions (members live in Constants())
  conversion.go
  datetime.go
  filesystem.go
  financial.go
  information.go
  interaction.go
  math.go
  strings.go
  classes.go             Collection, Err, Global declarations
binder/
  library.go             descriptor-to-AST/symbol adapter
  intrinsics.go          scope assembly only
checker/
  library.go             exceptional call policies
lowering/
  library.go             argument preparation and library dispatch
llvm/
  library.go             native ABI descriptors and emission
runtime/
  stdlib_strings.h / .c
  stdlib_math.h / .c
  stdlib_conversion.h / .c
  stdlib_datetime.h / .c
  stdlib_financial.h / .c
```

Keep `stdlib` independent of binder, checker, IR, and LLVM types. Use its own
language-level type/default descriptors; adapters translate them at each layer.
Give each declaration an explicit stable member ID with enough capacity, such as
`uint16`, independent of declaration enumeration order. Migrate existing intrinsic
identities incrementally; avoid two competing sources of semantic truth.

Each descriptor records its owner, name, declaration kind, exact formal names,
language types, array shape, ByVal/ByRef, Optional/ParamArray, return type,
accessors, constant value, and specification reference. Defaults must distinguish
explicit values, Missing, implicit typed defaults, and caller-dependent values.
Exceptional binding/result policies are small identifiers, not cross-package
checker callbacks.

The catalog is immutable. Materialize fresh AST nodes, parent links, symbols,
parameter symbols, and defaults for each Program. Never share mutable declaration
trees between project snapshots. The testing library remains a separate ambient
library from VBA.

### Names and checker behavior

Build the actual project/module/enum/class hierarchy. Unqualified and qualified
exports refer to the same declaration, preserving ordinary shadowing and lookup:

```vb
Left$("abc", 1)
VBA.Left$("abc", 1)
VBA.Strings.Left$("abc", 1)
```

Preserve case-insensitive lookup and distinct library `$` forms. Model enum types
and values, properties and their supported accessors, and class/default-member
metadata as declarations rather than treating all names as free functions.

Reuse the existing checker for argument names/counts, conversions, ByRef storage,
Optional arguments, ParamArray, properties, and default members. Put only genuine
library exceptions in `checker/library.go`, selected by resolved declaration
identity. User functions with the same spelling must never receive these rules.

Audit exact declared result types against the pinned specification. Keep
representation optimizations separate from language-visible typing. Preserve
existing behavior during structural migration; review semantic corrections as
explicit subsequent changes.

### A verified library-call IR contract

The initial `OpLibraryCall` contract is implemented with a stable library member
ID and typed operands. Eligible ordinary functions share this path. Expand its
prepared-argument contracts as needed for additional families.
The verifier checks member identity, arity, operand types/value-reference modes,
result type, ownership, effects, and required error continuations. Invalid or
unsupported IDs must fail verification before backend emission.

Keep dedicated operations for semantics that need them: storage references,
conversions, array bounds, Mid assignment, Err state, and object invocation. A
catalog entry can select an existing specialized path; not every declaration
must use `OpLibraryCall`.

Lower from `CallResolution` and its conversion/storage plans. Evaluate explicit
arguments in the specified order, retain snapshots or storage references as
required, then arrange operands into formal order. Preserve Missing, ParamArray,
ByRef aliases and supported copy-back, managed lifetimes, and partial side effects
when a later argument fails. Variant snapshots are a useful ABI choice for some
functions, not a universal replacement for typed values or references.

Capture caller-dependent defaults where needed. Comparison defaults may depend on
the source module; array behavior outside this inventory may depend on
`Option Base`. Ambient declarations must not be evaluated as source-file code.
`IIf`, `Choose`, and `Switch` must preserve their specified eager evaluation.

### Runtime ABI and effects

Keep C helper names, layouts, and native argument conventions in the backend ABI
adapter. Typed helpers may avoid boxing when that preserves semantics. Reuse the
status/output convention and Result/Try error edges: successful managed outputs
have explicit ownership, failures preserve output guarantees, and partial
allocations are cleaned up.

Runtime families now compile as separate `.c` translation units, listed explicitly
in `runtime/sources.txt`. Shared implementation helpers are declared in the private
`runtime/runtime_internal.h`; file-local helpers and state remain static. Build
clients compile the manifest sources together, preserving public helper names and
layouts. See [runtime build instructions](runtime/README.md).

Extend effects before enabling stateful functions. RNG, clocks, directory search,
file tables, environment and host calls must not be treated as pure or reordered
across observable operations. Define state ownership and reset/lifetime rules for
the relevant runtime context, consistent with existing ambient error state.

## Delivery stages and acceptance criteria

| Stage | Work | Acceptance |
|---|---|---|
| A | Catalog and declaration builder; migrate existing built-ins; remove enum-range dispatch; establish generic library-call IR/ABI. | Existing behavior and qualified/shadowed calls remain covered; adding an ordinary function no longer requires a dedicated opcode. |
| B | Inventory-wide declarations, enum/constants, namespaces, signatures, properties, and checker policies. | Every inventory entry is accounted for; available declarations are checked; incomplete contracts remain explicitly deferred rather than fabricated. |
| C | Portable core strings, conversions, information/color functions, math, eager selectors. | Functions execute with documented supported value types, Null/Missing behavior, errors, and ownership. |
| D | Byte, Date, Currency, Error/Decimal Variants, and target-dependent wider integers. | Representations, storage, coercions, boundaries, and dependent library functions work; pointer width follows an explicit target profile. |
| E | Date arithmetic and financial functions; formatting, locale/encoding-sensitive and byte-oriented strings. | Each shipped operation has a defined profile and conformance cases; unsupported modes are explicit. |
| F | RNG, clocks, environment, directories, and file services. | State/effects are modeled; file-number operations share the file-statement subsystem; portable/Windows differences are explicit. |
| G | UI, settings, process/window services, events, COM, and form lifecycle. | Required host adapters exist and are validated; object work follows the class/COM plan. |

All planned entries first need stage B declaration/checker work. Inventory stage
annotations below indicate the proposed executable delivery, not permission to
skip that frontend work. Existing entries migrate in A and retain their current
limits. `A + E`, for example, preserves an existing subset in A and expands it in E.
Stages express dependency groups; independent families can be delivered separately.

Important family dependencies:

- Date functions need Date representation and coercion before date arithmetic,
  parsing, calendar behavior, or clock properties can be considered complete.
- Currency and Decimal require their own arithmetic/coercion rules. Mapping every
  value to Double would change observable behavior. LongPtr is target-dependent.
- `Format`, text comparison, case conversion, `StrConv`, and `B` functions need
  explicit locale, code-page, UTF-16/byte semantics. Do not inherit Go defaults.
- Financial functions need checked domains and convergence behavior, including
  array inputs and failure cases for iterative algorithms.
- `Rnd`/`Randomize`, `Dir`, and file numbers have shared state contracts. `EOF`,
  `Loc`, `LOF`, `Seek`, `FileAttr`, and `FreeFile` depend on the file subsystem;
  syntax such as Open/Close is outside this inventory but is a real prerequisite.
- `CallByName` can share native object invocation infrastructure before general
  COM support. `CreateObject` currently has a catalog declaration with a restricted
  virtual testing-object runtime path;
  broader activation and `GetObject` follow
  [CLASS_COM_SUPPORT_PLAN.md](CLASS_COM_SUPPORT_PLAN.md).
- Keep Collection and Err execution during migration. Do not conflate existing
  native class/object behavior with completed Windows COM or host services.

## Coverage tracking and validation

Track declaration availability, checker support, portable execution, Windows
execution, and restrictions independently. Runtime availability belongs to the
selected target/capabilities, not a mutable supported flag on a declaration symbol.
Report unavailable compilation by qualified member and required capability.

Measure function coverage against all 160 indexed spellings, with subs, properties,
class members, and constants counted separately. The final executable count is
established by validated implementations; this roadmap does not label all catalog
entries executable. Broader names found outside section 6 must not silently change
this denominator.

Validate catalog IDs, owner links, duplicate/colliding exports, distinct `$` forms,
formal/default consistency, and exact inventory coverage. Validate ABI coverage
for executable entries and make unsupported entries explicit.

Behavioral checks should include frontend lookup/shadowing and call plans,
malformed library-call IR, original/optimized LLVM execution, runtime errors,
Null/Empty/Missing distinctions, eager selection and argument side effects,
embedded NUL/surrogates, array bounds and ownership, ByRef mutation, allocation
failure cleanup, and targeted reference-host comparisons. Run existing relevant
frontend/lowering/runtime checks per change and the repository's full checks at
integration milestones. Windows-only behavior needs Windows evidence.

The source index is a name list, not a complete signature or behavior specification.
Before implementing each family, verify its exact contracts against the pinned
MS-VBAL revision and record unresolved details. Retain the source index's version
caveat; this merge does not verify a newer specification revision.

Recommended PR sequence:

1. Catalog, fresh declaration materialization, existing built-in migration, and
   explicit dispatch. Preserve execution behavior and the separate testing library.
2. Generic verified library-call IR and ABI adapter; migrate a representative
   existing function and its String/Variant variants, then the remaining ordinary
   helpers. Retain specialized operations where required.
3. Complete namespace/constants/enums and incrementally verified family signatures.
4. Deliver portable core families, then value-type foundations and their dependent
   functions. Broaden locale/formatting, financial, stateful, and host families in
   the staged order above.

## Annotated complete inventory

The following tables retain every source inventory row and its original value or
description and link. Two columns connect each row to this implementation plan:

- **Baseline:** `Existing subset` means a declaration and execution path were
  identified in the inspected code, with existing documented restrictions;
  `Existing constant` means the name/value has an ambient constant declaration
  (it does not imply its enclosing enum is already modeled);
  `Planned` means no corresponding existing path was identified in this review.
  These are source-review annotations, not results of a conformance test suite.
- **Delivery:** A–G refer to the stages above. `B (constant)` includes declaration,
  checking, and constant materialization; defining a system/UI constant does not
  implement the service it describes. All 269 constant rows are now marked
  `B (implemented constant)`; the Baseline column retains the original review
  state. Extra text identifies significant dependencies.

### Companion language-built-in coverage outside the source index

The existing `LBound` and `UBound` declarations and array-bound lowering migrate in
A; they do not appear in the supplied section 6 index. Track `Array` and any other
language-level operations outside that index separately, including qualification,
Option Base, ParamArray and ownership semantics as applicable. This companion
list does not claim completeness for sections outside section 6.

## Reference inventory — MS-VBAL section 6

**Sources:** [Microsoft Learn — §6 VBA Standard Library](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/c645c903-9bd4-4849-8735-3136e867536a); [MS-VBAL specification, v20241112 (PDF)](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf); [specification version history](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/d5418146-0bd2-45eb-9c7a-fd9502722c74).

**Scope:** All names explicitly defined in §6.1 of the linked Microsoft *VBA Language Specification*: predefined enums and their members; global procedural modules, their constants, functions, subroutines, and properties; and predefined classes/members. Names ending in `$` and byte-oriented (`B`) variants are indexed individually. Host application object models (Excel, Word, Access, etc.) and syntax/operators elsewhere in the language specification are **not** part of this inventory.

**Version note:** This is checked against the **12 November 2024 (revision 2.2)** specification, whose PDF is accessible page-by-page. Microsoft lists later revisions (including 18 August 2026); changes made only in those newer revisions have not been independently verified here. Clicking any name opens the applicable numbered PDF page. Values are shown for constants; descriptions are deliberately brief.

### Reference inventory contents

- [1. Predefined enums and constants (§6.1.1)](#1-predefined-enums-and-constants-611)
- [2. Predefined procedural modules (§6.1.2)](#2-predefined-procedural-modules-612)
- [3. Predefined classes (§6.1.3)](#3-predefined-classes-613)

## 1. Predefined enums and constants (§6.1.1)

### 6.1.1.1 [`FormShowConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — Form display mode

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbModal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` | Planned | B (implemented constant) |
| [`vbModeless`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `0` | Planned | B (implemented constant) |

### 6.1.1.2 [`VbAppWinStyle`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — External application window style

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbHide`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `0` | Planned | B (implemented constant) |
| [`vbMaximizedFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `3` | Planned | B (implemented constant) |
| [`vbMinimizedFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `2` | Planned | B (implemented constant) |
| [`vbMinimizedNoFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `6` | Planned | B (implemented constant) |
| [`vbNormalFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` | Planned | B (implemented constant) |
| [`vbNormalNoFocus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `4` | Planned | B (implemented constant) |

### 6.1.1.3 [`VbCalendar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — Calendar type

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbCalGreg`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `0` | Planned | B (implemented constant) |
| [`vbCalHijri`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` | Planned | B (implemented constant) |

### 6.1.1.4 [`VbCallType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) — `CallByName` invocation type

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbGet`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `2` | Planned | B (implemented constant) |
| [`vbLet`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `4` | Planned | B (implemented constant) |
| [`vbMethod`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `1` | Planned | B (implemented constant) |
| [`vbSet`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=174) | `8` | Planned | B (implemented constant) |

### 6.1.1.5 [`VbCompareMethod`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — String comparison type

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbBinaryCompare`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` | Existing constant | B (implemented constant) |
| [`vbTextCompare`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `1` | Planned | B (implemented constant) |

### 6.1.1.6 [`VbDateTimeFormat`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — Built-in date/time display formats

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbGeneralDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` | Planned | B (implemented constant) |
| [`vbLongDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `1` | Planned | B (implemented constant) |
| [`vbLongTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `3` | Planned | B (implemented constant) |
| [`vbShortDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `2` | Planned | B (implemented constant) |
| [`vbShortTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `4` | Planned | B (implemented constant) |

### 6.1.1.7 [`VbDayOfWeek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — Day-of-week and first-day settings

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbFriday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `6` | Planned | B (implemented constant) |
| [`vbMonday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `2` | Planned | B (implemented constant) |
| [`vbSaturday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `7` | Planned | B (implemented constant) |
| [`vbSunday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `1` | Planned | B (implemented constant) |
| [`vbThursday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `5` | Planned | B (implemented constant) |
| [`vbTuesday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `3` | Planned | B (implemented constant) |
| [`vbUseSystemDayOfWeek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` | Planned | B (implemented constant) |
| [`vbWednesday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `4` | Planned | B (implemented constant) |

### 6.1.1.8 [`VbFileAttribute`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) — File attribute flags

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbNormal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=175) | `0` | Planned | B (implemented constant) |
| [`vbReadOnly`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `1` | Planned | B (implemented constant) |
| [`vbHidden`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `2` | Planned | B (implemented constant) |
| [`VbSystem`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `4` | Planned | B (implemented constant) |
| [`vbVolume`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `8` | Planned | B (implemented constant) |
| [`vbDirectory`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `16` | Planned | B (implemented constant) |
| [`vbArchive`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `32` | Planned | B (implemented constant) |
| [`vbAlias`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `64` | Planned | B (implemented constant) |

### 6.1.1.9 [`VbFirstWeekOfYear`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) — First-week-of-year rules

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbFirstFourDays`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `2` | Planned | B (implemented constant) |
| [`vbFirstFullWeek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `3` | Planned | B (implemented constant) |
| [`vbFirstJan1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `1` | Planned | B (implemented constant) |
| [`vbUseSystem`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `0` | Planned | B (implemented constant) |

### 6.1.1.10 [`VbIMEStatus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) — Input Method Editor modes

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbIMEAlphaDbl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `7` | Planned | B (implemented constant) |
| [`vbIMEAlphaSng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `8` | Planned | B (implemented constant) |
| [`vbIMEDisable`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `3` | Planned | B (implemented constant) |
| [`vbIMEHiragana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `4` | Planned | B (implemented constant) |
| [`vbIMEKatakanaDbl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `5` | Planned | B (implemented constant) |
| [`vbIMEKatakanaSng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `6` | Planned | B (implemented constant) |
| [`vbIMEModeAlpha`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `8` | Planned | B (implemented constant) |
| [`vbIMEModeAlphaFull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `7` | Planned | B (implemented constant) |
| [`vbIMEModeDisable`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `3` | Planned | B (implemented constant) |
| [`vbIMEModeHangul`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `10` | Planned | B (implemented constant) |
| [`vbIMEModeHangulFull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `9` | Planned | B (implemented constant) |
| [`vbIMEModeHiragana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `4` | Planned | B (implemented constant) |
| [`vbIMEModeKatakana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `5` | Planned | B (implemented constant) |
| [`vbIMEModeKatakanaHalf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=176) | `6` | Planned | B (implemented constant) |
| [`vbIMEModeNoControl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` | Planned | B (implemented constant) |
| [`vbIMEModeOff`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` | Planned | B (implemented constant) |
| [`vbIMEModeOn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1` | Planned | B (implemented constant) |
| [`vbIMENoOp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` | Planned | B (implemented constant) |
| [`vbIMEOff`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` | Planned | B (implemented constant) |
| [`vbIMEOn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1` | Planned | B (implemented constant) |

### 6.1.1.11 [`VbMsgBoxResult`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) — Message-box button results

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbAbort`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `3` | Planned | B (implemented constant) |
| [`vbCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` | Planned | B (implemented constant) |
| [`vbIgnore`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `5` | Planned | B (implemented constant) |
| [`vbNo`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `7` | Planned | B (implemented constant) |
| [`vbOK`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1` | Planned | B (implemented constant) |
| [`vbRetry`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `4` | Planned | B (implemented constant) |
| [`vbYes`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `6` | Planned | B (implemented constant) |

### 6.1.1.12 [`VbMsgBoxStyle`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) — Message-box flags (buttons, icons, modality and position)

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbAbortRetryIgnore`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `2` | Planned | B (implemented constant) |
| [`vbApplicationModal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` | Planned | B (implemented constant) |
| [`vbCritical`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `16` | Planned | B (implemented constant) |
| [`vbDefaultButton1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `0` | Planned | B (implemented constant) |
| [`vbDefaultButton2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `256` | Planned | B (implemented constant) |
| [`vbDefaultButton3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `512` | Planned | B (implemented constant) |
| [`vbDefaultButton4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `768` | Planned | B (implemented constant) |
| [`vbExclamation`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `48` | Planned | B (implemented constant) |
| [`vbInformation`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `64` | Planned | B (implemented constant) |
| [`vbMsgBoxHelpButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `16384` | Planned | B (implemented constant) |
| [`vbMsgBoxRight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `524288` | Planned | B (implemented constant) |
| [`vbMsgBoxRtlReading`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=177) | `1048576` | Planned | B (implemented constant) |
| [`vbMsgBoxSetForeground`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `65536` | Planned | B (implemented constant) |
| [`vbOKCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `1` | Planned | B (implemented constant) |
| [`vbOKOnly`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `0` | Planned | B (implemented constant) |
| [`vbQuestion`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `32` | Planned | B (implemented constant) |
| [`vbRetryCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `5` | Planned | B (implemented constant) |
| [`vbSystemModal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4096` | Planned | B (implemented constant) |
| [`vbYesNo`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4` | Planned | B (implemented constant) |
| [`vbYesNoCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `3` | Planned | B (implemented constant) |

### 6.1.1.13 [`VbQueryClose`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) — Form-closing reasons

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbAppTaskManager`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `3` | Planned | B (implemented constant) |
| [`vbAppWindows`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `2` | Planned | B (implemented constant) |
| [`vbFormCode`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `1` | Planned | B (implemented constant) |
| [`vbFormControlMenu`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `0` | Planned | B (implemented constant) |
| [`vbFormMDIForm`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4` | Planned | B (implemented constant) |

### 6.1.1.14 [`VbStrConv`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) — String conversion flags

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbFromUnicode`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `128` | Planned | B (implemented constant) |
| [`vbHiragana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `32` | Planned | B (implemented constant) |
| [`vbKatakana`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `16` | Planned | B (implemented constant) |
| [`vbLowerCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `2` | Planned | B (implemented constant) |
| [`vbNarrow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `8` | Planned | B (implemented constant) |
| [`vbProperCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `3` | Planned | B (implemented constant) |
| [`vbUnicode`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `64` | Planned | B (implemented constant) |
| [`vbUpperCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `1` | Planned | B (implemented constant) |
| [`vbWide`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `4` | Planned | B (implemented constant) |

### 6.1.1.15 [`VbTriState`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) — Three-way Boolean/default setting

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbFalse`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=178) | `0` | Planned | B (implemented constant) |
| [`vbTrue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `-1` | Planned | B (implemented constant) |
| [`vbUseDefault`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `-2` | Planned | B (implemented constant) |

### 6.1.1.16 [`VbVarType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) — Value-type codes returned by `VarType`

| Name | Value | Baseline | Delivery |
|---|---:|---|---|
| [`vbArray`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `8192` | Planned | B (implemented constant) |
| [`vbBoolean`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `11` | Planned | B (implemented constant) |
| [`vbByte`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `17` | Planned | B (implemented constant) |
| [`vbCurrency`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `6` | Planned | B (implemented constant) |
| [`vbDataObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `13` | Planned | B (implemented constant) |
| [`vbDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `7` | Planned | B (implemented constant) |
| [`vbDecimal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `14` | Planned | B (implemented constant) |
| [`vbDouble`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `5` | Planned | B (implemented constant) |
| [`vbEmpty`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `0` | Planned | B (implemented constant) |
| [`vbError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `10` | Planned | B (implemented constant) |
| [`vbInteger`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `2` | Planned | B (implemented constant) |
| [`vbLong`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `3` | Planned | B (implemented constant) |
| [`vbLongLong`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `20` | Planned | B (implemented constant) |
| [`vbNull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `1` | Planned | B (implemented constant) |
| [`vbObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `9` | Planned | B (implemented constant) |
| [`vbSingle`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `4` | Planned | B (implemented constant) |
| [`vbString`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `8` | Planned | B (implemented constant) |
| [`vbUserDefinedType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `36` | Planned | B (implemented constant) |
| [`vbVariant`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=179) | `12` | Planned | B (implemented constant) |

## 2. Predefined procedural modules (§6.1.2)

### 6.1.2.1 [`ColorConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) — Eight predefined RGB color values.

| Constant | Value | Meaning | Baseline | Delivery |
|---|---|---|---|---|
| [`vbBlack`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `0` | — | Planned | B (implemented constant) |
| [`vbBlue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16711680` | — | Planned | B (implemented constant) |
| [`vbCyan`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16776960` | — | Planned | B (implemented constant) |
| [`vbGreen`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `65280` | — | Planned | B (implemented constant) |
| [`vbMagenta`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16711935` | — | Planned | B (implemented constant) |
| [`vbRed`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `255` | — | Planned | B (implemented constant) |
| [`vbWhite`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `16777215` | — | Planned | B (implemented constant) |
| [`vbYellow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `65535` | — | Planned | B (implemented constant) |

### 6.1.2.2 [`Constants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) — Special-character strings and an object error base.

| Constant | Value | Meaning | Baseline | Delivery |
|---|---|---|---|---|
| [`vbBack`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(8)` | Backspace | Existing constant | B (implemented constant) |
| [`vbCr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(13)` | Carriage return | Existing constant | B (implemented constant) |
| [`vbCrLf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(13) + Chr$(10)` | Windows line ending | Existing constant | B (implemented constant) |
| [`vbFormFeed`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(12)` | Form feed | Existing constant | B (implemented constant) |
| [`vbLf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(10)` | Line feed | Existing constant | B (implemented constant) |
| [`vbNewLine`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `implementation-defined` | Line separator | Planned | B (implemented constant) |
| [`vbNullChar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(0)` | Null character | Existing constant | B (implemented constant) |
| [`vbTab`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(9)` | Horizontal tab | Existing constant | B (implemented constant) |
| [`vbVerticalTab`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `Chr$(11)` | Vertical tab | Existing constant | B (implemented constant) |
| [`vbNullString`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `null string pointer` | Null-string value | Existing constant | B (implemented constant) |
| [`vbObjectError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | `-2147221504` | Object error code base | Existing constant | B (implemented constant) |

### 6.1.2.3 [`Conversion`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) — Explicit type conversions and numeric-to-string conversions.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`CBool`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=180) | Convert to Boolean | Existing subset | A + D |
| [`CByte`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=181) | Convert to Byte | Planned | D (implemented; current numeric profile) |
| [`CCur`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=181) | Convert to Currency | Planned | D (implemented; checked fixed-point profile) |
| [`CDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=182) | Convert to Date | Planned | D (implemented; numeric/ISO profile) |
| [`CVDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=182) | Date conversion variant | Planned | D (implemented; numeric/ISO profile) |
| [`CDbl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=182) | Convert to Double | Existing subset | A + D |
| [`CDec`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=183) | Convert to Decimal Variant | Planned | D (implemented; exact Decimal profile) |
| [`CInt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=183) | Convert to Integer | Existing subset | A + D |
| [`CLng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=183) | Convert to Long | Existing subset | A + D |
| [`CLngLng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=184) | Convert to LongLong (when supported) | Planned | D |
| [`CLngPtr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=184) | Convert to pointer-sized integer | Planned | D |
| [`CSng`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=185) | Convert to Single | Existing subset | A + D |
| [`CStr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=185) | Convert to String | Existing subset | A + D |
| [`CVar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=185) | Convert to Variant | Existing subset | A + D |
| [`CVErr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=186) | Create Error Variant | Planned | D (implemented; see Stage D progress) |
| [`Error`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=186) | Error message for an error number | Planned | C |
| [`Error$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=186) | String-returning error message variant | Planned | C |
| [`Fix`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=187) | Truncate fractional part toward zero | Planned | C |
| [`Hex`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=188) | Hexadecimal representation | Planned | C |
| [`Hex$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=188) | String-returning hexadecimal variant | Planned | C |
| [`Int`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=188) | Round down to an integer | Planned | C |
| [`Oct`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=189) | Octal representation | Planned | C |
| [`Oct$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=189) | String-returning octal variant | Planned | C |
| [`Str`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=190) | Convert number to string | Planned | C |
| [`Str$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=190) | String-returning numeric conversion | Planned | C |
| [`Val`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=190) | Parse leading numeric text | Planned | C |

### 6.1.2.4 [`DateTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=191) — Date/time extraction, arithmetic and clock properties.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`DateAdd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=191) | Add interval to a date | Planned | E |
| [`DateDiff`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=192) | Count intervals between dates | Planned | E |
| [`DatePart`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=194) | Extract a date/time component | Planned | E |
| [`DateSerial`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=195) | Build a date from year/month/day | Planned | E |
| [`DateValue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=196) | Convert date text into a date | Planned | E |
| [`Day`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=196) | Day of month | Planned | E |
| [`Hour`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=197) | Hour of day | Planned | E |
| [`Minute`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=197) | Minute component | Planned | E |
| [`Month`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=197) | Month number | Planned | E |
| [`Second`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=198) | Second component | Planned | E |
| [`TimeSerial`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=198) | Build time from hour/minute/second | Planned | E |
| [`TimeValue`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=199) | Convert time text into a time | Planned | E |
| [`Weekday`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=199) | Day-of-week number | Planned | E |
| [`Year`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=200) | Year component | Planned | E |

**Properties**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Calendar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=200) | Current calendar selection | Planned | E |
| [`Date`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | Current system date; can be set | Planned | F; writes need host adapter |
| [`Date$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | String-returning date property | Planned | F; writes need host adapter |
| [`Now`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | Current date and time | Planned | F |
| [`Time`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | Current system time; can be set | Planned | F; writes need host adapter |
| [`Time$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=201) | String-returning time property | Planned | F; writes need host adapter |
| [`Timer`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | Seconds since midnight | Planned | F |

### 6.1.2.5 [`FileSystem`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) — File, directory and sequential-file access.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`CurDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | Current directory | Planned | F |
| [`CurDir$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | String-returning current directory | Planned | F |
| [`Dir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=202) | File/directory name search | Planned | F |
| [`EOF`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=203) | End-of-file flag | Planned | F (file table) |
| [`FileAttr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=203) | Open-file mode | Planned | F (file table) |
| [`FileDateTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=204) | File timestamp | Planned | F |
| [`FileLen`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=204) | File length in bytes | Planned | F |
| [`FreeFile`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=205) | Unused file number | Planned | F (file table) |
| [`Loc`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=205) | Current file position | Planned | F (file table) |
| [`LOF`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=206) | Length of open file | Planned | F (file table) |
| [`Seek`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=206) | Position in open file | Planned | F (file table) |

**Subroutines**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`ChDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=207) | Change current directory | Planned | F |
| [`ChDrive`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=207) | Change current drive | Planned | F |
| [`FileCopy`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=207) | Copy a file | Planned | F |
| [`Kill`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=208) | Delete file(s) | Planned | F |
| [`MkDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=208) | Create directory | Planned | F |
| [`RmDir`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=209) | Remove directory | Planned | F |
| [`SetAttr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=209) | Set file attributes | Planned | F |

### 6.1.2.6 [`Financial`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=210) — Finance, depreciation, interest, payments and valuation.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`DDB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=210) | Double-declining-balance depreciation | Planned | E |
| [`FV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=210) | Future value | Planned | E |
| [`IPmt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=211) | Interest portion of a payment | Planned | E |
| [`IRR`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=212) | Internal rate of return | Planned | E |
| [`MIRR`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=213) | Modified internal rate of return | Planned | E |
| [`NPer`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=213) | Number of payment periods | Planned | E |
| [`NPV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=214) | Net present value | Planned | E |
| [`Pmt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=215) | Periodic payment amount | Planned | E |
| [`PPmt`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=215) | Principal portion of a payment | Planned | E |
| [`PV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=216) | Present value | Planned | E |
| [`Rate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=217) | Interest rate per period | Planned | E |
| [`SLN`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=218) | Straight-line depreciation | Planned | E |
| [`SYD`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=218) | Sum-of-years-digits depreciation | Planned | E |

### 6.1.2.7 [`Information`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=219) — Type tests, IME state and color/type information.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`IMEStatus`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=219) | Current input-method status | Planned | G |
| [`IsArray`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=219) | Test whether value is an array | Existing subset | A + D |
| [`IsDate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test whether expression is a date | Planned | D + E |
| [`IsEmpty`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test for Empty | Existing subset | A + D |
| [`IsError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test for Error value | Planned | D (implemented) |
| [`IsMissing`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=220) | Test for missing optional argument | Existing subset | A + D |
| [`IsNull`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=221) | Test for Null | Existing subset | A + D |
| [`IsNumeric`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=221) | Test for numeric value | Planned | C + D |
| [`IsObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=222) | Test for object reference | Planned | C |
| [`QBColor`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=222) | Color from historical QB palette | Planned | C |
| [`RGB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=223) | Color from red, green and blue | Planned | C |
| [`TypeName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=223) | Type name as text | Planned | C + D |
| [`VarType`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=224) | Variant subtype code | Existing subset | A + D |

### 6.1.2.8 [`Interaction`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=226) — System interaction, dialogs, settings and runtime object invocation.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`CallByName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=226) | Invoke object member by name | Planned | G (native invocation can precede COM) |
| [`Choose`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=226) | Select list entry by index | Planned | C |
| [`Command`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=227) | Process command-line arguments | Planned | F |
| [`CreateObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=227) | Create automation object | Existing subset | A + G |
| [`DoEvents`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=228) | Yield execution to event processing | Planned | G |
| [`Environ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=228) | Read environment setting | Planned | F |
| [`Environ$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=228) | String-returning environment setting | Planned | F |
| [`GetAllSettings`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=229) | Read application's saved settings | Planned | G |
| [`GetAttr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=229) | Read file attributes | Planned | F |
| [`GetObject`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=230) | Retrieve automation object | Planned | G |
| [`GetSetting`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=231) | Read one saved setting | Planned | G |
| [`IIf`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=231) | Conditional value selection | Existing subset | A |
| [`InputBox`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=232) | Show text-input dialog | Planned | G |
| [`MsgBox`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=233) | Show message dialog | Planned | G |
| [`Partition`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=235) | Place number in a range bucket | Planned | C |
| [`Shell`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=236) | Run an external program | Planned | G |
| [`Switch`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=237) | First matching condition's value | Planned | C |

**Subroutines**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`AppActivate`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=238) | Activate a program window | Planned | G |
| [`Beep`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=238) | Sound system alert | Planned | G |
| [`DeleteSetting`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=238) | Delete saved application setting | Planned | G |
| [`SaveSetting`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=239) | Save application setting | Planned | G |
| [`SendKeys`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=239) | Send keystrokes | Planned | G |

### 6.1.2.9 [`KeyCodeConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) — Keyboard, mouse button and virtual-key codes.

| Constant | Value | Meaning | Baseline | Delivery |
|---|---|---|---|---|
| [`vbKeyLButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `1` | — | Planned | B (implemented constant) |
| [`vbKeyRButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `2` | — | Planned | B (implemented constant) |
| [`vbKeyCancel`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `3` | — | Planned | B (implemented constant) |
| [`vbKeyMButton`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `4` | — | Planned | B (implemented constant) |
| [`vbKeyBack`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `8` | — | Planned | B (implemented constant) |
| [`vbKeyTab`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `9` | — | Planned | B (implemented constant) |
| [`vbKeyClear`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `12` | — | Planned | B (implemented constant) |
| [`vbKeyReturn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `13` | — | Planned | B (implemented constant) |
| [`vbKeyShift`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `16` | — | Planned | B (implemented constant) |
| [`vbKeyControl`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `17` | — | Planned | B (implemented constant) |
| [`vbKeyMenu`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `18` | — | Planned | B (implemented constant) |
| [`vbKeyPause`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `19` | — | Planned | B (implemented constant) |
| [`vbKeyCapital`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `20` | — | Planned | B (implemented constant) |
| [`vbKeyEscape`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `27` | — | Planned | B (implemented constant) |
| [`vbKeySpace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `32` | — | Planned | B (implemented constant) |
| [`vbKeyPageUp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `33` | — | Planned | B (implemented constant) |
| [`vbKeyPageDown`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `34` | — | Planned | B (implemented constant) |
| [`vbKeyEnd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `35` | — | Planned | B (implemented constant) |
| [`vbKeyHome`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `36` | — | Planned | B (implemented constant) |
| [`vbKeyLeft`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `37` | — | Planned | B (implemented constant) |
| [`vbKeyUp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `38` | — | Planned | B (implemented constant) |
| [`vbKeyRight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `39` | — | Planned | B (implemented constant) |
| [`vbKeyDown`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=242) | `40` | — | Planned | B (implemented constant) |
| [`vbKeySelect`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `41` | — | Planned | B (implemented constant) |
| [`vbKeyPrint`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `42` | — | Planned | B (implemented constant) |
| [`vbKeyExecute`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `43` | — | Planned | B (implemented constant) |
| [`vbKeySnapshot`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `44` | — | Planned | B (implemented constant) |
| [`vbKeyInsert`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `45` | — | Planned | B (implemented constant) |
| [`vbKeyDelete`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `46` | — | Planned | B (implemented constant) |
| [`vbKeyHelp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `47` | — | Planned | B (implemented constant) |
| [`vbKeyNumlock`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `144` | — | Planned | B (implemented constant) |
| [`vbKeyA`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `65` | — | Planned | B (implemented constant) |
| [`vbKeyB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `66` | — | Planned | B (implemented constant) |
| [`vbKeyC`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `67` | — | Planned | B (implemented constant) |
| [`vbKeyD`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `68` | — | Planned | B (implemented constant) |
| [`vbKeyE`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `69` | — | Planned | B (implemented constant) |
| [`vbKeyF`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `70` | — | Planned | B (implemented constant) |
| [`vbKeyG`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `71` | — | Planned | B (implemented constant) |
| [`vbKeyH`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `72` | — | Planned | B (implemented constant) |
| [`vbKeyI`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `73` | — | Planned | B (implemented constant) |
| [`vbKeyJ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `74` | — | Planned | B (implemented constant) |
| [`vbKeyK`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `75` | — | Planned | B (implemented constant) |
| [`vbKeyL`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `76` | — | Planned | B (implemented constant) |
| [`vbKeyM`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `77` | — | Planned | B (implemented constant) |
| [`vbKeyN`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `78` | — | Planned | B (implemented constant) |
| [`vbKeyO`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `79` | — | Planned | B (implemented constant) |
| [`vbKeyP`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `80` | — | Planned | B (implemented constant) |
| [`vbKeyQ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `81` | — | Planned | B (implemented constant) |
| [`vbKeyR`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `82` | — | Planned | B (implemented constant) |
| [`vbKeyS`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `83` | — | Planned | B (implemented constant) |
| [`vbKeyT`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `84` | — | Planned | B (implemented constant) |
| [`vbKeyU`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `85` | — | Planned | B (implemented constant) |
| [`vbKeyV`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `86` | — | Planned | B (implemented constant) |
| [`vbKeyW`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `87` | — | Planned | B (implemented constant) |
| [`vbKeyX`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `88` | — | Planned | B (implemented constant) |
| [`vbKeyY`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `89` | — | Planned | B (implemented constant) |
| [`vbKeyZ`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `90` | — | Planned | B (implemented constant) |
| [`vbKey0`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=243) | `48` | — | Planned | B (implemented constant) |
| [`vbKey1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `49` | — | Planned | B (implemented constant) |
| [`vbKey2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `50` | — | Planned | B (implemented constant) |
| [`vbKey3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `51` | — | Planned | B (implemented constant) |
| [`vbKey4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `52` | — | Planned | B (implemented constant) |
| [`vbKey5`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `53` | — | Planned | B (implemented constant) |
| [`vbKey6`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `54` | — | Planned | B (implemented constant) |
| [`vbKey7`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `55` | — | Planned | B (implemented constant) |
| [`vbKey8`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `56` | — | Planned | B (implemented constant) |
| [`vbKey9`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `57` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad0`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `96` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `97` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `98` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `99` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `100` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad5`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `101` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad6`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `102` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad7`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `103` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad8`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `104` | — | Planned | B (implemented constant) |
| [`vbKeyNumpad9`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `105` | — | Planned | B (implemented constant) |
| [`vbKeyMultiply`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `106` | — | Planned | B (implemented constant) |
| [`vbKeyAdd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `107` | — | Planned | B (implemented constant) |
| [`vbKeySeparator`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `108` | — | Planned | B (implemented constant) |
| [`vbKeySubtract`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `109` | — | Planned | B (implemented constant) |
| [`vbKeyDecimal`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `110` | — | Planned | B (implemented constant) |
| [`vbKeyDivide`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `111` | — | Planned | B (implemented constant) |
| [`vbKeyF1`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `112` | — | Planned | B (implemented constant) |
| [`vbKeyF2`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `113` | — | Planned | B (implemented constant) |
| [`vbKeyF3`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `114` | — | Planned | B (implemented constant) |
| [`vbKeyF4`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `115` | — | Planned | B (implemented constant) |
| [`vbKeyF5`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `116` | — | Planned | B (implemented constant) |
| [`vbKeyF6`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `117` | — | Planned | B (implemented constant) |
| [`vbKeyF7`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `118` | — | Planned | B (implemented constant) |
| [`vbKeyF8`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `119` | — | Planned | B (implemented constant) |
| [`vbKeyF9`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `120` | — | Planned | B (implemented constant) |
| [`vbKeyF10`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=244) | `121` | — | Planned | B (implemented constant) |
| [`vbKeyF11`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `122` | — | Planned | B (implemented constant) |
| [`vbKeyF12`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `123` | — | Planned | B (implemented constant) |
| [`vbKeyF13`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `124` | — | Planned | B (implemented constant) |
| [`vbKeyF14`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `125` | — | Planned | B (implemented constant) |
| [`vbKeyF15`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `126` | — | Planned | B (implemented constant) |
| [`vbKeyF16`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | `127` | — | Planned | B (implemented constant) |

### 6.1.2.10 [`Math`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) — Numeric and trigonometric calculations; random numbers.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Abs`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | Absolute value | Planned | C + D |
| [`Atn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=245) | Arctangent | Planned | C |
| [`Cos`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=246) | Cosine | Planned | C |
| [`Exp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=246) | Exponential (e to a power) | Planned | C |
| [`Log`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=246) | Natural logarithm | Planned | C |
| [`Rnd`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=247) | Pseudo-random number | Planned | F (RNG state) |
| [`Round`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=248) | Round to decimal places | Planned | C + D |
| [`Sgn`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=248) | Sign of a number | Planned | C + D |
| [`Sin`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=249) | Sine | Planned | C |
| [`Sqr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=249) | Square root | Planned | C |
| [`Tan`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=249) | Tangent | Planned | C |

**Subroutines**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Randomize`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=250) | Initialize random-number generator | Planned | F (RNG state) |

### 6.1.2.11 [`Strings`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=250) — Character codes, text search, slicing, formatting and case conversion.

**Functions**

| Name | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Asc`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=250) | Character code of first character | Planned | E (encoding) |
| [`AscB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=251) | First byte of a character | Planned | E (encoding) |
| [`AscW`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=251) | Unicode character code | Existing subset | A |
| [`Chr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | Character from numeric code | Planned | E (encoding) |
| [`Chr$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | String-returning character conversion | Planned | E (encoding) |
| [`ChrB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | Byte-character from code | Planned | E (encoding) |
| [`ChrB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=252) | String-returning byte-character conversion | Planned | E (encoding) |
| [`ChrW`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=253) | Unicode character from code | Existing subset | A |
| [`ChrW$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=253) | String-returning Unicode conversion | Planned | C |
| [`Filter`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=253) | Filter string array entries | Planned | C; text modes in E |
| [`Format`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=254) | Format a value as text | Planned | E (locale/formatting) |
| [`Format$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=256) | String-returning formatter | Planned | E (locale/formatting) |
| [`FormatCurrency`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=256) | Format as currency | Planned | E (locale/formatting) |
| [`FormatDateTime`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=257) | Format as date/time | Planned | E (locale/formatting) |
| [`FormatNumber`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=258) | Format as number | Planned | E (locale/formatting) |
| [`FormatPercent`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=259) | Format as percentage | Planned | E (locale/formatting) |
| [`InStr`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=260) | Find substring, left to right | Existing subset | A + E |
| [`InStrB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=260) | Byte-oriented substring search | Planned | E (byte semantics) |
| [`InStrRev`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=261) | Find substring from right | Planned | C; text modes in E |
| [`Join`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=262) | Join array elements into text | Planned | C |
| [`LCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | Lowercase conversion | Planned | E (locale/formatting) |
| [`LCase$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | String-returning lowercase conversion | Planned | E (locale/formatting) |
| [`Left`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | Leftmost characters | Existing subset | A |
| [`LeftB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=263) | Leftmost bytes | Planned | E (byte semantics) |
| [`Left$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | String-returning left substring | Existing subset | A |
| [`LeftB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | String-returning left byte substring | Planned | E (byte semantics) |
| [`Len`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | Length of text/variable | Existing subset | A + D + E (layout) |
| [`LenB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=264) | Length in bytes | Existing subset | A + D + E (layout) |
| [`LTrim`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Remove leading spaces | Planned | C |
| [`RTrim`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Remove trailing spaces | Planned | C |
| [`Trim`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Remove outer spaces | Planned | C |
| [`LTrim$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | String-returning leading trim | Planned | C |
| [`RTrim$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | String-returning trailing trim | Planned | C |
| [`Trim$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | String-returning outer trim | Planned | C |
| [`Mid`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Substring starting at a position | Existing subset | A |
| [`MidB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=265) | Byte-oriented substring | Planned | E (byte semantics) |
| [`Mid$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | String-returning substring | Existing subset | A |
| [`MidB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | String-returning byte substring | Planned | E (byte semantics) |
| [`MonthName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | Localized name of month | Planned | E (locale/formatting) |
| [`Replace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=266) | Replace substring occurrences | Planned | C; text modes in E |
| [`Right`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=267) | Rightmost characters | Planned | C |
| [`RightB`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=267) | Rightmost bytes | Planned | E (byte semantics) |
| [`Right$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=268) | String-returning right substring | Planned | C |
| [`RightB$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=268) | String-returning right byte substring | Planned | E (byte semantics) |
| [`Space`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=268) | Make string of spaces | Planned | C |
| [`Space$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=269) | String-returning spaces | Planned | C |
| [`Split`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=269) | Split text into array | Existing subset | A + E |
| [`StrComp`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=269) | Compare two strings | Existing subset | A + E |
| [`StrConv`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=270) | Convert string case/encoding | Planned | E (locale/formatting) |
| [`String`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=271) | Repeat a character | Existing subset | A + E |
| [`String$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=272) | String-returning repeated characters | Existing subset | A + E |
| [`StrReverse`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=272) | Reverse text characters | Planned | C |
| [`UCase`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=272) | Uppercase conversion | Planned | E (locale/formatting) |
| [`UCase$`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=273) | String-returning uppercase conversion | Planned | E (locale/formatting) |
| [`WeekdayName`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=273) | Localized weekday name | Planned | E (locale/formatting) |

### 6.1.2.12 [`SystemColorConstants`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) — System-dependent user-interface color codes.

| Constant | Value | Meaning | Baseline | Delivery |
|---|---|---|---|---|
| [`vbScrollBars`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000000` | Scroll bar | Planned | B (implemented constant) |
| [`vbDesktop`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000001` | Desktop | Planned | B (implemented constant) |
| [`vbActiveTitleBar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000002` | Active title bar | Planned | B (implemented constant) |
| [`vbInactiveTitleBar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000003` | Inactive title bar | Planned | B (implemented constant) |
| [`vbMenuBar`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000004` | Menu background | Planned | B (implemented constant) |
| [`vbWindowBackground`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000005` | Window background | Planned | B (implemented constant) |
| [`vbWindowFrame`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000006` | Window frame | Planned | B (implemented constant) |
| [`vbMenuText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000007` | Menu text | Planned | B (implemented constant) |
| [`vbWindowText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000008` | Window text | Planned | B (implemented constant) |
| [`vbTitleBarText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000009` | Title bar text | Planned | B (implemented constant) |
| [`vbActiveBorder`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000A` | Active border | Planned | B (implemented constant) |
| [`vbInactiveBorder`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000B` | Inactive border | Planned | B (implemented constant) |
| [`vbApplicationWorkspace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000C` | MDI workspace | Planned | B (implemented constant) |
| [`vbHighlight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000D` | Selected item background | Planned | B (implemented constant) |
| [`vbHighlightText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000E` | Selected item text | Planned | B (implemented constant) |
| [`vbButtonFace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000F` | Button face | Planned | B (implemented constant) |
| [`vbButtonShadow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000010` | Button shadow | Planned | B (implemented constant) |
| [`vbGrayText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000011` | Disabled text | Planned | B (implemented constant) |
| [`vbButtonText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000012` | Button text | Planned | B (implemented constant) |
| [`vbInactiveCaptionText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000013` | Inactive caption text | Planned | B (implemented constant) |
| [`vb3DHighlight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000014` | 3D highlight | Planned | B (implemented constant) |
| [`vb3DDKShadow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000015` | 3D dark shadow | Planned | B (implemented constant) |
| [`vb3DLight`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000016` | 3D light | Planned | B (implemented constant) |
| [`vb3DFace`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H8000000F` | 3D face | Planned | B (implemented constant) |
| [`vb3Dshadow`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000010` | 3D shadow | Planned | B (implemented constant) |
| [`vbInfoText`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=274) | `&H80000017` | Tooltip text | Planned | B (implemented constant) |
| [`vbInfoBackground`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) | `&H80000018` | Tooltip background | Planned | B (implemented constant) |

## 3. Predefined classes (§6.1.3)

### 6.1.3.1 [`Collection`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) — Ordered collection of keyed/positioned items.

| Member | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Collection.Count`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) | Number of items | Existing subset | A |
| [`Collection.Item`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=275) | Look up item by position/key | Existing subset | A |
| [`Collection.Add`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=276) | Insert an item | Existing subset | A |
| [`Collection.Remove`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=276) | Delete an item | Existing subset | A |

### 6.1.3.2 [`Err`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=277) — Global error-state object.

| Member | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Err.Clear`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=277) | Reset error state | Existing subset | A |
| [`Err.Raise`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=277) | Raise a runtime error | Existing subset | A |
| [`Err.Description`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=278) | Text describing the error | Existing subset | A |
| [`Err.HelpContext`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=278) | Help topic identifier | Existing subset | A |
| [`Err.HelpFile`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Help-file path | Existing subset | A |
| [`Err.LastDllError`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Last DLL-call error code | Planned | G (external-call state) |
| [`Err.Number`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Numeric error code | Existing subset | A |
| [`Err.Source`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=279) | Origin of error | Existing subset | A |

### 6.1.3.3 [`Global`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=280) — Global procedures for object lifecycle.

| Member | Brief description | Baseline | Delivery |
|---|---|---|---|
| [`Global.Load`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=280) | Load an object/form | Planned | G (host lifecycle) |
| [`Global.Unload`](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf#page=280) | Unload an object/form | Planned | G (host lifecycle) |

---

### Inventory counts

- **16 enums** containing **124 enumerated constant names**.
- **12 procedural modules:** **145 module constants**, **160 function name entries**, **13 subroutine name entries**, **7 property name entries**.
- **3 predefined classes** containing **14 named members**.
- **Total named entries:** 494 (counting category names; class-member names are qualified; aliases are separate entries).

**Conventions:** `VBA` is the global standard-library project. Many names are available without module qualification; for example `VBA.Strings.Left` may be called `Left`. Identically named constants in different enum families are listed at their definitions. The enumerated names follow Microsoft’s published capitalization (VBA identifiers are case-insensitive). `LongLong` support is implementation-dependent, as described by Microsoft.
