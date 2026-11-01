# VBA projects and modules

`compiler.Program` owns an immutable project snapshot: named source files, module
symbols, a project namespace, and parse/binding diagnostics. It follows
`typescript-go`'s separation of the program from the parser and its per-file symbol
ownership. VBA has no import declarations, and the current binder resolves values
eagerly, so project binding first collects declarations from **all** modules and
then visits initializers and procedure bodies. Input order does not affect forward
resolution. The single-file parser and `binder.BindSourceFile` API still work.

```go
p := compiler.NewProgram(compiler.ProgramOptions{
    ProjectName: "Demo",
    Sources: []compiler.SourceFileInput{
        {FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef result As Long)
    result = Utilities.Answer()
End Sub`},
        {FileName: "Utilities.bas", Text: `Public Function Answer() As Long
    Answer = 42
End Function`},
    },
})
main := p.Module("Main")
answer := p.Module("Utilities").Locals.Lookup("Answer")
_ = main
_ = answer
// p.ParseDiagnostics() and p.BindDiagnostics() are both empty.
```

Import `sihlfall.ch/vambas-vai/compiler` and `sihlfall.ch/vambas-vai/ast` when
specifying module kinds. `SourceFiles()` returns files in input order, while
`SourceFile(filename)` and `Module(name)` provide lookup. Paths are cleaned,
case-insensitive, and accept Windows separators; filenames retain their spelling.
Neither method reads the filesystem. `Symbol()` returns the project symbol, whose
`Exports` table contains module symbols. The project name defaults to `Project`.

## Module identity and syntax

Each `SourceFileInput` supplies code in `Text`, with optional `FileName`,
`ModuleName`, and `ModuleKind`. The module name comes from the explicit input,
then `Attribute VB_Name`, then the filename without its extension. Omitted
filenames become `Module1.bas`, `Module2.bas`, etc. Module names preserve spelling
and use case-insensitive lookup. Duplicate module names and filenames are diagnosed;
lookup retains the first entry for recovery. The supported ASCII module identifier
must start with a letter, contain only letters/digits/underscores, and be at most
31 characters. A module cannot share its project's name.

`ast.ModuleKindStandard`, `ModuleKindClass`, `ModuleKindDocument`, and
`ModuleKindUserForm` describe host components. An unspecified kind infers Class
from `.cls`, UserForm from `.frm`, and Standard otherwise. Supply Document
explicitly: a `.cls` filename cannot distinguish a document from an ordinary class.
Explicit kind always wins over extension.

`Attribute name = value[, value...]` is retained as `AttributeStatement`, including
member attributes within procedures. Metadata values have no executable flow and
do not create variables under `Option Explicit`. Module-level `VB_Name` and
`VB_PredeclaredId` populate source-file metadata. The last occurrence wins, as
specified by [MS-VBAL modules](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/4599fae2-3f41-4e70-968e-2398741f446b).
Other attributes retain their syntax for later consumers. Document and UserForm
components have host-provided default instances; ordinary classes require
`VB_PredeclaredId = True` to be used directly as values.
Named default instances can be read but cannot be Set assignment targets; see
[default-instance semantics](https://learn.microsoft.com/en-us/openspecs/microsoft_general_purpose_programming_languages/ms-vbal/189fb41b-cc3a-4999-a6d2-ba89f72d2870).

`Option Private Module` is retained as a distinct option statement and initializes
`SourceFile.OptionPrivateModule` during binding. Like `Option Explicit`, it must
precede procedures. It does not hide public members from other modules **within**
the current project; exclusion from referenced projects will belong to the future
reference layer. See Microsoft's [scope and visibility rules](https://learn.microsoft.com/en-us/office/vba/language/concepts/getting-started/understanding-scope-and-visibility).

Inputs are VBA code and optional Attribute lines, rather than whole Office files,
`.vbp` manifests, or class/form exports with VERSION/designer preambles. Extract
those host-specific wrappers before constructing a Program. This API does not
load referenced projects, COM type libraries, or built-in host members.

The higher-level `project.LoadConfig` now loads `.vbp` manifests or discovers source
files, and `project.Session` supplies versioned unsaved overlays and immutable
snapshots for editor queries. That import layer masks exported class/form designer
preambles while preserving source offsets before constructing a Program. See the
[language server and workspace boundary](../lsp/README.md).

## Name resolution and symbols

Lookup proceeds through procedure locals, the current module's declarations, and
public standard-module declarations in the project, then module/project names.
Module/procedure declarations are visible before their textual definitions.
Local values shadow module names, including when their type is `Object` or
`Variant`; `local.Member` does not accidentally resolve a similarly named module.
Unresolved values retain the existing `Option Explicit` behavior. Two exported
standard-module declarations of the same name cause an ambiguity diagnostic only
where unqualified lookup needs that name; they remain usable through qualification.
Ambiguity never introduces an implicit local.

`Utilities.Answer`, `Demo.Utilities.Answer`, and unambiguous `Demo.Answer` resolve
namespace members. A module can qualify its own private declarations; other modules
can access its public declarations. Public/Friend procedures and accessors are
project-visible; module Dim/unmodified Const and explicitly Private declarations
remain private. A merged property's visibility is checked on the selected
Get/Let/Set declaration, rather than granting access to all operations because
one accessor is public. Qualified constant references work in initializers, array
bounds, fixed-string lengths, and Optional defaults without introducing call flow.

Standard-module symbols use `SymbolFlagsValueModule`. Class/document/form symbols
use `SymbolFlagsClass`, at the same bit position as typescript-go; `Members` aliases
the class module's `Locals`. Every module's `Exports` contains its public surface
within this project. Module symbols have the project symbol as their `Parent`;
member symbols have their module as parent. The project symbol is not included in
any individual file's `SymbolCount`.

`Program.GetTypeChecker()` adds a checker-owned semantic view without changing
these bound trees. It provides symbol/type/signature queries and scalar typing and
conversion diagnostics. `Program.GetSemanticDiagnostics(file)` returns semantic
diagnostics for one file, or all files when nil. See the
[checker API and supported subset](../checker/README.md).

For compilation, `lowering.Lower(program)` consumes that checked snapshot and
returns verified, LLVM-independent semantic IR or source diagnostics. It currently
supports scalar procedures and rejects unsupported or recovered source throughout
the input program. See the [IR contract and compiler subset](../ir/README.md).

Class members do not enter the project's unqualified value scope. `Me` resolves to
the current class/host module symbol and is diagnosed in standard modules. Local
class types resolve on annotations and `New`, including `Demo.Counter`; these use
type lookup independently of similarly named local values. Known class receivers
resolve members through explicit variable/parameter annotations, declared function
or getter results, New, default instances, and With receivers. Member access sets
both `PropertyAccessExpression.Symbol` and its name node's symbol. Calls, getters,
setters, arrays, constants, and AddressOf retain their existing evaluation/flow
distinctions across module boundaries.

This is declaration-based lookup, not a complete type checker. Unknown external
type names stay unresolved without becoming implicit variables. Dynamic receivers,
bang/default-member selection, assignment-based type inference, argument matching,
accessor signature compatibility, and instantiability checks remain deferred in
binding. The checker now supplies [argument/default-member/storage plans](../checker/CALLS.md)
for local declarations while preserving the syntax and these binding links.

## Ownership and diagnostics

`NewProgram` finishes binding before returning. Programs built concurrently own
distinct trees and symbols, even for identical inputs. Treat trees, symbol tables,
and diagnostics as read-only afterward; build a new Program after edits. Calling
`BindSourceFile` on an already completed program file remains an idempotent no-op.
An already-bound standalone tree cannot be adopted into a project: its implicit
locals and diagnostics were computed in a different scope.

The lower-level `binder.BindProject(name, files)` accepts distinct, fresh trees
whose module metadata the caller has supplied. It rejects nil, repeated, and
already-bound files before modifying any of them. It requires exclusive ownership
until completion; `NewProgram` supplies this lifecycle automatically.

Parse diagnostics already carry their source file. Binding diagnostics now carry
`File` and optional `RelatedFile` as well as nodes and spans, so a duplicate or
private-access error can point into another module correctly. Aggregation preserves
input-file order and each file's diagnostic order. Recovery still returns usable,
bound trees even when a module contains errors.
