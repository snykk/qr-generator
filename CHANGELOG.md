# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/snykk/qr-generator/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/snykk/qr-generator/releases/tag/v0.1.0
