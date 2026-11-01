# Err and executable error policies

The shared parser/binder/checker retain ordinary syntax, symbols, and call plans.
Error handling executes in semantic IR and LLVM, independently of binder FlowNodes.
The private procedure ABI remains `status + output + error provenance`; compiled
code uses no native exception unwinder or setjmp/longjmp.

## Supported behavior

- `On Error GoTo label`, numeric handler labels, `On Error GoTo 0`,
  `On Error Resume Next`, and `On Error GoTo -1` (reset the active error).
- `Resume`/`Resume 0`, `Resume Next`, and `Resume label`.
- `Err.Raise Number, [Source], [Description], [HelpFile], [HelpContext]`, including
  named/omitted arguments and negative custom numbers such as `vbObjectError + n`.
- `Err.Clear`, bare `Err` as the Number value, and qualified `VBA.Err`.
- Reading/writing Number, Description, Source, HelpFile, and HelpContext.
- The legacy `Error number` statement, which resets previous properties and uses
  default error information instead of inheriting Err.Raise's omitted properties.

Each activation starts with no enabled handler. An enabled GoTo handler becomes
active when it catches an error. A further error while it is active propagates
to the caller; it cannot recursively catch itself. Called procedures have their
own policy. A caller can catch an otherwise unhandled callee error, including
errors produced by arithmetic, coercion, allocation, and descriptor operations.
Inline Resume Next retains error information and skips the failed statement;
it also handles failures propagated from a nested call at that call's statement.

Resume restarts the failing statement in the handling activation; for a callee
failure this is the caller's call statement. Resume Next uses the statement's
recovery continuation. Resume label chooses the named target. Resume without an
active error raises error 20, which can itself be handled. Resume clears Err and
deactivates the handler. Err.Clear only clears the properties; it does not end
an active handler. On Error statements clear Err. Explicit procedure Exit and
normal completion of an active handler clear Err as well.

The implementation follows Microsoft's [On Error semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/e2561165-c99a-444b-8bc0-be60a196867a),
[Resume rules](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/resume-statement),
and [Err clearing rules](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/clear-method-visual-basic-for-applications).

## State, effects, and cleanup

`Statement.ErrorMode`/`Handler` record policy changes; the Resume terminator
carries `ResumeMode` and an optional label target. Retry/Next metadata remains
separate from ordinary branches and now drives recovery. Err operations have
explicit ReadsAmbientErr/WritesAmbientErr effects. LLVM owns activation-local
policy, active-handler state, and saved recovery destinations. It uses ordinary
branches/switches so LLVM can eliminate unnecessary policy machinery.

The C runtime owns the intrinsic Err properties. Number/HelpContext are Long;
text properties use owned UTF-16 String values. Property getters return snapshots,
so clearing/replacing Err cannot invalidate a previously acquired String.
Err.Raise prepares conversions and owning values before committing its properties.
A failed preparation becomes the new coercion/allocation error. Omitted Raise
arguments use current property values, or the runtime's defaults when empty.

A failed Try first commits any language-required error-side write (including the
existing Double division behavior), records the original provenance, updates Err
at the originating operation, and cleans statement snapshots/argument temporaries.
A local handler retains the activation's variables and completed ByRef/global
writes. Propagation releases activation owners. The caller preserves the callee's
origin and custom Err properties while selecting its own recovery continuation.
Function output storage is committed only on success.

Only the error-side write, provenance, and Err update are emitted at each
failing operation. The provenance record is one aggregate store. Each region then
shares one failure tail (its cleanup and handler routing, entered with the status
through a phi), and each procedure shares one `unwind` block that releases
activation owners and returns the status. Region cleanup clears all of the
region's values; values not yet acquired are still zero. For the vba-regex test
project this reduces the LLVM IR from 72.6 MB to 31.8 MB, and Clang `-O2` time
from 213 s to 53 s.

Captured With receivers and enumeration state remain available while a GoTo
handler runs, so Resume can return to them. Resume releases scopes it leaves;
normal jumps and procedure exit use the same resource cleanup. Captured pointers
are checked before dereference (error 91), and live descriptor locks prevent
resize/replacement with error 10. See [UDT arrays and With](UDTS_ENUMS.md).

`@vba.project.clear()` clears Err as well as project globals, and exists for an
error-using module even without global variables. The native main wrapper calls
it on success/failure. An LLVM embedding harness must clear the quiescent project
at the end of its lifetime. Runtime Err state follows the current single-threaded
project profile; there is no COM ErrObject/IErrorInfo adapter.

## Runtime profile and limits

Default Source is the compiled project's name. Common runtime errors have English
descriptions; unknown numbers use a generic application/object error description.
Host-localized messages, host HelpFile/context catalogs, Erl, LastDllError, and
Error(number) as a description function are not implemented. Err properties can
be assigned directly; native address-taking of those properties is not provided
(use values/parenthesized arguments for a ByRef temporary).

The private four-i32 error record still contains number, originating procedure,
statement, and source byte offset. It is provenance, separate from the observable
Err properties, and is not a public VBA ABI. Activation setup failures before
executing On Error propagate to the caller. [GoSub/Return](GOSUB.md) now preserve
a per-activation resumption stack across handled errors and Resume, with error 3
for an empty Return. [Collection](COLLECTION.md) now uses the same error protocol;
general COM execution and public interop remain separate work.

Tests execute original/optimized LLVM and native O0/O2 code, check custom numbers,
property snapshots/defaults, active-handler propagation, retry/next/label recovery,
Double error-side stores, With locks across recovery, and allocation failures.

## Try it

The [example](../examples/errors_records/Main.bas) combines UDT arrays,
ReDim Preserve, captured With, a custom Err.Raise, and a GoTo handler:

```sh
go run ./cmd/vambas-compile -root examples/errors_records -entry Main.Run -o /tmp/vba-errors-records.ll
clang-20 -O2 /tmp/vba-errors-records.ll runtime/*.c -lm -o /tmp/vba-errors-records
/tmp/vba-errors-records
echo $? # 42
```
