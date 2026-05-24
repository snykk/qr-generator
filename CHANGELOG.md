# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/snykk/qr-generator/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/snykk/qr-generator/releases/tag/v0.4.0
[0.3.0]: https://github.com/snykk/qr-generator/releases/tag/v0.3.0
[0.2.0]: https://github.com/snykk/qr-generator/releases/tag/v0.2.0
[0.1.0]: https://github.com/snykk/qr-generator/releases/tag/v0.1.0
