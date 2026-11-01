# VBA in upstream Compiler Explorer

This runs the actual [Compiler Explorer application](https://github.com/compiler-explorer/compiler-explorer)
locally, with this repository's compiler and selectable outputs:

- **VBA LLVM (local)** — our LLVM output; the Go backend constructs scalar SSA values and phi nodes itself.
- **VBA native assembly (Clang …)** — that LLVM output compiled to native assembly by Clang (when installed).

The source editor uses upstream's Visual Basic highlighting; the output uses its
LLVM IR viewer. Source ranges connect emitted statements to editor lines, and
compiler diagnostics use editor line/column positions, including Unicode source.
This setup does not add the compiler to the public godbolt.org service.

## Start

Install Go 1.23.1+, Node.js 22.22.1+ with npm, Git, and Bash. From the repository root:

```sh
tools/compiler-explorer/start.sh
```

Open **http://127.0.0.1:10240**. The first launch downloads the pinned upstream
revision and installs its locked npm dependencies. It then starts upstream's
development server; allow a few seconds for its initial frontend build. LLVM and
Clang are not needed to use the LLVM view.

Edit the default loop example to compile automatically. Use **Add new… → Compiler**
in the editor to compare compiler choices side by side. Compiler options can
include `-entry=example.Main` for a parameterless procedure in the editor's
default `example.bas` module. A native entry wrapper is
optional; procedures are emitted without one by default. Binary execution is not
enabled.

## Observe native assembly

Install **Clang 20 or newer**, then restart the explorer. Setup prefers `clang-20`
and falls back to `clang`. To select another installation explicitly:

```sh
CLANG=/path/to/clang tools/compiler-explorer/start.sh
```

Add a compiler pane and select **VBA native assembly (Clang …)** to inspect:

```text
VBA → our typed lowering and direct SSA → LLVM IR → Clang → native assembly
```

The default Clang optimization level is **`-O2`**. Enter **`-O0`**, `-O1`, `-O3`,
`-Og`, `-Os`, or `-Oz` in that pane's compiler options to compare levels. On x86-64,
Compiler Explorer's Intel syntax filter switches between Intel and AT&T syntax.
The resulting instructions link back to VBA source lines where Clang retains
their locations; optimized-away statements may have no corresponding instructions.

This is an assembly-only Clang compilation, so runtime libraries do not have to
be linked. Calls to external runtime helpers remain visible in assembly.
For inspection, the adapter adds LLVM retention metadata for private VBA
procedures so Clang does not remove them just because no native entry point
references them. It also translates existing region provenance into line-table
metadata. These additions are confined to Clang's temporary input; the LLVM
pane shows the original compiler output. They do not introduce a public VBA ABI.

Compiler Explorer's additional `.bas`/`.cls` files are compiled together through
the existing project loader. Source links and diagnostics retain each module's
filename, so another module's lines are not attributed to the main editor.

Press Ctrl+C to stop. To use another port:

```sh
PORT=10241 tools/compiler-explorer/start.sh
```

Each start rebuilds `vambas-compile`, so restart after changing the Go compiler.
Compilation caching is disabled so previous output cannot mask those changes.
The instance binds to loopback and accepts only the documented compiler options.
The source directory and output path are selected by the adapter for each request.

## Files and maintenance

All downloaded/build artifacts live in ignored `.work/`, including the upstream
checkout, npm dependencies/cache, and the local compiler binary. `setup.sh` can be
run separately to prepare the checkout without starting a server. It is safe to
rerun and only reinstalls dependencies when their lockfile changes.

- `upstream-revision` pins the tested upstream commit.
- `vba.ts` is copied to upstream's `lib/compilers/vba.ts`.
- `vba-native.ts` runs Clang after our compiler; `native-ir.ts` prepares inspection metadata.
- `configure.mjs` registers the language and compiler and generates local properties.
- `default.bas` is the initial source example.
- `smoke-test.mjs` exercises the real upstream compilation API.

Update the revision deliberately: stop the server, change `upstream-revision`,
move `.work/upstream` aside, and rerun setup. Registration checks fail if the
upstream integration points have changed. Edits to generated local configuration
and the copied adapter are overwritten by setup; edit the files here instead.

The adapter follows upstream's
[language integration guide](https://github.com/compiler-explorer/compiler-explorer/blob/main/docs/AddingALanguage.md).
The existing compiler frontend, checker, semantic lowering, and SSA implementation
are used directly through the built `vambas-compile` executable.

## Verify

With the server running:

```sh
node tools/compiler-explorer/smoke-test.mjs
```

This checks compiler discovery, SSA loop phis without stack slots, source links
with comments shown/hidden, option validation, Unicode diagnostics, and
additional-module compilation/diagnostics, plus rejection of compiler
input-directory overrides.
When Clang is configured, it also checks native assembly, optimization levels,
retained procedures, native source links, syntax selection, and frontend errors.

Verify Clang O0/O2 compilation and source metadata for every example project:

```sh
node --import ./tools/compiler-explorer/.work/upstream/node_modules/tsx/dist/loader.mjs \
    --test tools/compiler-explorer/native-ir.test.ts
```

Check the adapter against upstream's backend types:

```sh
cd tools/compiler-explorer/.work/upstream
npm run ts-check:backend
npm run ts-check:frontend
```
