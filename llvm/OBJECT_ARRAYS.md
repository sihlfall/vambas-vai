# Object and class arrays

Object arrays and arrays of compiled or supported built-in classes execute through
the native backend. This includes fixed/dynamic and multidimensional arrays,
module/global storage, class and UDT fields, typed ByRef parameters, typed returns,
Variant boxing and references, late-bound public fields/methods, `ReDim Preserve`,
`Erase`, bounds queries, and enumeration through Variant controls. Ordinary array
elements start as `Nothing`. `As New` array elements remain explicitly diagnosed;
they are not silently treated as ordinary object elements.

```vb
Dim items() As C, copy() As C, boxed As Variant
ReDim items(0 To 2)
Set items(0) = New C
copy = items
boxed = items
ReDim Preserve items(0 To 4)
```

Array descriptors and element slots are copied independently; object references
are retained, preserving instance identity rather than cloning the objects.
Resizing, shrinking, erasing, and cleanup release references. Ref-count cycles
still require explicit breaking, as for scalar private objects.

IR uses `ArrayObject` and `ElementObject`. `Type.Class` or `Type.Builtin` retains
the nominal array element constraint, with zero identifying `Object()`. Scalar
object values remain erased references with explicit class checks. The verifier
checks array constraints, matching reference/result types, and resize contracts.

The runtime stores private object pointers in a SAFEARRAY-shaped descriptor with
a 24-byte private prefix carrying the class/built-in constraint. This is a private
extension, **not** an array of Automation interface pointers. Private tags are
never sent to SDK object-array management functions; ownership and locks use the
native runtime. `VarType` presents the language value `vbArray + vbObject` (8201).
External COM marshaling remains separate work, and Windows execution is unverified.

Variant copies, typed ByRef Variant views, unallocated arrays, and late-bound calls
retain the nominal constraint. Wrong-class or differently declared whole-array
assignments fail with type mismatch. A real Variant can be retyped by non-Preserve
`ReDim ... As ...`; a view of declared array storage cannot change its element type.

Replacements prepare their storage and retained elements before publishing the
new array. Allocation failure preserves the old array; removed element terminators
observe the published replacement. Fixed-array `Erase` keeps the descriptor and
cell addresses stable, including through active ByRef element aliases, while
publishing `Nothing` in every cell before releasing old references. Borrowed
elements still prevent resizing, dynamic erasure, and whole-array replacement.

Coverage lives in [object_arrays_test.go](object_arrays_test.go),
[object_arrays_runtime_test.go](object_arrays_runtime_test.go), and
[the IR/lowering tests](../lowering/object_arrays_test.go). Native checks cover
both optimization levels, nominal failures, aliases, late-bound invocation,
allocation rollback, and termination during replacement.

Unchanged `../stdvba/src/stdQuadTree.cls` and `stdFiber.cls` with `stdICallable.cls`
now compile, pass LLVM verification, and native-link at `-O0` and `-O2`. This is
compilation evidence, not a claim that all stdVBA APIs execute: external services,
library functions, and interface implementations may have further dependencies.
The [constructor smoke probe](audits/stdvba/object-arrays-probe.bas) also runs at
both optimization levels. A query probe reaches the still-missing `IsObject`
predicate in stdQuadTree's `CopyVariant` helper; query/insertion support is not
claimed. The [updated audit](audits/stdvba/after-object-arrays/README.md) keeps
compilation/linking results separate from these runtime checks.
