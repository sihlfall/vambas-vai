# vba-regex compiler gap audit

Snapshot: 2026-10-25. Compiler commit `001386f6c39bddb2d19cc8fb2c0443b12517e34e`;
vba-regex commit `1619cd75526903045b8c5cb8c6048f93bc4160be` (clean `src` tree).
The [source manifest](audits/vba-regex/source-manifest.json) records the exact file hashes.

## Current result: complete original test project

With [Collection](COLLECTION.md), [GoSub/Return](GOSUB.md), and
[ParamArray](PARAMARRAY.md), the complete original TestVbaRegex.vbp compiles
and passes **all 316 tests in ten modules at O0 and O2** (vba-regex commit
`49b698a`, checked 2026-11-01, including direct array access through
[shape snapshots](ARRAYS.md#shape-snapshots)). The full project uses all core
source modules, including StaticRegex.SplitByRegex. Reproduce with:

```sh
python3 llvm/audits/vba-regex/run-tests.py
```

The [recorded result](audits/vba-regex/after-collection.json) is the earlier
203-test snapshot, with module counts, source hash verification, and the tested
toolchain.

The script runs the original manifest directly, without copying, rewriting, or
filtering source. This supersedes the earlier explicitly selected 180-test core
audit (which used a different set of test modules). General user-class/COM and
object-Variant support remain compiler limitations; they do not block this corpus.
The sections and recorded artifacts below are historical snapshots.

## Historical progress after Err, On Error, UDT arrays, and captured With

The unchanged full-project compilation now reports **3 diagnostics**, all unknown
`Collection` types in `StaticRegex.SplitByRegex`, down from 45. Err.Raise and the
On Error handler bind/check successfully. See the
[current diagnostics](audits/vba-regex/after-errors-and-record-arrays-diagnostics.txt).

All **13 unchanged modules other than StaticRegex** now pass lowering, LLVM
emission, assembly, and LLVM verification. A separate temporary execution copy
removes **only StaticRegex.SplitByRegex**, preserving the other API procedures,
and adds an [execution probe](audits/vba-regex/execution-probe.bas). Native O0/O2
runs both return 42: a valid repeated capture matches, a nonmatching input fails,
and an invalid pattern is caught by the original TryInitializeRegex handler.
This is executable regex progress, not full-project/Collection support.

The original source files remain unchanged and match the baseline manifest.
Reproduce the subset checks with:

```sh
python3 llvm/audits/vba-regex/run-subset.py
```

See the [record-array/With contract](UDTS_ENUMS.md),
[error-policy contract](ERROR_HANDLING.md), and
[recorded results](audits/vba-regex/after-errors-and-record-arrays.json).
Collection/object support is the remaining blocker identified for this corpus.
Older sections below retain historical findings and diagnostic counts.

## Progress after project storage and optional defaults

Module/global storage, numeric Const, typed Optional/default arguments, and
Boolean/integer assignment conversions now pass checked lowering and native
execution tests. Their previous minimal failure probes now all succeed; see the
[updated probes](audits/vba-regex/after-project-storage-probes.json) and
[execution/lifetime contract](PROJECT_STORAGE.md).

The unchanged full project still produces **45 front-end diagnostics**, because
`Err`, `On Error`, and `Collection` remain unsupported. These diagnostics occur
before body lowering, so this change removes hidden lowering blockers without
changing that count. See the
[current command output](audits/vba-regex/after-project-storage-diagnostics.txt).
No regex sources were modified. Baseline source hashes are unchanged.

Remaining work from this audit: UDT arrays and captured element With receivers,
Err/On Error, and Collection/object support. The older sections below preserve
the original findings; they describe historical restrictions rather than the
current implementation of the completed features.

## Progress after integral operators and string built-ins

The requested executable hex literals, `\`/`Mod`, and `AscW`, `StrComp`,
`vbBinaryCompare`, `InStr`, `Left$`, `String`, `Split`, and `IIf` now have binding,
checked lowering, runtime support, and native execution tests. See the
[built-in contract and runtime limits](BUILTINS.md).

Re-running the same full-project command now produces **45 diagnostics**, down
from 68: 1 unsupported `On Error`, 41 undeclared `Err` references, and 3 unknown
`Collection` types. All 23 requested library-name diagnostics are gone. The
front-end gate still prevents full-project LLVM emission, so the other lowering
gaps below remain relevant. See the
[updated diagnostics](audits/vba-regex/after-builtins-diagnostics.txt) and
[updated unchanged-module/minimal probes](audits/vba-regex/after-builtins-probes.json).

Unchanged `ArrayBuffer.bas` now independently passes lowering, LLVM emission,
assembly, and LLVM verification. Both enum-only modules still pass lowering.
This validates the next stage for a real source module; it does not establish
execution of the complete regex engine. The baseline evidence below is retained
for comparison, including the original source hashes and diagnostic counts.

At that stage the remaining work included numeric Const, Boolean/integer
coercions, typed Optional/defaults, and module storage, completed above.

## Baseline compilation result

From the repository root:

```sh
GOCACHE=/tmp/vambas-vai-go-cache go run ./cmd/vambas-compile \
  -root ../vba-regex/src -o /tmp/vba-regex.ll
```

**Compilation fails with 68 diagnostics; this attempt emits no LLVM.** The input
is a library, so no `-entry` was requested. Loading/encoding succeeds for all 14
standard modules: 4,545 physical lines, 87 procedures, 15 UDTs, 17 enums, and 234
enum members. No input files were modified.

| Stage | Diagnostics | Root causes |
|---|---:|---|
| Parser | 1 | `On Error GoTo Fail` |
| Binder | 64 | Missing VBA library names, listed below |
| Checker | 3 | `Collection` in a return annotation, local declaration, and `New` expression |

The compiler returns at this diagnostic gate, before native lowering. Therefore
68 is not the number of missing compiler features. See the
[complete diagnostic log](audits/vba-regex/compile-diagnostics.txt).

## Visible front-end/library gaps

| Missing support | Occurrences | Representative source |
|---|---:|---|
| `Err.Raise` ambient binding and executable custom-error raising | 41 references to `Err` | `RegexLexer.bas:206`, `RegexAst.bas:290`, `RegexDfsMatcher.bas:744` |
| `On Error GoTo` handler execution | 1 statement | `StaticRegex.bas:36`, in `TryInitializeRegex` |
| `AscW` | 10 calls | `RegexDfsMatcher.bas:122`, `RegexLexer.bas:904` |
| `StrComp` | 2 calls | `RegexBytecode.bas:100`, `RegexIdentifierSupport.bas:94` |
| `vbBinaryCompare` | 4 references | `RegexBytecode.bas:103`, `RegexReplace.bas:32` |
| `InStr` | 2 calls | `RegexReplace.bas:32`, `RegexReplace.bas:57` |
| `Left$` | 2 calls | `RegexReplace.bas:129`, `StaticStringBuilder.bas:71` |
| `String` repetition function | 1 call | `StaticStringBuilder.bas:106` |
| `Split` | 1 call | `StaticRegex.bas:293` |
| `IIf` | 1 call | `RegexCompiler.bas:294` |
| `Collection`, `New`, reference assignment/return, and `.Add` method calls | 3 unknown-type diagnostics; one API uses the object | `StaticRegex.bas:187–221`, `SplitByRegex` |

These are valid standard-library uses, not undeclared variables in the regex
implementation. They need library declarations **and** lowering/runtime support.
An `IIf` implementation must preserve eager argument evaluation. Error handling
must catch errors propagated from nested regex calls, rather than only explicit
raises in the handler's own procedure. This corpus uses `Err.Raise` and one GoTo
handler; it does not require `Resume` or `On Error Resume Next` to compile.

## Native-lowering gaps hidden by the first gate

These were identified in the checked source tree and confirmed against current
lowering restrictions. Self-contained compiler probes reproduce the restrictions;
the array-element With case is a downstream requirement after UDT arrays exist.
Counts below are **source occurrences**, not extra diagnostics or independent
failures. Some categories overlap at the same location.

| Missing support | Scope in this source | Representative source / work required |
|---|---|---|
| UDT arrays | 3 dynamic array fields | `RegexDfsMatcher.bas:24` (`StartLengthPair()`), `:41` (`DfsMatcherStackFrame()`), `RegexIdentifierSupport.bas:21` (`IdentifierTreeNode()`). Needs allocation, indexing, Preserve/copy/clear, and field/ByRef access to record elements. |
| Captured `With` receivers over UDT array elements | 18 With expressions in 10 procedures | `RegexDfsMatcher.bas:182`, `RegexIdentifierSupport.bas:78`, `RegexReplace.bas:140`, `StaticRegex.bas:107`. Current lowering accepts stable variable/field paths only. Element receivers need capture/lifetime handling across statements and jumps. |
| Module/global storage | 4 declarations | `RegexAst.bas:54` initialization flag; `RegexUnicodeSupport.bas:4–5` flags and `:37` fixed `StaticData()` Long array. Needs project lifetime, initial values, and qualified reads/writes/ByRef access. |
| Numeric `Const` declarations and references | 19 Long constants; 1 module declaration and 18 local declarations | `RegexLexer.bas:59`, `RegexAst.bas:61–65`, `RegexReplace.bas:18–26`, `RegexUnicodeSupport.bas:166`. General numeric Const lowering is still rejected even though enum constant evaluation works. |
| Executable hexadecimal literals | 85 literals | `RegexDfsMatcher.bas:122` (`&HFFFF&`), `RegexLexer.bas:309`, Unicode initialization starting `RegexUnicodeSupport.bas:51`. Correct signed width/suffix handling is required. Hex literals **inside enums already work**. |
| Integer division `\` and `Mod` | 19 divisions and 2 Mod expressions, in 13 procedures | `ArrayBuffer.bas:133,148`; `RegexLexer.bas:301,582`. Must implement integral coercion, quotient/remainder, zero-divisor errors, and overflow—not floating `/` as a substitute. |
| Typed optional parameters/default arguments | 36 parameters in 16 procedures: 29 Boolean, 5 Long, 2 String | `RegexCompiler.bas:29`, `RegexDfsMatcher.bas:63–65`, `StaticRegex.bas:226–227`. Needs declaration support and omitted-argument/default materialization, including ByRef String defaults. Existing Optional Variant/Missing support does not cover these. |
| Boolean/integral Let coercions | 2 assignments | `RegexLexer.bas:655` (`dash = 0`, Integer → Boolean), `StaticRegex.bas:51` (`regex.isCaseInsensitive = caseInsensitive`, Boolean → Long). The checker accepts them; lowering rejects them. |

The [source-occurrence CSV](audits/vba-regex/source-findings.csv) contains all 188
catalogued locations. The [probe log](audits/vba-regex/lowering-probes.json)
contains complete minimal sources and their actual diagnostics. The probes do
not stub library behavior, remove original diagnostics, or claim that the full
project would work after one substitution.

## What is already working

- `RegexErrors.bas` and `RegexNumericConstants.bas` independently pass checked
  lowering. These are enum-only modules, so this validates declarations and
  constant evaluation, not execution of the regex engine.
- `ArrayBuffer.bas` independently reaches body lowering. Its two `\` expressions
  are the root failures; the other two reported conversion errors cascade from
  those unsupported operations. Its Long-array UDT, stable With receivers, member
  accesses, loops, ordinary calls, and ReDim Preserve pass the earlier checks.
- The project parses the existing general control flow: 55 For loops, 41 Do
  loops, 164 GoTo statements, and 12 Select Case statements. The one parser gap
  reported is the error-handler statement.
- Plain/nested UDTs, scalar-array fields, enums, ordinary ByRef/named calls,
  Len/Mid/Mid$/ChrW$, and Mid assignment should not be counted as wholly missing
  features. The remaining record gap is specifically record arrays and captured
  element receivers. Library uses of `Collection` still need object support.

## Suggested implementation order

1. **Scalar lowering foundations:** `\`/`Mod`, executable hex literals, numeric
   Const, Boolean/integer coercions. Use `ArrayBuffer.bas` as the first unchanged
   module to bring through lowering and LLVM verification.
2. **Library bindings and runtime:** AscW, StrComp/vbBinaryCompare, InStr,
   Left$, String repetition, Split, and eager IIf. Declarations alone are insufficient.
3. **Project storage and record arrays:** globals/default initialization, then
   UDT-array operations and captured With/field aliases. These unblock Unicode
   tables, capture storage, the matcher stack, and the identifier tree.
4. **Typed optional/default arguments and error behavior:** both are needed by
   the unchanged public APIs. Err.Raise must preserve custom numbers through
   propagation, and TryInitializeRegex needs its GoTo handler.
5. **Collection/object API:** required for the unchanged complete project;
   `SplitByRegex` is the isolated Collection-using API if a deliberately smaller
   core-only target is chosen later. The current compiler checks unused code too.

This is an evidence-based gap list, not an end-to-end conformance result. Native
IR verification, linking, regex execution, and differential behavior cannot be
validated for the unchanged complete project until the blocking gates are removed;
additional defects may become visible then.
