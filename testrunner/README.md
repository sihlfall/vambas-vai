# Native Rubberduck-style tests

`vambas-test` discovers annotation metadata and runs supported VBA tests as native
code, outside Excel and the language server. This is an independently authored
compatibility implementation. No Rubberduck implementation or test code is
vendored or copied. Its inputs were this repository's
[test design](../VBA_LLVM_Test_Design.md), the
[documented unit-testing API](https://github.com/rubberduck-vba/Rubberduck/wiki/Unit-Testing),
and the user's `vba-regex` tests, exercised in their original checkout.

Build and try it from the checkout (Go and Clang/LLVM 20 required):

```sh
go build -o bin/vambas-test ./cmd/vambas-test
bin/vambas-test -root examples/testing -list
bin/vambas-test -root examples/testing
bin/vambas-test -root examples/testing -opt O2 -json results.json -junit results.xml
bin/vambas-test -root examples/testing -filter 'ArithmeticTests.*' -category Arrays
```

Use `-project path/to/Project.vbp` for an existing VB project, and
`-source-encoding auto|utf8|windows1252` for exports. `-clang` selects the compiler;
`-runtime` selects the directory containing `sources.txt` and the runtime C sources/headers.
The default runtime location refers to this build's source checkout; a distributed
binary needs those runtime sources and an explicit path. No Rubberduck COM server
or Office installation is required. Native execution is currently validated on
64-bit Linux; Windows native test linking/packaging remains unvalidated.

`-list` prints a JSON manifest without native tools or VBA execution. It validates
test declarations, but allows unrelated body diagnostics so an editor can discover
tests before the project becomes executable. Execution compiles and checks **all**
loaded procedures, including unselected and ignored ones. A filter selects execution;
it does not remove unsupported source from compilation. Empty selections are an
error. Reports never overwrite a loaded source or project manifest.

Exit codes are 0 for passing/ignored tests, 1 for test/fixture/process failures
(including inconclusive results), and 2 for discovery, compilation, or invocation
errors. `-json -` writes machine-readable results to stdout and human output to
stderr. A compile failure still produces requested JSON/JUnit reports. JSON keeps
assertion failure, runtime error, blocked, skipped, inconclusive, crashed, timed_out,
and cancelled results distinct. JUnit uses failure/error/skipped elements.

## Discovery and lifecycle

The scanner retains actual apostrophe/`Rem` comments with byte ranges. The parser
recognizes `@Name` metadata and quoted arguments, including doubled quotes; a
string containing `'@TestMethod` cannot create an annotation. The AST keeps unknown
metadata too. Standalone leading comment blocks attach to existing declarations;
inline comments and comments inside procedure bodies do not annotate procedures.
Module metadata attaches to the file. Repeated roles on the same declaration use
the closest annotation, allowing commented-out old bodies in a leading comment
block. Different roles on one procedure, duplicate fixture procedures, malformed
arguments, non-Sub tests, and parameterized tests produce discovery diagnostics.

The adapter recognizes `TestModule`, `TestMethod` with an optional category,
`ModuleInitialize`, `ModuleCleanup`, `TestInitialize`, `TestCleanup`, and `IgnoreTest`.
Categories default to `Uncategorized`. Public and private parameterless Subs in
standard modules are supported; declaration identity determines the entry point.
`Folder`, descriptions, and other annotations are retained metadata without
execution behavior. `IgnoreTest` currently takes no arguments.

Each selected module runs in a fresh worker process. Tests in that module execute
sequentially in source order and share project/module globals:

1. Run `ModuleInitialize` once.
2. For each nonignored test, run `TestInitialize`, the test, then `TestCleanup`.
3. Run `ModuleCleanup` once.

Failed setup skips the body. Test cleanup runs after ordinary setup/body assertion
failures and unhandled VBA errors. Failed module setup blocks its tests, while
module cleanup is still attempted. Module cleanup issues are separate suite issues.
A crash or `-timeout 30s` terminates that module's worker; remaining modules still
run. A killed process cannot execute VBA cleanup. Completed results are preserved,
and unfinished tests get a process outcome. Err is cleared at test boundaries;
other globals survive until the worker exits.

## Virtual assertion library

Ambient library declarations support:

```vb
Private Assert As New Rubberduck.AssertClass
Private Loose As Rubberduck.PermissiveAssertClass
Private Late As Object

' Inside ordinary procedures:
Set Loose = New Rubberduck.PermissiveAssertClass
Set Late = CreateObject("Rubberduck.AssertClass")
```

`As New` constructs lazily, including after `Set ... = Nothing`. Handles are
reference counted, with independent owned snapshots and ordinary ByVal/ByRef
aliasing. Typed assignments check the virtual class at runtime when needed.
They are private runtime references, **not COM interface pointers**; they are never
boxed as fake Automation `VT_DISPATCH`/`VT_UNKNOWN` values. General classes,
Object arrays, Object-containing UDTs, object Variants, and remote `CreateObject`
are outside this subset. Unknown ProgIDs raise error 429.

Implemented assertion methods are `AreEqual`, `AreNotEqual`, `AreSame`,
`AreNotSame`, `IsTrue`, `IsFalse`, `IsNothing`, `IsNotNothing`, `SequenceEquals`,
`NotSequenceEquals`, `Fail`, `Inconclusive`, and `Succeed`. Optional messages,
explicit/implicit calls, and named arguments are supported. Arguments evaluate in
source order. Late-bound calls validate the receiver, arity, names, and value kinds
at runtime; unknown methods remain compilation diagnostics.

Assertion failures record test results and return normally. They never set Err or
raise a VBA error, and therefore cannot be swallowed by `On Error`. Multiple
failures can be recorded in one test. Invalid calls/coercions and calls on Nothing
still use ordinary VBA error handling. Assertions outside an active test context
raise error 5 rather than silently passing.

The current comparison profile is explicit:

- Strict equality checks Variant subtype and value. Integer and Long differ;
  strings compare case-sensitively by UTF-16 units. Null equals Null; Empty equals
  Empty; unequal subtypes fail equality.
- Permissive equality applies the existing VBA scalar coercion services, including
  numeric strings versus numeric values. It is not a locale-complete Automation
  comparison implementation.
- Object references use `AreSame`/`AreNotSame` and Nothing predicates. Late-bound
  scalar equality on objects is inconclusive.
- Sequence comparisons check rank, every lower bound and length, then elements;
  strict comparisons also require matching subtypes. Nested supported Variant
  arrays are bounded in recursion. Scalar equality on arrays, sequence comparison
  of nonarrays, and unsupported comparisons are inconclusive.

Reports record failed/inconclusive assertion method, message, phase, and source
position, plus unhandled error number and description. This profile is tested at
O0/O2 but has not been differentially validated against a VBA host/Rubberduck for
every coercion edge case.

`Rubberduck.FakesProvider` supports construction, identity, and release so unused
fixture fields can load. **No fake dispatch or COM mocking is implemented.** Calling
fake members is diagnosed; no override is silently simulated. VS Code Test Explorer
integration and context-scoped intrinsic overrides remain future work.

## Architecture

The immutable Program/checker remain shared with editor tooling, following the
`typescript-go` separation between frontend semantics and consumers. VBA annotations
are lexer-backed comment metadata rather than executable AST nodes. Discovery
produces a framework-neutral manifest with resolved declaration/symbol links;
serialized manifests include IDs, names, categories, fixture names, and source
locations (UTF-8 byte offsets, one-based UTF-16 line/column positions).

The adapter resolves declarations to semantic IR procedure IDs. `llvm.EmitTests`
registers direct pointers to the normal private procedures; it does not look up
procedure names at runtime or alter their VBA visibility. Typed IR operations
express virtual construction, ownership, class checks, and assertion invocation
with the existing Result/Try error convention. The C runtime records test outcomes
independently of that convention. The Go supervisor owns native linking, process
isolation, timeouts, and reports. The language server never launches tests.

## Original regex test audit

```sh
bin/vambas-test -project ../vba-regex/test/TestVbaRegex.vbp -list
python3 llvm/audits/vba-regex/run-tests.py
```

The complete original manifest discovers and runs **203 tests in eight modules**
at both O0 and O2: TestArrayBuffer (1), TestAst (4), TestCompiler (71),
TestDfsMatcher (58), TestLexer (12), TestRangeConstants (12), TestReplace (11),
and TestStaticRegex (34). The audit uses that original `.vbp` directly, including
all 14 core modules. It does not copy, rewrite, or filter source files.

[Collection](../llvm/COLLECTION.md), [GoSub/Return](../llvm/GOSUB.md), and
[ParamArray helpers](../llvm/PARAMARRAY.md) now compile natively. This replaces
the earlier 180-test audit of an explicit core subset, which used TestCompileMatch
instead of TestStaticRegex. The test sets are different; neither count represents
a filtered run of the other. General class/COM and object-Variant execution remain
outside the current native subset.

Run all repository checks with `go test -timeout 20m ./...`. The existing LLVM
native suite recompiles many programs and can take about ten minutes. Runner-only
checks use `go test ./testdiscovery ./testrunner ./cmd/vambas-test`.
