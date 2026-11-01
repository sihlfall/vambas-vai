# VS Code development client

This is a thin stdio client for the Go server, using Microsoft's
`vscode-languageclient`, as does `typescript-go`. No JavaScript build is required.

1. Build the server from the repository root:
   `go build -o bin/vambas-language-server ./cmd/vambas-language-server`
   (use an `.exe` output name on Windows).
2. Run `npm ci` in this directory.
3. Open this directory in VS Code and press F5, or run
   `code --extensionDevelopmentPath=/absolute/path/to/extensions/vscode /path/to/project`.
4. In the development window, set `vba.server.path` to the absolute server
   executable path, then run **VBA: Restart Language Server**. Open a `.bas`,
   `.cls`, or `.frm` file in **VBA** language mode.

Open one workspace folder. A sole `.vbp` at its root is selected automatically;
set `vba.projectFile` to select a different manifest. With no manifest, the server
infers a project from source files. Restart after changing server settings.
The client forwards disk changes and unsaved edits. Diagnostics appear in Problems;
hover, Go to Definition, the Outline, UDT member completion, and semantic highlighting
use the shared checker.

## UDT member completion

Typing `.` after a UDT value opens field proposals with their types. For example,
given `Dim point As Shapes.Point`, typing `point.` offers that type's fields.
**Ctrl+Space** also requests proposals after a partially typed member. VS Code
filters the full field list as you type, using its normal case-insensitive matching.
Accepting a proposal replaces the existing member name, including text after the
cursor, and preserves the declaration's spelling and any required brackets.

Completion works with nested fields (`state.Position.`), UDT array elements
(`points(i).`), UDT-returning calls (`MakePoint().`), and implicit receivers inside
`With` blocks (`With point` followed by `.`). Nested With blocks and explicit line
continuations use the parser/binder's existing syntax. Field details include array
rank, for example `Values(,) As Long`. Incomplete source and unsaved edits work too.

This first completion provider handles UDT value fields. General local/global,
module, enum, class/COM, and late-bound Object/Variant proposals remain future work.
The server sends no proposals in comments, strings, or non-member contexts.

After rebuilding `bin/vambas-language-server`, run **VBA: Restart Language Server**
in the development window. No new JavaScript provider or editor setting is needed;
the existing language client registers the server's completion capability.

## Syntax and semantic highlighting

The extension contributes an independently authored TextMate grammar for VBA/VB6.
Keywords, declarations, strings/doubled quotes, apostrophe/Rem comments,
annotations, numbers, dates, labels, type suffixes, operators, continuation markers,
and conditional-compilation directives receive ordinary theme scopes. This works
as soon as the document is in **VBA** language mode, including while the server is
starting or unavailable. Directive coloring does not evaluate conditional branches.

The Go server adds semantic tokens for resolved modules, classes, UDTs, enums,
enum members, procedures/methods, properties/fields, parameters, variables, and
labels. Declaration, readonly, and default-library modifiers distinguish constants
and built-ins. Shadowing uses actual symbols, so a local named Len remains a local.
Named arguments refer to their formal parameters; Function result variables remain
variables. Unresolved references retain the grammar's lexical coloring.

Colors come from the active theme; no palette is imposed. For semantic colors,
use a theme that enables semantic highlighting, or set:

```json
"[vba]": {
    "editor.semanticHighlighting.enabled": true
}
```

After updating this checkout, rebuild the Go server and run `npm ci` in this
directory. Relaunch the F5 development host, or run **Developer: Reload Window**
in that host so VS Code reloads the grammar contribution and starts the new server.
**VBA: Restart Language Server** alone updates server behavior, not grammar files.
Use **Developer: Inspect Editor Tokens and Scopes** on a name to inspect the
TextMate scopes, semantic type/modifiers, and theme rule that supplies its color.

`npm test` runs client tests and real TextMate/Oniguruma tokenization tests. Those
two packages are development dependencies. The server smoke test below also checks
semantic-token negotiation, full/range requests, positions after emoji, and unsaved
edits, as well as UDT completion and completion edits. Interactive theme rendering
and the suggestion popup still need a VS Code development-host check.

Run `npm run test:server -- /absolute/path/to/vambas-language-server` for an executable
smoke test through Microsoft's protocol client, without starting the editor. This
checks framed messages, diagnostics, UTF-16 positions, named-argument
definitions, document symbols, semantic tokens, and shutdown. Interactive VS Code UI behavior still
needs a development-host check.

The executable is supplied separately and must be trusted. The extension does not
activate in untrusted workspaces. See the
[server architecture and limitations](../../lsp/README.md).

## Legacy source encodings

The default `vba.sourceEncoding: "auto"` accepts UTF-8, BOM-marked UTF-16 LE/BE,
and falls back to Windows-1252 for non-UTF-8 exports and `.vbp` manifests. Files on
disk are never converted or rewritten. To force Windows-1252 for BOM-less files
(including byte sequences that also happen to be valid UTF-8), set
`vba.sourceEncoding` to `"windows1252"`. `"utf8"` requires valid UTF-8 for BOM-less
files. Other legacy code pages are not supported yet.

VS Code controls decoding of **open** documents, and the server uses the Unicode
text sent by the editor. If an ANSI file displays replacement characters or
garbled text, click the encoding in the bottom status bar and choose **Reopen with
Encoding → Western (Windows 1252)**. This reopens the file without converting it.
The server setting does not change VS Code's file encoding.

After an initialization error, the client keeps the restart command available and
does not automatically retry the same failed initialization. Correct the settings
and run **VBA: Restart Language Server**.
