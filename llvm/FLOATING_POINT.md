# Single and Double in the native compiler

The existing typed AST/checker remain the source of operator types and conversion
plans. Lowering and LLVM now support IEEE binary32 Single and binary64 Double in
locals, parameters, results, ByVal copies, and ByRef aliases. Type characters `!`
and `#`, decimal/exponent literals (including D exponents), and CSng/CDbl use the
ordinary parser and ambient declaration machinery.

Supported operations include unary minus, addition, subtraction, multiplication,
floating division `/`, relational comparisons, numeric conditions, floating For
loops, Select Case, and floating indices for On GoTo/array operations. Integral
conversions use ties-to-even rounding with checked bounds. Single + Long uses
Double, while Single + Integer remains Single. The checker's existing type
rules choose these effective types; lowering does not infer them from values.

## Precision, errors, and optimization

Single literals are parsed directly at binary32 precision, avoiding decimal to
binary64 to binary32 double rounding. IR constants store exact finite values;
the verifier rejects unrounded Single constants and NaN/infinity literals. LLVM
hexadecimal constants preserve exact bits. Typed arithmetic uses float/double
operations without fast-math, reassociation, contraction, or signed-zero promises.
Single operations round to binary32; subnormals and underflow to signed zero are
permitted by the current target profile.

Arithmetic checks its result before continuing. Overflow raises error 6. For
division, nonzero/zero raises 11 and zero/zero raises 6. Variant division also
preserves VBA's special floating-zero-over-Empty error 11. Comparisons reject NaN
with error 6; finite and infinite values otherwise use IEEE ordered comparison.
Numeric conditions reject nonfinite values, then test against zero.

Floating-to-integral conversion uses llvm.roundeven followed by ordered bounds
checks. The potentially poison-producing fptosi instruction is emitted only on
the success edge. Double-to-Single conversion checks the source magnitude and
rejects overflow; widening Single-to-Double is exact. Normal failing assignments
leave their destination unchanged and preserve earlier calls/writes.

The Double division assignment exception is explicit: when both operands are
declared Double within a Let RHS and the destination is Double storage, division
by zero stores the IEEE infinity/NaN before propagating its error. This includes
ByRef and typed array-cell destinations and division nested in the RHS. Ordinary
arithmetic overflow does not perform that store. OpDivide's optional ref<Double>
operand describes this failure-side effect and carries WritesStorage; the
verifier and emitter preserve it independently of LLVM optimization. This does
not enable On Error/Resume. The special assignment behavior for destinations of
other declared types has not been validated against a VBA/VB6 host.

LLVM can fold finite constants, remove proven-safe checks, promote local storage,
and optimize switches/loops. It must retain observable errors and stores. There
are no custom floating constant-folding passes or fast-math assumptions.

## Variant and arrays

Single and Double use VT_R4 (4) and VT_R8 (5) in the existing 24-byte Win64
Automation Variant layout. Typed ByRef Variant views point at the original four-
or eight-byte slot; successful assignments coerce before writing, and failed
coercions leave it untouched. Snapshots preserve the subtype and value.

Portable dynamic arithmetic retains Single when required, uses Double for mixed
Single/Long arithmetic, and promotes Single overflow to Double. Integer overflow
promotion is unchanged. Single()/Double() use SAFEARRAY with four-/eight-byte
elements and their original VARTYPE, including Variant boxing, copies, Preserve,
ByRef arrays/elements, and For Each. No private float boxes replace ABI values.

Windows uses the existing OleAut32 conversion/arithmetic path; Linux uses the
portable runtime. Text conversions retain its C-locale profile (seven significant
digits for Single formatting, fifteen for Double), rather than full host locale
and formatting behavior. Ambient floating-point environment/legacy x87 excess
precision are not emulated. Windows/VBA/VB6 differential execution is outstanding.

## Try it and limits

```sh
go run ./cmd/vambas-compile -root examples/floating -entry Main.Main -o /tmp/floating.ll
clang-20 -O2 /tmp/floating.ll runtime/*.c -lm -o /tmp/floating
/tmp/floating
echo $? # 42
```

Pure typed floating arithmetic/control flow does not need the C runtime. Explicit
conversion intrinsics and Variant/array operations currently use it. Native entry
wrappers still require a Sub or Boolean/Integer/Long result; a floating Function
can be called from such a wrapper. Public/COM calling conventions remain separate.

The compiler still diagnoses exponentiation, integer division/Mod, unsupported
math intrinsics, non-String Const declarations, Byte/LongLong/Currency/Decimal/Date
execution, globals/Static, objects, and On Error/Resume. Mixed statically
numeric/String relational comparisons retain their existing limitation.

Tests execute original and optimized LLVM, native O0/O2 code, and independent C
ABI checks. They cover precision/promotion, rounding ties and overflow, NaN,
division error stores, parameter aliases, SAFEARRAY element widths and ownership,
floating loops/dispatch, and allocation cleanup.

References: [arithmetic/effective types](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/e070115f-8d40-40cf-ac6d-ab18b9c6c906),
[division and its assignment exception](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/8628f21c-3f02-42b1-908c-201bd7ffe1b5),
[numeric Let-coercion](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/56fb78c3-a0ea-4bf0-8ee4-eb5f89bda4f7),
and [relational operators](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/f8acd631-55c1-4199-bc1e-022aaab6d9c8).
