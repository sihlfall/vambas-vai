# Integral operators and string library

The ambient `VBA` library now declares `AscW`, `StrComp`, `InStr`, `Left`/`Left$`,
`String`/`String$`, `Split`, `IIf`, and `vbBinaryCompare`. Ordinary binder symbols
and checker signatures identify these operations; user declarations can shadow
unqualified names. `VBA.AscW`, for example, still selects the library declaration.
This follows the existing typescript-go-style separation of declarations,
checking, and code generation. VBA's Variant, mutable storage, and error behavior
remain explicit in semantic IR and the runtime.

The nine runtime-helper forms listed above use the generic `OpLibraryCall` and
catalog member IDs. [The IR contract](../ir/library.go) derives arity, exact result
type, and base effects from the catalog; the
[backend ABI table](library.go) owns native helper names and trailing selectors.
Helper declarations are generated from that table and emitted once per native
symbol. Verification rejects malformed calls before emission, and the backend
reports a missing implementation by qualified member name. Left/Left$ and
String/String$ retain distinct identities and exact result types while sharing
their existing C helpers.
Explicit scalar conversions, `CVErr`, and `IsError` also use this verified path;
see the [Stage D data-type contract](DATA_TYPES.md).

Procedural-module qualification resolves to those same declarations and runtime
calls. `VBA.Strings.Left$` and `Strings.Left$`, for example, select the library
function; ordinary user modules may shadow `Strings`, while `VBA.Strings` remains
explicit. The current catalog exposes Strings, Conversion, Information,
Interaction, and Constants namespaces. Namespace qualifiers have no runtime
receiver evaluation. See the [binding contract](../binder/README.md).

## Literals and integral arithmetic

Executable hex and octal literals reuse the checker's signed-width interpretation:
`&HFFFF` is Integer -1, `&HFFFF&` is Long 65535, and `&H80000000` is Long minimum.
Enum initializers and executable expressions therefore agree. Decimal floating
literals retain their original precision-specific parsing.

`\` rounds operands to their effective integral type using ties-to-even rounding,
then truncates the quotient toward zero. `Mod` uses the corresponding signed
remainder. Division by zero raises error 11; a typed quotient outside its effective
Integer/Long range raises error 6. Variant arithmetic retains Null and can promote
an overflowing Integer quotient to Long, or Long to Double. These operations use
new checked IR operations with mandatory Result/Try continuations.

Native code widens operands to i64 and selects a safe nonzero divisor before
`sdiv`/`srem`. It then checks the result's declared-width round trip. Integer/Long
minimum divided by -1 cannot introduce LLVM poison, and minimum `Mod -1` is zero.
LLVM performs any constant folding and strength reduction. See Microsoft's
[operator semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/09fdf266-a9d5-4ecb-b7d8-ffd1361cb349)
and [arithmetic promotion rules](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/e070115f-8d40-40cf-ac6d-ab18b9c6c906).

## Library behavior

| Operation | Result and behavior |
|---|---|
| `AscW(String)` | Integer containing the signed first UTF-16 code unit; empty input raises 5, Null raises 94. |
| `StrComp(String1, String2, [Compare])` | Variant(Integer) -1, 0, or 1; Null operands propagate. Binary comparison includes embedded NULs and orders UTF-16 code units. |
| `InStr([Start], String1, String2, [Compare])` | Variant(Long), one-based match position or zero; accepts the two-string shorthand, named arguments, and omitted Start. Start defaults to 1; Null strings propagate, invalid Start raises 5. |
| `Left(String, Length)` / `Left$` | Variant(String)/Null versus String. Length is coerced to Long, negative lengths raise 5, zero yields empty, excessive lengths yield the whole input. |
| `String(Number, Character)` / `String$` | Repeat the first UTF-16 unit of a String, or an ANSI character code modulo 256. Null arguments propagate in the Variant form; the String form raises 94. Negative counts and empty character strings raise 5. |
| `Split(Expression, [Delimiter], [Limit], [Compare])` | Owned, zero-based String SAFEARRAY; defaults space/-1/binary. Preserves empty fields, trailing separators, and the unsplit remainder at the limit. Empty input or limit zero yields a zero-element array, with upper bound -1. Null raises 94. |
| `IIf(Expr, TruePart, FalsePart)` | Variant snapshot of the selected value. All arguments, including both alternatives, evaluate in source order before selection. A Null condition selects FalsePart. |
| `vbBinaryCompare` | Ambient Long constant zero, including qualified `VBA.vbBinaryCompare`. |

`InStr` overload selection is recorded in the checker call plan. Lowering evaluates
explicit arguments in source order, snapshots their values, then materializes
omitted library defaults using the call's source provenance. It never evaluates
an ambient declaration as if it belonged to a source file.

The runtime preserves embedded zero, surrogate pairs, and unpaired UTF-16 units.
String results have independent mutable storage: later ByRef or Mid assignment
cannot change an earlier argument snapshot. Split elements are BSTRs in a genuine
String SAFEARRAY; boxing into Variant preserves `VT_ARRAY | VT_BSTR` (8200).
Producing helpers take fresh output storage and leave it untouched on failure.
Partial arrays and intermediate allocations are released on every failure path.

`IIf` is an eager library call, so lowering does not turn it into conditional
control flow. An error in the unselected alternative still propagates, and
already completed ByRef writes remain visible. See the
[IIf reference](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/iif-function).

## Runtime profile and remaining limits

Comparisons currently support binary mode 0 (`vbBinaryCompare`), the omitted
mode, and -1 (the current binary Option Compare profile). Text, database, and
LCID comparison modes are unsupported and return error 5; `Option Compare Text`
is not accepted by the compiler. Numeric character codes in `String` use Windows'
active ANSI code page on Windows, and Windows-1252 in the portable runtime.
DBCS multi-byte numeric characters are not implemented. A String character
argument and `AscW` operate directly on UTF-16 and bypass ANSI conversion.
These limits are explicit; this is not full host locale emulation.

Numeric coercions use the existing Variant runtime profile (Windows Automation
or portable C-locale decimal coercion). Remaining Variant subtypes, objects, and
error handlers retain their existing compilation restrictions. Numeric Const declarations and typed Optional parameters are covered by the
[project storage contract](PROJECT_STORAGE.md). Variant logical operators remain
separate work.

Link generated modules with the sources in `runtime/sources.txt`, just as for existing
[Variant](VARIANT.md) and [array](ARRAYS.md) operations; Windows also needs OleAut32.
The native tests execute original and optimized LLVM plus clang O0/O2, track
allocations, check eager side effects/errors, and inject failures into every
allocation of substring/repetition/Split/IIf result production.

## Try it

The [example](../examples/builtins/Main.bas) exercises every requested built-in
and both integral operators. From the repository root, with LLVM 20:

```sh
go run ./cmd/vambas-compile -root ./examples/builtins -entry Main.Run -o /tmp/vba-builtins.ll
clang-20 -O2 /tmp/vba-builtins.ll runtime/*.c -lm -o /tmp/vba-builtins
/tmp/vba-builtins
echo $? # 42
```
