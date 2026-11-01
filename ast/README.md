# AST reference

The parser produces an in-memory syntax tree rooted at `*ast.Node` with
`KindSourceFile`. Binding enriches that same tree with symbols, scopes, diagnostics,
and control flow. This document describes the current output, including recovered
trees; it is not a schema for every construct in the VBA grammar.

The design follows `typescript-go`: a common node header, kind-specific payloads,
ordered node lists, declaration links, and an antecedent-based flow graph. VBA
requires procedure-wide variable scopes, separate statement-label namespaces,
function result variables, and its own parameter and array syntax.

## Getting a tree

```go
package main

import (
    "fmt"

    "sihlfall.ch/vambas-vai/ast"
    "sihlfall.ch/vambas-vai/binder"
    "sihlfall.ch/vambas-vai/parser"
)

func main() {
    source := `Option Explicit
Const Offset As Long = 1
Function Increment(ByVal value As Long) As Long
    Dim result As Long
    result = value + Offset
    Increment = result
End Function
`
    root := parser.ParseSourceFile(source)
    file := root.AsSourceFile()
    fmt.Println(len(file.ParseDiagnostics)) // 0

    // Binding also works on recovered trees. Inspect both diagnostic lists.
    binder.BindSourceFile(file)
    fmt.Println(len(file.BindDiagnostics)) // 0

    procedure := file.Locals.Lookup("increment").ValueDeclaration
    fn := procedure.AsFunctionDeclaration()
    fmt.Println(fn.Name().AsIdentifier().Text) // Increment
    fmt.Println(fn.ResultSymbol == procedure.Locals.Lookup("Increment")) // true

    var walk func(*ast.Node) bool
    walk = func(node *ast.Node) bool {
        if node.Kind == ast.KindIdentifier {
            name := node.AsIdentifier().Text
            if node.Loc.IsValid() {
                spelling := file.Text[node.Pos():node.End()]
                fmt.Printf("%s: %q\n", name, spelling)
            }
        }
        return node.ForEachChild(walk)
    }
    walk(root)
}
```

`ParseSourceFile` returns the common node; `AsSourceFile` returns its concrete
payload. Both refer to the same node. Before binding, the syntax, parents, source
ranges, and parse diagnostics are available. Symbol and flow fields are initially
empty. `SourceFile.OptionExplicit` is also initialized by binding; before then,
inspect the option statement in `Statements`.

For multiple modules, use [`compiler.NewProgram`](../compiler/README.md). It owns
the file set and binds every module against the complete project declaration scope.

## Common representation

Every concrete payload embeds the following header, sometimes through base structs:

```go
type Node struct {
    Kind     Kind
    Flags    NodeFlags
    Loc      core.TextRange
    Parent   *Node
    Symbol   *Symbol
    Locals   SymbolTable
    FlowNode *FlowNode
    // Private data links the header to its concrete payload.
}

type NodeList struct {
    Loc   core.TextRange
    Nodes []*Node
}
```

| Field | Meaning |
| --- | --- |
| `Kind` | Discriminant used to select an `As…` accessor. |
| `Flags` | Syntax, recovery, and binding flags; see below. |
| `Loc` | Half-open source range; `Pos()` and `End()` expose its byte offsets. |
| `Parent` | Immediate syntax parent; nil on the source file. Parsing sets it, and binding establishes it again. |
| `Symbol` | Associated declaration or resolved reference, where applicable; not a type. |
| `Locals` | Value declarations on source files and procedures; blocks do not introduce scopes. |
| `FlowNode` | Incoming executable flow where applicable; a procedure points to its start flow. |

Check `Kind` before using an accessor such as `AsIdentifier()` or
`AsVariableDeclaration()`. Accessors assert the concrete payload type and panic
when used on the wrong payload. The aliases `Statement`, `Expression`, `BlockNode`,
`IdentifierNode`, `LineNumberNode`, and `TokenNode` all mean `Node`; their names do
not enforce kinds in Go. Similarly, `ParameterList` is an alias for `NodeList`.

Lists retain order and have no parent pointer of their own: a declaration's parent
is its `VariableDeclarationList` node, and a parameter's parent is its procedure.
Optional child pointers may be nil. An empty list and a nil list can mean different
things, especially for arrays.

## Source files and procedures

| Kind / payload | Syntax fields |
| --- | --- |
| `KindSourceFile` / `SourceFile` | `Text string`, `Statements *NodeList`, `FunctionDeclarations *NodeList`, `EndOfFileToken *TokenNode`, `ParseDiagnostics []*Diagnostic` |
| `KindFunctionDeclaration` / `FunctionDeclaration` | `Name() *IdentifierNode`, `Modifiers() Modifiers`, `Parameters *NodeList`, `Type *Node`, `Body *BlockNode` |
| `KindSubDeclaration` / `SubDeclaration` | Same payload and accessors as a function. `SubDeclaration` is an alias for `FunctionDeclaration`. |
| `KindGetAccessor` / `GetAccessor` | Same procedure payload, with an optional getter return type and result symbol after binding. |
| `KindLetAccessor` / `LetAccessor` | Same procedure payload; final parameter is the assignment value. No result variable or valid return annotation. |
| `KindSetAccessor` / `SetAccessor` | Same procedure payload; final parameter is the object-reference assignment value. No result variable or valid return annotation. |
| `KindBlock` / `Block` | `Statements *NodeList` |
| `KindOptionExplicitStatement` / `Token` | Leaf representing the whole `Option Explicit` statement. |
| `KindOptionPrivateModuleStatement` / `Token` | Leaf for `Option Private Module`. |
| `KindAttributeStatement` / `AttributeStatement` | `Name *Node`, `Values *NodeList`; exported module/member metadata, never executable value references. |
| `KindImplementsStatement` / `ImplementsStatement` | `Interface *Node`; an identifier or project-qualified class name, resolved in type scope. |

`SourceFile.Statements` contains all module elements in source order: options,
attributes, variable/constant statements, and procedures. `FunctionDeclarations` is a
compatibility view of the same Function, Sub, and property accessor nodes. It can even be the same
list as `Statements`; do not traverse both. The EOF token is a syntax child after
the statements and retains trailing trivia.

`SourceFile` also carries `FileName`, `ModuleName`, `ModuleKind`, and `PredeclaredID`.
The parser initializes `ModuleName` and `PredeclaredID` from
module-level `VB_Name`/`VB_PredeclaredId` attributes (last occurrence wins).
`compiler.NewProgram` supplies filenames, module kinds and host overrides.
`ModuleKind` distinguishes Unspecified (standalone), Standard, Class, Document,
and UserForm. This metadata is excluded from syntax equality and child traversal.

Procedure parameters are `KindParameter` nodes. `Type` is a `KindTypeReference`
when a return annotation is present; omitted annotations remain nil rather than
becoming synthetic `Variant` nodes. Valid Subs and property setters have no return
annotation. `Body` is a block. `ast.IsProcedureDeclaration(node)` recognizes all
procedure kinds; `AsFunctionDeclaration()` works for each. Property payload aliases
and their `AsGetAccessor`/`AsLetAccessor`/`AsSetAccessor` accessors are defined in
[accessors.go](accessors.go). `ast.IsPropertyAccessor(node)` identifies those three
kinds. `Name()`, `Modifiers()`, `Parameters`, `Type`, and `Body` remain common.

Procedure modifiers are stored privately and exposed by `Modifiers()`. Variable
statements expose a field instead. The `Modifiers` bitmask defines `ModifierNone`,
`ModifierPrivate`, `ModifierPublic`, `ModifierGlobal`, `ModifierStatic`, and the
currently misspelled exported name `VisibiltiyFriend`. A defined modifier does not
imply that every VBA use of it is supported.

Binding adds these payload fields:

| Payload | Binding fields |
| --- | --- |
| `SourceFile` | `OptionExplicit bool`, `OptionPrivateModule bool`, `SymbolCount int`, `ClassifiableNames collections.Set[string]`, `BindDiagnostics []BindingDiagnostic` |
| `FunctionDeclaration` (including Subs and accessors) | `Labels SymbolTable`, `ResultSymbol *Symbol`, `EndFlowNode *FlowNode`, `ReturnFlowNode *FlowNode` |

Functions and Property Get receive result symbols. `NodeFlagsHasImplicitReturn` indicates
that the procedure's end is reachable; it does not indicate that a result value
was assigned.

## Variables, constants, parameters, and types

These payloads are defined in [declarations.go](declarations.go). All fields below
are syntax, including type annotations. [The checker](../checker/README.md)
produces semantic types in its own tables; these fields remain source syntax.

| Kind / payload | Fields |
| --- | --- |
| `KindVariableStatement` / `VariableStatement` | `Modifiers Modifiers`, `Keyword Kind`, `DeclarationList *Node` |
| `KindVariableDeclarationList` / `VariableDeclarationList` | `Declarations *NodeList` |
| `KindVariableDeclaration` / `VariableDeclaration` | `Name *IdentifierNode`, `Dimensions *NodeList`, `Type *Node`, `Initializer *Expression` |
| `KindParameter` / `ParameterDeclaration` | `Name *IdentifierNode`, `Dimensions *NodeList`, `Type *Node`, `Initializer *Expression`, `Modifiers ParameterModifiers` |
| `KindTypeReference` / `TypeReference` | `TypeName *Node`, `NewKeyword *TokenNode`, `IsArray bool`, `Length *Expression` |
| `KindQualifiedName` / `QualifiedName` | `Left *Node`, `Right *Node` |
| `KindArrayDimension` / `ArrayDimension` | `LowerBound *Expression`, `UpperBound *Expression` |

A variable statement wraps a declaration list, which wraps one declaration per
name. `Keyword` is `KindDIM`, `KindSTATIC`, or `KindCONST`, or `KindNONE` for a
visibility-only declaration such as `Private total As Long`. Constant lists carry
`NodeFlagsConst`. As in `typescript-go`, constness belongs to the list; VBA scope
rules still apply. Each declaration has its own annotation, so in `Dim x, y As
Long`, only `y` has a `Type` child. `Initializer` holds a constant value expression
or is nil. Recovery can retain invalid variable initializers with a diagnostic.

Parameter modifiers are `ParameterOptional`, `ParameterByVal`, `ParameterByRef`,
and `ParameterParamArray`. They record explicit syntax. An omitted passing mode
sets neither ByVal nor ByRef; `IsByRef()` reports the effective mode and returns
false for ByVal and ParamArray. `Initializer` is an Optional default expression,
not an evaluated value. Legacy factory parameter lists containing identifiers
are still accepted by the binder; the parser emits parameter declaration nodes.

Array and type distinctions are preserved:

| Source fragment | Representation |
| --- | --- |
| `x` | `Dimensions == nil`, `Type == nil`. |
| `x()` | Non-nil `Dimensions` with no elements: a dynamic array designator. |
| `x(1 To 10, size)` | Two `ArrayDimension` nodes. First: lower `1`, upper `10`; second: lower nil, upper identifier `size`. No default lower bound is synthesized. |
| `x As Long` | `TypeReference.TypeName` is a `Token` of `KindLONG`. |
| `x As Project.Models.Widget` | `TypeName` is `QualifiedName(QualifiedName(Project, Models), Widget)` with identifier leaves. |
| `x As New Widget` | `NewKeyword` is a `KindNEW` token; `TypeName` is identifier `Widget`. |
| `x As String * 12` | `TypeName` is a `KindSTRING` token; `Length` is numeric literal `12`. |
| `x As Byte()` | `TypeReference.IsArray == true`, independently of the name's `Dimensions`. |

Parameter array designators use the same `Dimensions` field. Invalid fixed bounds
can be retained with diagnostics. Named type syntax is not resolved through the
value symbol tables. `NewKeyword` records VBA's lazy construction annotation;
binding does not turn it into an immediate constructor call.

## Statements and expressions

The following table lists the other payloads currently emitted by the parser.
Each has the corresponding `As…` accessor except the fieldless `MeExpression`
and `OmittedExpression` leaves.

| Kind / payload | Fields and interpretation |
| --- | --- |
| `KindLetStatement` / `LetStatement` | `LExpression`, `RExpression` (`*Expression`). Explicit `Let` and implicit assignments have the same shape. |
| `KindMidAssignmentStatement` / `MidAssignmentStatement` | `Mode`, `Target`, `Start`, `Length`, `Expression` (`*Node`). Mode retains Mid/Mid$ spelling; nil Length preserves omission. This is a special statement, not a call lvalue. |
| `KindSetStatement` / `SetStatement` | Same fields as Let, with a distinct kind for object-reference assignment. `SetStatement` is an alias for `LetStatement`; both accessors work. |
| `KindIfStatement` / `IfStatement` | `Expression *Expression`, `ThenStatement *Statement`, `ElseStatement *Statement`. Then is a block; Else is nil, a block, or another If for `ElseIf`. Single-line branches also use blocks. |
| `KindLabeledStatement` / `LabeledStatement` | `LineNumber *LineNumberNode`, `Label *IdentifierNode`, `Statement *Statement`. Numeric and named labels can coexist; a label-only statement has nil `Statement`. |
| `KindGotoStatement` / `GotoStatement` | `LineNumber *LineNumberNode` or `Label *IdentifierNode`. Binding adds `Target *Symbol`; it is not a syntax child. |
| `KindOnErrorStatement` / `OnErrorStatement` | `Mode int` (1 disable, 2 GoTo, 3 Resume Next, 4 reset active error), optional `Target *Node` containing GoTo target syntax. |
| `KindResumeStatement` / `ResumeStatement` | `Mode int` (0 retry/Resume 0, 1 Next, 2 label), optional GoTo `Target *Node`. |
| `KindErrorStatement` / `ErrorStatement` | `Expression *Node` for the legacy Error statement. |
| `KindOnGotoStatement` / `OnGotoStatement` | `Expression *Expression`, `Destinations *NodeList` of GoTo nodes in source order. Each child retains its named/numeric target and binder-resolved `Target`; the parent represents conditional computed dispatch. |
| `KindLineNumber` / `LineNumber` | `Text string` retains digits; `HasMinus()` exposes the separately stored sign. |
| `KindIdentifier` / `Identifier` | `Text string`, `TypeHint Kind`. Text preserves source spelling, including a declaration type character when present. |
| `KindNumericLiteral` / `NumericLiteral` | `Text string`, `TokenFlags TokenFlags`. Text retains numeric spelling rather than an evaluated number. A negative value uses a prefix unary node. |
| `KindStringLiteral` / `StringLiteral` | `Text string`, `TokenFlags TokenFlags`. Text is decoded: no delimiters and doubled quotes become one quote. Unterminated literals retain partial content and `TokenFlagsUnterminated`. |
| `KindMeExpression` / `MeExpression` | Leaf for `Me`. Binding sets `NodeFlagsContainsThis` on its containing procedure. |
| `KindBinaryExpression` / `BinaryExpression` | `Left *Expression`, `OperatorToken *TokenNode`, `Right *Expression`. The operator is a visited child. |
| `KindPrefixUnaryExpression` / `PrefixUnaryExpression` | `Operator Kind`, `Operand *Expression`. The operator is an enum value, not a child token. |
| `KindParenthesizedExpression` / `ParenthesizedExpression` | `Inner *Expression`. `ast.SkipParentheses(node)` unwraps it for semantic consumers. |
| `KindPropertyAccessExpression` / `PropertyAccessExpression` | `Expression *Expression`, `OperatorToken *TokenNode`, `Name *IdentifierNode`. Operator is `KindDOT` or `KindEXCLAMATIONPOINT`; a nil Expression preserves an omitted With receiver. Binding adds `WithTarget *Node`. |
| `KindNewExpression` / `NewExpression` | `Expression *Node`: an Identifier or QualifiedName referring to a class in type syntax, not a value reference. No constructor argument list. |
| `KindCallExpression` / `CallExpression` | `Expression *Expression`, `Arguments *NodeList`, `HasParentheses bool`. Represents calls, indexing, and default-member applications with the same syntax. |
| `KindCallStatement` / `CallStatement` | `CallKeyword *TokenNode` (nil for implicit calls), `Expression *Expression` (an application, including for a bare parameterless call). |
| `KindNamedArgument` / `NamedArgument` | `Name *IdentifierNode`, `Expression *Expression`. The name is argument-label syntax, not a value reference. |
| `KindOmittedExpression` / `OmittedExpression` | Leaf for an intentionally empty argument slot; no payload fields or `As…` accessor. |
| Various token kinds / `Token` | No additional fields. Used for operators, built-in type names, keyword literals (`True`, `False`, `Nothing`, `Empty`, `Null`), options, and EOF. |

Declaration type characters `%`, `&`, `^`, `!`, `#`, `@`, and `$` are recorded as
`Identifier.TypeHint` kinds; no suffix is `KindNONE`. Name lookup strips the suffix.
The current expression parser retains its operator interpretation for ambiguous
`&`/`^` spellings; declaration support does not imply complete expression syntax.

[kind.go](kind.go) also includes scanner tokens and scaffolding that the parser
does not emit as dedicated AST nodes. For example, built-in annotations use
`TypeReference` plus a token, not the existing `KeywordTypeNode` scaffold.

### Application syntax and statement context

The payloads in [expressions.go](expressions.go) preserve VBA's call distinctions:

| Source | Shape |
| --- | --- |
| `obj.Member` | `PropertyAccessExpression(obj, DOT, Member)` |
| `obj!Field` | `PropertyAccessExpression(obj, EXCLAMATIONPOINT, Field)` |
| `f(x)` or `values(i)` in an expression | `CallExpression`, `HasParentheses=true`; resolution determines call versus indexing/default-member access. |
| `Call P(x)` | `CallStatement` with `CallKeyword`, wrapping a parenthesized `CallExpression`. |
| `P x` | `CallStatement` without keyword, wrapping `CallExpression` with `HasParentheses=false`. |
| `P (x)` | An implicit application whose argument is a `ParenthesizedExpression`, preserving possible ByVal evaluation. |
| `P(x)` as a statement | A parenthesized application within an implicit `CallStatement`; legality for the resolved target remains checker work. |
| `P` or `Call P` | A statement application with an empty argument list and `HasParentheses=false`. |
| `f(, x,)` | Three argument slots: omitted, identifier, omitted. |
| `f(count:=x)` | One `NamedArgument` with name `count` and expression `x`. |

Ordinary whitespace before `(` starts implicit statement arguments; the grammar's
explicit line continuation before an index suffix is treated separately.
Call-site `ByVal value` and `AddressOf Callback` use `PrefixUnaryExpression` with
operators `KindBYVAL` and `KindADDRESSOF`. Declaration passing modes still use
parameter modifier flags. Bracketed identifiers preserve the brackets in `Loc`
and expose their inner name in `Text`.

`ForEachChild` visits property receiver, operator, and name; call target followed
by arguments; statement keyword followed by application; and named-argument name
followed by value. Omitted arguments are visited leaves with zero-width ranges
and no parse-error flags on valid input. Recovery placeholders for missing values
remain distinct from deliberately omitted slots.

### Object expressions and With receivers

`Set obj = New Library.Widget` produces a `KindSetStatement` with an identifier
target and a `NewExpression` value. Its operand is `QualifiedName(Library, Widget)`.
No constructor-call node or value symbol is synthesized for the class name.
Set and Let syntax compare unequal even though they share their payload. `Set obj
= Nothing` retains the existing `KindNOTHING` token as its value. `Is` comparisons
use `BinaryExpression` with an `OperatorToken` of `KindIS`.

`WithStatement` has `Expression *Expression` and `Statement *Statement` (a body
block), with kind `KindWithStatement`. The receiver expression is evaluated once
when entered. `.Child` is `PropertyAccessExpression(nil, DOT, Child)`;
`!Key` preserves an EXCLAMATIONPOINT operator instead. A following access in
`.Child.Name` wraps the first access in an ordinary property-access node.

Binding sets `WithTarget` only on accesses with an omitted receiver. It points to
the nearest enclosing With statement; for an inner header such as `With .Child`,
that is the outer With. Explicit receivers leave `WithTarget` nil. After the inner
body, accesses use the outer With again. The back reference is binding metadata:
traversal excludes it and syntax equality ignores it. Binding reports an omitted
receiver outside any With without inventing an empty-name variable.

Traversal visits a With header before its body, a New class name as type syntax,
and Set's target before its value. Bound New nodes have incoming flow and add a
creation call flow; their class names have no value symbols or executable flow.
With captures its receiver without inserting synthetic AST children or local
symbols, and member accesses do not rebind the header. Dynamic class/member
resolution, object assignment compatibility, and runtime receiver errors require later checks.
Project binding resolves known local class names on New and type annotations,
and members of namespace/declared class receivers, including With and Me.

### Control-flow statements

The payloads in [statements.go](statements.go) follow TypeScript's statement and
case-block structure while retaining VBA's loop headers and typed exits.

| Kind / payload | Syntax fields |
| --- | --- |
| `KindWhileStatement` / `WhileStatement` | `Expression *Expression`, `Statement *Statement` (body block); represents `While ... Wend`. |
| `KindDoStatement` / `DoStatement` | `Expression *Expression`, `Statement *Statement`, `ConditionKind Kind` (`KindWHILE`, `KindUNTIL`, or `KindNONE`), `ConditionAtEnd bool`. |
| `KindForStatement` / `ForStatement` | `Initializer *Node` (LetStatement for counter/start), `Limit *Expression`, `Step *Expression`, `Statement *Statement`, `NextVariable *Expression`. |
| `KindForEachStatement` / `ForEachStatement` | `Initializer *Expression` (control variable), `Expression *Expression` (collection), `Statement *Statement`, `NextVariable *Expression`. |
| `KindSelectStatement` / `SelectStatement` | `Expression *Expression` (selector), `CaseBlock *Node`. |
| `KindCaseBlock` / `CaseBlock` | `Clauses *NodeList`, in source order. |
| `KindCaseClause` / `CaseClause` | `Expressions *NodeList` (alternatives), `Statement *Statement` (body block). |
| `KindDefaultClause` / `CaseClause` | Same payload/accessor; nil `Expressions` means `Case Else`. |
| `KindCaseRangeClause` / `CaseRangeClause` | `LowerBound *Expression`, `UpperBound *Expression` for inclusive `To` ranges. |
| `KindCaseComparisonClause` / `CaseComparisonClause` | `IsKeyword *TokenNode` (optional), `OperatorToken *TokenNode`, `Expression *Expression`. |
| `KindExitStatement` / `ExitStatement` | `BlockKind Kind`: `KindDO`, `KindFOR`, `KindFUNCTION`, `KindSUB`, or `KindPROPERTY`. Binding adds `Target *Node`, excluded from traversal. |
| `KindEndStatement` / `Token` | Leaf for a standalone `End`; distinct from block terminators. |

Omitted For steps, Next names, and unconditional Do expressions remain nil. Shared
`Next inner, outer` lists assign each name to its own For node; they do not share
syntax children. Loop counters are variable references, not declarations or new
scopes. Numeric single-line If branches use `GotoStatement` nodes, just like
explicit GoTo statements. Named and numeric labels inside all these bodies retain
the existing `LabeledStatement` wrapper and procedure-wide namespace.

Traversal remains in source order. In particular, post-test Do visits its body
before its condition; pre-test Do visits the condition first. Case alternatives
precede the body, and Next names follow it. `EndFlowNode` on a procedure describes
normal fall-through; `ReturnFlowNode` joins it with explicit Exit Function/Sub/Property
paths. Assigning a function result is still an assignment, not an exit.

## Worked tree

The source in the Go example above produces this syntax tree. Prefix `Kind` is
omitted from kind names, and repeated `As Long` subtrees are abbreviated.

```text
SourceFile
├─ Statements[0]: OptionExplicitStatement (Token)
├─ Statements[1]: VariableStatement (Keyword=CONST)
│  └─ DeclarationList: VariableDeclarationList (Flags includes Const)
│     └─ Declarations[0]: VariableDeclaration
│        ├─ Name: Identifier("Offset")
│        ├─ Type: TypeReference → TypeName: Token(LONG)
│        └─ Initializer: NumericLiteral("1")
├─ Statements[2]: FunctionDeclaration
│  ├─ Name(): Identifier("Increment")
│  ├─ Parameters[0]: Parameter (Modifiers=ParameterByVal)
│  │  ├─ Name: Identifier("value")
│  │  └─ Type: TypeReference(LONG)
│  ├─ Type: TypeReference(LONG)
│  └─ Body: Block
│     ├─ Statements[0]: VariableStatement (Keyword=DIM)
│     │  └─ DeclarationList: VariableDeclarationList
│     │     └─ Declarations[0]: VariableDeclaration
│     │        ├─ Name: Identifier("result")
│     │        └─ Type: TypeReference(LONG)
│     ├─ Statements[1]: LetStatement
│     │  ├─ LExpression: Identifier("result")
│     │  └─ RExpression: BinaryExpression
│     │     ├─ Left: Identifier("value")
│     │     ├─ OperatorToken: Token(PLUS)
│     │     └─ Right: Identifier("Offset")
│     └─ Statements[2]: LetStatement
│        ├─ LExpression: Identifier("Increment")
│        └─ RExpression: Identifier("result")
└─ EndOfFileToken: Token(EndOfFile)
```

After binding, `file.Locals` contains keys `offset` and `increment`.
`file.Symbol.Exports` contains `increment` because procedures are public by
default; the unmodified constant is private. The procedure's `Locals` contains
`value`, `result`, and `increment` (its result variable). The declaration's
`Name().Symbol` is the procedure symbol, while the assignment to `Increment`
resolves to the distinct `ResultSymbol`. All uses of `result` share its variable
declaration's symbol, and the use of `Offset` points to the module constant.

## Traversal and source fidelity

`Node.ForEachChild` visits immediate syntax children in source order, skipping
nil children and flattening lists. Return false to continue and true to stop.
It does not recurse by itself; the example supplies recursion and propagates the
stop result. The implementation in [visitor.go](visitor.go) is the authoritative
child list. It excludes compatibility views, parents, symbols, flow nodes, and
diagnostics. Walking every exported pointer would revisit nodes and encounter
cycles.

All `Loc` values are half-open UTF-8 byte ranges into `SourceFile.Text`, not rune
or UTF-16 positions. Parsed node ranges include leading trivia and exclude
trailing trivia, following `typescript-go`. Lists cover their contents; the file
covers `[0, len(Text))`. EOF retains trailing trivia. `core.TextRange` exposes
`Pos()`, `End()`, `Len()`, and `IsValid()`; undefined factory ranges are `[-1,-1)`.

Comments, separators, and much punctuation do not have their own nodes. For
example, assignment `=`, declaration `As`, parameter modifier keywords, and
parentheses around a parenthesized expression are represented by fields, flags,
or wrappers rather than individual token children. Use the source text and ranges
when original spelling or trivia matters; decoded literal text alone is not
enough to reproduce the source.

Recovery returns partial trees and diagnostics. Missing names/expressions are
usually zero-width identifiers with empty text and parse-error flags; a missing
numeric GoTo target can instead be an empty line-number node. The binder ignores
these placeholders. Zero width alone does not indicate an error: empty blocks,
lists, and EOF may legitimately have zero-width ranges. Unsupported statements
may be skipped, so a recovered AST is not a complete record of invalid input.

| Node flag | Meaning |
| --- | --- |
| `NodeFlagsConst` | A constant declaration list (syntax). |
| `NodeFlagsThisNodeHasError` | A parser error associated with this node. |
| `NodeFlagsThisNodeOrAnySubNodesHasError` | Parse errors on this node or descendants. |
| `NodeFlagsContainsThis` | The containing procedure uses `Me` (binding). |
| `NodeFlagsHasImplicitReturn` | The procedure's end is reachable (binding). |
| `NodeFlagsUnreachable` | The binder determined that this executable syntax is unreachable. |

`ParseDiagnostics` contains pointers to `Diagnostic` records with `File`, `Loc`,
`Code`, and `Message`. `BindDiagnostics` contains `BindingDiagnostic` values with
`File`, `RelatedFile`, `Loc`, `RelatedLoc`, `Code`, `Message`, `Node`, and `RelatedNode`. An absent related
location is undefined. Scanner/current-token parse diagnostics and binding
diagnostics exclude leading trivia; node-based parse diagnostics can cover the
node's full range. See the [parser documentation](../parser/README.md) for recovery
behavior.

## Symbols, scopes, and control flow

```go
type Symbol struct {
    Flags            SymbolFlags
    Name             string
    Declarations     []*Node
    ValueDeclaration *Node
    Members          SymbolTable
    DefaultMember    *Symbol // Optional ambient-library default member, e.g. Collection.Item.
    Exports          SymbolTable
    Parent           *Symbol
    Intrinsic        IntrinsicKind // Identity of an unmigrated ambient declaration.
    Library          stdlib.MemberID // Catalog identity; zero for user/unmigrated declarations.
}
type SymbolTable map[string]*Symbol

type FlowNode struct {
    Flags       FlowFlags
    Node        *Node
    Antecedent  *FlowNode
    Antecedents *FlowList
}
type FlowList struct {
    Flow *FlowNode
    Next *FlowList
}
```

Use `SymbolTable.Lookup(name)` for source names: keys are case-insensitive and
ignore declaration type characters. `Symbol.Name` preserves the declaration's
spelling, except numeric labels use canonical names without leading zeros.
The anonymous source-file symbol has `SymbolFlagsValueModule` and name `""`.
Functions and Subs use `SymbolFlagsFunction`. Property declarations share an
accessor symbol with Get/Let/Set flags rather than that function flag. Variables, parameters,
constants, implicit locals, and function results use
`SymbolFlagsFunctionScopedVariable`; this flag is also used for module variables.
Use `ast.IsConstantSymbol` to identify constants. Labels use `SymbolFlagsLabel`
and live in a procedure's separate `Labels` table.

Ordinary symbol names use canonical keys without declaration type characters.
Ambient library declarations can also have exact dollar-form keys: Mid and Mid$
are distinct declarations with Variant and String return types. Lookup checks
those keys before the ordinary canonical fallback, preserving user shadowing.

Explicit declarations and their name nodes link to their symbols.
`ValueDeclaration` points to the declaration node; for an implicit local it points
to the first identifier use, and for a function result to the function declaration.
`Symbol.Parent` is the containing project, module, or procedure symbol. For class,
document, and form modules, `Members` aliases the file's `Locals`. Named standard
modules use `SymbolFlagsValueModule`, classes use `SymbolFlagsClass`, and the
project's `Exports` contains module symbols. `ClassifiableNames` contains normalized value-name keys;
`SymbolCount` counts allocated symbols, including results and labels, rather
than just the entries in `file.Locals`.

Property symbols retain each accessor in `Declarations`, using the getter as
`ValueDeclaration` when available, otherwise the first setter. All accessor name
nodes and declaration headers point to that merged symbol. Each declaration's
Locals and Labels are independent even though their symbols have the same parent
property symbol. A getter's `ResultSymbol` is separate from the property symbol,
and its `ValueDeclaration` is that getter node. `ast.PropertyAccessor(symbol, kind)`
finds a specific accessor. `SymbolFlagsAccessor` combines the Get/Let/Set bits;
Get and Set retain TypeScript's bit positions, while Let uses a VBA-specific bit.

Resolved unqualified property identifiers point to this symbol. Indexed property
applications also expose it on `CallExpression.Symbol`. Assignment kind selects
the Let or Set accessor without adding synthetic call syntax; flow records the
setter invocation followed by the write. Bare reads add a getter call flow, while
the getter's bare self-name denotes its result variable. Explicit receiver member
resolution is supported for namespaces and declared local class receivers by a
Program; dynamic receivers and full signature/type compatibility remain deferred.

Module and procedure declarations are collected before references are resolved.
Blocks share their procedure's scope. `Option Explicit` leaves unresolved value
references with nil `Symbol` and a diagnostic; otherwise unresolved value names
in a procedure become implicit locals. Project binding links local class type
names without using the value namespace; unknown library types stay unresolved. These
rules and export visibility are described in the [binder documentation](../binder/README.md).

Flow is a separate graph with links pointing backward to predecessor flows.
Ordinary flows use `Antecedent`; junctions (`FlowLabel`, an alias for `FlowNode`)
use the linked list `Antecedents`. Its `Node` is associated syntax, not a child.
Jumps can create graph cycles, so graph consumers need a visited set.

The binder currently creates start, assignment, call, branch/loop-label, case-test,
true/false condition, and unreachable flows. Referenced/shared flags are bookkeeping.
Other flags in [flow.go](flow.go) are scaffolding for future constructs. An
assignment statement's `FlowNode` is its incoming flow; later syntax and
`EndFlowNode` expose flow after the write. A nil `FlowNode` does not mean
unreachable: type annotations and compile-time expressions have no executable flow.
VBA logical operands both evaluate; TypeScript short-circuit flow is not applied.

Call flows associate their `Node` with the application, after binding its target
and argument values. Declared array indexing does not introduce a call flow;
unknown/default-member applications conservatively can. Member names on known
project/class receivers resolve during project binding; dynamic members and
argument-label names remain unresolved in the AST. The checker resolves named
labels and exposes [call and storage plans](../checker/CALLS.md) in its own tables.
Explicit recursive
function calls use the procedure symbol, while a bare function name in its body
continues to refer to the result variable.

## Type and Enum declarations

`KindTypeDeclaration` and `KindEnumDeclaration` retain `Name`, `Modifiers`, and a
source-ordered `Members` list. The `EnumDeclaration` payload shares this shape with
`TypeDeclaration`; their distinct kinds preserve the different semantics.

Record members use `KindFieldDeclaration` / `AsFieldDeclaration()`, sharing the
variable-declaration payload: `Name`, `Type`, and optional `Dimensions`, with no
initializer. This retains nested and qualified type references, dynamic/fixed
arrays, and fixed-length strings. Fields are symbols in the record's `Members`
table, rather than variables in the module's `Locals`.

`KindEnumMember` retains `Name` and an optional `Initializer`. Omitted initializers
stay nil; parsing does not synthesize zero or previous-plus-one expressions.
`Checker.GetEnumMemberValue(symbol)` supplies the checked Long value separately.
Enum member symbols have their enum as parent, appear in its `Members` table, and
are also visible as constants in the enclosing module, subject to visibility and
ambiguity checks. `ast.IsConstantSymbol` includes enum members.

`DeclarationName` and `ForEachChild` cover all four kinds. Neither computed enum
values nor checked record layouts become syntax children. Numeric literal text
now retains explicit type suffixes, such as `&HFFFF&` and `1#`.

## ReDim statements

`KindReDimStatement` retains an optional `PreserveKeyword` and a source-ordered
`Declarations` list. Each `KindReDimDeclaration` contains the target `Expression`,
`Dimensions` (ordinary `ArrayDimension` nodes), and optional `Type` annotation.
The final parentheses contain allocation bounds and are not a `CallExpression`.
Receiver calls/indexes still use the usual expression nodes, including named
arguments. Omitted lower bounds stay nil. Declaration targets can retain type
characters; an omitted receiver retains its lexical `WithTarget` after binding.

For a declarative ReDim, the array symbol's `ValueDeclaration` is the original
ReDim declaration; no synthetic Dim is inserted. Existing array references retain
their earlier declaration. Checker allocation plans are available separately via
`GetReDimResolution`, without changing syntax or the variable's scope type.

`KindEraseStatement` / `EraseStatement` retains a source-ordered `Expressions`
list of array designators. Each designator remains an ordinary expression node;
its storage and reset/deallocation behavior are resolved after parsing.

`KindOptionBaseStatement` / `OptionBaseStatement` retains `Base` (0 or 1). Binding
copies this module option to `SourceFile.OptionBase`. Omitted lower bounds stay
nil in `ArrayDimension`; `Checker.GetArrayBounds(declaration)` provides checked
constant bounds for fixed declarations separately from syntax. ReDim bounds
remain executable expressions in its checker plan.

## Ownership, equality, and exporting

Treat parsed syntax as immutable. `binder.BindSourceFile` binds once per file;
concurrent calls wait for the same binding to finish. Read binding metadata after
the call returns, and parse a new tree after edits. `file.IsBound()` reports
completion. Symbols and flows remain valid when the pooled binder is reused.
`compiler.NewProgram` constructs fresh trees and binds the entire project before
returning. Rebuild the program after edits; do not adopt already-bound trees into
a new project. `ast.SourceFileOf(node)` follows bound parent links to the owning
source file; source-file metadata is not an additional syntax child.

Use `NodeFactory` for manual construction. It initializes the private payload link
and usually allocates from typed pools. Do not assemble or copy node structs as
values: the private link refers to the original payload, and source files also
contain synchronization state. Manually built nodes start with undefined ranges;
the caller must position them if source locations are needed.

`Node.DeeplyEquals` compares represented syntax, including declaration modifiers,
constness, identifier type hints, and literal token flags. It ignores locations,
parents, symbols, and flows. Source-file equality compares statements, not original
text, EOF, or diagnostics; node-list equality ignores list ranges. It is not a
full-fidelity source comparison.

The structs are an internal Go representation, not a JSON wire format. Private
payload fields are not serialized by `encoding/json`, text ranges have private
fields, and metadata introduces cycles. To export an AST, select payload fields
by `Kind`, obtain offsets through their accessors, and traverse with `ForEachChild`.
Export symbol/flow references separately with explicit IDs if needed. Do not
assume numeric enum values form a versioned external schema.

When adding syntax, update the payload/factory and equality logic in
[ast.go](ast.go), [declarations.go](declarations.go), [expressions.go](expressions.go),
[statements.go](statements.go), or [accessors.go](accessors.go), kinds in [kind.go](kind.go),
child traversal in [visitor.go](visitor.go), and this reference alongside the parser
and binder changes.

## Comments and annotations

`SourceFile.Comments` preserves lexer-recognized `Comment {Range, Text,
Standalone}` entries. Text excludes the apostrophe or `Rem`; Range includes the
comment marker and excludes the newline. `SourceFile.Annotations` holds
`Annotation {Name, Arguments, Range, Target, Error}` metadata parsed from those
comments. Unknown annotation names are preserved. Argument errors belong to this
metadata and do not make ordinary VBA syntax invalid.

`Target` links to an existing procedure declaration, or to the SourceFile for
module metadata. It is a nonchild link, so normal syntax visitors do not follow it
and `DeeplyEquals` still compares syntax. Inline/body comments have no target.
No source text is rescanned as VBA to find annotations. The independent
[test discovery adapter and runner](../testrunner/README.md) interpret test roles;
annotation discovery itself never executes VBA.

## GoSub statements

`KindGosubStatement` uses the `GotoStatement` label payload, exposed by
`AsGosubStatement()` (and `AsGotoStatement()`), including the procedure-local
`Target *Symbol`. `KindOnGosubStatement` uses `OnGotoStatement`'s expression and
ordered destination list, exposed by `AsOnGosubStatement()`. The distinct kinds
preserve the resumption behavior; they are not ordinary GoTo statements.
`KindGosubReturnStatement` is a childless token statement with no result expression
or static target. Binding conservatively links possible Return flows to GoSub
continuations; native execution matches them with a procedure-local LIFO stack.
See the [GoSub contract](../llvm/GOSUB.md).
