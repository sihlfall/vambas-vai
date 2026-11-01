# Mid, Mid$, and Mid assignment

The current typed String compiler supports `Mid(s, start[, length])`,
`Mid$(s, start[, length])`, and both `Mid(s, start[, length]) = replacement` and
`Mid$(s, start[, length]) = replacement`.

Positions are one-based UTF-16 code-unit positions. Omitted length means the
remaining characters for reads and the available replacement for writes. Lengths
are clipped using subtraction/minimum, avoiding overflow in `start + length`.

Try the [example](../examples/mid/Main.bas):

```sh
go run ./cmd/vambas-compile -root examples/mid -entry Main.Main -o /tmp/mid.ll
clang-20 -O2 /tmp/mid.ll -o /tmp/mid
/tmp/mid
echo $? # 8
```

| Operation | Behavior |
|---|---|
| Function, start beyond end | Empty String |
| Function, zero length | Empty String, with argument validation preserved |
| Function, nonpositive start or negative length | Error 5 |
| Assignment, start outside 1..Len(target) or negative length | Error 5, even for zero-length/empty replacement |
| Assignment, valid bounds | Replace min(length, remaining destination, replacement length) units |

Assignment never changes the destination's length. A short replacement leaves
the rest untouched; a long replacement is clipped. Embedded NUL and isolated
surrogate units remain ordinary units. For example:

```vb
s = "abcdef"
copy = s
Mid$(s, 2, 2) = "XY"       ' s = "aXYdef", copy = "abcdef"
part = Mid$(s, 2, 2)       ' part = "XY"
Mid$(s, 2) = Mid$(s, 1, 4) ' overlapping values are preserved
```

`Mid` and `Mid$` have separate ambient declaration symbols: the former has a
Variant result, the latter String. The checker may represent the known String
result of a Mid call with a typed String input without boxing. The native compiler
currently requires typed String input; Variant/Null behavior remains unsupported,
including the different Null behavior of the two forms. `MidB`/`MidB$`, fixed-length
strings, numeric/String coercions, and foreign/COM pointer exposure remain
unsupported. Named function arguments, omitted Length, and `VBA.Mid$` qualification
are supported. User declarations can shadow function names normally.

## Syntax and semantics

`KindMidAssignmentStatement` is a distinct syntax node with Mode, Target, Start,
optional Length, and Expression children. The Mode keeps the `$` spelling; it is
statement syntax rather than a callable expression. Parsing distinguishes this
special form with lookahead, retaining normal expression parsing for Mid calls.
Targets must be writable bound variable storage of declared type String or Variant.
The checker publishes `GetMidAssignmentResolution` with storage and Start/Length/
replacement conversions. Variant targets are deferred by native lowering. The
current compiler accepts declared local, parameter, and function-result String
slots; other storage forms follow the existing compiler subset restrictions.

Lowering records evaluation explicitly:

1. Obtain the destination variable address and evaluate start and optional length.
2. `OpMidCheck` validates against its current length, with an explicit error edge.
3. Evaluate the replacement expression and its String conversion.
4. `OpMidAssign` mutates the current destination, revalidating if RHS calls changed
   that variable. Earlier argument/RHS side effects survive a later failure.

This staging follows the checks-before-RHS order in MS-VBAL's Mid statement
semantics. Reference-host validation of side-effecting RHS and reentrant mutation
corner cases is still outstanding. On Error/Resume source support remains future
work; every operation currently propagates through its semantic statement region.

`OpMid` returns an owned String through Result/Try. `OpMidCheck` returns
Result<Void>; `OpMidAssign` adds WritesStorage and ManagesLifetime effects and also
returns Result<Void>. The verifier checks bound reference/value types, effects,
error continuations, and ownership/cleanup.

## Mutation, sharing, and optimization

Read results use owned views for multiple units and inline values for one unit.
Views retain the underlying owner; a later mutation of either String detaches
when necessary, preserving both values. A view's logical end need not have a
terminator, so external BSTR/pointer boundaries must materialize proper storage.

Writes reuse an unshared managed buffer. Literal/shared storage is copied before
the first content write. RHS snapshots and substring views retain shared owners,
which also makes overlapping source/destination writes safe. A one-unit target
is updated inline. ByRef arguments alias the destination slot; ByVal and
parenthesized ByRef temporaries remain independent.

Detachment allocation failure raises error 14 and leaves the destination
unchanged. No allocation occurs for a valid zero-unit write or when an unshared
buffer can be reused. These runtime alternatives are visible to LLVM and can be
specialized without modifying the source AST or shared checker.

Tests cover syntax recovery, typing/storage plans, omitted/named arguments,
clipping/errors, UTF-16 units, shared and overlapping values, live-buffer cleanup,
and injected detachment failure. They assemble/verify and run original/optimized
bitcode and native Clang O0/O2 executables. A fault-injection test permits only the
first allocation and confirms that a subsequent unique-buffer write succeeds.

See Microsoft's [Mid function](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/4a65ee4e-b6b9-45d3-a3f6-576fed4bb227)
and [Mid statement](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/2a8f3567-c8e0-4176-a802-cf2edeba425f)
specifications, and the [String runtime contract](STRINGS.md).
