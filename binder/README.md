# Binding the supported VBA syntax

See the [AST reference](../ast/README.md) for the syntax tree, symbol and flow
structures, and their ownership and traversal rules.

```go
file := parser.ParseSourceFile(source).AsSourceFile()
parseDiagnostics := file.ParseDiagnostics
binder.BindSourceFile(file)
symbol := file.Locals.Lookup("FunctionName")
diagnostics := file.BindDiagnostics
```

Binding is once per immutable source file. Concurrent calls for the same file wait
for its binding to complete; separate files can also be bound concurrently. Parse
a new source file after an edit.

For multiple files, use [`compiler.NewProgram`](../compiler/README.md). It owns
fresh module trees and collects declarations from every module before binding
initializers and bodies. Its project scope resolves public standard-module names,
module/project qualification, class types and declared class receivers. It also
supplies source filenames, module kinds, default instances, and aggregated diagnostics.

The implementation follows `../typescript-go/internal/binder` in its pooled binder,
symbol tables, declaration links, antecedent-based flow graph, and `BindOnce`
lifecycle. `Node.ForEachChild` visits only syntax children, and binding sets their
`Parent` links. Bound symbols and flow nodes remain owned by the resulting tree
when the binder returns to its pool.

The [standard-library catalog](../stdlib/catalog.go) describes existing string,
conversion, information, and array-bound functions, restricted `CreateObject`,
Collection/Err members, and all 269 pinned inventory constants independently of syntax and
runtime layouts. Parameter metadata preserves
ByRef storage, Missing, and explicit typed defaults; language built-ins outside the
section 6 inventory are marked separately.
Constant descriptors retain signed numeric values, UTF-16 control characters,
and the distinction between `vbNullString` and an ordinary empty String.
[`library.go`](library.go) materializes fresh ambient declarations, defaults, and
symbols for every scope. Catalog-backed declarations use `Symbol.Library` for
checking and lowering; their `Symbol.Intrinsic` is zero. The separate Rubberduck
testing library retains legacy intrinsic identities. `Strings`, `Conversion`,
`Information`, `Interaction`, `Constants`, `ColorConstants`, `KeyCodeConstants`,
and `SystemColorConstants` are ambient procedural namespaces.
For example, `Left$`, `VBA.Left$`, `Strings.Left$`, and `VBA.Strings.Left$` share
one declaration; the canonical owner is `VBA.Strings`. Modules and their export
tables are fresh per scope. User declarations/modules retain lookup precedence,
and missing module members never fall back to unrelated root exports. Language
built-ins retain their root exports. All 16 predefined enums are actual enum
declarations: their members have enum owners and share declarations with root
aliases, and their type annotations resolve to Long. Distinct `$` forms remain
separate declarations.
Class descriptors preserve Collection's default Item getter and read-only Count,
and Err's existing singleton namespace, writable state variables, and Raise/Clear
Subs. Member declarations and parameter symbols are fresh per scope. `CreateObject`
also exports through `Interaction` while retaining its existing Class-only
signature and restricted virtual testing-object runtime support.
Ordinary runtime helpers use generic library-call IR derived from the catalog.
Additional families and richer call contracts remain in the
[integration plan](../VBA_STANDARD_LIBRARY_SUPPORT_PLAN.md).

The parser supports functions, Subs, and property accessors with signatures,
module/local variable and constant declarations, `Option Explicit`, assignments,
`If`/`ElseIf`/`Else`, statement
labels, `GoTo`, computed `On ... GoTo`, loops, `Select Case`, typed exits, member/index access, and explicit
and implicit calls, Set/New, and With blocks. Module declarations are collected before initializers and
procedure bodies, so forward names resolve. The module symbol's
`Exports` table contains procedures except those declared `Private`, and variables
or constants explicitly declared `Public` or `Global`. Module `Dim` and unmodified
`Const` declarations are private.

Computed GoTo destinations resolve in the same procedure label table as ordinary
GoTo, including forward, backward, numeric, and duplicate destinations. Each
destination contributes a possible flow edge; the following statement remains
reachable for zero or out-of-list indices. Binding does not evaluate the index.

VBA identifiers use case-insensitive symbol-table keys; use `SymbolTable.Lookup`
instead of indexing with a source spelling. A trailing String type suffix (`$`)
does not create a different name. A function's `Name().Symbol` describes the
procedure, while `ResultSymbol` describes its implicit local result variable.
Property Get has the same result-variable distinction; Let and Set have no result.
Local declarations are collected across the whole procedure before references are
bound, including declarations inside `If` branches. They shadow module names even
before their textual declaration. With `Option Explicit`, unresolved value names
produce diagnostics and do not introduce locals. Without it, unresolved names
become implicit procedure locals. Blocks do not introduce variable scopes. See the
[VBA identifier rules](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/7df907cb-ab6c-40d3-aa81-272742ce00c3)
and [function result rules](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/227005ad-78fb-479f-8145-fa3b8b610386).

Each function has a separate `Labels` table. Named and numeric labels attached to
the same statement share an entry flow; numeric names omit leading zeroes.
`GotoStatement.Target` points to the resolved label symbol. Duplicate procedures,
duplicate labels, missing labels, and invalid assignment targets produce binding
diagnostics with the offending node and, for duplicates, the previous declaration.
`BindingDiagnostic.File` and `RelatedFile` identify each span's owning source,
including when a project diagnostic refers to another module.
Diagnostics include source ranges; underlined spans exclude leading trivia.
Parser recovery placeholders are ignored instead of being declared as empty names.
See [source ranges and parser recovery](../parser/README.md).

`Node.FlowNode` records incoming flow. Assignments, calls, and bare references to
other parameterless functions add assignment and call flows. `If` splits
and joins paths; VBA logical operators evaluate both operands. Forward and
backward jumps are connected before a final reachability pass, including jumps
into branches and disconnected cycles. `NodeFlagsUnreachable` and
`FlowFlagsUnreachable` expose the result. `EndFlowNode` records fall-through at the
end of each procedure; `ReturnFlowNode` joins fall-through and explicit procedure
exits. `NodeFlagsHasImplicitReturn` means the normal end is reachable;
assigning the result variable does not exit the function.

Parameters and explicit variables/constants have declaration nodes as their
`Symbol.ValueDeclaration`, rather than an arbitrary first use. Duplicate names
across parameters, local declarations, and function result variables are reported.
Constants are distinguished by `NodeFlagsConst` on the parent declaration list;
they retain VBA scope and assignments to them are diagnosed. Initializers, bounds,
and Optional defaults are bound as constant expressions without creating executable
flows or implicit value symbols. Optional defaults use the module's constant scope,
not the procedure's local variables. Project binding resolves local class type
names separately from values; unknown external types remain for a future checker
and never introduce implicit variables. `As New` describes VBA's lazy
variable construction, not an immediate constructor call at the declaration.

This is binding for the current AST, not a full VBA compiler. Type checking,
constant evaluation/cycle detection and external libraries remain later passes.
Standalone binding has no project scope or class-context validation; use a Program
for cross-module resolution and `Me` checking. Legacy factory parameter lists
made of identifiers remain accepted alongside proper parameter declaration nodes.

`Program.GetTypeChecker()` now provides an initial semantic type/signature layer
and scalar diagnostics. It reads these binding links without changing them; see
the [checker documentation](../checker/README.md) for its scope and deferred work.

Loops use `FlowFlagsLoopLabel` junctions and explicit back edges, following
`typescript-go`. Disconnected loop headers remain graph nodes because a later
`GoTo` may enter their bodies. `While` and pre-test `Do` may skip their bodies;
post-test `Do` enters its body first. `Until` reverses the condition edges without
changing the syntax. Constant Boolean conditions can expose infinite loops or
dead bodies; counted-loop bounds are treated conservatively without constant
evaluation. `For` captures its limit and step on entry and records counter writes;
`For Each` captures its collection and writes the control variable per iteration.

Typed exit targets select the nearest enclosing loop of the requested kind, so
`Exit For` can cross an inner Do and `Exit Do` can cross a While. VBA has no
`Exit While` or `Exit Select`. Procedure exits must match Function versus Sub.
`ExitStatement.Target` points to the loop/procedure; invalid exits and mismatched
`Next` names produce diagnostics. A standalone `End` has no procedure-return edge.

Select evaluates its selector once and tests alternatives in order along the
unmatched path. A match enters that clause's body, which branches to the selection
exit instead of the next clause. Without Case Else, a no-match path remains.
`FlowFlagsSwitchClause` marks case-test edges. Label collection traverses these
bodies before binding, and the final reachability pass accounts for jumps into
otherwise unreachable loops and case bodies. Runtime error transfers and
GoSub targets use the same local labels. GoSub continuations conservatively merge
possible Return flows; runtime LIFO matching belongs to execution. Return does
not add a procedure-exit edge.

Property Get/Let/Set declarations of the same normalized name share one symbol,
following typescript-go's accessor merging with a third VBA-specific accessor
kind. The symbol unions `SymbolFlagsGetAccessor`, `SymbolFlagsLetAccessor`, and
`SymbolFlagsSetAccessor`; `Declarations` retains source order and
`ValueDeclaration` prefers the getter when present. Duplicate accessor kinds and
collisions with functions, Subs, variables, or constants produce declaration
diagnostics. An accessor retains its own parameters, Locals, Labels, result
variable (Get only), and flow graph. `Exit Property` targets that accessor's return
join; it cannot exit an ordinary Function or Sub.

Unqualified reads invoke the getter. Value assignments select Let, while Set
assignments select Set; indexed arguments and the RHS are evaluated before a single
setter call and write flow. A write does not also invoke the getter. Reads of
write-only properties, writes to getter-only properties, and writes without a
matching Let/Set accessor are diagnosed. Getter-body assignments to its own name
write the result variable; an explicit recursive application resolves the property
symbol. `P()(i)` writes through a getter result rather than invoking P's setter.

`ast.PropertyAccessor(symbol, kind)` retrieves a particular declaration. A property
is present in module Exports if any accessor is non-private; per-accessor visibility
remains on each declaration. Project binding checks visibility on the selected
accessor, including qualified and declared-instance accesses. Index-signature
matching, return/value-type compatibility, argument checking, and dynamic receiver
resolution still require a checker.

Call targets and argument values are bound in evaluation order. Explicit
applications create one call flow, without also invoking a bare function callee.
Known declared arrays are indexed without a call flow; unresolved default-member
applications conservatively retain a possible call. Member names and named-argument
labels are not local value references. Project binding resolves members of known
namespace/class receivers; dynamic members and argument labels remain unresolved.
Member/element assignments create assignment flow;
their setter and index compatibility still require checking. `AddressOf` retains a
procedure reference without invoking it.

Subs can be invoked in statement context but still produce a diagnostic if used
as values. Inside a Function, `F(args)` resolves the callee to the procedure symbol,
while bare `F` and assignments to `F` resolve to its result variable. Named-argument
matching, argument counts, passing-mode checks, built-in/library procedures, and
dynamic member resolution are not implemented by the binder.

The checker now resolves argument lists and storage independently of these flow
links; see [calls and storage](../checker/CALLS.md). The binder accepts function
applications as candidate assignment targets because a returned object/array can
supply writable storage. The checker rejects scalar results in that context.

Set shares Let's assignment checks and writes while retaining object-assignment
syntax, including assignments to a function result or an indexed/property target.
New class names use type syntax, even under Option Explicit and when a value has
the same name. A Program resolves known project classes. Executable New adds one call flow for creation;
an `As New` declaration still does not eagerly construct an object. Object-type
compatibility and class instantiability require a checker.

A With header is bound once, in the enclosing receiver context. Its body then
uses the captured receiver, including when the original variable is reassigned.
An omitted receiver's `PropertyAccessExpression.WithTarget` points to the enclosing
With statement; it is not a syntax child or an implicit local symbol. Nested With
bodies mask outer receivers and restore them on exit. A With block introduces no
variable scope and does not intercept typed exits. Implicit members resolve when
a Program knows the receiver's class, and never introduce value locals. Binding outside a With reports
`with-block-expected`, and receiver context is reset for each procedure.

GoTo reachability still follows explicit flow edges. A jump into a With body does
not evaluate the skipped header; its lexical receiver link is retained. The binder
does not simulate runtime receiver initialization or error 91. This reflects the
receiver lifetime distinction described by the [VBA With reference](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/with-statement).

Module-level Type and Enum declarations participate in the declaration prepass,
so forward type annotations work across files. Type lookup uses the annotation's
own module and the project type scope, independently of procedure value names.
Module-qualified UDTs such as `ArrayBuffer.Ty` resolve; enum annotations may be
unqualified or project-qualified. Private types remain within their module.

Record fields have property symbols owned by their type. Enum constants have
enum-member symbols owned by their enum and are also available in module/project
constant scope. Public enums in class modules enter project scope without becoming
class instance members. Ambiguous unqualified enum constants require an enum
qualifier; their individual symbols remain distinct. The checker supplies nominal
record compatibility and enum values; the binder never creates synthetic initializers.

ReDim first resolves existing procedure/module/project names. If no name exists,
a simple target declares a local resizable array, including under Option Explicit.
Explicit declarations are collected before this step, so a later Dim or a
declaration in another module takes precedence. Runtime bounds are bound as
ordinary expressions. Each target adds an `ArrayMutation` flow node after its
receiver/bounds, not a call for its final allocation parentheses. Preserve remains
syntax; compatibility and runtime validation belong to the checker/runtime.

A small ambient scope currently declares `vbObjectError As Long = -2147221504`,
including `VBA.vbObjectError` qualification. User declarations take precedence.
These source-less, ordinary constant symbols are fresh for each project and do not
add fake modules, variables, or source files to the user's program. They enable
enum expressions without spelling-based special cases in the checker. More VBA
library declarations and navigable declaration-file locations remain future work.

The same ambient scope now declares Len, LenB, ChrW, vbNullString, and vbNullChar.
Intrinsic identity is immutable symbol metadata, so a user declaration with the
same spelling keeps normal scope/visibility behavior. These declarations support
the [initial String compiler subset](../llvm/STRINGS.md), not the entire VBA library.

`Collection`/`VBA.Collection` are ambient class declarations with Add/Remove
methods and read-only Item/Count getters. `Symbol.DefaultMember` links the class
to Item, allowing ordinary checker default-member resolution without changing
source syntax. These symbols remain separate from project declarations, which
retain normal type lookup precedence. See the [Collection contract](../llvm/COLLECTION.md).

The VBA rules followed here are described in Microsoft's
[variable declarations](https://learn.microsoft.com/en-us/office/vba/language/concepts/getting-started/declaring-variables),
[constants](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/const-statement),
[Option Explicit](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/option-explicit-statement),
and [parameter list specification](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/78bcb344-4966-4401-bb55-72729790ebee).
