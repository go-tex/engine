#!/bin/sh
# Build the browser wasm module + copy the Go runtime shim next to the demo.
set -e
cd "$(dirname "$0")/../.."
GOOS=js GOARCH=wasm go build -o examples/wasm/gotex.wasm ./cmd/gotex-wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" examples/wasm/wasm_exec.js
echo "built examples/wasm/gotex.wasm ($(du -h examples/wasm/gotex.wasm | cut -f1))"
