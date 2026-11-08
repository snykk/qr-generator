# QR Generator — Web Demo Plan

This document describes the implementation plan for a **web demo** of the library, targeting the `v0.18.0` minor release: a static page, hosted on GitHub Pages, that creates, takes apart, and scans QR Code, Micro QR Code, and rMQR Code symbols with the library itself, compiled to WebAssembly. The milestone also adds `qrgen.Inspect`, a public API that reports how a symbol was built, layer by layer, since the page's teaching view needs it and so does anyone studying the library.

> Status: **draft / living document.** Milestones WD1..WD6 land incrementally on the `web-demo` branch; each is a focused commit (or small series) with tests, matching the cadence of prior milestones. This milestone was implemented before its documents were written, at the owner's request, and the documents describe what was built.

> Indonesian version: [docs/plan-web-demo.id.md](plan-web-demo.id.md).

---

## 1. Vision & Goals

- Let anyone try the library without installing Go: every encoding option, the payload builders, structured append, and image decoding, in the browser.
- Run the **real library**, not a JavaScript imitation: the Go code compiled to WebAssembly, so the page shows exactly what `Encode`, `Inspect`, and `DecodeAll` do.
- Teach: show what each module of a symbol is for, the message's segments and bit budget, the codewords and blocks, and how the mask is chosen, linking each part to its theory doc.
- Keep the repository's character: no runtime dependencies for the library, no JavaScript toolchain, no build step for the page beyond compiling the Go code.
- Validate the new API against facts that do not come from itself: the worked examples of theory docs 10 and 28, hand-counted layouts, and the existing encoder, which `Inspect` must reproduce exactly.

## 2. What the demo is

- **Create.** Text, a URL, or a structured payload (Wi-Fi, vCard, email, phone, SMS, location) encoded as a QR Code, Micro QR Code, or rMQR Code, with every option the library has: error-correction level, version, rMQR height, mask, ECI, quiet zone, module size, colours, a logo, and structured append. A live preview, PNG and SVG downloads, and the text rendering of `EncodeTerminal`.
- **Inside the symbol.** The same symbol taken apart with `Inspect`: a module map coloured by role, with the symbol before masking and the mask pattern as extra layers, and every data module tied to the codeword bit it carries; the segments and the bit budget; the codewords grouped by Reed-Solomon block; and every candidate mask with its score, any of which can be forced with a click.
- **Scan.** Images picked, dropped, or pasted, read with `DecodeAll` (and `Decode` as a fallback), with each symbol's outline and finder centres drawn over the image; a structured-append set reassembled from several images; and the camera.

## 3. Constraints, and what they decided

- **GitHub Pages serves static files.** No server can run the library, so the browser must: Go's `js/wasm` target compiles the library and a small bridge into one module. The standard toolchain is enough; TinyGo would give a smaller file but adds a second compiler with its own gaps in the standard library.
- **`/docs` is taken.** Pages can publish a branch's root or its `/docs` folder, and `/docs` holds the plans and theory notes. A workflow publishes the page instead (`actions/deploy-pages`), which also builds the WebAssembly module, so no binary is committed.
- **No JavaScript toolchain.** Plain HTML, CSS, and ES modules, with the Go compiler as the only build step.
- **A busy main thread freezes the page.** Go's WebAssembly runtime is single-threaded, and a camera frame or a version 40 symbol takes visible time, so the module runs in a Web Worker and the page talks to it through messages.
- **The teaching view needs internals.** `Matrix` returns only the finished grid. The role of each module, the grid before masking, the stream bit in each data module, and the mask scores live inside the encoder, so they need a public API, decided with the owner before implementation (section 11).

## 4. Approach, grounded in the code

- **Reusable as-is:** every public encoder and decoder, the payload builders, `EncodeStructuredAppend` and `DecodeStructuredAppend`, and the option set, which the page maps one to one.
- **New in the library:** `Inspect`, built on the encoders' own pipeline. Each placement function tags the modules it draws with a role when the matrix is traced, `placeBits` records the stream bit it puts in each data module, and `Inspect` runs `buildMatrix` with tracing on, then rebuilds the message (segments, data codewords, blocks) for the symbol already chosen.
- **Hidden assumptions to avoid:** QR's reserved format area takes in the timing modules (6, 8) and (8, 6), which must stay timing modules; in R7 symbols the bottom-left corner pattern falls on the finder's bottom edge; Micro QR M1 and M3 end their data with a 4-bit codeword, which shifts every later codeword by four bits in the stream; remainder bits carry no codeword; a forced mask skips the encoder's scoring; and the decoder's multi-symbol detector is slow on frames of fine noise (section 9).

## 5. Design decisions

- **`Inspect(text, opts...) (*Inspection, error)`.** It accepts every option `Matrix` does and fails exactly as `Matrix` fails. `Inspection` reports the symbology, version, level (with `DetectionOnly` for M1), the mask and the scores of every candidate (`MaskScores`, plus `MaskPenalties` with the four QR rules), the segments with their field widths, the data, terminator, and capacity bit counts, the Reed-Solomon blocks, the codewords in placement order, and per-module layers: `Modules` (equal to `Matrix`), `Unmasked`, `MaskPattern`, `Roles`, `ModuleCodeword`, and `ModuleBit`.
- **`ModuleRole`.** Data, remainder, finder, separator, timing, alignment, format, version, dark module, finder sub-pattern, and corner pattern; `RoleData` is the zero value, and `IsFunction` tells function modules from message modules.
- **Tracing costs the encoders nothing.** The trace is a pointer that only `Inspect` sets; every encoder builds an untraced matrix, as a test checks.
- **Mask scores are always computed.** Scores for a forced mask are what the encoder would have weighed, so the page can show why the automatic choice differs.
- **A JSON bridge.** `web/internal/bridge` maps eight operations (encode, inspect, masks, payload, structuredAppend, decode, decodePixels, decodeStructuredAppend) from JSON requests and binary blobs to the library and back. `web/wasm` registers one JavaScript function that hands calls to it. The bridge is ordinary Go, tested natively and under WebAssembly.
- **Camera frames.** The page grabs the centre square of each frame, scales it to at most 480 pixels, converts it to grey with the weights of `color.GrayModel`, and sends one frame at a time.
- **Deployment.** `.github/workflows/pages.yml` builds and publishes the page on every push to `master`, which only receives release merges, and on demand.
- **Layout.** Everything for the page lives in `web/`: `site/` (the page), `wasm/` (the entry point), `internal/bridge/`, and `build.sh`.

## 6. Milestones

Milestones land sequentially. **Checkpoint A** (after WD5) is a working page in a browser. **Checkpoint B** (WD6) is the `v0.18.0` release.

### WD1 — Plan Doc `(S)`

- [x] This document and its Indonesian counterpart.

### WD2 — Reference Doc `(S)`

- [x] `docs/theory/29-symbol-inspection.md` (EN + ID): the layers of a symbol and the census of each role, how tracing records roles and stream bits, mapping stream bits to codewords, the grid before masking and the mask scores, the message's bit budget, locating codewords in their blocks, and running the library in a browser. Plus the theory README entry and code-mapping row.

### WD3 — Inspect `(M)`

- [x] `ModuleRole`, `Inspection`, `InspectSegment`, `InspectBlock`, and `Inspect`, with tracing in the QR, Micro QR, and rMQR placement code.
- [x] Tests: `Modules` equals `Matrix` across symbologies and options; every data module carries the bit of `Codewords` it claims; hand-counted role census for QR versions 1 and 7, Micro M1, and rMQR R7x43 and R11x27; the worked examples of docs 10 and 28; mask scores that agree with the encoder's choice; field widths; M1's half codeword; and errors identical to `Matrix`'s.

### WD4 — WebAssembly Bridge `(M)`

- [x] `web/internal/bridge` with the eight operations, `web/wasm`, and `web/build.sh`.
- [x] Tests: each setting reaches the library, codeword locations match the blocks, mask variants match `Matrix`, payloads match the builders, structured append round-trips, image files and raw pixels decode with outlines, and the grey conversion matches `color.GrayModel`; the same tests pass under WebAssembly in Node. Benchmarks for a camera frame with and without a symbol.

### WD5 — The Page `(L)`

- [x] `web/site`: the Create, Inside the symbol, and Scan views, the worker, and light and dark themes.
- [x] One-off check, reported in the commit: headless Chrome drives every view, including the camera through a fake video device, in both themes and at phone width, with no console errors.

### Checkpoint A — the page works in a browser.

### WD6 — Pages & Release `(S)`

- [x] `.github/workflows/pages.yml`, a WebAssembly build step in CI, and `web/README.md`.
- [x] README (introduction, demo link, API table, scope, roadmap), and the theory code map.
- [x] `CHANGELOG.md` `v0.18.0` entry, left unstaged for the release branch.
- [x] `go test -race ./...` clean, gofmt-clean.

## 7. Validation strategy

1. **`Inspect` equals the encoder.** For every symbology and the options that change the encoding, `Modules` equals `Matrix`'s output.
2. **The layers explain the symbol.** Every data module, unmasked, holds the bit of `Codewords` that `ModuleCodeword` and `ModuleBit` name, each codeword bit appears exactly once, remainder modules are light before masking, and function modules are never masked.
3. **Facts from outside.** The codewords of theory doc 10 (`HELLO WORLD`, V1-M) and doc 28 (two R11x27-M streams, both from zint), and module counts worked out by hand from the layouts in docs 5, 26, and 28.
4. **The mask scores explain the choice.** The automatic mask is the lowest QR penalty or the highest Micro score, and forcing a mask leaves the scores unchanged.
5. **The bridge, twice.** Native tests, and the same tests compiled to WebAssembly and run in Node.
6. **The page in a browser.** A headless Chrome run through every view, with a fake camera, both themes, and phone width.

## 8. Proposed File Layout Delta

```
qrgen/
├── inspect.go             # new — ModuleRole, Inspection, Inspect, tracing
├── inspect_test.go        # new
├── matrix.go, micro_matrix.go, rmqr_matrix.go   # role tags and stream bits when traced
├── encode.go, micro_encode.go, options.go       # tracing switch; microData helper
web/
├── README.md
├── build.sh               # compiles site/qrgen.wasm, copies site/wasm_exec.js
├── .gitignore             # the two generated files
├── wasm/main.go           # js/wasm entry point
├── internal/bridge/       # the eight operations, tests, benchmarks
└── site/                  # index.html, style.css, app.js, client.js, worker.js,
                           # create.js, inspect.js, scan.js, util.js
.github/workflows/pages.yml   # new — build and publish to GitHub Pages
.github/workflows/ci.yml      # a WebAssembly build step
docs/
├── plan-web-demo.md / .id.md
└── theory/29-symbol-inspection.md / .id.md
```

## 9. Risks & Technical Notes

- **Download size.** The WebAssembly module is about 4.7 MB, 1.3 MB compressed as GitHub Pages serves it. It loads once and the browser caches it.
- **Noise is slow to scan.** A 640 by 480 frame of fine random noise takes about 0.7 s natively and 2.6 s in WebAssembly, because every speck becomes a connected component and the finder search compares nested components pairwise; a realistic frame without a symbol takes about 30 ms natively, and a frame with one about 6 ms (20 ms in WebAssembly). The page bounds the cost by scanning a centre square of at most 480 pixels, one frame at a time, in the worker. Speeding up the detector on such frames is a library task for a later milestone.
- **`wasm_exec.js` must match the compiler.** `build.sh` copies it from the toolchain that compiles the module, so the two never drift.
- **The camera needs a secure context.** GitHub Pages is HTTPS and `localhost` counts as secure; the page says so when the camera is unavailable.
- **Pages must be switched on once.** The repository's Pages source has to be set to "GitHub Actions" before the first deployment.

## 10. References

- Go, `syscall/js` and the `js/wasm` port (`lib/wasm/wasm_exec.js`).
- GitHub Pages with custom workflows: `actions/configure-pages`, `actions/upload-pages-artifact`, `actions/deploy-pages`.
- `docs/theory/05-matrix-construction.md`, `06-masking.md`, `04-reed-solomon.md`, `02-data-encoding.md`, `10-worked-example.md`, `26-micro-qr.md`, `28-rmqr.md` — what the inspection layers show.

## 11. Open Questions

- **Scope of the first version.** Resolved before implementation: all four parts (full encoding, image decoding, camera, and the teaching view).
- **The stack.** Resolved: plain HTML, CSS, and JavaScript with no build step.
- **The folder and deployment.** Resolved: `web/`, published on every push to `master`.
- **Data for the teaching view.** Resolved: a public `Inspect` API, released with the demo, rather than recomputing layers in JavaScript or postponing the view.
