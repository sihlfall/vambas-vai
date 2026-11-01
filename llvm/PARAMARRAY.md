# ParamArray calls

Native Function/Sub calls now support a final `ParamArray values() As Variant`
(or the implicit-Variant spelling). The parser/binder/checker still own syntax,
names, types, argument mapping, and diagnostics. Lowering consumes their existing
`CallResolution.ParamArray` and `ArgumentBinding.ParamArrayIndex` plans without
rewriting syntax or reinterpreting argument spellings.

This follows VBA's parameter bindings rather than .NET parameter-array copying.
MS-VBAL defines ParamArray elements as ByRef Variant positional parameters;
see [parameter lists](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/78bcb344-4966-4401-bb55-72729790ebee)
and [invocation argument processing](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/1fb9af32-fc48-4c4f-998a-ed8047048ca5).

```vb
Sub Change(ParamArray values() As Variant)
    values(0) = values(0) + 1
End Sub

Sub Example()
    Dim count As Long
    count = 10
    Change count          ' count becomes 11
    Call Change((count))  ' parenthesized value is a temporary; count stays 11
End Sub
```

## Language behavior

- Each invocation gets a one-dimensional Variant array with lower bound zero,
  regardless of Option Base. With no extra arguments its upper bound is -1.
- Addressable variable arguments preserve aliases. Writes update caller storage
  immediately, including before an unhandled error; they are not deferred copyback.
- A typed variable retains its declared assignment/coercion rules. For example,
  assigning 32768 through an Integer binding raises overflow and leaves it intact.
- Literals, expressions, function results, and forced parenthesized values supply
  independent temporary values. Assigning those elements does not change callers.
- Explicitly omitted elements preserve the existing Missing sentinel and can be
  queried with IsMissing. This differs from passing no elements.
- Arguments evaluate in source order. A later argument can change a variable that
  an earlier ByRef element refers to; earlier value snapshots remain independent.
- Duplicate variable arguments share storage. Forwarding an individual element to
  another ParamArray or a normal ByRef Variant formal preserves that binding.
- An array argument is one element, not a spread argument. Typed array references
  retain their subtype; parenthesized arrays are independent snapshots.
- LBound/UBound, indexing, For Each, passing the whole parameter array to a matching
  ByRef Variant-array formal, and ordinary value copies use existing array paths.
  Ordinary copies dereference bindings, so returned/copied arrays own their values.
- Frontend restrictions apply: a final Variant-array formal, no
  ByVal/ByRef/Optional modifier, no Optional parameters in the same list, no named
  calls, and no ReDim or Erase of the ParamArray itself.

## Semantic IR, ABI, and lifetime

`Slot.ParamArray` distinguishes this binding from a source-declared array formal.
Its private ABI uses `ByRef=true` and `ArrayVariant` storage: the callee borrows the
caller's call-specific descriptor slot rather than making a normal ByVal array
copy. The immutable AST/checker parameter's IsByRef remains unchanged.

`OpParamArrayPack` takes a `ref<ArrayVariant>` destination followed by Variant
values or references. It declares MayRaise, WritesStorage, and ManagesLifetime,
and uses the existing Result/Try continuation. Its temporary descriptor slot is
released immediately after a successful call, before later outer arguments run.
Error propagation, handled errors, and Resume/retry use statement-boundary cleanup.
ByRef array-element arguments retain their descriptor locks through evaluation of
later arguments and the call, then release them as the call returns.

LLVM passes `{Variant*, byref}` records to `vba_paramarray_pack` in the existing C
runtime. A value argument is copied with normal managed ownership; a reference is
represented with genuine Automation VT_BYREF wrappers. Typed String references
use the existing pinned BSTR slot. Forwarded Variant bindings are flattened so
VT_BYREF|VT_VARIANT never points to another such wrapper. The VARIANT/SAFEARRAY
layouts are unchanged; no private handles are disguised as Automation values.
Owned Variant-array copies use the element-copy path on Windows too, because
[VariantCopy](https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-variantcopy),
as used by SafeArrayCopy, preserves VT_BYREF wrappers. Native execution was validated
on Linux; the Windows SDK layout checks remain in place, but Windows execution
was not available in this environment.

Packing commits its destination only after all allocations and value copies
succeed. Failed packing never enters the callee, releases partial allocations,
and preserves side effects already performed by argument evaluation. Resume
retries the whole call statement and reevaluates its arguments.

The existing compiler subtype limits still apply: Byte/Date/Currency/Decimal,
COM/object Variants, UDT/UDT-array Variant boxing, property execution, and external
variadic calls are not added by this feature. Private virtual assertion objects
cannot be passed as Automation Variant elements.

## Validation

```sh
go test ./lowering ./ir ./checker
go test ./llvm -run 'TestNativeParamArray|TestParamArrayRuntime'
python3 llvm/audits/vba-regex/run-tests.py
```

Native tests assemble and verify IR, execute original and optimized bitcode, and
run Clang O0/O2 binaries with allocation tracking. They cover aliases, typed
coercions, String ownership, empty/omitted arguments, nested arrays, source order,
recursion, forwarded bindings, element locks, errors, allocation failure, and
Resume. The independent C harness injects failures into descriptor/data/value-copy
allocations and checks rollback and zero remaining owners.

The original regex audit now executes the complete unchanged TestVbaRegex.vbp:
**203 tests across eight test modules at O0/O2**, including MakeArray helpers
using ParamArray. [Collection](COLLECTION.md) and [GoSub/Return](GOSUB.md) now
compile natively too.
