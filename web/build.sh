#!/bin/sh
# Builds the demo page into web/site: compiles the qrgen library and the page's
# bridge to WebAssembly (web/site/qrgen.wasm) and copies the wasm_exec.js glue
# that matches the Go toolchain doing the build. Both outputs are generated, so
# they are not committed. Run from anywhere: sh web/build.sh
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
site="$root/web/site"

cd "$root"
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$site/qrgen.wasm" ./web/wasm

goroot=$(go env GOROOT)
for glue in "$goroot/lib/wasm/wasm_exec.js" "$goroot/misc/wasm/wasm_exec.js"; do
	if [ -f "$glue" ]; then
		cp "$glue" "$site/wasm_exec.js"
		echo "built $site/qrgen.wasm and $site/wasm_exec.js"
		exit 0
	fi
done
echo "wasm_exec.js not found under $goroot" >&2
exit 1
