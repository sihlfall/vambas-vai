# Semantic checking

`Program.GetTypeChecker()` supplies the semantic view shared by future language
services and compiler lowering. The checker reads completed bound source files;
it owns type, signature, conversion, and resolved-symbol tables. It does not add
types to `ast.Node`, replace `Node.Symbol` with a type, or modify binding/flow
metadata. See the runnable [two-module example](example_test.go).

```go
c := program.GetTypeChecker()
symbol := c.GetSymbolAtLocation(reference)
expressionType := c.GetTypeAtLocation(expression)
declaredType := c.GetTypeOfSymbol(symbol)
signature := c.GetSignatureFromDeclaration(procedure)
resolvedSignature := c.GetResolvedSignature(call)
conversion := c.GetAssignmentConversion(assignment)
operation := c.GetCallResolution(call)
storage := c.GetStorageAtLocation(argument)
defaultValueCalls := c.GetValueResolutions(expression)
diagnostics := program.GetSemanticDiagnostics(nil) // nil means all source files.
```

The Program creates its checker lazily, once. Checker construction currently
checks the entire project eagerly, so subsequent queries are read-only and safe
to run concurrently. Treat the returned types/signatures as immutable. A separate
`checker.NewChecker(host)` produces independent semantic tables; its Host supplies
`SourceFiles()` and `Symbol()`. All files must already be bound. Rebuild the Program
after edits; this does not yet introduce incremental AST reuse.

The structure follows typescript-go's checker host boundary, semantic queries,
and node/symbol link ownership. Existing binder-produced reference links are
imported as input for compatibility. Moving the remaining name resolution and
semantic binding diagnostics out of the binder is a subsequent migration, rather
than a new responsibility added to that binder here.

## Types and signatures

`checker.Type.Kind` distinguishes the built-in scalar types Boolean, Byte, Integer,
Long, LongLong, LongPtr, Single, Double, Currency, Date, String, Variant and Object;
local classes; arrays; procedures; and modules. Types describe the language,
not Go storage, LLVM layouts, or runtime Variant tags. LongPtr retains its own
identity; choosing a target-dependent arithmetic width requires a future profile.

Three additional kinds have distinct meanings:

| Kind | Meaning |
| --- | --- |
| `TypeUnknown` | Semantic work is deferred, or the queried node has no value type. It is never a substitute for Variant. |
| `TypeError` | A declaration/reference/expression is invalid or recovered. Dependent checking suppresses cascading errors. |
| `TypeVoid` | A Sub or property setter has no language result value. |

`TypeError` denotes an analysis error, not a runtime CVErr/Variant Error value.

Omitted variable, parameter, and Function/Get return annotations default to
Variant; declaration type characters select their corresponding scalar type.
Conflicting type characters and As annotations are diagnosed. Constant types
come from an annotation/type character or from their initializer's declared type.
Constant dependency traversal detects cycles across modules even when every
constant has an explicit type. Object/array constant declarations are rejected.
This is type inference and cycle checking, not full constant evaluation.

Local class annotations and New resolve in type space independently of local
values, including `Project.Class` names. Unknown external named types now produce
`unknown-type` semantic diagnostics; the binder still leaves them untouched.
Built-in types cannot be instantiated with As New. Fixed-length String annotations
currently query as String; their length constraints remain source syntax.

Array types retain `ElementType` and `Rank` (zero means unspecified rank).
Procedure types have `Signatures`. Each signature retains its declaration,
parameters, and return type; parameters retain their symbols, types, passing
modifiers, and default expressions. Function/Get result symbols query as the
return storage type, separately from the procedure or merged property symbol.
Get/Let/Set declarations retain independent signatures.

`GetTypeAtLocation` returns a declaration's type or an expression's result type.
Thus a Function symbol has a procedure type, but an expression invoking that
function, explicitly or implicitly, has its return type. Direct calls and property
operations expose the selected signature. Basic array indexing exposes the element
type without claiming a call signature; nested applications do not invoke the
same procedure twice. Argument mapping and storage/passing plans are described in
[Calls, default members, and storage](CALLS.md).
An explicit call's function callee and an AddressOf operand retain the procedure
type; AddressOf does not record an invocation signature. Its pointer result type
remains deferred.

## Scalar expressions and conversions

Literal and scalar operator typing follows the MS-VBAL declared-type tables:
[number tokens](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/685ad840-accb-4bdb-8bfd-f3d88498547a),
[arithmetic](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/e070115f-8d40-40cf-ac6d-ab18b9c6c906),
[relational](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/f8acd631-55c1-4199-bc1e-022aaab6d9c8),
and [logical operators](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/2d70780d-6e99-47d6-9759-2b3a46b8e862).
Per-operator exceptions are preserved, including [Date subtraction](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/c966fa50-f49f-44ab-a8f8-f2691fffe048).
For example, Single + Long has declared type Double, Currency + Double has type
Currency, and Variant comparisons retain declared type Variant. Empty and Null
literal expressions also have declared type Variant; Nothing has type Object.

Assignments, constant initializers, and Optional defaults expose `Conversion`
records with `From`, `To`, and one of:

| Kind | Meaning |
| --- | --- |
| `ConversionIdentity` | The declared scalar types match. |
| `ConversionLet` | A VBA Let-coercion must be performed. |
| `ConversionSet` | An object-reference coercion must be performed. |
| `ConversionDeferred` | Required storage/default-member semantics are unresolved. |
| `ConversionInvalid` | The types or dependent source are invalid. |

A Let may coerce String to Long: even `n = "not a number"` requires a potentially
failing runtime coercion, rather than a TypeScript-style static incompatibility.
A Set involving known scalar storage or a known scalar RHS is invalid. A property
write uses the selected setter's final parameter type, not the getter's return
type. These records make coercions explicit for future IR; they do not insert
synthetic AST nodes or encode runtime operations/effect signatures.

## Records and enumerations

VBA `Type` declarations have nominal `TypeStructure` types identified by their
declaration symbol. Nested field access, With receivers, array fields, matching
record assignments, and matching ByRef parameters retain type/storage information.
Different records do not become compatible just because their fields match.
Records are value types: Set/New and scalar operators are invalid. Empty and
statically embedded recursive types are diagnosed. Dynamic-array fields do not
embed their elements. The [native UDT subset](../llvm/UDTS_ENUMS.md) now implements
managed record ownership and typed scalar-array fields; UDT arrays and public
record layouts remain separate work.
Fixed-string field passing retains the existing deferred constraint handling.

Enum declarations are qualification namespaces; enum annotations and constants
have declared type Long, following [MS-VBAL enum
semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/da1d4885-946f-4937-9487-488143b97f08).
`GetEnumMemberValue(symbol) (int64, bool)` returns a computed 32-bit Long value or
false for an invalid/unresolved member. Zero and previous-plus-one defaults,
numeric arithmetic/bitwise/comparison expressions, numeric strings, and references
to constants/earlier enum members are supported. Long coercion uses ties-to-even
rounding. Overflow, divide-by-zero, cycles, and forward enum-member references in
the same module are diagnosed. Values remain checker-owned and read-only.

The evaluator is deliberately scoped to numeric enum values, not a complete VBA
constant interpreter: intrinsic function calls, general string operations, full
Currency/LongLong precision and all host-specific coercions still need work.
Host/type-library exposure restrictions on public records and signatures are not
fully validated. Local record-to-Variant coercion is rejected; external record
metadata and marshaling support remain future reference/runtime work.

## ReDim allocation plans

`GetReDimResolution(declaration)` returns a read-only `ReDimResolution` for each
`ReDimDeclaration`. It describes the destination storage, allocation array type,
ordered bounds and their Long conversions, Preserve mode, and static validity.
Declarative ReDim creates a resizable array type; a formally declared Variant
continues to have scope type Variant, regardless of an allocation's `As` clause.
Known fixed-size arrays, ParamArrays, scalar targets, value/property results, and
element-type changes to declared arrays are diagnosed.

`CheckPreservedBounds` retains the runtime requirements on rank, lower bounds,
and all but the last upper bound. `CheckPreservedElementType` applies to a Variant
with Preserve and an explicit element type. `CheckArrayLock` retains the runtime
ByRef-lock constraint. Allocation failure, bound coercion/ranges, and releasing or
initializing elements also require runtime lowering. These operations have no LLVM
dependency and do not execute in the language server. Variant/late-bound operations
remain marked Deferred. No lexical "last ReDim" heuristic is used across branches,
calls, aliases, or GoTo; Preserve constraints depend on the actual runtime array.
See [MS-VBAL ReDim](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/22b5d372-0a54-4617-9462-4934b5edc88c).

Enum initializers can use the ambient `vbObjectError` constant, such as
`REGEX_ERR = vbObjectError + 3000`, and later enum members can build on that value.
It is supplied by an ordinary declaration scope, not a numeric-evaluator shortcut.
Other built-in/library declarations still need to be supplied as support expands.
The ambient library now also supplies Len/LenB, ChrW, vbNullString, and vbNullChar.

Mid and Mid$ have distinct Variant/String declarations with optional Length.
Typed String input allows unboxed String call results; Variant/Null inputs remain
outside native compiler support. `GetMidAssignmentResolution` returns bound
storage and Start/Length/replacement conversions for the statement special form.
See the [Mid contract](../llvm/MID.md).

Len/LenB keep Variant signatures but refine a statically typed String call to
Long. [Library call policies](library.go) select these refinements and InStr's
two-argument mapping through the resolved symbol's catalog member ID. Ordinary
call binding remains shared with user procedures; shadowing user declarations
receive their own signatures and result types. See the
[compiled String subset](../llvm/STRINGS.md).
Its value follows Microsoft's [VBA miscellaneous constants](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/miscellaneous-constants).

## Diagnostics and current limits

Semantic diagnostics carry `File`, `Node`, `Loc`, `Code`, and `Message`; spans use
the AST's half-open UTF-8 byte offsets and exclude leading trivia. Results are
ordered by input file and source position and returned in a copied slice. Query
order does not add or change diagnostics. Parse and bind diagnostics retain their
existing APIs; inspect all three categories.

Recovered trees can be queried. Calls, named/omitted arguments, ParamArray packing,
ByRef storage and temporaries, and local-class default members have semantic plans.
Project-local `Implements` checks public method/property and scalar-field contracts,
including signatures, optional defaults, and Implements cycles.
`GetImplementedInterfaces` exposes the checked member mappings; see
[the native interface contract](../llvm/IMPLEMENTS.md).
Late-bound Object/Variant applications retain Variant results with deferred
resolution, rather than fabricating signatures. AddressOf pointer types,
target-dependent LongPtr operations, intrinsic/library signatures,
accessor declaration compatibility, full constant evaluation/overflow, fixed-string
constraints, whole-array assignment/value-temporary ownership, runtime array bounds,
and control-condition validation still require further work. Numeric literal type
suffixes are retained and checked, including Long suffixes on hexadecimal masks.

An empty semantic diagnostic list is not permission to emit native code. Future
lowering must reject parse/bind errors, error/unknown types, deferred conversions,
and executable forms without complete semantic support, as required by the
[LLVM design](../VBA_LLVM_Compiler_Design.md).
