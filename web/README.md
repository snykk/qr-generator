# qrgen web demo

A static page that runs the qrgen library in the browser: the Go code is compiled to WebAssembly and runs in a Web Worker, so nothing is sent to a server. It has two views.

- **Create**: encode text, a URL, or a structured payload (Wi-Fi, vCard, email, phone, SMS, location) as a QR Code, Micro QR Code, or rMQR Code, with every option of the library: error-correction level, version, mask, ECI, quiet zone, module size, colours, a logo, and structured append. Download the symbol as PNG or SVG, or copy its text rendering. The "Inside the symbol" tab takes the symbol apart with `qrgen.Inspect`: a module map coloured by role, with the symbol before the mask and the mask pattern as extra layers; the segments and the bit budget of the message; the codewords and Reed-Solomon blocks, each codeword linked to its eight modules; and every candidate mask with its score.
- **Scan**: read symbols from image files (picked, dropped, or pasted) with `qrgen.DecodeAll`, drawing each symbol's outline and finder centres, reassemble a structured-append set, or scan with the camera.

## Layout

- `site/`: the page itself, plain HTML, CSS, and JavaScript modules with no build step. `worker.js` loads the WebAssembly module; `client.js` is the page's handle on it.
- `wasm/`: the WebAssembly entry point, which exposes one function, `qrgenCall`, to the worker.
- `internal/bridge/`: the operations behind `qrgenCall` (encode, inspect, masks, payload, structuredAppend, decode, decodePixels, decodeStructuredAppend), JSON in and out, tested with the ordinary Go toolchain.
- `build.sh`: compiles `site/qrgen.wasm` and copies the matching `site/wasm_exec.js` from the Go toolchain. Both are generated and not committed.

## Run it locally

```sh
sh web/build.sh
python3 -m http.server 8080 --directory web/site
```

Then open http://localhost:8080. Any static file server works; the page must be served over HTTP, not opened as a file, and the camera needs `localhost` or HTTPS.

## Deploy

`.github/workflows/pages.yml` builds the page and publishes it to GitHub Pages on every push to `master`, and can be run by hand. Set the repository's Pages source to "GitHub Actions" once, under Settings, Pages.
