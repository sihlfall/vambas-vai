# Arrays and ByRef storage

Array storage now extends the existing parser, binder, checker, semantic IR, and
LLVM path. Like the `typescript-go` consumers, lowering uses checked symbols,
types, and call plans. VBA requires additional explicit operations for descriptor
ownership, runtime bounds, and references to mutable array elements.

The compiler supports Boolean, Byte, Integer, Long, Single, Double, Date, Currency,
String, Variant, Object, and supported class elements. UDT arrays also have a
separate private descriptor path. See [object/class arrays](OBJECT_ARRAYS.md).

- Dynamic `Dim a() As Long`, and fixed `Dim a(1 To 3) As Long` declarations.
- One to sixty dimensions, explicit lower bounds, and module `Option Base 0/1`.
  Fixed bounds are checked constants and arrays are initialized on procedure
  entry. ReDim bounds are executable expressions evaluated in source order.
- Checked element reads/writes, direct and forwarded whole-array ByRef calls,
  ByRef elements, parenthesized value temporaries, and typed array returns.
- `ReDim`, `ReDim Preserve`, and multiple declarations in one ReDim statement.
  Preserve permits only the last upper bound to change, retaining element type,
  rank, every lower bound, and all earlier dimensions.
- `Erase`, `LBound`, `UBound`, `IsArray`, and array-aware `VarType`.
- Whole-array assignment and independent copies, including arrays boxed into
  Variant, arrays of Variant, and nested Variant-held arrays. Boxing retains the
  actual element tag: Long arrays use `VT_ARRAY | VT_I4`, not `VT_ARRAY | VT_VARIANT`.

## Memory ABI

Array variables contain SAFEARRAY pointers. ByRef declared-array parameters
receive SAFEARRAY**, so replacing a dynamic descriptor through one alias is
immediately visible through another. No `noalias` assumption is made.

The current 64-bit SAFEARRAY representation is:

| Offset | Field | Representation |
|---|---|---|
| 0 | cDims | uint16 |
| 2 | fFeatures | uint16 |
| 4 | cbElements | uint32 |
| 8 | cLocks | uint32 |
| 16 | pvData | pointer |
| 24 | rgsabound | count/lower-bound pairs, uint32/int32 |

The one-bound descriptor is 32 bytes, aligned to eight. Descriptors retain the
element type and BSTR/Variant ownership flags. Fixed arrays carry FADF_FIXEDSIZE,
including when passed through an unspecified-rank parameter. Source dimensions
are mapped to the descriptor's reversed bound order; data is column-major, with
the first source subscript varying fastest. String cells contain real BSTR
pointers; Variant cells use the existing 24-byte Automation container.
Single and Double cells use four/eight bytes with VT_R4/VT_R8 respectively;
typed ByRef Variant element aliases keep those storage types enforced.

Windows Automation-compatible arrays use OleAut32 SAFEARRAY APIs. Private object
and UDT payloads have explicit runtime ownership paths. Compile-time assertions compare the layout with Windows
SDK types. Linux implements the same memory layout and supported ownership rules
with its own allocator; these allocations cannot be freed by Windows OleAut32.
The allocation metadata preceding a portable descriptor is runtime-private.
The compiled procedure calling convention remains the private status/output ABI;
this does not add public COM or Declare adapters. Windows execution and VBA/VB6
host differential tests remain outstanding.

## String and Byte() assignment

Direct assignments copy the String's UTF-16LE payload to a resizable Byte array,
and copy a Byte array back to a String:

```vb
Dim bytes() As Byte, text As String
bytes = "AB"                 ' 65, 0, 66, 0
text = bytes                 ' "AB"
bytes(0) = 67                ' text retains "AB"
```

The result of String-to-Byte() assignment has one dimension, lower bound zero,
and two bytes per UTF-16 unit, with no length prefix or terminator. `Option Base`
does not affect this bound. Empty and null Strings produce allocated zero-element
arrays (`LBound = 0`, `UBound = -1`). Conversion back reads the complete contiguous
payload, including fixed arrays and nonzero/multidimensional bounds. It drops a
trailing odd byte; an unallocated array produces an empty String. Embedded nulls,
surrogate pairs, and individual UTF-16 units retain their binary values.

Assignments through Variant sources and typed ByRef Variant views use the same
conversion. A real Variant assigned a String retains a String subtype; a Variant
assigned Byte() retains `VT_ARRAY | VT_UI1` (8209). Fixed arrays cannot be assigned
as a whole. Replacing a borrowed/locked array raises error 10; allocation failure
preserves the previous value. Byte storage and array elements use unsigned i8,
checked 0..255 coercion, and `VT_UI1` (17) for Variant views. This does not add
`CByte`, `StrConv`, or ANSI/code-page conversion.

See the [runnable example](../examples/bytes/Main.bas) and
[MS-VBAL 5.5.1.2.6](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/acf31907-ebaf-4830-9892-bfabe6fe1416).
Native tests check these rules, snapshots/aliases, and allocation-failure rollback;
Windows/VB6-host differential validation remains outstanding.

## Aliases and lifetimes

Whole-array references and element references have different lifetimes:

| Use | Behavior |
|---|---|
| `Grow a` / `Grow a()` with `ByRef a() As Long` | Alias the descriptor slot; ReDim updates the caller |
| `Change a(i)` with matching ByRef scalar type | Bounds-check, lock the descriptor, and alias that cell |
| Typed array passed to `ByRef v As Variant` | Borrowed `VT_BYREF | VT_ARRAY | element-tag` view of SAFEARRAY**; the declared element type remains enforced |
| String element passed ByRef | Borrowed proxy over the original BSTR*; String/Variant aliases observe writes immediately |
| ByVal Variant or parenthesized array argument | Independent deep copy, including BSTR and Variant payloads |
| Ordinary element read | Copy the value, then release its descriptor lock before evaluating later expressions |

An element passed ByRef stays locked through later argument evaluation and the
callee. A destination element stays locked across RHS evaluation. Resizing,
erasing dynamic storage, or replacing locked storage raises error 10; resizing a fixed array
through a parameter also raises error 10. Completed element writes survive a
later failure. Parenthesized arguments use temporary storage and do not retain a
lock on the original array after copying the value.

String element proxies refresh the current BSTR pointer on every access; they
never embed the private String descriptor in an array cell. Mid assignment can
modify typed String cells directly. A String read snapshots the current contents
before releasing its lock, preserving old values across later array changes.

Array values own their descriptors and payloads. ReDim/assignment prepare new
storage before committing; allocation or copy failure leaves the old destination
intact. Array allocation raises error 7, BSTR copying can raise error 14, and
invalid indices, dimensions, or Preserve bounds raise error 9. Dynamic Erase
releases its descriptor; fixed Erase resets cells while preserving shape and
live cell addresses. Erasing a Variant-held array retains its array subtype with
unallocated storage; IsArray stays True and bounds queries raise error 9.

IR represents element access with an owned lock token, a typed reference, and
explicit checked reads/writes. Value reads release the token immediately; ByRef
uses retain it until region cleanup. The verifier checks type compatibility,
Result/try edges, complete cleanup, and use after explicit release. Error exits
release tokens before destroying activation-owned arrays.

## Inline typed array access

Boolean, Byte, Integer, Long, Single, Double, Date, and Currency arrays use
compiler-generated LLVM helpers marked `alwaysinline`, whether the array is a
local, a UDT field, a module global, or any typed ByRef parameter. ByRef arrays
bind only to storage of exactly their declared type, so a typed descriptor is
never erased storage. Managed element indexing, erased Variant arrays, and
ParamArray retain the general runtime path. Clang optimizes the helpers with
the procedure body without linking runtime bitcode or enabling LTO.

### Shape snapshots

Each array keeps a snapshot of its descriptor pointer, data pointer, rank, lock
count, and leading bounds in a private alloca. SROA promotes it to registers.
Element stores therefore cannot clobber it, and LLVM can hoist loads and checks
out of loops. Element access is then arithmetic, a bounds check, and a load or
store. `LBound` and `UBound` read the snapshot; a dimension beyond the stored
bounds reads the descriptor.

The snapshot is reloaded wherever the
[identity retention analysis](../ir/identity/README.md) says it may be stale:

- in the prologue;
- on the success path right after an instruction that may invalidate the
  array (`ReDim`, `Erase`, replacement, or a call, release, or reentrant
  operation whose summary may change it, through any alias);
- after a real lock acquisition or release, which reloads only the lock count;
- at the entry of a region that some incoming edge, including error, Resume,
  and GoSub edges, may reach with a stale snapshot. This is a must-analysis
  over the region graph; a loop without such events reloads nothing.

The helpers preserve unallocated-array, rank, dimension, and subscript errors
(9), overflowing upper bounds (6), and lock saturation (10), at their original
positions. Index differences and upper bounds use widened arithmetic. There is
no `noalias` assumption or unchecked `inbounds` pointer arithmetic.

### Element locks

Each element token takes one of three lock modes:

- **Omitted:** the analysis proves the lock unobservable. From acquisition to
  release, no path may invalidate the array's dependency group or observe its
  lock count. Calls are covered by their summaries, so `Touch a(i)` needs no
  lock when `Touch` cannot resize `a`.
- **Virtual:** the only observers are inline accesses to the same array, as in
  `a(i) = a(i - 1) + i`. There is no runtime call. Each observer checks
  saturation against the snapshot's count plus the virtual locks live at it, as
  if the lock were held. The array must still be retained, so no ReDim can
  notice the missing lock.
- **Real:** a call that may resize or index `a`, another alias, or a token moved
  into With scope storage needs the actual descriptor lock.

Scalar lock tokens need no Variant view or String proxy; cleanup calls the
runtime's narrow `vba_array_lock`/`vba_array_unlock` ABI, preserving platform
lock behavior and failure cleanup. Managed elements retain their existing
ownership machinery.

The [array-access example](../examples/array-access/Main.bas) contains sum and
fill loops over a ByRef array. With Clang 20 `-O2` on Linux x86-64, the fill
loop previously reloaded the descriptor pointer, rank, count, lower bound, lock
count, and data pointer on every iteration, because the element store may alias
them. With snapshots its body has no descriptor loads; only the store touches
memory. A prefix sum `a(i) = a(i - 1) + i` previously made two runtime lock calls
per iteration and now makes none. The prologue reload adds about 20 static
instructions per routine. These are static observations, not execution-time
measurements. `TestInlineArrayAccessNativeCode` checks that both routines remain
call-free without runtime LTO. `TestNativeArraySnapshotRefresh` exercises every
reload trigger and fails if any is omitted. Native tests also check error
ordering, lock saturation, nested access, and preservation of the destination on
failure.

```sh
go run ./cmd/vambas-compile -root examples/array-access -entry Main.Main -o /tmp/array-access.ll
clang-20 -O2 /tmp/array-access.ll runtime/*.c -lm -o /tmp/array-access
/tmp/array-access
echo $? # 55
```

## Try it

From the repository root:

```sh
go run ./cmd/vambas-compile -root examples/arrays -entry Main.Main -o /tmp/arrays.ll
clang-20 -O2 /tmp/arrays.ll runtime/*.c -lm -o /tmp/arrays
/tmp/arrays
echo $? # 42
```

Array helpers are compiled from `runtime/arrays.c`; link all sources listed in
`runtime/sources.txt`. See [runtime build instructions](../runtime/README.md).
On Windows link `oleaut32` instead of `m`. For whole-module LLVM optimization,
compile that runtime to LLVM and link it with the emitted module as described in
the [Variant instructions](VARIANT.md#try-it).

Tests cover descriptor offsets and dimension ordering, aliases, fixed/dynamic
Erase, preservation, nested copies, String snapshots, active element locks,
cleanup on failure, and injected allocation/copy failures. Native tests run
original/optimized bitcode and Clang O0/O2 executables, tracking live allocations
in both the String and array runtimes.

Remaining limits include Date/Currency/object element
types, fixed-length String elements, Array constructor intrinsics,
Variant actual arguments adapted to a declared typed-array ByRef formal, and
indexing a procedure-returned array directly without first storing its value.
Supported loops and jumps now use resource-scope cleanup; see
[control flow](CONTROL_FLOW.md). On Error/Resume, globals/Static storage,
and foreign interop retain their existing compiler limitations. General numeric
Const declarations also remain outside the lowering subset.

References: [SAFEARRAY layout](https://learn.microsoft.com/en-us/windows/win32/api/oaidl/ns-oaidl-safearray),
[reversed bounds in Automation](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-oaut/2e87a537-9305-41c6-a88b-b79809b3703a),
[column-major data and API ownership](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/automat/array-manipulation-functions),
[fixed bounds](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/7c97ad30-4b76-45d9-9827-7455f98a5503),
[ByRef array arguments](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/array-argument-must-be-byref),
[locked-array error 10](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/this-array-is-fixed-or-temporarily-locked-error-10).
[Erase semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/f7958382-95a7-47fa-91bd-42262ab9ad32).

[ParamArray calls](PARAMARRAY.md) now create call-specific Variant arrays whose
variable elements preserve ByRef bindings. Ordinary array copies still snapshot values.
