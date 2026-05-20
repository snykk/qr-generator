# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/snykk/qr-generator/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/snykk/qr-generator/releases/tag/v0.2.0
[0.1.0]: https://github.com/snykk/qr-generator/releases/tag/v0.1.0
