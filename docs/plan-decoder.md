# QR Decoder — Plan

This document describes the implementation plan for the QR **decoder** feature that targets the `v0.2.0` minor release. It is parallel to [`docs/plan.md`](plan.md), which covered the encoder for `v0.1.0` and is now mostly done.

> Status: **draft / living document.** Milestones D1..D14 land incrementally; each is a focused commit (or small commit series) with tests, just like the encoder milestones M1..M11.

> Indonesian version: [docs/plan-decoder.id.md](plan-decoder.id.md).

---

## 1. Vision & Goals

- Extend the `qrgen` package with a **decoder** that reads a QR symbol back to its source text, exposing two entry points:
  - `qrgen.DecodeMatrix([][]bool) (string, error)` — operates on a clean top-down boolean matrix; useful for callers that already have one.
  - `qrgen.Decode(img image.Image) (string, error)` — reads a real image (camera photo, scan, generated PNG) and runs the full pipeline.
- Keep the **same philosophy** as the encoder: pure Go, zero runtime dependencies beyond the standard library, spec-first, with bilingual theory docs and golden test fixtures.
- Cross-validate against the encoder so the package becomes a true closed loop: encoder output decoded by our own decoder must round-trip exactly across all modes, versions, and EC levels.

## 2. Design Principles

1. **Two-stage pipeline.** `DecodeMatrix` does pure logic (RS decoding, mask reversal, bit-stream parsing). `Decode` adds image processing on top (binarisation, finder detection, perspective transform, sampling). Each can be tested in isolation.
2. **Reed–Solomon decoding is its own beast.** RS encoding (M4) used polynomial division; RS decoding uses syndromes, Berlekamp–Massey, Chien search, and Forney's algorithm. We treat it as a new topic with its own theory doc, not as "the inverse of M4".
3. **Tolerate real-world input.** Decoder receives noisy / skewed / partially-damaged input. It must use the spec's error-correction budget (`floor(n/2)` corrupted codewords per block) before declaring failure.
4. **Clear error returns.** Decode failures are returned as typed errors (`ErrFinderNotFound`, `ErrFormatUnreadable`, `ErrTooManyErrors`, …) so callers can branch on cause.
5. **Pure Go, no CGo, no third-party CV libraries.** Image processing happens with `image.Image` + a small custom binarisation and homography routine.

## 3. Scope

### In scope for the decoder release

- Decoding modes: **numeric, alphanumeric, byte** (mirror of encoder scope).
- All **40 standard QR versions**.
- All **4 EC levels** (L, M, Q, H), with full error-correction up to the spec budget.
- Two entry points: matrix-based (`DecodeMatrix`) and image-based (`Decode`).
- Convenience helper `DecodeBytes([]byte) (string, error)` for in-memory PNG bytes.

### Out of scope (still)

- Kanji mode and ECI segments — symmetric to the encoder's gap.
- Micro QR / rMQR.
- Structured-append reassembly across multiple symbols.
- Locating multiple QRs in one image.
- ML-assisted decoding for severely damaged symbols.

---

## 4. Milestones

Milestones are split by checkpoint. **Checkpoint 1** (after D7) gives a working matrix-to-text decoder. **Checkpoint 2** (after D12) extends it to image-to-text. **Checkpoint 3** (D14) is the `v0.2.0` release.

### D1 — Decoder Theory Docs `(S)`

Goal: cover the new algorithms in `docs/theory/` before any code lands, matching the spec-first approach from the encoder.

- [x] `docs/theory/11-rs-decoding.md` — syndromes, Berlekamp–Massey, Chien search, Forney's algorithm, error vs. erasure correction.
- [x] `docs/theory/12-image-processing.md` — grayscale conversion, Otsu thresholding, finder-pattern scanning, homography, alignment-pattern refinement.
- [x] `docs/theory/13-decoder-pipeline.md` — overall flow `image → matrix → text` plus error-handling rules.
- [x] Indonesian counterparts for each.
- [x] Update `docs/theory/README.md` index to include the new entries.

### D2 — GF(256) Decode-Side Operations `(S)`

Goal: extend `qrgen/gf256.go` with the polynomial and field operations RS decoding needs.

- [x] `gf256Inverse(a) uint8` — multiplicative inverse using the existing log/exp tables. Panics on zero.
- [x] `polyDivQR(dividend, divisor) (quotient, remainder []uint8)` — full division returning both parts; tolerates non-monic divisors by normalising the leading coefficient.
- [x] `polyEval(p []uint8, x uint8) uint8` — Horner's method evaluation, used by syndrome calculation, Chien search, and Forney.
- [x] `polyDeriv(p []uint8) []uint8` — formal derivative; keeps only odd-degree terms (characteristic-2 collapse).
- [x] Tests: exhaustive 255-element sweep for `gf256Inverse` (`a · a⁻¹ = 1`); panic test for zero input; table-driven cases for `polyEval` / `polyDeriv`; `polyDivQR` correctness on direct cases plus an 11-pair property test that reconstructs the dividend via `q · divisor + r`.

### D3 — Reed–Solomon Decoder `(M)`

Goal: a `rsDecode(block []byte, n int) ([]byte, error)` that recovers `block[:k]` from up to `floor(n/2)` corrupted codewords.

- [x] **Syndrome calculation** — `n` syndromes by evaluating the received polynomial at `α⁰..α^(n−1)` via `polyEval`.
- [x] **Berlekamp–Massey** — `berlekampMassey` works in lowest-degree-first internally and returns Λ reversed to high-degree-first for downstream stages.
- [x] **Chien search** — `chienSearch` returns parallel `(positions, locators)` slices for the rest of the pipeline.
- [x] **Forney's algorithm** — `forneyMagnitudes` uses the standard `Y_k = X_k · Ω(X_k^{-1}) / Λ'(X_k^{-1})` form (generator roots start at α⁰).
- [x] Return `ErrTooManyErrors` when degree(Λ) exceeds correction capacity or the position count disagrees with `L`.
- [x] Tests: HELLO WORLD fixture with 0, 1, 2..5 byte corruptions, an over-capacity bucket, and a 250-trial random property test across V1-M / V1-L / V1-H / V5-M / larger block shapes.

### D4 — Format Information Reader `(S)`

Goal: read the 15-bit format codeword from the matrix and recover (EC level, mask).

- [x] Read both redundant copies of the 15-bit codeword. — `qrgen/format_decode.go` `readFormatInfo`
- [x] BCH(15,5) decoder by brute force over the 32 precomputed entries from M2: minimum combined Hamming distance wins. — joint budget set at 6 (3+3) per the BCH minimum distance.
- [x] Return `ErrFormatUnreadable` only when both copies exceed budget. — sentinel exported alongside the decoder.
- [x] Extract `ECLevel` and `mask`.
- [x] Tests: full 32-pair round-trip plus per-copy and combined corruption cases including the asymmetric "one clean, one trashed" scenario.

### D5 — Mask Reversal & Data-Area Walk `(S)`

Goal: invert the zig-zag walk from M5 to produce the interleaved codeword byte stream from a known (version, mask, matrix).

- [x] Reuse the `placeData` walk in reverse via `qrgen/decode_matrix.go` `readCodewordStream`, plus `matrixFromGrid` that rebuilds the reserved-area mask from a `[][]bool` input.
- [x] XOR the mask out during the read so the encoder's `applyMask` is undone.
- [x] Strip the remainder bits per `Version.RemainderBits()`.
- [x] Return the raw interleaved `[]byte`.
- [x] Tests: round-trip across V1..V10 (single-block, multi-block, with version info) and across all 8 masks; matrix-size / row-raggedness validation in `matrixFromGrid`.

### D6 — Block Deinterleaving + Error Correction `(M)`

Goal: reverse the column-major interleave from M4 and run `rsDecode` on each block.

- [x] Compute the block layout from `Version.ECBlocks(ec)` (reusing existing M2 tables) in `deinterleaveBlocks`.
- [x] Walk the interleaved stream column-by-column to split it back into per-block data + EC slices, mirroring the encoder's interleaver.
- [x] Run `rsDecode` on each block via `deinterleaveAndCorrect`; wrap `ErrTooManyErrors` with the failing block index.
- [x] Concatenate corrected data codewords from all blocks into a single byte stream.
- [x] Tests: per-block layout reversal V1-M / V1-H / V5-Q / V10-M; round-trip with 0 / 5 / 6-flip noise (within and beyond the V1-M budget).

### D7 — Bit Stream → Text + `DecodeMatrix` Public API `(M)`

Goal: parse the data codeword stream back into the source text, then expose it as a public function.

- [x] Read 4-bit mode indicator and dispatch by mode via `decodeText` + `bitReader`.
- [x] Read character count indicator using `Mode.CharCountBits(v)` (reuses from M3).
- [x] Per-mode decoder: `decodeNumeric` (10 / 7 / 4 bit groups), `decodeAlphanumeric` (11 / 6 bit), `decodeByteMode` (raw 8-bit → UTF-8).
- [x] Stop at terminator (`0000`) or when fewer than 4 bits remain; pad bytes are ignored implicitly.
- [x] Public API: `qrgen.DecodeMatrix([][]bool) (string, error)` in `qrgen/decode.go` — runs D4 → D5 → D6 → D7.
- [x] Tests: 15 round-trip cases across modes, EC levels, V1..V10, multi-block, version-info, and forced version+mask, plus typed-error coverage for corrupted input.

### Checkpoint 1 — Matrix-to-Text decoder is feature-complete.

### D8 — Image Preprocessing `(S)`

Goal: turn an arbitrary `image.Image` into a binary 2D grid suitable for pattern detection.

- [x] Convert to single-channel grayscale (handle `image.Gray`, `image.RGBA`, `image.NRGBA`). — `qrgen/decode_image.go` `imageToGrayscale` via `color.GrayModel`.
- [x] **Otsu thresholding** — find the global threshold that maximises between-class variance. — `qrgen/decode_image.go` `otsuThreshold`.
- [ ] Optional local thresholding fallback for highly non-uniform images (Sauvola or block-based). — deferred to v0.3 per the open question in §8.
- [x] Return a `bitmap` struct (width, height, `[]bool` for cells). — `qrgen/decode_image.go` `bitmap` + `get` helper.
- [x] Tests: monochrome edge cases, bimodal histograms, sub-images with non-zero bounds, and a per-module integration check that every cell of an encoder-generated PNG comes back classified correctly through `binarise`.

### D9 — Finder Pattern Detection `(M)`

Goal: locate the three finder patterns in the bitmap.

- [x] Horizontal scan for the **1:1:3:1:1 dark/light ratio** across rows. — `qrgen/decode_image.go` `scanRowForFinders`.
- [x] Vertical scan to confirm candidates. — `crossCheckVertical` cross-validates each row hit and refines the centre y.
- [x] Cluster candidate centres and validate **triple geometry** (right-angle triangle, similar module sizes). — `clusterFinderCandidates` + `orderFinderTriple` (leg ratio < 1.5, Pythagoras within 15%).
- [x] Compute estimated module pitch from finder spacing. — exposed via `finderCandidate.moduleSize`, averaged from horizontal and vertical fits.
- [x] Return three `(x, y)` centres ordered as top-left, top-right, bottom-left. — assumes right-side-up symbol; full-rotation support deferred.
- [x] `ErrFinderNotFound` exported when fewer than three valid finders survive clustering or the triple's geometry is implausible.
- [x] Tests: V1 and forced-V5 encoder PNGs with centre-within-2px tolerance, plus all-white rejection and a wiped-corner negative case. Arbitrary rotations are roadmap.

### D10 — Perspective Transform `(M)`

Goal: map source pixel coordinates → grid module coordinates.

- [x] Estimate the fourth (bottom-right) corner from the three finder centres + version-dependent geometry. — `qrgen/decode_image.go` `estimateBottomRight` (parallelogram completion) plus `estimateVersion` from `(distance − 14) / 4 + 1`.
- [x] Compute a **3×3 homography matrix** mapping (matrix module coords) → (source pixel coords). — `computeHomography` solves the standard 8×8 linear system with `solveLinear8` (Gaussian elimination + partial pivoting + singularity guard).
- [x] Provide an inverse map for sampling. — `homography.apply(col, row)` is the forward map used directly by the sampler; for the QR decoder we only need module → pixel so there is no separate inverse helper.
- [x] Tests: identity round-trip, translate-and-scale, degenerate (collinear) input returns an error, version estimation across V1/V5/V10, and a per-module sample of HELLO WORLD V1 where every sampled bit matches the original matrix.

### D11 — Alignment Pattern Refinement (V2+) `(S)`

Goal: refine the perspective transform using alignment patterns to reduce sampling error at high versions.

- [x] For each expected alignment-pattern centre, search a small window in the source image for a 5×5 alignment pattern. — `qrgen/decode_image.go` `checkAlignmentAt` + `searchAlignmentPattern`.
- [x] Adjust the homography by replacing the parallelogram-completed BR anchor with the located alignment-pattern centre. — `refineHomography`.
- [x] Skip cleanly when no alignment pattern is found (V1 always; very damaged symbols). — falls back to the input transform if the search window contains no valid pattern or if the recomputed system is singular.

### D12 — Module Sampling + `Decode` Public API `(M)`

Goal: tie the image and matrix pipelines together.

- [x] At each module centre, sample the source image (point sample) and threshold. — `qrgen/decode_image.go` `sampleMatrix` reads one pixel per module from the binarised bitmap via the refined homography.
- [x] Build a `[][]bool` matrix and hand off to `DecodeMatrix`. — `decodeImage` chains the whole image stage and delegates to the matrix decoder.
- [x] Public API: `qrgen.Decode(img image.Image) (string, error)` and convenience `qrgen.DecodeBytes(data []byte) (string, error)`. — both live in `qrgen/decode.go`; `DecodeBytes` registers PNG/JPEG/GIF codecs.
- [x] Tests: 10-case Encode → DecodeBytes round-trip across modes, V1..V10, multi-block, custom colours, larger modules / quiet zone, plus `Decode(image.Image)` and invalid-bytes error paths.

### Checkpoint 2 — Image-to-Text decoder is feature-complete.

### D13 — Quality Gate `(M)`

Goal: ensure the decoder is robust before release.

- [x] Cross-validation: encode → decode round-trip across the same 12-case matrix used for the encoder's `roundtrip_test.go`, now closing the loop with our own `DecodeBytes`. — `qrgen/decode_roundtrip_test.go` `TestRoundTripWithOwnDecoder`.
- [x] Robustness: deliberately corrupt N modules in the matrix (within RS capacity) and assert recovery. — `TestRoundTripRobustnessFlippedBits` covers V1-Q with 3 flips and V1-H with 5 flips.
- [x] Image robustness: render with custom colours, low contrast, varied module sizes, larger quiet zone. — `TestRoundTripImageRobustness`. Arbitrary rotations stay roadmap.
- [x] Benchmarks: `BenchmarkDecodeMatrixSmall` / `MultiBlock`, `BenchmarkDecodeImageSmall` / `MultiBlock` / `URL`, plus `FromPNGDecode` that isolates CV cost from PNG parsing. — `qrgen/decode_bench_test.go`.
- [x] `go test -race ./...` remains clean.

### D14 — Polish & Release `(S)`

Goal: cut `v0.2.0`.

- [ ] README updates: new API summary rows, decode usage examples, updated Limitations (decoder added, ECI/Kanji still pending).
- [ ] `CHANGELOG.md` `v0.2.0` entry under "Added" and "Validated".
- [ ] New `examples/decode/main.go` showing `Decode` on a saved PNG.
- [ ] Tag `v0.2.0`.

---

## 5. Proposed Folder Layout Delta

Decoder code lands alongside the encoder in the same package, so users can call `qrgen.Decode` symmetrically with `qrgen.Encode`. Suggested file split:

```
qrgen/
├── decode.go              # public Decode / DecodeMatrix / DecodeBytes
├── decode_matrix.go       # mask reversal, data-area walk in reverse, bit-stream parsing
├── decode_image.go        # binarisation, finder detection, homography, module sampling
├── rs_decode.go           # syndromes, Berlekamp-Massey, Chien, Forney
├── format_decode.go       # 15-bit format-info reader with BCH error correction
├── gf256.go               # extended with Inverse, PolyDivQR, PolyEval, PolyDeriv
└── *_test.go              # mirror tests per file
```

## 6. Risks & Technical Notes

- **Berlekamp–Massey correctness** is the highest-bug-risk component. Mitigation: validate against the worked example (HELLO WORLD's 10 EC codewords with deliberate corruption) plus property tests over random blocks.
- **Finder-pattern false positives** can defeat detection on busy backgrounds. Mitigation: require the right-angle geometry check; reject triples where the inter-finder distances are wildly different.
- **Homography numerical stability** at high versions. Mitigation: use `float64` throughout, prefer least-squares over direct inversion.
- **UTF-8 handling on decode** mirrors the encoder limitation: byte-mode bytes are treated as UTF-8 without consulting any ECI segment. This is documented as a known limitation, not a bug.
- **Library size** grows materially. We should not regress the encoder benchmarks; consider keeping decoder behind a build tag if it ever bloats the binary for encode-only users. Probably not needed for v0.2 but worth measuring.

## 7. References

- ISO/IEC 18004:2015 — §9 (Reference decode algorithm), Annex C / D (BCH codes), Annex E (alignment positions).
- Berlekamp, E. R. — *Algebraic Coding Theory* (1968), the original Berlekamp algorithm.
- Massey, J. L. — "Shift-Register Synthesis and BCH Decoding," IEEE Trans. Info. Theory, 1969.
- Forney, G. D. — "On Decoding BCH Codes," IEEE Trans. Info. Theory, 1965.
- ZXing project — *open-source decoder reference*: <https://github.com/zxing/zxing>.
- Project Nayuki — *QR Code generator library, decoder companion notes*.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Berlekamp–Massey vs. Peterson–Gorenstein–Zierler.** The latter is simpler conceptually but less efficient. We'll start with Berlekamp–Massey to match the QR community standard.
- **Local vs. global binarisation.** Otsu works for most synthetic inputs; do we ship local thresholding in v0.2 or defer to v0.3?
- **Image-side input formats.** Just `image.Image`, or also raw bytes + content-type sniffing (PNG / JPEG / etc.)? `image.Decode` already covers PNG/JPEG/GIF, so probably "just `image.Image`" plus a `DecodeBytes` convenience.
- **Error type hierarchy.** Sentinel errors (`var ErrXxx = errors.New(...)`) or a typed `DecodeError struct`? Default to sentinels for v0.2; revisit if callers want richer info.
