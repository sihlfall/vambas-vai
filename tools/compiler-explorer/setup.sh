#!/usr/bin/env bash
set -euo pipefail

integration_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(cd -- "$integration_dir/../.." && pwd)"
upstream_dir="$integration_dir/.work/upstream"
upstream_revision="$(cat "$integration_dir/upstream-revision")"

for dependency in git go node npm; do
    command -v "$dependency" >/dev/null || { echo "Required command not found: $dependency" >&2; exit 1; }
done
node -e 'const [major, minor, patch] = process.versions.node.split(".").map(Number); if (major < 22 || (major === 22 && (minor < 22 || (minor === 22 && patch < 1)))) { console.error("Compiler Explorer requires Node.js >=22.22.1"); process.exit(1); }'

if [[ ! -d "$upstream_dir/.git" ]]; then
    mkdir -p "$upstream_dir"
    git -C "$upstream_dir" init -q
    git -C "$upstream_dir" remote add origin https://github.com/compiler-explorer/compiler-explorer.git
fi
# Resume an interrupted first fetch without discarding a completed checkout.
if ! git -C "$upstream_dir" rev-parse --verify HEAD >/dev/null 2>&1; then
    git -C "$upstream_dir" fetch --depth 1 origin "$upstream_revision"
    git -C "$upstream_dir" checkout --detach FETCH_HEAD
fi
if [[ "$(git -C "$upstream_dir" rev-parse HEAD)" != "$upstream_revision" ]]; then
    echo "The generated upstream checkout does not match upstream-revision. Move .work/upstream aside and rerun setup." >&2
    exit 1
fi

mkdir -p "$integration_dir/.work/bin"
echo 'Building the local VBA compiler...'
(cd "$project_dir" && go build -o "$integration_dir/.work/bin/vambas-compile" ./cmd/vambas-compile)

# Clang is optional: the LLVM/SSA views remain usable without it. An explicit
# CLANG selection must resolve to a supported compiler rather than being ignored.
clang_exe=""
clang_version=""
clang_target=""
if [[ -n "${CLANG:-}" ]]; then
    clang_exe="$(command -v "$CLANG")" || { echo "Clang not found: $CLANG" >&2; exit 1; }
else
    clang_exe="$(command -v clang-20 || command -v clang || true)"
fi
if [[ -n "$clang_exe" ]]; then
    clang_version="$("$clang_exe" --version)"
    if [[ "$clang_version" =~ clang\ version\ ([0-9]+) ]] && (( BASH_REMATCH[1] >= 20 )); then
        clang_target="$("$clang_exe" -dumpmachine)"
    else
        if [[ -n "${CLANG:-}" ]]; then
            echo 'Native assembly requires Clang 20 or newer.' >&2
            exit 1
        fi
        clang_exe=""
    fi
fi
if [[ -z "$clang_exe" ]]; then
    echo 'Clang 20+ was not found; native assembly is unavailable. Set CLANG to enable it.'
fi
node "$integration_dir/configure.mjs" "$(git -C "$project_dir" describe --always --dirty)" \
    "$clang_exe" "${clang_version%%$'\n'*}" "$clang_target"

lock_hash="$(git -C "$upstream_dir" hash-object package-lock.json)"
if [[ ! -f "$integration_dir/.work/dependencies" || ! -d "$upstream_dir/node_modules" || "$(cat "$integration_dir/.work/dependencies")" != "$lock_hash" ]]; then
    echo 'Installing upstream Compiler Explorer dependencies...'
    (cd "$upstream_dir" && CYPRESS_INSTALL_BINARY=0 HUSKY=0 npm ci --no-audit --no-fund --cache "$integration_dir/.work/npm-cache")
    printf '%s\n' "$lock_hash" > "$integration_dir/.work/dependencies"
fi
echo 'Ready. Run tools/compiler-explorer/start.sh to open the local explorer.'
