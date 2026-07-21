#!/usr/bin/env bash
# Builds the SlideLang/DocLang playground's WebAssembly artifact (issue
# #134) and refreshes its matching wasm_exec.js shim.
#
# Usage: ./playground/build.sh   (run from the repo root or from playground/)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_DIR="$REPO_ROOT/slidelang"

echo "Building $SCRIPT_DIR/slidelang.wasm ..."
(cd "$CLI_DIR" && GOOS=js GOARCH=wasm go build -o "$SCRIPT_DIR/slidelang.wasm" ./cmd/wasm)

GOROOT="$(go env GOROOT)"
WASM_EXEC="$GOROOT/lib/wasm/wasm_exec.js"
if [ ! -f "$WASM_EXEC" ]; then
  # Older Go versions ship it one directory up (misc/wasm instead of lib/wasm).
  WASM_EXEC="$GOROOT/misc/wasm/wasm_exec.js"
fi
cp "$WASM_EXEC" "$SCRIPT_DIR/wasm_exec.js"

ls -lh "$SCRIPT_DIR/slidelang.wasm"
echo "Done. Serve $SCRIPT_DIR/ with any static file server that sets the"
echo "application/wasm MIME type, e.g.:"
echo "  cd $SCRIPT_DIR && python3 -m http.server 8080"
