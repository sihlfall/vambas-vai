# Scalar LLVM backend

[Ordinary class modules](CLASSES.md) now support instance fields, implicit
receivers, construction, direct/dynamic methods and properties, lifecycle hooks,
lazy/predeclared instances, object-valued Variants, identity, and managed cleanup.

[Collection](COLLECTION.md) now supports construction, keyed/positional items,
insertion/removal, private object items, captured With, and For Each.

Err and On Error/Resume now execute through the existing status ABI; see the
[error-handling contract](ERROR_HANDLING.md). UDT arrays and captured With
receivers follow the [record lifecycle contract](UDTS_ENUMS.md).

Module globals, numeric Const, typed Optional/default arguments, and Boolean/integer
assignments now execute; see [project storage and defaults](PROJECT_STORAGE.md).

Executable hex/octal literals, integral `\`/`Mod`, and additional string library
functions now lower to checked operations; see the [built-in contract](BUILTINS.md).

Single and Double now use native IEEE arithmetic and checked conversions; see
the [floating-point semantics, ABI, and example](FLOATING_POINT.md).
UDTs now have nominal fields, managed value copies, returns, and ByRef aliases;
enums use Long storage. See the [UDT/enum contract and example](UDTS_ENUMS.md).

Variable-length Strings now add an ownership-bearing value type. See the
[String runtime, mutable storage, and optimization contract](STRINGS.md).
Variant values use the Automation memory layout and typed ByRef views; see the
[Variant runtime, linking instructions, and supported subset](VARIANT.md).
Arrays use SAFEARRAY storage with whole-array and element ByRef aliases; see the
[array runtime and example](ARRAYS.md). Byte scalars/arrays and direct String ↔ Byte()
assignment follow the [byte conversion rules](ARRAYS.md#string-and-byte-assignment).
`CByte`, `CVErr`, `IsError`, and explicit Error coercions follow the
[Stage D data-type contract](DATA_TYPES.md).
That contract also covers typed Date/Currency storage and calls, `CDate`/`CVDate`,
`CCur`, and exact Decimal Variant conversion/arithmetic through `CDec`.
If, Select Case, For/For Each, While, Do, labels/GoTo, computed On GoTo, and loop exits have executable
regions; see the [control-flow contract and example](CONTROL_FLOW.md).

`llvm.Emit` consumes verified semantic IR and emits a textual LLVM module. The
compiler now has an executable scalar path:

```text
project snapshot -> parser/binder/checker -> typed semantic IR + verifier
                 -> local success/error blocks -> textual LLVM -> native code
```

The shared Program, AST, binder, and checker retain the `typescript-go`-style
separation used by the editor. The backend consumes their resolved semantics
through `ir`; it does not re-resolve names or traverse binder FlowNodes. VBA's
checked arithmetic, storage aliases, and error alternatives require the new layer.

## Try it

With Go and LLVM 20 installed, run these commands from the repository root:

```sh
go run ./cmd/vambas-compile -root ./examples/scalar -entry Main.Main -o /tmp/vba-scalar.ll
opt-20 -passes=verify -disable-output /tmp/vba-scalar.ll
clang-20 -O2 /tmp/vba-scalar.ll -o /tmp/vba-scalar
/tmp/vba-scalar
echo $? # 42
```

The [example](../examples/scalar/Main.bas) passes a local Long ByRef to Bump and
returns 42. The native entry wrapper uses a Function's result as the process exit
status. A Sub returns zero; an unhandled error returns its VBA error number. Shells
may expose only the low eight bits of an exit status. This wrapper is a development
entry convention, not VBA's host startup semantics or a COM adapter. A Function
result equal to an error number has the same process exit status; the internal ABI
keeps success values and errors distinct.

For an existing project, select its manifest explicitly:

```sh
go run ./cmd/vambas-compile -project /path/to/Demo.vbp -entry Main.Run -o /tmp/demo.ll
```

Without `-project`, `-root` selects the sole manifest or discovers `.bas/.cls/.frm`
files, just like the workspace loader. The default root is the current directory.
`-source-encoding auto` uses the same Unicode/BOM and Windows-1252 import rules as
the language server; `utf8` and `windows1252` can be selected explicitly. The
compiler rejects unsupported modules and procedures throughout the project,
including unused code. Existing projects using unsupported array element types,
class features or other unsupported runtime forms still produce unsupported-subset
diagnostics.

`-o -` (the default) writes LLVM to stdout. Diagnostics go to stderr and do not
produce partial LLVM or overwrite an existing output on compilation failure.
Output cannot overwrite a loaded source or project manifest. `-entry` is optional;
omitting it emits private procedures for inspection or an LLVM-level test harness,
without a native main or library ABI. An entry must use a qualified VBA name and
have no parameters and return Boolean/Integer/Long (or be a Sub). Floating
Functions can be called from that entry wrapper. Native linking and execution
remain explicit LLVM commands.

## Direct SSA output

The Go backend constructs scalar SSA values and phi nodes itself:

```sh
go run ./cmd/vambas-compile -root examples/control-flow -entry Main.Main -o /tmp/control.ll
opt-20 -passes=verify -disable-output /tmp/control.ll
```

Emission requires only Go. The optional `opt` command above verifies the already
constructed SSA; the compiler invokes no LLVM tool or promotion pass. Output
preserves source comments, is deterministic, and leaves the input IR unchanged.
There is no separate storage-only emission mode, so a single path is maintained
and tested.

Boolean, Byte, Integer, Long, Single, Double, Date, and Currency locals, ByVal
parameters, and function-result variables use their native scalar types when their
addresses are not used. Reads and writes become SSA definitions, with phi nodes at
control-flow joins. Checked scalar operations produce values directly without
unused output slots. ByRef aliases, globals, used addresses, managed storage, and
actual call output buffers retain memory storage. In particular, a Double division
destination with an observable error-side write remains addressable.

Scalar arithmetic and phis do not introduce Variant boxing. Conversion occurs at
the existing semantic/runtime boundaries that require it, such as assigning a
scalar result to Variant storage or supplying a Variant-snapshot library argument.
This does not specialize dynamically tagged Variant variables or change their
overflow, subtype, coercion, ownership, or callback behavior.

SSA uses the existing frontend's resolved targets and typed semantic lowering.
The backend records the complete executable graph, including checked-operation
errors, scope cleanup, On Error/Resume, and GoSub indirect-return destinations.
Reads reserve block-entry phis; once all edges exist, their incoming definitions
are filled and trivial phis removed. Binder FlowNodes remain source-level analysis
metadata and are not treated as the complete executable graph. Duplicate switch
edges, forward/backward jumps, and unreachable cycles are supported.

Run the direct-SSA structural and execution tests with:

```sh
VBA_LLVM_TEST_REQUIRED=1 go test -timeout 20m ./llvm ./cmd/vambas-compile -run TestSSA
```

Every native fixture also runs through this emission, verifying and executing it
at the interpreter, optimized, and native O0/O2 levels. The CLI suite also checks
emission with an empty PATH.

## API and representation

```go
semantic, diagnostics := lowering.Lower(program)
if len(diagnostics) != 0 {
    // Report source diagnostics.
    return
}
text, diagnostics := llvm.Emit(semantic, llvm.Options{Entry: 1})
if len(diagnostics) != 0 {
    // Report IR/backend diagnostics before writing any output.
    return
}
// Write text to a .ll file, then invoke LLVM tools as needed.
```

Import `lowering` and `llvm` from `sihlfall.ch/vambas-vai/`. Entry IDs index the
semantic program's procedure list; `llvm.Options{}` emits no main. Emission is
deterministic, does not mutate the semantic program, performs no I/O, and needs no
Go LLVM bindings or development headers. It always runs `ir.Verify` before
emission. LLVM 20 is the initial tested toolchain; generated modules use opaque
pointers and leave target triple/data layout selection to the LLVM tools. Native
execution has been validated on Linux x86-64; other targets are not yet validated.
Variant/array code currently requires 64-bit pointers and links the sources listed in `runtime/sources.txt` (or `runtime/*.c`);
String-only modules remain self-contained.

Boolean and Integer use `i16`; Long uses `i32`; Single/Double use `float`/`double`.
Floating operations retain checked errors and use no fast-math flags.
Boolean comparisons sign-extend an
`i1` condition so VBA True remains -1. ByRef parameters receive storage pointers;
ByVal parameters receive values copied into local slots. Locals, temporaries, and
function-result slots are allocated in the entry block and initialized according
to the semantic IR. No `noalias` promise is attached to VBA ByRef arguments.

Checked add/subtract/multiply and negation use LLVM's signed overflow intrinsics.
They branch on the overflow bit rather than assuming overflow cannot occur.
Narrowing from Long to Integer checks the signed truncate/sign-extend round trip;
widening sign-extends. These operations raise [Overflow, error 6](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/overflow-error-6)
when the semantic result is out of range. See the [LLVM 20 language reference](https://releases.llvm.org/20.1.0/docs/LangRef.html#arithmetic-with-overflow-intrinsics)
for the intrinsic contract.

Each semantic statement starts a named LLVM block. A try splits that sequence
into local success and error blocks. A checked operation extracts its value only
on success; a call's output slot is loaded only on success. Assignment stores
remain after those checks. Failure returns immediately, preserving earlier stores
and calls while skipping pending work. There is no runtime error dispatcher.
The block representation is private to this backend. Source loops and branches
use verified region terminators and resource-scope edge cleanup.

## Initial private ABI

Every compiled procedure has internal LLVM linkage under a generated symbol
`@vba.p<ID>`. VBA visibility and names are retained in semantic IR and comments;
they do not create native/COM exports. The initial calling convention is:

```text
i32 @vba.pN(ptr error, [ptr output for Function], scalar-or-pointer parameters...)
```

Status zero means success. A Function writes its language result to `output` only
on success. A failed procedure returns its error number and leaves that output
untouched. Direct calls allocate activation-local output slots and pass the error
pointer onward. Errors propagate without replacing the originating operation's
provenance. Parameters are supplied in checker parameter order after semantic IR
has evaluated their expressions in source order.

The current caller-owned `%vba.error` record has four `i32` fields: error number,
originating procedure ID, originating statement ID, and the operation's UTF-8 byte
offset. Statement comments retain filenames and ranges. This record is a private
prototype: it represents runtime error numbers and their origins, not the full `VbaError`
record, observable Err object, descriptions/help fields, or a foreign ABI. Success
does not read or clear it. The error record needs no heap allocation or global state.
Later runtime/error features must extend this convention before exposing them.

Even infallible procedures initially share this private signature. LLVM can inline
and remove unused status, output slots, and error branches; specialization before
LLVM remains future work. Strings, Variants, and arrays have ownership cleanup;
numeric values do not.

## Subset and validation

The backend covers the [step (1) scalar subset](../ir/README.md), including checked
arithmetic/conversions, Boolean/integral logic and comparisons, function result
storage, ByVal/ByRef, aliasing, forced value temporaries, direct/named/implicit/
explicit calls, forward references, procedure exits, and the supported
[control-flow constructs](CONTROL_FLOW.md), [GoSub/Return](GOSUB.md), and
[error handling](ERROR_HANDLING.md). Unsupported array/Variant forms and fixed-length/other String operations,
deferred class features and COM, and foreign calls
remain unsupported. Ambient/external effects are rejected at the backend boundary.

[Mid/Mid$ and Mid assignment](MID.md) now provide substring reads and content writes
for the typed String subset, including ownership-aware mutation and error paths.

Run the execution suite with LLVM tools required:

```sh
VBA_LLVM_TEST_REQUIRED=1 go test -timeout 20m ./llvm ./cmd/vambas-compile
```

Tests assemble and verify modules with `llvm-as`/`opt`, run both original and
optimized bitcode with `lli`, and compile/run native executables with Clang at
`-O0` and `-O2`. They check scalar boundaries, output commit points, alias identity,
parenthesized temporaries, argument order, default/early returns, and original
error provenance across calls. Tool lookup prefers suffixed LLVM 20 names, then
unversioned tools. Without the environment variable, execution tests skip when
tools are missing; pure Go emission/CLI tests still run. These are not yet
differential conformance tests against a VBA/VB6 host.

[Native test execution](../testrunner/README.md) adds direct procedure registration,
virtual assertion-library handles, isolated module workers, and JSON/JUnit reports.
Use `go run ./cmd/vambas-test -root examples/testing` for the runnable example.
