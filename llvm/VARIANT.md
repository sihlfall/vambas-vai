# Variant values and ByRef storage

The compiler supports Variant locals, untyped declarations and function results,
parameters, assignment, and value snapshots. Empty is the initial value. Null is
a value distinct from an execution error. Supported dynamic operators are `+`,
`-`, `*`, `/`, unary `-`, and `&`; numeric arithmetic promotes its subtype on overflow.
`VarType`, `IsEmpty`, `IsNull`, `IsError`, `CVar`, `CByte`, `CVErr`, `CInt`, `CLng`, `CSng`, `CDbl`, `CBool`, `CStr`, and Variant
`Len`/`LenB` are ordinary ambient declarations identified by their symbols.
Relational comparisons retain Null and declared-operand coercion distinctions;
numeric For loops, array For Each, and implicit Null-as-False conditions are
described in the [control-flow contract](CONTROL_FLOW.md).
User declarations can shadow them. Optional Variant parameters without defaults
use Missing (`VT_ERROR`, `DISP_E_PARAMNOTFOUND`); `IsMissing` recognizes that value.
Error values, explicit conversion versus implicit assignment, and Byte boundaries
follow the [Stage D data-type contract](DATA_TYPES.md).

## Memory ABI

On the currently supported **64-bit targets**, the value is a genuine Automation
VARIANT container, not a pointer to a private boxed object:

| Offset | Size | Meaning |
|---|---|---|
| 0 | 2 | VARTYPE |
| 2 | 6 | Reserved, initialized to zero |
| 8 | 16 | Value union, including space for BRECORD's two pointers |

The container is 24 bytes, aligned to 8 bytes. Integer uses `VT_I2`, Long `VT_I4`,
Byte uses `VT_UI1`, Single `VT_R4`, Double `VT_R8`, Boolean `VT_BOOL` with True=-1, String `VT_BSTR`, Empty `VT_EMPTY`, and Null
`VT_NULL`. Arithmetic can produce `VT_R8` after Long overflow. The runtime can
also constructs, transports, queries and explicitly converts `VT_ERROR` values. It rejects
unsupported subtype ownership/coercions with type mismatch; it never places a
private String descriptor in a BSTR arm. The C header reserves the full DECIMAL
overlay and record arm. The Decimal overlay now carries validated 96-bit
coefficients and scale/sign metadata; Date (`VT_DATE`) and scaled Currency
(`VT_CY`) use eight-byte payloads. See the [numeric profile](DATA_TYPES.md).

On Windows, BSTR allocation/free uses `SysAllocStringLen`/`SysFreeString`.
Coercions and dynamic arithmetic use OleAut32's `VariantChangeType` and `Var*`
helpers. The runtime checks size, alignment, and field offsets against the
Windows SDK's VARIANT at compile time. Linux uses length-prefixed UTF-16 buffers
with embedded NUL support and the same container layout, **not allocations that
can be handed to Windows for freeing**. Its numeric text conversions implement
a C-locale decimal subset, not full Windows locale/Automation behavior.

Compiled procedures still use the [private status/output calling convention](README.md#initial-private-abi).
This implements the value memory ABI and internal ByRef semantics; it does not
export VBA-native/COM entry points or implement `Declare` calls. The emitter and
runtime currently require 64-bit pointers. Windows execution and differential
tests against VBA/VB6 remain outstanding; Linux execution is tested.

## ByRef rules

These cases intentionally produce different storage:

| Actual argument to `ByRef value As Variant` | Representation and assignment |
|---|---|
| Variant variable | Pointer to its original container; assignment may change subtype |
| Integer/Long/Single/Double/Boolean variable | Temporary `VT_BYREF \| original-type` container pointing at the original slot; assignment coerces to that declared type before writing |
| String variable | `VT_BYREF \| VT_BSTR` containing a real BSTR*; typed String and Variant aliases observe every write immediately |
| Parenthesized argument or value expression | Independent Variant temporary; assignment may retag the temporary |
| Forwarded ByRef Variant parameter | The same pointer/view; no nested BYREF-Variant wrapper |
| Typed array variable | `VT_BYREF \| VT_ARRAY \| element-tag` view over the original SAFEARRAY**; writes retain declared element type |

Arrays now have explicit descriptor ownership and active element-reference locks;
see the [array ABI and lifetime contract](ARRAYS.md). Arrays of Variant and
Variants holding typed arrays preserve their distinct element tags.

There is no deferred copyback for typed Variant views. Conversion failure leaves
the failing destination unchanged; earlier writes survive failure and propagation.
A ByVal Variant or ordinary value read dereferences borrowed views and owns its
snapshot. BSTR copies have independent ownership. Clearing a borrowed wrapper
never frees its pointee. `VT_BYREF | VT_VARIANT` inputs are followed once; invalid
nested wrappers are rejected.

Typed String slots normally use the optimized private String representation.
Exposing one through Variant ByRef first materializes a real BSTR slot. That
storage mode persists until the slot is destroyed. Later String loads snapshot
the current BSTR, String assignments update the same BSTR*, and Mid assignment
can mutate its units. Existing String snapshots retain their old contents.
Function returns lift exposed String storage back into an ordinary owned String
value, so substring views never outlive a borrowed BSTR. Materialization and
copies can raise error 14; all commits happen after successful allocation.

Semantic IR makes managed loads/stores, boxing, coercion, and String exposure
explicit fallible operations. Each is followed by `try`; errors use the existing
status/error channel rather than `VT_ERROR`. Parameter acquisition or result
lifting failure records statement ID zero for the activation boundary. Cleanup
covers acquired parameters, locals, temporaries, and evaluated value snapshots.
LLVM can specialize linked runtime code while preserving tags, aliases, and
surviving errors/effects.

## Try it

Run from the repository root:

```sh
go run ./cmd/vambas-compile -root examples/variant -entry Main.Main -o /tmp/variant.ll
clang-20 -O2 /tmp/variant.ll runtime/*.c -lm -o /tmp/variant
/tmp/variant
echo $? # 42
```

Variant modules reference the C runtime. On Windows link `oleaut32` instead of
`m`. To expose runtime alternatives to LLVM optimization on Linux:

```sh
mkdir -p /tmp/vba-runtime-ir
while IFS= read -r source; do
  clang-20 -O1 -S -emit-llvm "runtime/$source" -o "/tmp/vba-runtime-ir/$source.ll"
done < runtime/sources.txt
llvm-link-20 /tmp/variant.ll /tmp/vba-runtime-ir/*.ll -o /tmp/variant.bc
opt-20 '-passes=default<O2>,verify' /tmp/variant.bc -o /tmp/variant-opt.bc
lli-20 /tmp/variant-opt.bc
```

Native tests assemble and verify modules, execute original/optimized bitcode,
and run Clang O0/O2 executables. Allocation tracking includes both private String
and C runtime allocations. An independent C harness checks the ABI, BSTR layout,
typed references, ownership, and injected allocation/coercion failures.

The compiler still diagnoses Variant logical operators, unsupported comparison
subtypes/modes, Mid reads
or assignment on Variant storage, unsupported array types/forms, Decimal/Currency/Date values,
and unsupported optional parameter types. [ParamArray calls](PARAMARRAY.md) now
preserve Variant parameter bindings, including typed ByRef storage views. Supported scalar defaults
now follow the [project contract](PROJECT_STORAGE.md). Byte execution is still pending;
[Single/Double execution and subtype promotion](FLOATING_POINT.md) are supported.
Errors use the existing On Error/Resume status contract; public interop adapters
remain separate work. See the
[Microsoft VARIANT definition](https://learn.microsoft.com/en-us/windows/win32/api/oaidl/ns-oaidl-variant),
[Automation ownership rules](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/automat/variant-manipulation-functions),
and [VBA language specification](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/).


Native classes extend the container with an explicitly private object tag and
constrained object ByRef views. This is not an Automation object payload; future
interop must marshal it through COM adapters. See the [native object contract](CLASSES.md)
for default value coercion, ownership, runtime invocation, and cycle behavior.
