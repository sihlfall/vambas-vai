# Source ranges and recovery

See the [AST reference](../ast/README.md) for node shapes, a worked tree, traversal,
and the fields populated by parsing and binding.

`Implements ClassName` retains an `ImplementsStatement` with an identifier or
qualified type name. It can appear in the declaration section or between
procedures. Class-only and contract restrictions belong to the checker; see
[Implements](../llvm/IMPLEMENTS.md).

Mid/Mid$ calls use ordinary call expressions. Their assignment special form uses
`MidAssignmentStatement`, retaining Mode, Target, Start, optional Length, and RHS.
Lookahead distinguishes the statement without changing normal call parsing.
See the [Mid implementation contract](../llvm/MID.md).

Module-level `Type ... End Type` and `Enum ... End Enum` declarations preserve
visibility and member lists. UDT fields retain qualified annotations, array bounds,
and fixed-length strings, and may use reserved names such as `End`. Enum members
retain optional initializers. Numeric literals preserve explicit type suffixes,
including `&HFFFF&`. Missing declaration terminators preserve subsequent module
declarations/procedures for recovery; malformed input still returns usable trees.

Procedure statements support `ReDim [Preserve] target(bounds) [As type]`, including
multiple targets, runtime `lower To upper` bounds, With members, and calls/indexes
inside a target's receiver. The final parentheses are retained as allocation bounds,
so resize syntax is never mistaken for an implicit call or ordinary array index.

In implicit statement calls, horizontal whitespace also separates a callee from
an argument beginning with `.` or `!`. For example, `Helpers.Update .Value, n`
passes the enclosing With's `.Value`; it does not append `.Value` to the callee.
Adjacent member suffixes and explicitly continued member chains still use the
normal member/call expression layers. This also applies when the callee itself
uses an omitted With receiver, such as `.Update .Value, n`.

```go
file := parser.ParseSourceFile(source).AsSourceFile()
for _, diagnostic := range file.ParseDiagnostics {
    // Half-open byte offsets into file.Text, excluding leading trivia.
    fmt.Printf("%s [%d,%d): %s\n",
        diagnostic.Code, diagnostic.Pos(), diagnostic.End(), diagnostic.Message)
}
binder.BindSourceFile(file) // Recovered trees can still be bound.
```

The scanner and parser follow the corresponding `typescript-go` components:

- `TokenFullStart` includes leading trivia, `TokenStart` starts the token itself,
  and `TokenEnd` ends it. `TokenRange` is the precise token span. Positions are
  UTF-8 byte offsets, not rune or UTF-16 offsets.
- Scanner errors use `SetOnError` and become source-file parse diagnostics.
  Unknown characters produce an advancing `KindUnknown` token. Unterminated
  strings retain their partial value and token flags without consuming a newline.
- `finishNode` assigns `Node.Loc`, links immediate child parents, and propagates
  parse-error flags. Like TypeScript's parser, syntax ranges include leading
  trivia and exclude trailing trivia. Factory-created nodes have undefined ranges
  until positioned by a parser or caller. Node-list ranges cover their contents;
  the source file covers all input and includes an EOF token for trailing trivia.
- Parenthesized expressions retain their own syntax node and range. Semantic
  consumers can use `ast.SkipParentheses`.
- Missing names and expressions are zero-width identifiers with empty text and
  parse-error flags. The binder ignores these placeholders, so recovery does not
  introduce empty-name symbols or spurious missing-target diagnostics.

VBA needs different synchronization points from TypeScript: newlines and colons
separate statements, while `Else`, `ElseIf`, and `End If` delimit branches. `End
Function` and a following procedure header must survive recovery inside a damaged
block. Explicit line continuations hide the physical newline from the parser;
apostrophe and `Rem` comments preserve the following newline. LF, CRLF, CR, and
the Unicode newline characters recognized by the reference grammar are supported.

Malformed or currently unsupported syntax produces diagnostics instead of input-
driven panics. Recovery retains enclosing block terminators and skips otherwise
unrecognized text to a statement boundary. Delimited declaration and parameter
lists retain missing elements and recover at commas, closing parentheses, and
procedure boundaries. A nesting limit protects recursive parsing from pathological
input. Recovery is approximate, so consumers should inspect diagnostics before
treating the tree as valid VBA.

## Declarations and signatures

```vb
Option Explicit
Private total As Long
Public Const Limit As Long = 10

Function Compute(ByVal value As Long, Optional factor As Long = Limit) As Long
    Dim result As Long
    result = value * factor
    Compute = result
End Function

Sub Reset()
    total = 0
End Sub
```

`SourceFile.Statements` is the ordered list of all module elements, following
TypeScript's source-file shape. `FunctionDeclarations` remains a compatibility
view containing Function, Sub, and property accessor declarations; traversal visits each element
once through `Statements`. `Sub` shares the function signature/body representation
with its own node kind and has no function result variable.

`Attribute name = value[, value...]` retains module/member metadata as syntax
without executing its expressions during binding. Module-level `VB_Name` and
`VB_PredeclaredId` initialize source metadata; the last occurrence wins.
`Option Private Module` has its own leaf node and placement/duplicate diagnostics.
Use [`compiler.NewProgram`](../compiler/README.md) to supply filenames and module
kinds, bind cross-module references, and resolve local class types and receivers.
Inputs contain code and optional attributes; exported VERSION/designer preambles
and project manifests require host-specific extraction.

Variable statements use `VariableDeclarationList` and `VariableDeclaration`
nodes. `Const` uses `NodeFlagsConst` on the list, as in TypeScript, while its scope
remains VBA's procedure/module scope. Each declarator retains its own name, type,
array dimensions, and constant initializer. Supported declaration forms include
`Dim`, local `Static`, module visibility modifiers, dynamic and bounded arrays,
fixed-length strings, and `As New` variable annotations. Named and qualified types
are represented as type syntax. Declaration type characters are retained on names;
references without the suffix resolve to the same declared name. Decimal,
exponent, hex, and octal literals can be used in constant expressions.

Signatures retain parameter nodes, explicit `ByVal`/`ByRef`, `Optional` defaults,
array designators, and `ParamArray`. Omitted passing modes mean `ByRef`.
Functions and Property Get retain their `As` return type; return annotations on
Sub and Property Let/Set are diagnosed. The
parser checks parameter ordering and incompatible modifiers, such as a required
parameter after `Optional`, `ByVal` arrays, and non-final or non-Variant
`ParamArray` parameters. Module declaration and `Option Explicit` placement errors
are diagnosed without discarding following procedures.

These additions represent signatures and declarations rather than implementing
a type checker or runtime. Name/type compatibility, dynamic class resolution, constant
evaluation and cycles, and call-argument checking remain later work. `WithEvents`,
`ReDim`, and other module options remain outside the supported syntax. The existing
operator interpretation is retained for ambiguous `&`/`^` expression spellings.

## Member access, indexing, and calls

```vb
Sub UseValues(obj As Object, values() As Long, index As Long)
    Dim value As Long
    value = obj.Items(index).Value
    values(index) = value
    obj.Update value, count:=2
    Call obj.Update(value, count:=2)
    obj.Update (value)
End Sub
```

Expression parsing follows `typescript-go`'s primary, member-rest, and call-rest
layers above unary/binary parsing. Dot and dictionary (`!`) access can be chained
with applications, including `Factory()(index).Value` and `obj!Field`. Member and
named-argument names admit VBA's unrestricted keywords and bracketed names such as
`obj.[Column Name]`. Explicit line continuations are retained in source ranges.

VBA uses parentheses for procedure calls, array indexing, and default-member
application. All use `CallExpression` with ordered `Arguments`; parsing does not
guess their resolved meaning. Assignment targets can contain member and indexing
expressions. A `CallStatement` records statement context and an optional explicit
`CallKeyword`. Each application records `HasParentheses`.

`Call P(x)` retains an explicit keyword and a parenthesized argument list. `P x`
uses an unparenthesized application. In `P (x)`, ordinary whitespace starts the
implicit argument list and `(x)` remains a `ParenthesizedExpression`; this matters
for VBA's ByRef/ByVal behavior. An adjacent suffix `P(x)` remains application
syntax, with statement context preserved for later validation. The reference
grammar also permits an explicit line continuation before an index suffix.

Named arguments use `NamedArgument` nodes for `name:=value`. Empty slots, including
trailing slots, use zero-width `OmittedExpression` nodes rather than error
placeholders: `P(, x,)` has three arguments. Call-site `ByVal` and `AddressOf` are
prefix unary expressions with their respective operator kinds. Duplicate argument
names and positional/omitted arguments after a named argument produce diagnostics.
Malformed lists recover at commas, closing parentheses, and enclosing statement
boundaries while retaining subsequent procedures.

Member/type resolution, overload/default-member selection, parameter matching,
argument counts, and ByRef coercion remain checker work. Special output-list
syntax remains unsupported.

## Object assignment, creation, and With

```vb
Sub UpdateWidget(other As Object)
    Dim obj As Object
    Set obj = New Library.Widget
    If obj Is Nothing Then Exit Sub
    With obj
        Set .Child = other
        With .Child
            .Name = "Child"
        End With
        .Update
    End With
    Set obj = Nothing
End Sub
```

`Set` has its own statement kind while sharing the Let assignment payload and
target parsing. It supports variables, member/property targets, indexed targets,
function results, and `Nothing`. Object identity comparisons use `Is` at the
comparison precedence level. Object/value compatibility remains checker work.

`New` is a dedicated primary expression whose operand is a simple or qualified
class name, including bracketed names. It uses the same type-name syntax as
annotations, without resolving it as a value identifier. Unlike TypeScript's
constructor expression, VBA New has no constructor argument list; any following
application remains separate application syntax for later validation. `As New`
annotations retain their existing lazy-declaration representation.

`With ... End With` retains its receiver expression and body. Leading `.Member`
and `!Field` use property-access nodes with an omitted receiver, preserving the
leading operator and any later member/index/call chain. They parse outside a With
too, but binding reports `with-block-expected`. A missing member name still has a
parse diagnostic. Nested With headers use the enclosing receiver; their bodies
use the inner receiver, and the outer context is restored afterward. Recovery
retains `End With` and other enclosing block/procedure boundaries.

These distinctions follow VBA's [Set statement](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/set-statement),
[New expression grammar](https://download.microsoft.com/download/0/a/6/0a6f7755-9af5-448b-907d-13985accf53e/%5Bms-vbal%5D.pdf)
(section 5.6.8), and [With statement](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/with-statement).

## Property Get, Let, and Set

```vb
Private currentValue As Variant

Public Property Get Value() As Variant
    Value = currentValue
End Property

Public Property Let Value(ByVal rhs As Variant)
    currentValue = rhs
End Property

Public Property Set Value(ByVal rhs As Variant)
    Set currentValue = rhs
End Property
```

Property accessors share the procedure signature/body representation, with
distinct `KindGetAccessor`, `KindLetAccessor`, and `KindSetAccessor` kinds. Headers
retain visibility, Static, names, parameters, and the getter's optional return
type. Bodies end with `End Property` and support `Exit Property`. All accessors
appear in source order in `Statements` and the compatibility procedure list.

Let and Set require a final assignment-value parameter. It cannot be Optional,
ParamArray, or have a default. Optional index parameters are permitted before it;
a ParamArray may be the final index parameter before the value. Validation applies
the ordinary signature rules to the index list separately from the value rather
than treating the latter as an invalid required parameter after Optional.

Parsing and recovery retain procedure/block boundaries, including a following
property header when `End Property` is missing. Binding merges compatible accessor
kinds by case-insensitive name, but validates duplicate kinds and declaration
conflicts. Getter/setter type and index-signature compatibility, class-module
placement, default-property attributes, and dynamic receiver resolution are
later checker work. Programs resolve known module/class receivers. See VBA's [Property Get](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/property-get-statement),
[Property Let](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/property-let-statement),
and [Property Set](https://learn.microsoft.com/en-us/office/vba/language/reference/user-interface-help/property-set-statement) syntax.

## Loops, selection, exits, and jumps

```vb
Function FindValue(values() As Long, last As Long) As Long
    Dim index As Long
    For index = 0 To last
        Select Case values(index)
        Case 1 To 3
            FindValue = index
            Exit Function
        Case Is < 0
            GoTo Finished
        Case Else
            FindValue = -1
        End Select
    Next index
Finished:
End Function
```

Supported structured statements include `For ... To ... [Step ...] ... Next`,
`For Each ... In ... Next`, `While ... Wend`, and all pre-test, post-test, and
unconditional `Do ... Loop` forms. `Until` and test placement are preserved in the
AST rather than rewritten. Optional `Next` variables and shared closing lists
such as `Next inner, outer` are retained on their corresponding For nodes.
Counters refer to VBA procedure/module variables; loops do not introduce scopes.

`Select Case` retains expression alternatives, inclusive `lower To upper` ranges,
relational alternatives with optional `Is`, and `Case Else`. A default clause must
be last. The syntax follows TypeScript's statement/case-block/clause structure,
with VBA-specific range and relational nodes and no case fall-through.

`Exit For`, `Exit Do`, `Exit Function`, and `Exit Sub` retain their exit kind and
are validated during binding. A standalone `End` terminates execution and is
distinct from `End If`, `End Select`, and procedure terminators. Named/numeric
`GoTo` targets remain procedure-wide, including forward and backward jumps into
or out of structured bodies. Single-line numeric branches (`If ready Then 100
Else 200`) produce GoTo nodes as well.

`On expression GoTo first, 100, last` retains a distinct computed-dispatch node,
its expression, and source-ordered GoTo destination children. Destination names
are label references, not expressions. Mixed named/numeric targets, continuations,
colon separators, and single-line If branches are supported.

Recovery retains enclosing `Next`, `Loop`, `Wend`, `Case`, and `End ...` boundaries
when inner syntax is damaged. Exception handling (`On Error`/`Resume`) now has
explicit statement nodes and executable [error policies](../llvm/ERROR_HANDLING.md).
`GoSub`/`Return` and `On ... GoSub` now preserve their own statement kinds while
sharing procedure-local label syntax with GoTo; `Go Sub` is also accepted.

Each parse diagnostic includes its source file, stable code, message, and range.
The legacy individual parser entry points expose their diagnostics through
`Parser.Diagnostics()`. Binding diagnostics also include primary and related
ranges while retaining their original node references.

Regression tests cover recovery into subsequent functions and binding recovered
trees. Native fuzz targets check scanner progress, range containment, diagnostics,
and termination of parsing and binding on arbitrary input.
