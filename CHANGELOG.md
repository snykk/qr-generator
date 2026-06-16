# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-06-16

This release adds a **terminal / ASCII renderer**, a third output target alongside PNG and SVG. `EncodeTerminal` returns a multi-line `string` of block characters that prints straight to a terminal and scans from the screen; it runs the same encoding pipeline as `Encode` and `EncodeSVG` and differs only in the final render step.

### Added

- `EncodeTerminal(text string, opts ...Option) (string, error)` in `qrgen/render_terminal.go` — renders the finished matrix to text. By default it packs two module rows per text row with Unicode half-block glyphs (`█ ▀ ▄`) so modules stay near-square against a terminal's roughly 2:1 cell aspect ratio; the trailing unpaired row of an odd-sized symbol renders as a top-half glyph.
- `WithTerminalInvert(bool)` — swaps dark/light polarity for dark-background terminals, so the rendered dark modules still read as dark to a scanner.
- `WithTerminalASCII(bool)` — a portable double-width ASCII fallback (`##` per dark module) for terminals or fonts without block-element support. `WithQuietZone` applies; `WithModuleSize` and `WithColors` are no-ops for text output, as they are for `Matrix`.
- CLI: `-format terminal` and `-format ascii` route through `EncodeTerminal`, an `-invert` flag maps to `WithTerminalInvert`, and both text formats default to stdout when `-out` is omitted.
- New reference doc `docs/theory/19-terminal-rendering.md` (English plus Indonesian) on the cell aspect ratio, half-block packing, the glyph mapping, the ASCII fallback, polarity/inversion, and the quiet-zone and odd-row handling.
- Plan doc `docs/plan-terminal-renderer.md` (plus Indonesian) with milestones TR1..TR5.
- Runnable example `examples/encode/terminal` printing a URL QR in the default and inverted forms.

### Validated

- `TestTerminalRoundTrip` renders a range of payloads in half-block, inverted, and ASCII modes, parses the block characters back into a `[][]bool` grid, and confirms `DecodeMatrix` recovers the exact input — proving the rendering is loss-free without an image decoder.
- Golden-string tests pin the half-block, inverted, ASCII, and quiet-zone renderings against the worked example in doc 19, plus CLI tests for the terminal and ASCII formats and the stdout default. gofmt-clean, `go test -race ./...` clean.

### Design note

`renderTerminal` is a sibling of `renderPNG` and `renderSVG` but returns a `string` rather than `[]byte`, because terminal output is human-facing text meant to be printed. Polarity is the one thing that decides scannability — a block glyph reads dark on a light terminal and light on a dark one — so the default targets a light background and `WithTerminalInvert` covers the dark case; a theme-independent ANSI-colour mode honouring `WithColors` is deferred to a follow-up.

## [0.7.0] - 2026-06-16

This release adds **convenience payload builders** for the common real-world QR conventions. They are pure string formatting — no encoder or decoder change — and return a `string` so they compose with `Encode`, `EncodeSVG`, and `Matrix` alike.

### Added

- `qrgen/payload.go` with six builders that format and escape well-known payloads:
  - `WiFiPayload(WiFi) string` — `WIFI:` join string; `WiFi{SSID, Password, Security, Hidden}` with the `WiFiSecurity` type (`WiFiWPA`/`WiFiWEP`/`WiFiNoPass`), defaulting to WPA and omitting the password for open/empty networks.
  - `VCardPayload(VCard) string` — vCard 3.0 contact with CRLF lines; `VCard{Name, FamilyName, GivenName, Org, Title, Phones, Emails, URL, Address, Note}`, empty fields omitted.
  - `MailtoPayload(addr, subject, body) string` — RFC 6068 `mailto:` with percent-encoded subject/body (`%20`, not `+`).
  - `TelPayload(number) string` — RFC 3966 `tel:`.
  - `SMSPayload(number, message) string` — the `SMSTO:` scheme.
  - `GeoPayload(lat, lon) string` — RFC 5870 `geo:` with shortest-exact coordinate formatting (no scientific notation).
- Escaping helpers: Wi-Fi escapes `\ ; , : "`, vCard escapes `\ ; ,` and newlines (RFC 6350), mailto query parts via `net/url`.
- New reference doc `docs/theory/18-payload-formats.md` (English plus Indonesian) documenting each scheme, its escaping, a citation, and a worked example.
- Plan doc `docs/plan-convenience-helpers.md` (plus Indonesian) with milestones P1..P5.
- Runnable example `examples/encode/payloads` building a Wi-Fi PNG and a vCard SVG from the same builder pattern.

### Validated

- `TestPayloadRoundTrip` builds all six payloads (including escaped Wi-Fi specials and a full vCard) and confirms `DecodeBytes(Encode(...))` returns the exact string; the digit-heavy ones also exercise the v0.6 segmenter.
- `TestRoundTripWithThirdPartyDecoder` gained four payloads (Wi-Fi, vCard, mailto, geo); the independent gozxing decoder reads each exactly.
- Table-driven golden-string and escaping tests for every builder and both escapers. gofmt-clean, `go test -race ./...` clean.

### Design note

The builders deliberately return a `string` rather than PNG bytes (no `EncodeWiFi`-returns-PNG wrappers), so a single builder composes with every output format the library has — PNG, SVG, and the raw matrix — instead of locking each helper to one. They escape but do not validate input.

## [0.6.0] - 2026-06-16

This release replaces the encoder's single-mode greedy analyzer with **DP-optimal mixed-mode segmentation**, closing the long-standing "greedy mode analyzer" limitation. Encoder-only, no public API change, no decoder change — the decoder already parses an arbitrary sequence of mode segments.

### Added

- `qrgen/segment.go`: a `segment` type, `segmentText(text, v)` that computes a minimum-bit-length segmentation by dynamic programming, and `segmentsBitLength`. The DP splits a payload into numeric / alphanumeric / byte segments to minimise total encoded size; e.g. `"Order #1234567890"` drops from 148 bits (greedy byte) to 116 bits (byte + numeric) at V1, and `"x"+16×"9"+"x"` drops from V2-L to V1-L.
- New theory doc `docs/theory/17-optimal-segmentation.md` (English plus Indonesian) covering the cost model, the DP, the version-group interplay, the UTF-8 rune-boundary rule, and the homogeneous-input identity guarantee.
- Plan doc `docs/plan-segmentation.md` (plus Indonesian) with milestones MM1..MM6.
- `BenchmarkEncodeMixed` for the segmented encode path.

### Changed

- `selectVersion(text, ec)` now sizes the optimal segmentation per candidate version; because the segmentation only changes at the three character-count groups, a per-group cache runs the DP at most three times per encode.
- `encodeText` now emits one `[mode indicator][char count][payload]` block per segment followed by the shared terminator and padding, and its internal `m Mode` return (already discarded by `buildMatrix`) was dropped.
- `docs/theory/02-data-encoding.md` (both languages) now points at doc 17 and describes segmentation as shipped rather than deferred.

### Validated

- `TestRoundTripWithThirdPartyDecoder` gained four segmented payloads (byte+numeric, an invoice, a UTF-8+numeric case, and a 60-digit run in byte text); the independent gozxing decoder reads all of them, confirming the multi-segment stream is spec-valid.
- `TestEncodeSegmentationDropsAVersion` proves a real version drop end to end; `TestEncodeMixedPayloadRoundTrip` round-trips five mixed payloads through both the matrix and byte paths; the segmenter's own suite covers the identity invariant, never-worse-than-greedy across a payload×version sweep, version-group recompute, and UTF-8 rune integrity.
- The `HELLO WORLD` golden bytes are unchanged: a homogeneous input collapses to a single segment, so pure-mode payloads encode byte-for-byte as before.
- `go test -race ./...` clean.

### Performance note

The DP adds modest cost to the encode hot path: `BenchmarkEncodeURL` is flat versus v0.5 (~850us), `BenchmarkEncodeSmall` is ~550us versus ~451us (~+20%, driven by the DP's slice allocations on tiny payloads, not the O(n²) loop). `EncodeLarge` is unchanged in character — dominated by Reed–Solomon and PNG rendering. The per-group cache holds the DP to three runs; a Nayuki O(n) DP and a homogeneous fast path remain available if a profile ever shows the DP hot.

## [0.5.0] - 2026-06-16

This release adds a **scalable SVG renderer** alongside the original PNG output, opening an encoder/output-breadth phase after two decoder-robustness releases. No breaking change: `Encode` still returns PNG bytes; SVG is a new additive surface that reuses the entire encoding pipeline.

### Added

- `EncodeSVG(text, opts...) ([]byte, error)` and `EncodeSVGToFile(text, path, opts...) error` in `qrgen/api.go`. Both run the identical `resolveOptions -> validate -> buildMatrix` front-half as `Encode` and only swap the final render call, so every existing option (`WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`) works unchanged.
- `renderSVG` in `qrgen/render_svg.go`, a sibling of `renderPNG` sharing the identical signature and the `renderOptions` struct — deliberately no `Renderer` interface and no `WithFormat` enum (YAGNI for two non-polymorphic renderers, mirroring the v0.3 Sauvola dispatch decision). Output is a module-unit `viewBox` for clean scaling, pixel `width`/`height` matching the equivalent PNG's nominal size, `shape-rendering="crispEdges"` for decodability, a single background `<rect>`, and a single foreground `<path>` (one closed unit square per dark module rather than one `<rect>` per module). `colorToHex` un-premultiplies `color.Color` channels and emits `fill-opacity` only for non-opaque colours.
- `cmd/qrgen` gains a `-format png|svg` flag; when unset, a `.svg` extension on `-out` infers SVG, otherwise PNG. The default output filename is `qr.svg` for SVG, `qr.png` otherwise. Existing PNG invocations are unchanged.
- New theory doc `docs/theory/16-svg-rendering.md` (English plus Indonesian) and plan doc `docs/plan-svg-renderer.md` (plus Indonesian) covering milestones S1..S6.
- Benchmarks `BenchmarkEncodeSVGSmall` and `BenchmarkEncodeSVGURL`.
- Runnable example `examples/encode/svg`.

### Changed

- Corrected `docs/theory/08-rendering.md` (both languages): the sentence promising a `Render` interface that was never built now describes sibling render functions sharing `renderOptions`, cross-linked to doc 16.

### Dependencies

- Bumped `golang.org/x/text` 0.3.7 → 0.3.8 (Dependabot). This is an indirect, **test-only** dependency pulled in transitively by `github.com/makiuchi-d/gozxing`; it never appears in `go list -deps` of `qrgen` or `cmd/qrgen`, so it does not affect consumers of the library. The bump picks up the fix for CVE-2022-32149 in `language.ParseAcceptLanguage`.

### Validated

- `TestEncodeSVGRoundTripGrid` closes an encode -> SVG -> grid loop: it reconstructs the module grid straight from the emitted path (reading the canvas dimension from the `viewBox`, deriving the quiet zone, walking each `M x y` subpath) and asserts it equals `Matrix` cell for cell across V1-M, a URL at EC-Q, a small-quiet-zone numeric payload, a multi-block EC-H payload, and a custom-colour case — dependency-free, consistent with the stdlib-only policy.
- `renderSVG` unit tests parse every output through `encoding/xml`, check `viewBox`/`width`/`height` against the option math, verify opaque and alpha colour handling, assert the move-command count equals the dark-module total, and confirm an all-light matrix emits no `<path>`. A real V1 symbol was rasterised via qlmanage and visually confirmed as a valid three-finder QR.
- CLI tests cover `-format svg` to file and stdout, `.svg` extension inference, an invalid `-format` error, and that the no-format default still produces PNG.
- `go test -race ./...` clean.

### Note on file size

SVG is **not** smaller than PNG for a QR symbol: PNG's zlib compresses a monochrome bitmap very tightly, so a V1 "HELLO WORLD" is 632 bytes as PNG versus 3209 bytes as raw SVG (719 bytes gzipped). Choose SVG for lossless scaling and HTML embedding, not for disk size. SVG encoding is, however, several times faster than PNG (it skips rasterisation and zlib). The theory doc and plan were corrected to stop claiming SVG is smaller.

## [0.4.0] - 2026-05-24

This release adds **axis-aligned rotation handling** to the decoder. The fix is one line of geometry: `orderFinderTriple` now disambiguates top-right from bottom-left via a cross-product handedness test instead of the upright `if tr.y > bl.y { swap }` shortcut. The rest of the image pipeline was already rotation-invariant, so no other code changes. Coverage includes 90 / 180 / 270 plus soft tilts up to about 30 degrees off-axis; the 30..90 degree band remains future work because it would need a wider finder scanner.

### Added

- Cross-product handedness check inside `qrgen/decode_image.go` `orderFinderTriple`:
  - Replaces `if tr.y > bl.y || (math.Abs(tr.y - bl.y) < 1 && tr.x < bl.x) { swap }` with `cross := (tr.x - tl.x) * (bl.y - tl.y) - (tr.y - tl.y) * (bl.x - tl.x); if cross < 0 { swap }`.
  - One multiply-subtract-compare per decode, allocation-neutral, undetectable on the Otsu fast-path benchmark.
  - Sign convention is image-coordinate cross product with `y` growing downward, so any un-mirrored real QR symbol sits on the positive side at every rotation — proved by a four-row table in `docs/theory/15-rotation-handling.md` §4.
- The `finderTriple` type comment and the `findFinders` doc comment no longer claim the upright assumption; both now point at `docs/theory/15-rotation-handling.md` for the rotation-invariance proof.
- New theory doc `docs/theory/15-rotation-handling.md` (English plus Indonesian counterpart) walking through each v0.3 image-pipeline stage to localise the upright assumption, the cross-product identity with a worked sign analysis at 0 / 90 / 180 / 270, an honest note on mirrored symbols, a homography-decomposition proof sketch, and the derivation of the 30-degree scope boundary from `cos(theta)` drift against `fitsFinderRatio`'s ±50% tolerance.
- Plan doc `docs/plan-decoder-rotation.md` (English plus Indonesian counterpart) with milestones R1..R6.

### Validated

- `TestOrderFinderTripleRotationInvariance` builds synthetic `finderCandidate` triples at 0 / 90 / 180 / 270 plus a hand-derived 30-degree tilt, then exercises every one of the six permutations of the input argument order against each rotation case (30 sub-cases total), asserting identities `(tl, tr, bl)` come back consistent every time.
- `TestOrderFinderTripleRejectsBadGeometry` keeps the collinear-triple and leg-ratio rejection paths covered so the cross-product change does not silently widen what the function accepts.
- Six synthetic rotation fixtures in `qrgen/decode_rotation_test.go` round-trip `"HELLO"` end-to-end via the public `DecodeBytes`: `TestRotation90`, `TestRotation180`, `TestRotation270`, `TestRotationSoftTilt15`, `TestRotationSoftTilt30`, plus an explicit negative `TestRotationSoftTiltOutOfBand` at 45 degrees that documents the scope boundary. The rotated images are built in memory by an inverse-mapped bilinear-sampling `rotateImage` helper, so no binary fixtures land under `testdata/`.
- `decodeImageDebug` reports `binariserOtsu` for every passing rotation fixture, confirming the Sauvola fallback is not perturbed by orientation alone — the dispatch only fires when the quiet zone is contaminated, as before.
- Empirical finding recorded in the negative-fixture comment and the theory doc: at exactly 45 degrees the failure mode is `ErrInvalidVersion` rather than `ErrFinderNotFound`. The scanner just squeaks past its tolerance and the version estimate from finder spacing falls outside 1..40 instead. The assertion accepts either sentinel so the test survives small empirical shifts.
- `go test -race ./...` remains clean.

### Documented limitation

Tilts in the `[30°, 90°)` band still defeat both the cross-product fix and the 1:1:3:1:1 row scanner. Closing the remaining gap requires either a wider tolerance on `fitsFinderRatio` (which would raise the false-positive rate on noisy backgrounds) or a different finder detector — contour tracing or a fan-of-orientations search. Both belong in a later release.

## [0.3.0] - 2026-05-24

This release adds an **adaptive thresholding fallback** to the decoder so QR codes whose quiet zone has been darkened by uneven lighting or soft shadows survive the binarisation stage. No public API change; the Otsu fast path stays within run-to-run variance of the v0.2 baseline.

### Added

- Sauvola adaptive thresholding in `qrgen/decode_image_sauvola.go`:
  - Two `uint64`-backed integral images (sum and sum-of-squares) sized `(w+1) * (h+1)` so per-pixel window mean and std are `O(1)` regardless of window size.
  - `windowMeanStd` helper with boundary clipping, guarded against tiny negative variances from floating-point rounding.
  - Hard-coded textbook defaults `sauvolaWindow = 25`, `sauvolaK = 0.2`, `sauvolaR = 128.0` (Sauvola & Pietikainen 2000, Shafait et al. 2008). No public option in v0.3.
- Internal Otsu-or-Sauvola dispatch inside `decodeImage`:
  - `otsuThreshold` now returns the threshold together with the separability ratio `η = σ²_B / σ²_T` ∈ `[0, 1]`, computed for free from the same histogram pass.
  - Stage 1 (proactive): if `η < etaMin = 0.5` the dispatch skips Otsu's binarisation entirely and routes straight to Sauvola, saving one wasted finder-detection pass on what would have been an unhealthy Otsu output.
  - Stage 2 (reactive): if Otsu's binarisation runs but `findFinders` fails, the grayscale buffer is rebinarised with Sauvola and finder detection re-runs.
  - Package-internal `decodeImageDebug` sibling exposes the chosen branch (`binariserOtsu`, `binariserSauvolaProactive`, `binariserSauvolaReactive`) to tests without surfacing anything on the public API.
- New theory doc `docs/theory/14-adaptive-thresholding.md` (English plus Indonesian counterpart) covering Otsu's failure modes, Sauvola's formula and parameters, integral images for `O(1)` window queries, comparison vs Niblack / Wolf / Bernsen / Adaptive Gaussian, and the runtime dispatch heuristic.
- Plan doc `docs/plan-decoder-thresholding.md` (English plus Indonesian counterpart) with milestones T1..T6.
- New benchmark `BenchmarkDecodeImageSauvolaFallback` that forces the reactive Sauvola branch through a constant-quiet-zone-darkening fixture so the fallback cost is visible alongside the Otsu fast-path benchmarks.

### Validated

- Five synthetic uneven-lighting fixtures in `qrgen/decode_image_sauvola_test.go` (`TestT4ConstantQuietZoneDarkening`, `TestT4LinearGradientOnQuietZone`, `TestT4RadialVignetteOnQuietZone`, `TestT4DropShadowOnQuietZone`, `TestT4DiagonalGradientOnQuietZone`) each prove Otsu alone fails `findFinders`, the public `DecodeBytes` round-trips the original payload, and `decodeImageDebug` reports the reactive Sauvola state.
- Dispatch unit tests in the same file cover all three runtime states: clean encoded PNG asserts `binariserOtsu` and round-trips its payload, monochrome 80x80 input asserts `binariserSauvolaProactive` via the variance-collapse `η = 0` branch, and a brightness-compression mutation of a clean QR asserts `binariserSauvolaReactive` after verifying Otsu alone fails and `η ≥ etaMin`.
- Sauvola unit tests over hand-checked integral image values, a naive `O(w²)` reference cross-check of `windowMeanStd`, the uniform-image no-noise property, a two-illumination-region fixture that classifies ink and paper correctly while also proving Otsu fails on the same input, and zero-size / smaller-than-window guards.
- Otsu fast path stays within run-to-run variance of v0.2 (Apple M5, `count=5`, `benchtime=1s`): `BenchmarkDecodeImageSmall` +0.5%, `BenchmarkDecodeImageMultiBlock` -0.8%, `BenchmarkDecodeImageURL` +1.6%, `BenchmarkDecodeImageFromPNGDecode` +1.6%. The new `BenchmarkDecodeImageSauvolaFallback` records the reactive cost at ~1074 ns/op (about 1.9x the Otsu fast path) with 1.07 MB/op driven by the two integral images.
- `go test -race ./...` remains clean across `qrgen` and `cmd/qrgen`.

### Documented limitation

Brightness-compression mutations that flatten the QR's own ink-paper contrast (very dim photographs, heavy gradient across the symbol itself) still defeat both Otsu and the Sauvola fallback at the default `sauvolaK = 0.2`, `sauvolaR = 128`. Recovering this family is left to future work, likely a `WithBinarisation` option and morphological cleanup once the v0.4 rotation handling lands.

## [0.2.0] - 2026-05-20

This release adds a full QR **decoder** so the package can now round-trip text → image → text end-to-end without any third-party dependency.

### Added

- Image-stage pipeline in `qrgen/decode_image.go`:
  - ITU-R BT.601 grayscale conversion and Otsu binarisation that handles non-zero image bounds and degenerate (monochrome) histograms.
  - 1:1:3:1:1 finder-pattern row scan with vertical cross-check, candidate clustering by local module pitch, and a right-angle-plus-leg-length geometry validator that orders the three centres as top-left, top-right, bottom-left.
  - 4-point perspective homography solved via Gaussian elimination with partial pivoting, version estimation from finder spacing, and alignment-pattern refinement for V2+.
  - Module sampling at the centre of each module to reconstruct a `[][]bool` grid for the matrix decoder.
- Matrix-stage pipeline in `qrgen/decode_matrix.go`, `qrgen/format_decode.go`, and `qrgen/rs_decode.go`:
  - Brute-force BCH(15, 5) format-info decoder with a combined-Hamming budget across both redundant copies.
  - Mask reversal and reverse zig-zag walk that reuses the encoder's geometry.
  - Block deinterleaving plus Reed–Solomon error correction via Berlekamp–Massey, Chien search, and Forney's algorithm; tolerates the spec's `floor(n/2)` budget per block.
  - Bit-stream parser for numeric, alphanumeric, and byte segments with terminator detection.
- New GF(256) helpers (`gf256Inverse`, `polyDivQR`, `polyEval`, `polyDeriv`) shared by the encoder and decoder.
- Three new public entry points in `qrgen`:
  - `Decode(img image.Image) (string, error)` — image → text.
  - `DecodeBytes(data []byte) (string, error)` — PNG / JPEG / GIF bytes → text.
  - `DecodeMatrix(grid [][]bool) (string, error)` — boolean module grid → text.
- `cmd/qrgen` CLI gains `-decode` mode with a matching `-in` flag, supports decoding from a file or from stdin, and writes the recovered text to stdout (default) or to `-out`.
- Five typed sentinel errors covering every reachable failure: `ErrFinderNotFound`, `ErrInvalidVersion`, `ErrFormatUnreadable`, `ErrTooManyErrors`, `ErrCorruptedPayload`.
- Three new bilingual theory docs (`docs/theory/11-rs-decoding`, `12-image-processing`, `13-decoder-pipeline`) and a parallel `docs/plan-decoder.md` (plus its Indonesian counterpart) covering milestones D1..D14.
- Runnable demos at `examples/decode/basic` (round-trip with default rendering) and `examples/decode/styled` (round-trip on a branded PNG with custom colours, larger modules, and a higher EC level). The pre-existing encoder demos have moved under `examples/encode/basic` and `examples/encode/styled` to mirror the new layout.

### Validated

- 12-case round-trip `Encode → DecodeBytes` mirroring the encoder's gozxing-backed test, now closing the loop with our own decoder.
- Robustness tests that flip data-area modules within the per-block RS budget at EC-Q and EC-H and assert recovery, plus an image-stage robustness suite covering small / large module sizes, larger quiet zones, low-contrast greys, and inverted-feel colours.
- Per-module sample fidelity test that compares every cell of a HELLO WORLD V1 PNG to the original matrix after binarise + homography sampling.
- Format-info decoder validated across all 32 valid `(EC, mask)` combinations plus per-copy and combined corruption variants up to the BCH budget.
- Reed–Solomon decoder property test with 250 random `(version, EC level, payload, corruption)` trials.
- New benchmarks for the matrix and image decoder stages (`BenchmarkDecodeMatrix*`, `BenchmarkDecodeImage*`).

## [0.1.0] - 2026-05-17

First public release. The encoder is feature-complete for the v0.1 scope and its output is validated against a third-party reference decoder.

### Added

- Pure-Go QR code encoder following ISO/IEC 18004:2015 with no runtime dependencies outside the standard library.
- Three encoding modes: numeric, alphanumeric, byte (UTF-8 passthrough).
- All 40 QR versions and all four EC levels (L, M, Q, H).
- Reed–Solomon error correction over GF(2⁸), including multi-block layouts with column-major interleaving.
- Mask selection across all eight mask patterns using the spec's four-rule penalty score.
- BCH-encoded format and version information, with precomputed lookup tables.
- PNG renderer with configurable module size, quiet zone, and foreground/background colours. Default output is 8-bit grayscale; the renderer switches to RGBA automatically when custom colours are passed.
- Public API in package `qrgen`:
  - `Encode(text, opts...) ([]byte, error)` — text → PNG bytes.
  - `EncodeToFile(text, path, opts...) error` — text → PNG file on disk.
  - `Matrix(text, opts...) ([][]bool, error)` — raw module grid for non-PNG rendering targets.
- Functional options: `WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`.
- Thin CLI binary at `cmd/qrgen` covering every relevant library option as a flag, with stdin/stdout pipe support.
- Two runnable examples (`examples/basic`, `examples/styled`).
- Bilingual literature review in `docs/theory/` covering every encoder stage (English plus Indonesian).

### Validated

- Round-trip decoding against `github.com/makiuchi-d/gozxing` (test-only dependency, never in the runtime import graph) for 12 representative `(payload × EC level × version)` combinations.
- Over 80 unit tests including per-version sweeps that verify every spec lookup table and a 160-combination data-plus-EC-equals-total invariant check.
- Race detector clean (`go test -race ./...`).

[Unreleased]: https://github.com/snykk/qr-generator/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/snykk/qr-generator/releases/tag/v0.8.0
[0.7.0]: https://github.com/snykk/qr-generator/releases/tag/v0.7.0
[0.6.0]: https://github.com/snykk/qr-generator/releases/tag/v0.6.0
[0.5.0]: https://github.com/snykk/qr-generator/releases/tag/v0.5.0
[0.4.0]: https://github.com/snykk/qr-generator/releases/tag/v0.4.0
[0.3.0]: https://github.com/snykk/qr-generator/releases/tag/v0.3.0
[0.2.0]: https://github.com/snykk/qr-generator/releases/tag/v0.2.0
[0.1.0]: https://github.com/snykk/qr-generator/releases/tag/v0.1.0
