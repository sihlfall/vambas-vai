# User-defined types and enums

The native compiler now consumes the existing Type/Enum AST, bound symbols, and
checker-owned field/constant information. It does not create a second name
resolver or attach executable data to the AST. Enum declarations remain
qualification namespaces; enum annotations, members, parameters, and array cells
use VBA Long values and the existing 32-bit operations/ABI.

## Supported UDT values

- Nominal Type declarations in standard modules, including cross-module Public
  types and distinct Private types. Identical field shapes do not imply identity.
- Boolean, Byte, Integer, Long, Single, Double, enum-typed, variable String, and Variant
  fields; nested UDT fields; fixed/dynamic arrays of the supported scalar types.
- Local variables, procedure result variables, whole-record assignment, nested
  field assignment, function returns, and fields read from returned values.
- Matching ByRef parameters, forwarding, overlapping parameter aliases, and
  parenthesized arguments that create independent value temporaries.
- With over stable UDT variables/nested fields, including nested omitted
  receivers; numeric/Variant field counters and field arrays in loops.
- Indexing, ReDim/Preserve, Erase, Mid assignment, and typed ByRef Variant views
  of compatible fields using their existing checked storage operations.

All source UDT slots are initialized at procedure entry, even when a GoTo skips
their Dim. Numeric fields begin at zero, Strings at the null String, Variants at
Empty, and dynamic arrays unallocated. Fixed array fields are initialized from
checked bounds, including Option Base, and retain fixed-size restrictions through
UDT copies and parameter forwarding. Empty and inline-recursive types are rejected.

Records are values. Copying snapshots every managed field, including independent
SAFEARRAY and Variant payloads. String snapshots retain the existing copy-on-write
semantics. A destination element remains locked during RHS evaluation. Replacing
a UDT whose owned arrays (including Variant-held nested arrays) are locked raises
error 10. Copies prepare all payloads before committing; allocation/coercion
failure leaves the destination intact and releases partial copies.

A String field exposed through ByRef Variant may already have a live BSTR* view.
Whole-record replacement prepares a BSTR for that destination before committing
and preserves the same field slot address. Existing String and numeric field
aliases therefore continue to observe writes after a record assignment. No
noalias promise is made for UDT or field parameters.

## IR, representation, and cleanup

`Program.Records` contains nominal `RecordDefinition` entries. A `Type` with
Scalar Record carries a program-local Record ID; fields retain their types,
source provenance, and fixed bounds. `OpFieldRef` returns an exact typed alias
into a record reference or an owned value snapshot. The verifier checks nominal
IDs, field indices/types, acyclic shapes, bounds, ownership, and effect contracts.
Managed loads/stores and reference reads/writes use the existing Result/try edges.

LLVM emits a distinct named aggregate for each type and typed getelementptr
instructions for fields. Ownership metadata uses LLVM constant expressions for
native sizes/offsets; the language lowerer never guesses a target layout. Runtime
init/load/store/clear operations recursively handle payloads and atomic copies.
Return transfers ownership. Regions release temporary snapshots; errors and
returns clear enumerators/element locks before activation-owned record arrays.

This UDT representation is **compiler-private**. Strings occupy private String
descriptors; fixed and dynamic array fields contain SAFEARRAY* slots; nested
records are inline. Variant fields retain the genuine 64-bit Automation layout,
and typed array elements retain their existing ABI. The containing UDT layout
does not claim compatibility with VBA/VB6 native UDT padding, inline fixed-array
storage, Declare parameters, COM records, VT_RECORD, or IRecordInfo. Public/foreign
adapters must define and validate those layouts before exposing compiled UDTs.

## Enums and limits

The checker computes explicit enum expressions and implicit zero/previous-plus-
one values, performs checked Long coercion with ties-to-even rounding, and reports
invalid values/overflow/dependency errors. Lowering reads those checked values;
it does not execute enum initializers at runtime. Qualified and unqualified
members, enum-typed fields/arrays, ByVal/ByRef parameters, and Select Case use the
normal Long paths. The native tests include cross-module enum/type use.

Remaining limits include UDT-to-Variant boxing, fixed-length String/unsupported scalar
fields, ByVal UDT declarations, UDT Len/LenB,
procedure Static storage, unsupported Const types, and foreign/public COM adapters.
Module globals and numeric Const now follow the [project contract](PROJECT_STORAGE.md).
Unsupported declarations are diagnosed even when unused. The front end supports
some forms that the native compiler still rejects. Windows/VBA/VB6-host
differential validation remains outstanding.

## Try it

```sh
go run ./cmd/vambas-compile -root examples/records -entry Main.Main -o /tmp/records.ll
clang-20 -O2 /tmp/records.ll runtime/*.c -lm -o /tmp/records
/tmp/records
echo $? # 42
```

Record helpers are compiled from `runtime/records.c`; link all sources listed in
`runtime/sources.txt`. See [runtime build instructions](../runtime/README.md).
Windows linking follows the OleAut32 instructions in [VARIANT.md](VARIANT.md).
Tests run original/optimized LLVM and native O0/O2 code with allocation tracking.
An independent C fault-injection harness checks nested copy rollback, pinned
String alias survival, locked arrays, and cleanup after constructor failure.

References: [UDT declarations](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/a5fe374b-eddf-4808-9e73-477914940dba),
[UDT values and Long enum values](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/c86480b2-aef2-4488-b177-f55e13cc51f2),
and [parameter declarations](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/78bcb344-4966-4401-bb55-72729790ebee).

## Arrays of UDT and captured With

Nominal UDT arrays now support fixed/dynamic storage, multidimensional indexing,
whole-array copies/returns, ByRef arrays/elements/fields, ReDim/Preserve, Erase,
and array fields inside other UDTs. IR uses ArrayRecord/ElementRecord with the
same nominal Record ID as the element type. Bounds, aliases, and element locks
follow the existing scalar-array model.

The runtime adds record-type metadata before a SAFEARRAY-shaped descriptor.
Elements use the compiler's private UDT layout and lifecycle functions. This is
compiler-private record-array storage, including on Windows; it does not supply
Automation IRecordInfo or expose VT_RECORD payloads through Variant. Scalar
SAFEARRAY and VARIANT layouts are unchanged. UDT/UDT-array boxing into Variant,
For Each over UDT elements, and foreign record-array interop remain unsupported.

Copy and Preserve acquire all managed fields before replacement. Growth initializes
new elements, shrink destroys discarded elements, and fixed Erase prepares fresh
default elements before committing. Failed allocation/copy leaves the old array
unchanged. Record cleanup traverses nested arrays, strings, Variants, and UDTs.
Array replacement checks locks on both the descriptor and owned nested arrays.

With now captures a UDT receiver once, including array element expressions and
nested With headers. IR Receiver slots keep the pointer; persistent element-token
slots keep all ancestor descriptor locks. Header expressions and indices are not
re-evaluated by member accesses. Value-producing UDT receivers use an owned
snapshot for reads; scalar writes to a read-only function result retain the
checker's storage restrictions.

Scope exit, outgoing GoTo, loop/procedure Exit, propagated errors, and Resume
release captured resources in order. A GoTo handler retains them until recovery
chooses its destination or the activation exits. Attempting to ReDim a captured
array raises error 10. Captured-pointer checks raise 91 for an uninitialized With
receiver. Ordinary GoTo cannot enter an uninitialized resource scope.

See [error handling](ERROR_HANDLING.md) for policy/recovery semantics. Native tests
cover managed copies, Preserve, fixed Erase, nested single-evaluation receivers,
locks across handlers, and failure injection through every preparation stage.


UDT fields can now own native object references. Copies acquire the new references
before releasing old ones, and whole-record replacement publishes the new value
before old child objects can run termination callbacks. Object With receivers and
Variant object references follow the [class contract](CLASSES.md).
