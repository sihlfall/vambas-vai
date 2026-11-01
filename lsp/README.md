# Language server milestone

Build and run the stdio server:

```sh
go build -o bin/vambas-language-server ./cmd/vambas-language-server
./bin/vambas-language-server
```

The process reads LSP Content-Length frames from stdin and writes only framed
JSON-RPC to stdout. Errors go to stderr. It is intended to be launched by an LSP
client, rather than used interactively. A small [VS Code development
client](../extensions/vscode/README.md) is included.

## Architecture

The boundaries follow the corresponding `typescript-go/internal` packages:

| Package | Responsibility |
| --- | --- |
| `project` | Workspace configuration, versioned overlays, immutable snapshots |
| `compiler` | Fresh parsing and project binding; lazy, read-only checker |
| `ls` | Diagnostics, hover, definition, document symbols, semantic tokens, UDT completions |
| `ls/lsconv` | UTF-8 byte offsets to/from LSP UTF-16 positions and file URIs |
| `lsp/lsproto` | Shared protocol data types for the supported subset |
| `lsp` | Lifecycle, ordered changes, cancellation, request dispatch, publication |
| `cmd/vambas-language-server` | Stdio executable |

`Session` publishes a new `Snapshot` after an overlay/configuration change. Requests
capture it before subsequent notifications are processed. Each snapshot lazily
constructs its own `compiler.Program`; bound ASTs and symbols are never reused
across programs. Go's garbage collector retains snapshots for outstanding requests,
so explicit reference counting from the much larger TypeScript server is unnecessary.
The editor-independent language service can be used directly:

```go
config, err := project.LoadConfig(ctx, workspaceDirectory, "Demo.vbp")
if err != nil { return err }
session := project.NewSession(config)
service := ls.NewLanguageService(session.Snapshot())
diagnostics, err := service.GetDiagnostics(ctx, documentURI)
```

`ls.Host` abstracts the program, original file text, and line maps. `Snapshot`
implements it; language services have no dependency on the `project` package.

Changes on the reader are ordered; semantic queries use worker goroutines.
Diagnostics are computed for all open documents after any edit, including effects
on callers in other modules. Publication checks snapshot identity while serialized
with edits, so superseded jobs cannot publish after a newer change was applied.
Closing a document clears its pushed diagnostics and restores the disk view.
Invalid/stale edits are rejected atomically and logged through `window/logMessage`.

## Protocol surface

The server implements LSP 3.17's `initialize`, `initialized`, `shutdown`, `exit`,
`$/cancelRequest`, document open/change/save/close notifications, and
`workspace/didChangeWatchedFiles`. Supported requests:

- `textDocument/hover`: checker-backed VBA signatures and symbol types, plaintext.
- `textDocument/definition`: cross-file declarations, selected property accessors,
  class types, parameters (including named-argument labels), and GoTo labels.
- `textDocument/documentSymbol`: procedures, separate Get/Let/Set entries, constants,
  variables, parameters, records/fields, enums/members, and labels. Hierarchical results are negotiated; clients
  without that capability receive flat `SymbolInformation` results.

Hover and definitions also resolve UDT/enum annotations, qualified record fields,
and enum constants. Enum-member hovers include their computed Long value when
available; the Outline nests fields and enum members under their declarations.
ReDim target references and runtime bounds use the same hover/definition queries.
A ReDim that introduces an array appears once as a local variable in the Outline;
subsequent resize statements do not duplicate that entry. The ambient `vbObjectError`
constant supports hover and enum evaluation, but has no declaration-file location
for Go to Definition yet.

`textDocument/publishDiagnostics` combines parse, binding, and checker diagnostics,
including related declaration locations, and includes the open document version.
Text synchronization accepts whole documents and sequential incremental changes;
the advertised position encoding is UTF-16. CRLF/LF/CR, astral Unicode, escaped file
URIs, and end-of-line clamping are handled at the conversion boundary. Edits that
split a surrogate pair are rejected. Messages are limited to 16 MiB.
The protocol behavior follows the [LSP 3.17 specification](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/).

## Highlighting

The [VS Code extension](../extensions/vscode/README.md#syntax-and-semantic-highlighting)
provides a TextMate grammar independently of the server. Semantic tokens refine
that lexical coloring using resolved symbols from the captured Program/checker.
Following `typescript-go`, the language service traverses name nodes, classifies
symbols, sorts their source ranges, and encodes document-relative UTF-16 positions
as LSP relative five-integer tuples. It does not rebind names or mutate the checker.

`textDocument/semanticTokens/full` and `/range` support standard types: namespace,
class, enum, struct (VBA UDT), parameter, variable, property, enumMember, function,
method, and label. Modifiers are declaration, readonly, and defaultLibrary.
Read-only getters and constants retain readonly; class fields are properties;
Function result references are variables. Named arguments use their formal's
symbol. Comments, strings, export attributes, unresolved references, and syntax
keywords are left to the grammar. Unsupported client types/modifiers are filtered
from the initialization legend while preserving stable server ordering.

Ranges select intersecting whole name tokens, with positions relative to the
document rather than the requested range. Output is ordered, single-line, and
nonoverlapping; empty data serializes as `[]`. Requests use the same immutable
snapshots, cancellation, URI handling, and unsaved overlays as other editor queries.
Malformed source still receives tokens for names that can be resolved. Tests cover
UTF-16 after astral characters, CRLF, labels, shadowing, cross-file changes, and
parallel queries. The executable smoke test uses Microsoft's protocol client.

The server advertises complete full/range results, without delta caching, dynamic
registration, or project-wide semantic-token refresh requests. Edits are reflected
on the next token request; changing another file does not proactively refresh all
open editors yet. Colors and semantic-highlighting enablement remain theme/editor
settings. Designer preambles are handled lexically; conditional directives are
colored without adding conditional-compilation evaluation to the frontend.

## UDT member completion

The server advertises `completionProvider` with `.` as its trigger character and
handles `textDocument/completion`. `ls.ProvideCompletion` finds the recovered
property-access node at the cursor, resolves its explicit or binder-linked With
receiver, and asks the checker for its type. For a UDT value, it enumerates the
nominal type's member symbols. It does not split receiver text or rebuild types
in JavaScript. Nested UDTs, indexed UDT arrays, UDT-returning calls, parenthesized
expressions, and nested With receivers use existing checked expression types.

Results are `CompletionList` objects with Field items, type details, stable sorting,
filter text, and UTF-16 text edits. Edits replace the full existing member name;
escaped declarations and partially entered brackets produce valid bracketed names.
Completion edits stay on one line; for a continued access, invoke completion on
the line containing its member name.
All fields are returned with `isIncomplete: false`, allowing the client to filter
and cache the complete list correctly as a prefix grows or shrinks. Comments,
strings, attributes, bang access, unrelated newlines, and non-UDT receivers produce
an empty array. Type names themselves do not expose instance fields.

Queries use captured snapshots, including unsaved cross-file changes, and check
cancellation during context lookup and item creation. Tests cover EOF/recovery,
array ranks, source spelling, edit ranges after emoji, CRLF/continuations, and
parallel queries. The executable smoke test checks trigger registration and field
insertion/replacement through Microsoft's protocol client. No completion resolve,
snippets, import edits, or general scope/class/module completion is advertised.

## VBA project loading

`initialize.rootUri` or the single `workspaceFolders` entry supplies the source
root. Optional `initializationOptions.projectFile` selects a `.vbp`, with an absolute
path or a path relative to that root. Otherwise a sole root-level `.vbp` is used;
multiple manifests require an explicit selection. Without a manifest, recursive
`.bas`, `.cls`, and `.frm` discovery excludes hidden folders, `node_modules`, `dist`,
and `build`. Opening additional local source documents adds them to this one project.

`initializationOptions.sourceEncoding` (VS Code's `vba.sourceEncoding`) defaults to
`"auto"`: Unicode BOMs select UTF-8/UTF-16 LE/BE, valid BOM-less UTF-8 is preferred,
and other BOM-less input uses Windows-1252. `"windows1252"` forces that code page
for BOM-less files; `"utf8"` rejects invalid BOM-less UTF-8. The selection applies
to source modules and manifests, and survives disk reloads. Automatic fallback is
a policy for common Western exports, not detection of arbitrary ANSI code pages.
Undefined Windows-1252 bytes become U+FFFD; other legacy code pages are not yet
supported. No source file is rewritten.

The manifest loader reads `Name`, `Module`, `Class`, and `Form` entries; Windows
path separators are accepted on all hosts. `Reference` and `Object` entries are
retained as metadata, with no library loading or invented signatures. Disk saves
and file-watch notifications reload configuration while retaining unsaved overlays.
An unreadable project leaves the previous disk view intact and logs the error.

The compiler continues to accept code-only UTF-8 inputs. Like `typescript-go`'s
filesystem reader, the project import boundary decodes disk text and strips BOMs
before parsing, matching editor document text. Class/form VERSION/designer
preambles are then masked with spaces, keeping byte lengths and newlines in that
decoded text. AST offsets refer to decoded UTF-8, not raw bytes in an ANSI/UTF-16
file. The decoded document supplies LSP coordinates. Designer controls are not yet
synthesized as members. Open-document overlays are already Unicode from the client
and are never decoded a second time; choose the matching encoding in the editor.
Office containers, `.frx` resources, and COM type libraries are not imported.

## Deliberate first-milestone limits

One workspace project per server; local file URIs only. The VS Code client watches
files in its workspace; external manifest sources reload on save/explicit watcher
notifications. General scope/class/module completion, rename, references, signature help, and external
library loading remain subsequent milestones. Diagnostics
reflect the [current front-end subset](../checker/README.md), so unsupported VBA or
host/library members can still produce errors.

Cancellation is checked before/after program construction and between query stages.
Parser/binder/checker passes are not yet preemptible. Superseded diagnostics are
suppressed immediately at publication, but a running front-end pass must finish.
There is no syntax reuse, incremental binding, filesystem watcher inside the Go
server, or compiled-object cache. These can be added behind the existing boundaries.

Failed initialization still permits `shutdown`/`exit` so clients can clean up.
The VS Code client stops automatic restart attempts after initialization failure
and leaves its manual restart command registered. Its normal crash restart policy
is preserved after successful startup.

Tests cover snapshot ownership, concurrent edits/queries, Unicode edits, exported
sources, project loading, checker-backed editor features, JSON-RPC framing,
lifecycle, cross-file diagnostic invalidation, and cancellation. Run
`go test -race ./...` and `go vet ./...` from the repository root.
