# Variable-length String support

The compiler supports an initial Windows-style Unicode String subset through
semantic IR and native LLVM execution. String variables are mutable. Assignment
and ByVal passing preserve independent values; ByRef aliases the variable's
storage and can replace it. Buffer sharing is a private representation choice.

```sh
go run ./cmd/vambas-compile -root examples/strings -entry Main.Main -o /tmp/strings.ll
clang-20 -O2 /tmp/strings.ll -o /tmp/strings
/tmp/strings
echo $? # 11: Len("Hello, VBA!")
```

The [example](../examples/strings/Main.bas) uses a String Const expression, a String
return value, a ByVal parameter, and ByRef mutation. Runtime helpers are embedded
in the LLVM module. The String runtime assumes a 64-bit C allocation ABI and is
tested on Linux x86-64 with LLVM 20. A 32-bit C ABI is unsupported for strings.

## Supported operations

- UTF-16 literals, including doubled quotes, embedded NULs, and surrogate pairs.
  IR and runtime character construction can also represent individual surrogates.
- Variable-length String locals, parameters, result variables, and `$` type hints.
- Assignment/self-assignment, ByVal copies, ByRef aliases, parenthesized ByRef
  temporaries, and String returns, including discarded results.
- String/String concatenation using `&` or `+`.
- Checked String Const expressions with literals, constant references, parentheses,
  and concatenation. Evaluation has a bounded expansion budget; unsupported forms
  receive diagnostics.
- `Len`/`LenB` on typed Strings, returning Long. Length counts UTF-16 units; LenB
  counts bytes and checks Long overflow.
- `ChrW`/`ChrW$`, taking Long in -32768..65535 and producing one UTF-16 unit.
  Out-of-range input raises error 5. Negative values map to high unsigned units.
- `vbNullString` and `vbNullChar`, also qualified with `VBA.`. Null String state is
  retained separately from empty String; both have length zero.
- Mid/Mid$ reads and assignment, with omitted/named function arguments, checked
  bounds, clipped replacement, and shared-buffer detachment; see [Mid support](MID.md).
- Variant boxing/unboxing, explicit `CStr`/numeric conversions, and immediate
  mutation through typed String/ByRef Variant aliases; see [Variant support](VARIANT.md).
- String arrays store BSTR cells. ByRef String/Variant element aliases share the
  same cell; value reads preserve snapshots. See the [array contract](ARRAYS.md).
- Direct `String` ↔ `Byte()` assignment copies UTF-16LE bytes; see the
  [array conversion rules](ARRAYS.md#string-and-byte-assignment).
- Binary String comparisons and control conditions; see [control flow](CONTROL_FLOW.md).

Intrinsics use fresh ordinary ambient declarations with explicit symbol identity;
user declarations named Len or ChrW keep their own behavior. Len/LenB retain
Variant library signatures, refined to Long for a known String argument.

Fixed-length `String * n`, other content-writing forms, text/locale comparisons,
general implicit numeric/String conversions,
byte-string operations, other String functions, StrPtr, Declare/COM calls, and
public String ABIs are diagnosed as unsupported. Fixed-length strings are rejected
even though the checker queries their type as String. On Error/Resume remains
unsupported; the current policy propagates errors.

## Mutable storage and ownership

An IR String is a value independent of allocation. Loading mutable String storage
acquires an owned snapshot. In `Capture(s, Replace(s))`, the first argument keeps
its original contents when evaluating the second replaces s. ByRef arguments
instead carry the slot address, so two aliases see successive assignments.

Every non-ByRef String slot owns its value. A store acquires its new value before
releasing the old one. ByVal parameters own local copies; ByRef parameters borrow
caller storage. Successful String returns transfer result-slot ownership to the
caller; failed calls leave output untouched.

Instructions mark String values `Immortal` or `Owned`. A region's `Cleanup` lists
all owned values in acquisition order, released in reverse. `ManagesLifetime`
records ownership work separately from language storage writes. `ir.Verify`
checks ownership, cleanup completeness/order, and type/alias/Result/effect rules.

LLVM owner slots are initialized before fallible work. Successful statements
release snapshots and argument temporaries. Propagating an error releases live
activation-owned values and slots; unexecuted owners remain null. Caller-owned
ByRef variables keep completed mutations; pending assignments never commit.
Cleanup preserves the original error record. Future resuming error policies must
select region cleanup while keeping surviving procedure locals alive.

## Private representation and optimization

`%vba.string = {ptr data, ptr owner, i32 length, i16 unit}`. Length -1 means null
String; zero means empty. In mutable storage only, -2 means the data field owns
a real BSTR exposed through a ByRef Variant view. A borrowed array-element proxy
uses -3 and its data field points at the BSTR slot. Loads snapshot that BSTR into
an ordinary private value; see the [boundary contract](VARIANT.md#byref-rules).
One unit lives inline with no buffer allocation. Longer
literals use constant UTF-16 arrays; runtime concatenations can own buffers with
a reference count, UTF-16 units, and terminating zero. Operations use length,
preserving embedded NULs. Runtime allocation/free uses the platform C library.

**Content writes detach shared or literal buffers before modifying them.**
Mid assignment now implements this contract, reusing unshared buffers. Other
content-writing forms remain rejected. Future StrPtr and foreign boundaries
must materialize appropriate mutable storage and enforce ownership/alias rules.
This descriptor is not a BSTR and must never cross an Automation boundary as one;
Variant boxing and typed ByRef views now materialize actual BSTRs. Public interop
adapters remain future work.

The general concat helper has empty, single-unit, shared-value, and allocation
paths visible to LLVM. Tests show `Len(ChrW(65) & "")` and `Len("ab" & "cd")`
optimize to constants with no allocation/free calls. Source Const evaluation is a
language requirement performed before emission. There is no general custom
String optimization pass.

Actual concatenation allocation/size failures raise error 14, preserving origin
procedure/statement/offset. Remaining allocations stay fallible. Legal allocation
elision may remove hypothetical resource-exhaustion failures under the optimization
contract; value-dependent ChrW and arithmetic errors remain required.

## Validation

`VBA_LLVM_TEST_REQUIRED=1 go test ./llvm ./lowering` runs assembly/verification,
original and optimized `lli`, and native Clang O0/O2 execution. Tests inspect UTF-16
contents, mutable aliases, independent copies, snapshots, returns, and error commit
points. Allocation wrappers check live buffers return to zero; fault injection
fails a later allocation after earlier successes. Optimized IR checks assert
allocation machinery disappears for the constant/character examples.

Differential VBA/VB6 host testing, full Err state, other content writes, fixed strings,
locale/pointer operations, and Automation ABI validation remain outstanding.

References: [String](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/string-data-type),
[Len/LenB](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/a4d9985a-e2cb-489c-a20a-5da09d77a59e),
[ChrW](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/chr-function),
[constants](https://github.com/MicrosoftDocs/VBA-Docs/blob/main/Language/Reference/User-Interface-Help/miscellaneous-constants.md),
[error 14](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/out-of-string-space-error-14).
