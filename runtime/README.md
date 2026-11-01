# Native runtime

The runtime consists of separately compiled C11 translation units. The explicit
[sources.txt](sources.txt) manifest is shared by the Go test runner, LLVM tests,
and the executable regex audit. Add new runtime source files to this manifest.

For a generated LLVM module, build all runtime sources together:

```sh
clang-20 -std=c11 -O2 /tmp/program.ll runtime/*.c -lm -o /tmp/program
```

On Windows, link OleAut32 (`-loleaut32`) instead of `-lm`. The native backend
currently requires a 64-bit target.

The public `.h` files describe the existing generated-code ABI. Implementation
files include [runtime_internal.h](runtime_internal.h) for shared coercion,
ownership, numeric, array, and error-state helpers. Those helpers use the
`vba_internal_` prefix and hidden visibility where supported; they are not part of
the generated-code ABI. Helpers used by only one implementation file remain
`static`. Numeric working types, current Err state, invocation context, and test
worker context remain owned by their implementation files.

`variant.c` implements Variant and String primitives; it no longer includes other
implementation files. The feature files implement arrays, records, numeric types,
conversions, built-ins, control flow, errors, objects, collections, invocation, and
the test worker. There are no implementation `.inc` files or include-order
dependencies between C files.

Generated scalar array access uses the narrow `vba_array_lock` and
`vba_array_unlock` entry points in [arrays.h](arrays.h). They delegate to the
same platform lock primitives as general element access; generated code retains
the original descriptor for cleanup.

Project-local interface dispatch uses private descriptors in [objects.h](objects.h).
See [Implements](../llvm/IMPLEMENTS.md) for descriptor layouts, exact field-layout
checks, interface casts, and native call signatures.

The C ABI tests compile every source independently with strict warnings, then
link and execute at `-O0` and `-O2`. Allocation-failure probes inject allocator
callbacks into each runtime translation unit through a forced-include test header;
the callbacks themselves use the real allocator. LLVM tests separately compile
each source to LLVM IR and link the modules before applying allocation tracking
and running verification, interpreted execution, and native optimization checks.
