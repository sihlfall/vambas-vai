#!/usr/bin/env bash
set -euo pipefail
integration_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
"$integration_dir/setup.sh"
cd "$integration_dir/.work/upstream"
echo "VBA Compiler Explorer: http://127.0.0.1:${PORT:-10240}"
# Compilation caching would otherwise retain results across rebuilds of the
# local compiler. This development instance should always reflect our sources.
exec env NODE_ENV=DEV node --no-warnings=ExperimentalWarning --import=tsx app.ts \
    --host 127.0.0.1 --port "${PORT:-10240}" --language vba --no-cache --no-remote-fetch "$@"
