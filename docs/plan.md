# QR Generator — Plan

This document describes the implementation plan for `qr-generator`: a Go library for generating QR codes, written from scratch following the **ISO/IEC 18004** specification, with no external dependencies (Go standard library only).

> Status: **draft / living document.** This plan will keep evolving as implementation progresses. A milestone is considered done when every item in it is tested and documented.

> Indonesian version: [docs/plan.id.md](plan.id.md).

---

## 1. Vision & Goals

- Provide a **pure-Go library** (`import "github.com/.../qrgen"`) other Go projects can import to generate PNG QR codes.
- Implement the encoder **from scratch** — not as a wrapper. The point is to learn how QR actually works (mode encoding, Reed–Solomon, masking, matrix layout) while producing a real, reusable artifact.
- Ship a **thin CLI** in `cmd/qrgen` as a sample consumer and quick demo tool.

## 2. Design Principles

1. **Zero external deps** — Go standard library only. No third-party image library, no third-party Reed–Solomon package.
2. **Spec-first** — every algorithm is anchored to a section of ISO/IEC 18004 (or an equivalent open reference such as the Thonky tutorial or Nayuki).
3. **Ergonomic API** — sensible defaults (auto version, EC level M, quiet zone of 4 modules); advanced configuration via *functional options*.
4. **Testable** — each layer (encoder, RS, matrix, mask, render) has its own unit tests; PNG output is verified via *golden tests*.
5. **API stability** follows semver. Pre-`v1.0.0` may still break, but every change is recorded in `CHANGELOG.md`.

## 3. Initial Scope

### In scope (target ≤ v0.1.0)

- Encoding modes: **numeric, alphanumeric, byte (UTF-8)**.
- QR versions: **1–40** (all standard versions).
- Error correction levels: **L, M, Q, H**.
- Output: **PNG** (via `image/png` from the standard library).
- CLI to encode a string/stdin into a PNG file.

### Out of scope (for now)

- **Kanji** mode and **ECI** (non-default character sets).
- **Micro QR**, **rMQR**, **structured append** (multi-symbol).
- **SVG / terminal / JPEG** output (candidates for v0.2+).
- **Logo embedding** in the QR center (candidate for v0.2+).
- **Decoder / reader** (outside this library's scope).
- HTTP service / web playground.

---

## 4. Milestones

Milestones are ordered because of natural dependencies between components. Sizing (`S` / `M`) is relative effort, not a time commitment.

### M1 — Foundation `(S)`

Goal: project skeleton ready for the next milestones.

- [ ] `go mod init` with the correct module path.
- [ ] Initial folder layout (see section 5).
- [x] `LICENSE` — Apache 2.0 (added via GitHub's license generator).
- [ ] Go-standard `.gitignore`.
- [ ] `README.md` skeleton (overview + "in development" status).
- [ ] Minimum CI workflow: `go vet`, `go test ./...`, `go build ./...`.

### M2 — Spec Tables & Constants `(M)`

Goal: static data from the spec is ready for the encoder layer.

- [x] **Character count indicator** table (bit width per mode × version range). — `qrgen/mode.go`
- [x] **Data capacity** table per (version × EC level). — `qrgen/version.go`
- [x] **Error correction block count & size** table per (version × EC level). — `qrgen/version.go`
- [x] **Alignment pattern positions** table per version. — `qrgen/version.go`
- [x] **Format info** (BCH-encoded) and **version info** (for version ≥ 7) bit strings. — `qrgen/formatinfo.go`
- [x] Reed–Solomon **generator polynomial** per EC codeword count. — `qrgen/reedsolomon.go` `genPoly` (validated against the α-exponent table for all 13 EC block sizes used by QR).

### M3 — Data Encoding `(M)`

Goal: input string → final bit stream (pre-RS).

- [x] **Mode analyzer**: detect numeric / alphanumeric / byte (greedy is fine for MVP; optimal segmentation can come later). — `qrgen/encode.go` `analyzeMode`
- [x] Per-mode encoder → bit stream. — `qrgen/encode.go` `writeNumeric` / `writeAlphanumeric` / `writeByte`
- [x] Pick the **minimum version** based on payload length + EC level. — `qrgen/encode.go` `selectVersion`
- [x] Mode indicator + character count indicator + payload + **terminator + pad bytes**. — `qrgen/encode.go` `encodeText` (validated end-to-end against the "HELLO WORLD" worked example).

### M4 — Reed–Solomon Error Correction `(M)`

Goal: data codewords → data + EC codewords, properly interleaved.

- [x] **GF(256)** arithmetic: exp/log tables using primitive polynomial `0x11D`. — `qrgen/gf256.go` `expTable` / `logTable` / `gf256Mul`
- [x] Polynomial multiply/divide over GF(256). — `qrgen/gf256.go` `polyMul` / `polyMod` (monic divisor; QR's generator is always monic)
- [x] Build the **generator polynomial** for n EC codewords. — `qrgen/reedsolomon.go` `genPoly`
- [x] Reed–Solomon encoder per block. — `qrgen/reedsolomon.go` `encodeBlock` (validated against the "HELLO WORLD" worked example's 10 EC codewords)
- [x] Split data into blocks per the M2 tables, then **interleave** data & EC. — `qrgen/reedsolomon.go` `splitAndEncodeBlocks` + `interleaveBlocks` + `rsEncode`

### M5 — Matrix Construction `(M)`

Goal: QR matrix populated with functional + data modules, before masking.

- [x] Matrix size = `21 + 4×(version-1)`. — `qrgen/matrix.go` `newMatrix`
- [x] Place **finder patterns** (3 corners) + separators. — `qrgen/matrix.go` `placeFinderPatterns` / `placeSingleFinder`
- [x] **Alignment patterns** per the M2 table. — `qrgen/matrix.go` `placeAlignmentPatterns` / `placeSingleAlignment`
- [x] **Timing patterns**. — `qrgen/matrix.go` `placeTimingPatterns`
- [x] **Dark module** at `(4*version+9, 8)`. — `qrgen/matrix.go` `placeDarkModule` (the original plan note had the coordinates swapped; the convention everywhere else is `(row, col)`).
- [x] Reserve the **format info & version info** areas. — `qrgen/matrix.go` `reserveFormatInfoArea` + `reserveVersionInfoArea`
- [x] Place **data bits** in zig-zag from the bottom-right, skipping functional areas. — `qrgen/matrix.go` `placeData` (validated against the `TestDataAreaCellsMatchesCapacity` invariant for all 40 versions).

### M6 — Masking & Format/Version Info `(M)`

Goal: pick the best mask, write final format & version info.

- [x] Implement the **8 mask patterns** (mask 0..7). — `qrgen/mask.go` `maskCondition`
- [x] Apply mask only to data modules (not functional areas). — `qrgen/mask.go` `applyMask` (skips reserved cells)
- [x] **Penalty evaluation** (the 4 rules from the spec). — `qrgen/mask.go` `penalty` / `penaltyRule1..4`
- [x] Choose the mask with the lowest total penalty. — `qrgen/mask.go` `selectAndApplyMask` (clones the matrix per trial; ties broken by lowest index)
- [x] Encode **format info** (EC level + mask) using BCH(15,5) + XOR mask `0x5412`. — `qrgen/matrix.go` `writeFormatInfo` (the BCH table is precomputed in `qrgen/formatinfo.go` from M2)
- [x] Encode **version info** for version ≥ 7 using BCH(18,6). — `qrgen/matrix.go` `writeVersionInfo`

### M7 — PNG Renderer `(S)`

Goal: `[][]bool` matrix → PNG bytes.

- [x] Convert matrix → `image.Gray` (or RGBA when custom colors are used). — `qrgen/render_png.go` `renderGray` / `renderRGBA` (default monochrome path keeps the PNG small).
- [x] **Quiet zone** (default 4 modules) around the matrix. — `qrgen/render_png.go` `renderOptions.quietZone` (default 4).
- [x] Configurable **module size** in pixels. — `qrgen/render_png.go` `renderOptions.moduleSize` (default 8).
- [x] Configurable **foreground & background color**. — `qrgen/render_png.go` `renderOptions.foreground` / `background`.
- [x] Encode via `image/png`. — `qrgen/render_png.go` `renderPNG` (validated via round-trip decode against pixel-centre sampling).

### M8 — Public API & Examples `(S)`

Goal: a comfortable library surface.

- [x] Functional options: `WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`, etc. — `qrgen/options.go`
- [x] Entry points: — `qrgen/api.go`
  - `qrgen.Encode(text string, opts ...Option) ([]byte, error)` → PNG bytes.
  - `qrgen.EncodeToFile(text, path string, opts ...Option) error`.
  - `qrgen.Matrix(text string, opts ...Option) ([][]bool, error)` (raw access).
- [x] Full godoc for every exported symbol. — all exports across `qrgen/*.go` carry doc comments starting with the symbol name.
- [x] `examples/basic/main.go` & `examples/styled/main.go`. — runnable via `go run ./examples/basic` and `go run ./examples/styled`.

### M9 — Thin CLI `(S)`

Goal: a `qrgen` binary for quick usage & demos.

- [x] `cmd/qrgen/main.go` with flags: — covers `-text`, `-out` (with `-` for stdout), `-size`, `-ec`, `-fg`, `-bg`, `-quiet-zone`, plus bonus `-version` and `-mask` overrides.
- [x] Clear exit codes & error messages. — `run()` returns errors that `main` prints with a `qrgen:` prefix and exits 1.
- [x] Usage examples in the README. — CLI section added with install, basic, stdin-pipe, and styled examples.

### M10 — Quality Gate `(M)`

Goal: confidence that the output is correct and stable.

- [x] Per-component unit tests (gf256, RS, per-mode encoder, matrix, mask). — split across `qrgen/*_test.go`; over 80 cases including alpha-exponent tables, GF(256) commutativity sweep, alignment counts per version, mask involutivity, etc.
- [x] **Golden tests** for selected (input, EC level) pairs — compare matrices against known-good references (e.g. Nayuki). — `TestEncodeTextHelloWorld`, `TestEncodeBlockHelloWorld`, `TestRSEncodeHelloWorld`, plus per-mode generator-polynomial table from Nayuki in `TestGenPolyAlphaExponents`.
- [x] Round-trip test using a third-party decoder as a test-only dependency (allowed, since it isn't a runtime dependency). — `qrgen/roundtrip_test.go` exercises 12 (text, EC, version) cases through `github.com/makiuchi-d/gozxing`.
- [x] Benchmarks for the main encode path. — `qrgen/bench_test.go` covers small/URL/multi-block/large payloads plus a matrix-only variant.
- [x] `go test -race ./...` clean. — every CI run executes with `-race`; no data races detected.

### M11 — Polish & Release `(S)`

Goal: ready for others to use.

- [x] Final `README.md`: badges, code examples, API summary, Go compatibility. — pkg.go.dev, Go version, license, and CI badges added; API summary table and a runnable terminal-render snippet included.
- [x] `CHANGELOG.md` (Keep a Changelog format). — `CHANGELOG.md` documents the v0.1.0 surface and validation evidence.
- [ ] Tag `v0.1.0`. — pending the first push to GitHub; once the remote has the M11 commit, run `git tag v0.1.0 && git push origin v0.1.0`.
- [x] Verify `go install` and `go get` work from the module path. — `go install ./cmd/qrgen` builds and runs locally producing a valid PNG; once the repo is pushed, `go install github.com/snykk/qr-generator/cmd/qrgen@latest` will resolve.

---

## 5. Proposed Folder Layout

```
qr-generator/
├── README.md
├── LICENSE
├── CHANGELOG.md
├── go.mod
├── go.sum
├── docs/
│   ├── plan.md             # this document (English)
│   ├── plan.id.md          # Indonesian version
│   └── theory/             # bilingual literature review (algorithms + data tables + worked example)
├── qrgen/                  # main importable package
│   ├── qrgen.go            # entry points + package doc
│   ├── options.go          # functional options
│   ├── mode.go             # mode analyzer + per-mode encoder
│   ├── version.go          # capacity tables, version selection
│   ├── gf256.go            # GF(256) arithmetic
│   ├── reedsolomon.go      # generator poly + encoder
│   ├── matrix.go           # finder/alignment/timing/data placement
│   ├── mask.go             # 8 masks + penalty + best-mask selection
│   ├── formatinfo.go       # BCH for format & version info
│   ├── render_png.go       # PNG renderer
│   └── *_test.go
├── examples/
│   ├── basic/main.go
│   └── styled/main.go
└── cmd/
    └── qrgen/
        └── main.go
```

## 6. Risks & Technical Notes

- **Reed–Solomon over GF(256)** is the bug-prone part. Mitigation: unit-test against spec/Nayuki test vectors before any upper layer relies on it.
- **Mask penalty rules** are easy to misread (rules 1 and 3 in particular). Mitigation: cross-check against published mask-score tables.
- **Optimal mode selection** is non-trivial (a DP over segmentation). For the MVP a single-mode greedy choice is enough; mixed-mode segmentation is deferred to post-v0.1.
- **Byte-mode character encoding** assumes raw UTF-8 (no ECI). Modern decoders usually guess UTF-8, but this is not ECI-compliant — to be noted as a limitation in the README.

## 7. References

- ISO/IEC 18004:2015 — *Information technology — Automatic identification and data capture techniques — QR code bar code symbology specification.*
- Thonky QR Code Tutorial — https://www.thonky.com/qr-code-tutorial/
- Project Nayuki — *QR Code generator library* (reference implementation).

## 8. Open Questions

To be filled in as implementation progresses. Initial examples:

- Final module path for `go.mod` (use `github.com/<user>/qr-generator`?).
- Go compatibility matrix (minimum 1.21? 1.22?).
- Is `qrgen.Matrix(...)` enough, or should we also expose a pluggable renderer?
