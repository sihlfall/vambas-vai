# GoSub and Return

Native execution supports `GoSub label`, the two-word `Go Sub label`, bare
`Return`, and computed `On expression GoSub label, ...` (also `Go Sub`). Named
and numeric labels use the existing procedure-local label namespace. Ordinary
GoTo remains a jump; Return here resumes a GoSub and does not exit a procedure.

The implementation follows the per-invocation resumption list in
[MS-VBAL's GoSub statement](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/492e1f84-c47f-40ef-819f-f1d23e475c91)
and the [VBA GoSub/Return documentation](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/gosubreturn-statement).
No Rubberduck implementation code was used.

## Runtime behavior

Each procedure activation owns an initially empty LIFO stack. Recursive procedure
calls have separate stacks; nested GoSubs within one activation share that
activation's stack. Procedure locals, function results, and On Error policy remain
shared across GoSub calls. Reaching End Sub/Function, Exit Sub/Function, or an
unhandled error discards all remaining resumption entries and releases the buffer.

Return pops the most recently executed GoSub and resumes immediately after it,
including colon-separated statements and calls in single-line If branches. An
empty Return raises error 3, with Err.Description `Return without GoSub`; it never
branches through a null or uninitialized address. Allocation failure while growing
the stack raises error 7 before entering the subroutine or changing the stack.
On Error Resume Next skips a failed GoSub/Return; Resume retries that statement.
A handled error inside a GoSub preserves its pending resumption entry.

Computed dispatch uses the existing On GoTo expression evaluation, Integer
coercion, and 0..255 range checks. Zero/out-of-list indices fall through without
pushing. Only the selected destination pushes a continuation. See the
[computed dispatch contract](CONTROL_FLOW.md).

GoSub preserves a caller's active With captures and For Each iterators when it
branches to a subroutine outside those lexical scopes. Returning from a subroutine
cleans scopes it leaves while preserving the caller's resumption scope. Like
GoTo, GoSub cannot enter an uninitialized resource scope; such entries produce a
compiler diagnostic. Return matching itself is dynamic, not a lexical pairing.

## Frontend and semantic IR

The AST distinguishes `KindGosubStatement`, `KindGosubReturnStatement`, and
`KindOnGosubStatement`. GoSub shares GoTo's label payload and target symbol;
computed GoSub shares the existing computed-dispatch payload. The parser, binder,
and checker retain their existing responsibilities. The binder conservatively
merges possible Return effects into GoSub continuations without pretending the
resumption stack is a statically paired branch graph.

Semantic IR adds `Gosub` and `GosubReturn` terminators and explicit fallible
`OpGosubPush`/`OpGosubPop` operations with immediate Result/Try continuations. A
push region names its subroutine with Target and its resumption site with Next.
The pop produces a private `GosubAddress` token consumed only by the Return
terminator; it is not a VBA value, numeric statement ID, or public/COM ABI value.
The verifier checks targets, error continuations, token type, and stack-operation
placement. `ir.Format` prints both transfer and resumption edges.

## LLVM and C helpers

Each GoSub pushes an LLVM block address in its own function:

```llvm
%status = call i32 @vba_gosub_push_ptr(
    ptr %gosub.stack,
    ptr blockaddress(@vba.p1, %gosub.cont7))
; Existing status/error handling checks %status before the branch.
br label %sWork
```

A successful pop obtains an opaque pointer and branches to a common dispatcher:

```llvm
gosub_return:
  %gosub.destination = load ptr, ptr %gosub.target
  indirectbr ptr %gosub.destination, [label %gosub.cont7, label %gosub.cont12]
```

Every pushed address appears in that function's complete indirectbr destination
list. Continuation trampolines reconcile resource scopes before entering the
normal statement region. A scope selector is used only for cleanup; execution
resumes through the blockaddress/indirectbr pointer, not a switch on return IDs.
LLVM can simplify the trampolines and dispatch when possible.

The C helper treats addresses as opaque. Its private stack is `{void **entries,
uint64_t count, uint64_t capacity}`, matching `%vba.gosub.stack`. Growth allocates
and copies before committing, so a failed allocation preserves existing entries.
Push returns the existing i32 error status; pop uses an output pointer plus i32
status so error 3 follows the same ABI. This is the necessary difference from an
infallible void-push/pointer-pop interface. Linking the sources in `runtime/sources.txt` supplies
these services; the public Automation Variant/SAFEARRAY layouts are unchanged.

## Try it and validation

```sh
go run ./cmd/vambas-compile -root examples/gosub -entry Main.Main -o /tmp/gosub.ll
clang-20 -O2 /tmp/gosub.ll runtime/*.c -lm -o /tmp/gosub
/tmp/gosub
# Expected exit status: 42.

go test ./parser ./binder ./checker ./lowering ./ir
go test ./llvm -run 'TestNativeGosub|TestGosubRuntime'
```

Native tests assemble and verify LLVM, execute original/optimized bitcode, and
run Clang O0/O2 binaries with allocation tracking. They cover nested GoSubs,
recursive procedure isolation, multiple Return sites, single-line If/colon
continuations, computed dispatch, abandoned entries on exit/error, captures and
iterators, empty-stack errors, allocation failure, and Resume. An independent C
harness checks LIFO ordering, growth rollback, untouched output on empty pop, and
idempotent cleanup. Native execution is validated on 64-bit Linux; Windows
execution remains unvalidated.

The complete original TestVbaRegex.vbp now passes all **203 tests at O0/O2**,
including [Collection](COLLECTION.md)-based APIs. Reproduce the unchanged project
run with `python3 llvm/audits/vba-regex/run-tests.py`.
