# Stage D data-type support

Byte conversions, Error/Decimal Variants, and typed Date/Currency values execute
through the catalog and existing verified library-call IR. LongLong and LongPtr
remain pending executable representation work. The runtime still requires 64-bit
pointers; VBA Long remains a signed 32-bit type. This increment does not define
a target-dependent LongPtr ABI.

The value layouts follow Windows Automation. This is a memory-layout contract;
compiled procedures retain the compiler's private status/output ABI.

| Value | Storage | Automation tag |
|---|---|---|
| Date | 8-byte IEEE binary64 `DATE` serial | `VT_DATE` (7) |
| Currency | 8-byte signed `CY` integer scaled by 10,000 | `VT_CY` (6) |
| Decimal | 16-byte `DECIMAL`: reserved/type at 0, scale at 2, sign at 3, high coefficient bits at 4, low 64 bits at 8 | `VT_DECIMAL` (14), overlaid inside the 24-byte Win64 Variant |

In a standalone/by-reference DECIMAL the reserved word is zero. In an owned
Variant it overlaps the `VT_DECIMAL` tag. Windows builds assert these sizes and
offsets against the SDK; C tests independently check the byte layout and borrowed
views. Linux native execution is tested. Execution against Windows/OleAut32 and
VBA/VB6 remains unvalidated.

## Byte

`CByte(Expression As Variant) As Byte` returns an unsigned 8-bit value. The result
keeps Byte identity through function returns, Variant boxing (`VT_UI1`, 17), and
typed scalar/array storage. `vbByte` and `VBA.vbByte` are now available.

Conversion rounds to nearest with ties to even, then checks 0..255. For example,
`CByte(254.5)` is 254, `CByte(255.5)` raises overflow 6, and `CByte(-0.5)` is zero.
Empty and False convert to zero; Boolean True converts to 255, including implicit
assignment and typed ByRef writes. Numeric -1 overflows. Null raises 94 and
invalid nonnumeric input raises 13. A failing conversion leaves its destination
unchanged, including a Byte cell exposed through a ByRef Variant view.

Numeric text follows the existing runtime profile: portable C-locale decimal
parsing on Linux and Automation conversion on Windows. Locale/encoding-sensitive
conversion expansion belongs to subsequent work.

## Error values

`CVErr(Expression As Variant) As Variant` creates a `VT_ERROR` (10) value, and
`IsError(Arg As Variant) As Boolean` tests that subtype. `vbError` is available
unqualified and through `VBA`. Function aliases include `VBA.Conversion.CVErr`
and `VBA.Information.IsError`; normal user shadowing and named argument binding
apply. Enum constants retain root exports until enum declarations are modeled.

`CVErr` returns an existing Error value unchanged. Otherwise it first performs
ordinary Long coercion, including ties-to-even rounding, and then requires a code
between 0 and 65535 inclusive. A successfully coerced out-of-range code raises 5;
coercion itself can raise 6, 13, or 94. For example, `CVErr(65535.5)` raises 5,
while `CVErr(1e30)` raises 6.

Error values preserve their code through Variant assignment, snapshots, returns,
ByVal/ByRef calls, Variant arrays, Collection items, and ParamArray packing.
Creating or returning an Error value does not raise a runtime error or update
Err. Missing remains the implementation-defined `VT_ERROR/DISP_E_PARAMNOTFOUND`
sentinel: `IsError(Missing)` and `IsMissing(Missing)` are both True, and `CVErr`
preserves it without applying the standard-code range check.

Explicit conversions differ from implicit Let coercion:

| Operation | Behavior for an Error value |
|---|---|
| `CByte`, `CInt`, `CLng`, `CBool`, `CSng`, `CDbl` | Convert the signed Long error code to the declared result type, with the normal range checks. |
| `CStr` | Produce `Error ` followed by the signed decimal code. |
| `CVar`, `CVErr` | Preserve the Error subtype and code. |
| Implicit typed assignment or typed ByRef write | Raise type mismatch 13 and preserve the destination. |
| Arithmetic or `&` concatenation | Raise 13, including when the other operand is Null. |
| Relational comparison of two standard Error values | Compare the codes numerically and produce a Variant(Boolean). |
| Relational comparison mixing Error with another subtype | Raise 13, including a Null operand. |

MS-VBAL leaves comparison of implementation-defined Error codes unspecified.
This profile rejects those comparisons with 13. Dynamic Variant logical operators
remain outside the executable subset.

The Error payload is a signed 32-bit field in the existing 24-byte Automation
Variant layout. It has no owned allocation. Borrowed `VT_BYREF|VT_ERROR` views
are dereferenced into snapshots. Runtime failures still use status/output and
Result/Try control edges; they do not become Error Variant results automatically.

## Date, Currency, and Decimal

`CDate(Expression) As Date`, `CVDate(Expression) As Variant`,
`CCur(Expression) As Currency`, and `CDec(Expression) As Variant` are catalog
declarations with ordinary module/root qualification and named arguments.
`vbDate`, `vbCurrency`, and `vbDecimal` are root-exported constants.

Date and Currency support locals, globals, UDT/class fields, typed function
parameters/results, ByVal snapshots, ByRef aliases, SAFEARRAY storage, and Variant
boxing/unboxing. Their typed arrays retain tags `VT_ARRAY|VT_DATE` (8199) and
`VT_ARRAY|VT_CY` (8198), with eight-byte cells. Omitted typed optional parameters
have fresh zero values. Decimal stays inside Variant, including Variant arrays,
Collection items, and parameters/results; there is no VBA `As Decimal` type.

Date stores Automation serials without losing the Date tag or the sign of a
negative serial's fraction. Numeric conversion accepts finite serials strictly
between -657435 and 2958466; ISO dates validate the Gregorian calendar and years
100..9999. The current portable text profile accepts `YYYY-MM-DD`, optionally
followed by ` HH:MM:SS` or `THH:MM:SS`, and time-only `HH:MM:SS`. Numeric text
also follows existing C-locale conversion. `CStr(Date)` emits the full ISO date
and time at whole-second resolution. This profile is not Windows regional date
parsing/formatting. Numeric conversion and arithmetic preserve the stored binary64
value. Subtracting two Dates produces Double; Variant date overflow can promote to
Double, while a typed Date destination checks its range before committing.

Currency keeps the full signed 64-bit range, including
`-922337203685477.5808` through `922337203685477.5807`. Decimal text and Currency
`@` literals are rounded exactly to four places using ties to even. Literals are
evaluated using arbitrary-precision integers in the compiler, with no Double
round-trip or runtime String allocation. Binary floating input is rounded from
its actual stored value. Add/subtract and Currency multiplication check overflow
using wider integer intermediates; mixed floating multiplication and `/` produce
floating results according to the existing declared result types.

Decimal uses a 96-bit unsigned coefficient, a scale from 0 to 28, and the Windows
sign encoding 0/0x80. Decimal text, integer and Currency inputs retain exact decimal
values. Single/Double input uses the existing 7/15-significant-digit numeric text
profile. Addition, subtraction, multiplication, division, negation and comparisons
use integer coefficients. Intermediates use eight 32-bit limbs; scale reduction
rounds once with guard/sticky information. Overflow raises 6; Decimal division by
zero raises 11. `CStr` emits exact decimal digits, and integral conversions round
without passing through Double. A deterministic set of 160 arithmetic cases is
checked independently against Go arbitrary-precision rational arithmetic at both
native optimization levels.

Construction, arithmetic, conversion and string-production failures preserve the
destination. `CDate`/`CVDate` and `CDec` reject Error inputs; `CCur` uses the Error
code for explicit conversion. Null keeps the existing 94/coercion and propagation
rules. Typed Date/Currency `Len`/`LenB` return eight; Variant arguments retain the
existing String interpretation.

The following remain deferred: date literals, regional date formats and calendar
library functions; Date/Currency Const evaluation and explicit optional defaults;
expansion of For-loop counters to these representations; wider integers and
LongPtr. Decimal/Currency text parsing is limited to 1024 ASCII units and a
256-bit intermediate coefficient. The runtime is built with Clang and uses its
128-bit integer extension for Currency; MSVC compilation is not claimed.

## Implementation and validation

Explicit scalar conversions share `vba_convert_explicit`; `vba_cverr` and
`vba_iserror` implement construction and subtype testing. These helpers are
declared in [conversion.h](../runtime/conversion.h), implemented in
[conversion.c](../runtime/conversion.c), and compiled with the other sources in
[`runtime/sources.txt`](../runtime/sources.txt). Native helper names and type selectors remain in
[the ABI table](library.go). Catalog calls retain exact result types, Variant
argument snapshots, effects, and mandatory error continuations. No per-function
opcode was added.

Native fixtures run original/optimized LLVM and Clang `-O0`/`-O2` with allocation
tracking. C tests independently check payload widths, alias writes, unchanged
outputs, nonfinite inputs, and every allocation failure in `CStr(Error)`.
Linux x86-64 is validated; Windows execution and comparison against a VBA/VB6
host remain outstanding.

The behavior follows the pinned
[MS-VBAL revision](https://officeprotocoldoc.z19.web.core.windows.net/files/MS-VBAL/%5BMS-VBAL%5D-241112.pdf),
especially §§5.6.9.3–5, 6.1.2.3.1.1–14 and 6.1.2.7.1.5. Microsoft also documents
the [explicit Error conversion distinction](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/cverr-function)
and [conversion ranges](https://learn.microsoft.com/en-us/office/vba/language/concepts/getting-started/type-conversion-functions).
Windows layouts are specified by Microsoft's
[CY](https://learn.microsoft.com/en-us/windows/win32/api/wtypes/ns-wtypes-cy-r1),
[DECIMAL](https://learn.microsoft.com/en-us/windows/win32/api/wtypes/ns-wtypes-decimal-r1),
and [VARIANT](https://learn.microsoft.com/en-us/windows/win32/api/oaidl/ns-oaidl-variant)
definitions.
