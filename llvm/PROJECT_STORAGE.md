# Project storage, constants, and optional parameters

The compiler now executes module globals, numeric Const declarations, typed
Optional parameters/defaults, and Boolean/integer assignments. The shared AST,
binder symbols, immutable checker, and checked call plans retain the existing
typescript-go-style separation. Project lifetime and VBA's ByRef defaults are
represented explicitly where they differ from JavaScript/TypeScript.

## Project lifetime

`ir.Program.Globals` contains one owning `Global` per module variable, in source
order, with an identity, qualified name, type, source, and optional fixed bounds.
`GlobalAlias` slots refer to that storage through `Slot.Global`. Existing load,
store, address, array, and record operations use these aliases. Aliases never
allocate, initialize, or release the global as part of a procedure activation.
The verifier checks alias identities/types, source provenance, bounds, and effects.

Globals support the existing executable value types: Boolean, Integer, Long,
Single, Double, String, Variant, scalar arrays, and UDTs. Public/Private visibility
and qualification continue to use the binder/checker. Access through calls,
recursive calls, ByRef parameters, qualified module names, and stable With
receivers reaches the same storage. Local declarations can shadow globals.

LLVM emits internal globals initialized to the language's zero/empty values.
Fixed arrays and record fields requiring initialization are prepared once before
the first procedure body, including globals that the entry procedure never reads.
The generated `@vba.project.ensure(ptr error)` is checked at activation entry.
Failure clears partially initialized storage, leaves the procedure output
untouched, and allows a later call to retry initialization. Initialization errors
use procedure/statement zero and the global declaration's byte offset in the
private error record; ordinary procedure errors retain their existing provenance.

Completed global writes survive procedure errors. Activation cleanup releases
local owners and argument temporaries; it leaves project owners intact.
`@vba.project.clear()` releases project owners in reverse declaration order,
resets scalar values, and resets initialization state. The native main wrapper
calls it after the entry returns, on both success and failure. An LLVM embedding
harness must call it when ending/resetting a quiescent project; repeated calls
are safe. There is still no public native/COM ABI.

The current execution profile is single threaded: first-entry initialization and
reset are not synchronization primitives. Concurrent project activation, active
borrowers during reset, class-module instance storage, procedure Static storage,
and explicit variable initializers remain unsupported. Scalar-array and UDT
ownership retain the limits in [ARRAYS.md](ARRAYS.md) and [UDTS_ENUMS.md](UDTS_ENUMS.md).

## Numeric constants

Module/local Boolean, Integer, Long, Single, and Double Const declarations reuse
the checker's numeric evaluator, also used for enums and array bounds. Supported
constant expressions include checked arithmetic, integral division/remainder,
logical operators, comparisons, exponentiation, parentheses, enum members, and
other constants, including qualified cross-module references. Declared numeric
coercion uses ties-to-even rounding and range checks. Hex/octal signed widths and
Single literal rounding agree with executable literals. Cycles, nonconstant
expressions, and overflow are diagnosed even in unused declarations.

Constant values and optional numeric default expressions are prepared during
checking. `GetNumericConstantValue` and `GetNumericConstantExpression` only read
completed maps, so lowering and concurrent language-server queries never start
new evaluation. Numeric constants lower to typed literals; they have no runtime
storage. Select Case can use integral constants in its existing Switch path;
LLVM still chooses machine dispatch and arithmetic optimization.

String Const support remains available. Byte/Currency/Date/LongLong execution
and explicitly Variant-typed Const declarations retain their existing subset
restrictions; this change does not add those value types or a second evaluator.

## Optional parameters and Boolean conversions

Supported scalar/String parameters can be Optional, ByVal or ByRef. Defaults can
be supported literal/constant expressions. Typed parameters without an explicit
default receive zero, False, or an empty String. Optional Variant without a
default retains the Missing sentinel; an explicit Variant default is a supplied
value, including Empty or Null. Array/UDT optional parameters and general objects
remain unsupported. [ParamArray](PARAMARRAY.md) now has separate native support.
Unsupported or out-of-range defaults are rejected when declaring the procedure,
including unused procedures.

The checker retains explicit argument evaluation order and formal parameter
indices. Lowering fills omitted/named/positional arguments from that plan.
Constant defaults receive the caller's source provenance. Every omitted ByRef
parameter gets a fresh writable temporary of its declared type, with normal
managed cleanup on success and failure. A callee's mutation cannot change the
next call's default. Explicit ByRef arguments still require matching types;
parenthesized expressions use converted temporaries and do not copy back.

Boolean assignments to Integer/Long preserve False = 0 and True = -1. Assigning
Integer/Long to Boolean tests nonzero and normalizes the result to 0/-1. These
conversions are infallible and use ordinary LLVM comparisons/sign extension.
They do not relax ByRef type matching.

## Try it

The [two-module example](../examples/project/Main.bas) uses a persistent counter,
a fixed global array, a numeric constant, typed defaults, a writable ByRef String
default, and Boolean-to-Long conversion:

```sh
go run ./cmd/vambas-compile -root examples/project -entry Main.Run -o /tmp/vba-project.ll
clang-20 -O2 /tmp/vba-project.ll runtime/*.c -lm -o /tmp/vba-project
/tmp/vba-project
echo $? # 42
```

Tests execute original/optimized LLVM and native O0/O2 code, track owners across
multiple calls and project reset, inject failures through partial global setup,
verify fresh ByRef defaults and error-side writes, and race-check constant queries.
