# QR Generator — rMQR Plan

This document describes the implementation plan for **rMQR** (Rectangular Micro QR Code, ISO/IEC 23941), targeting the `v0.17.0` minor release. rMQR is the third member of the QR family after QR Code and Micro QR: a rectangular symbol, 7 to 17 modules tall and 27 to 139 wide, built for the long, narrow spaces a square symbol cannot use. The milestone delivers the encoder for all 32 versions at both of their error-correction levels, plus a matrix-level decoder, so rMQR symbols round-trip through `DecodeMatrix`; locating rMQR symbols in images is left to a follow-up milestone, as it was for Micro QR.

> Status: **draft / living document.** Milestones RQ1..RQ6 land incrementally on the `rmqr` branch; each is a focused commit (or small series) with tests, matching the cadence of prior milestones.

> Indonesian version: [docs/plan-rmqr.id.md](plan-rmqr.id.md).

---

## 1. Vision & Goals

- Encode rMQR at all 32 versions, R7x43 to R17x139, at levels M and H: 64 symbol types in all.
- Make the existing API produce rMQR through options, the way Micro QR was added, so every encode function and renderer gains rMQR at once.
- Decode rMQR at the matrix level: `DecodeMatrix` accepts a rectangular grid and recognises an rMQR symbol by its shape, since no rMQR size is square.
- Validate against **independent** implementations, never only against ourselves: zint for byte-exact goldens, zxing-cpp's reader as an independent decoder, and every table cross-checked across three sources.
- Keep **zero runtime dependencies**, and keep test fixtures as text matrices.
- Be honest about scope: no image-level rMQR detection in this milestone.

## 2. What rMQR is

The facts below come from three implementations that share no code (section 3), which agree on every table; ISO/IEC 23941:2022 itself was not available, but its table numbers are cited from the implementations' source comments. Theory doc 28 holds the full per-version tables.

- **Versions.** 32 sizes named R*h*x*w*: heights 7, 9, 11, 13, 15, 17 and widths 43, 59, 77, 99, 139, plus width 27 for heights 11 and 13. Versions are numbered 1 to 32 in that order (R7x43 is 1, R11x27 is 11, R17x139 is 32), and the format information carries the number minus one.
- **Error correction.** Two levels only, M and H. A single Reed-Solomon block in small symbols, up to six in R17x139-H, interleaved exactly as in QR (short blocks first, one extra data codeword in the long ones). Data capacity runs from 3 codewords (R7x43-H) to 152 (R17x139-M).
- **Modes.** A 3-bit mode indicator for every version: terminator `000`, numeric `001`, alphanumeric `010`, byte `011`, Kanji `100`, FNC1 `101` and `110`, ECI `111`. The character-count widths depend on the version, not on a version group: numeric 4 to 9 bits, alphanumeric 3 to 8, byte 3 to 8, Kanji 2 to 7.
- **Terminator and padding.** A 3-bit terminator, truncated if the capacity runs out first, then zero bits to the codeword boundary and the QR pad codewords `11101100` and `00010001`. There are no half codewords.
- **Layout.** A 7 by 7 finder pattern at the top-left, with a separator on its right and, from height 9, below it; a 5 by 5 finder sub-pattern at the bottom-right; L-shaped corner finder patterns at the top-right and bottom-left; timing patterns along all four edges; and, for every width but 27, alignment patterns at the top and bottom edges joined by vertical timing patterns, at columns 21 (width 43), 19 and 39 (59), 25 and 51 (77), 23, 49, and 75 (99), and 27, 55, 83, and 111 (139).
- **Format information.** Two 18-bit copies: 6 data bits (the level, H as 1, above the 5-bit version indicator) protected by BCH(18,6) with QR's version-information generator `0x1F25`, then XORed with `0x1FAB2` for the copy beside the finder and `0x20A7B` for the copy beside the sub-pattern. The code has minimum distance 8, so each copy corrects up to 3 errors.
- **Placement.** QR's zigzag in two-module columns, but starting at the second column from the right, since the rightmost column is timing, and with no column skipped. Between 0 and 7 remainder bits fill the modules after the last codeword.
- **Masking.** A single fixed mask, QR's mask 4, `(⌊row / 2⌋ + ⌊column / 3⌋) mod 2 = 0`, applied to data and remainder modules. There is no mask selection and no mask in the format information.
- **Quiet zone.** 2 modules, as for Micro QR.

## 3. Oracles, and what they taught us

gozxing has no rMQR support, so as in v0.15 the milestone uses reference implementations offline, in a scratch virtual environment, never as module dependencies.

- **zint**, reached through zint-bindings 1.2.2, is the **golden source**. Unlike zxing-cpp's `create_barcode`, which v0.15 used for Micro QR but which cannot choose a symbol size, zint-bindings exposes zint's own options, so every version and level can be forced (`option_2` 1 to 32, `option_1` 2 for M and 4 for H).
- **zxing-cpp's reader** decodes rMQR: an independent decoder for our output. Its source (QRVersion, QRFormatInformation, QRBitMatrixParser, QRCodecMode) was read as a second transcription of the tables.
- **rmqrcode 0.3.2**, a Python encoder, is a third transcription. It agrees with the other two everywhere except two block definitions, R13x27-M (14 data codewords where zint and zxing-cpp have 12) and R17x43-M (60 codewords per block where they have 61), so, like segno in v0.15, it confirms tables but never supplies goldens.
- **Already checked while planning.** The three sources agree on every size, codeword count, block structure, character-count width, and alignment column; the 64 format words of each copy are reproduced exactly by the BCH computation and the two masks; and a layout model built from the facts above matches 64 zint symbols, every version at both levels, on every function module and both format copies, with remainder bits that agree with rmqrcode.
- **zint's automatic choice.** With no level requested, zint takes the smallest-area version that fits at M, then uses H instead when the data, without its terminator, is shorter than H's capacity in that same version: a probe of every slack from 0 to 3 bits showed it never upgrades when the data fills H exactly. With a level requested, it takes the smallest-area version that fits at that level. R7x99 ties R9x77 in area (693 modules) but is never chosen automatically, since a smaller symbol always holds more.

## 4. Approach, grounded in the code

A read of the encoder and decoder after v0.16 sorts the work into three groups.

- **Reusable as-is:** `bitBuffer` and the payload writers; the segmentation DP with its injectable cost model from v0.15; GF(256), `encodeBlock`, and `rsDecode`; `maskCondition(4, …)`; the ECI designator writer and reader; the decoder's payload readers (`decodeNumeric` and the rest); the option pattern with the EC-set and quiet-zone-set flags; and, once they accept rectangles, the three renderers.
- **rMQR-specific, in parallel:** an `RMQRVersion` type and the 32-version tables; the rMQR cost model (3-bit indicator, per-version count widths); `encodeRMQRText` with version and level selection, terminator, padding, block split, and interleaving; a BCH(18,6) word function; the function-pattern layout; the two-copy format writer and reader; placement from the second column; the matrix decoder and its segment parser; options and validation.
- **Hidden assumptions to avoid:** the `matrix` type's single `size`, which the PNG, SVG, and terminal renderers, `Matrix`, `applyMask`, and `placeBits` all read as both height and width; `matrixFromGrid` and `DecodeMatrix` rejecting non-square grids; `placeBits` starting its column pairs at the rightmost column, which for rMQR would pair the columns wrongly while still round-tripping internally; 15-bit format words; the QR `Version` type, which rMQR must stay out of, as Micro did; ECLevel L and Q being valid; mask selection; and QR's 4-bit ECI indicator.

## 5. Design decisions

- **API by options (open question 1).** `WithRMQR()` selects the smallest symbol that fits; `WithRMQRVersion(v RMQRVersion)` forces one, with constants `R7x43` to `R17x139` named as the standard names them; and `WithRMQRHeight(rows int)` fixes the height and lets the width follow, the case rMQR exists for (zint offers the same choice as R7xW to R17xW). One branch in `buildMatrix` gives rMQR to `Encode`, `EncodeToFile`, `EncodeSVG`, `EncodeSVGToFile`, `EncodeTerminal`, and `Matrix`.
- **Validation.** Rejected with specific errors: `WithVersion`, `WithMicro` or `WithMicroVersion` together with rMQR; `WithMask` (rMQR's mask is fixed); `WithLogo` (a 7 to 17 module tall symbol has no room for one); EC levels L and Q; `EncodeStructuredAppend` (rMQR defines none). `WithECI` is allowed, since rMQR defines ECI: the header is the 3-bit indicator `111` followed by the same designator as QR.
- **Level and size policy: zint's.** With no level set, the smallest-area version that fits at M, then H if the data leaves at least one bit spare at H in that version: the extra correction costs nothing. An explicit M or H selects the smallest-area version at that level. With a height fixed, the same rules apply among that height's widths. If nothing fits, an `ErrCapacityExceeded` error, never a silent QR symbol.
- **Rectangular matrices.** The `matrix` type gains separate row and column counts; the renderers, `Matrix`, `applyMask`, and `placeBits` read them; QR and Micro keep square matrices and byte-identical output.
- **Quiet zone.** 2 modules by default, as the standard asks; an explicit `WithQuietZone` still wins.
- **Matrix decoding.** `DecodeMatrix` sends a rectangular grid whose shape is one of the 32 rMQR sizes to `decodeRMQRMatrix`. It reads both format copies, decodes each against its own 64 words within 3 bits, takes the nearer, and requires the version to match the grid's shape. Then it unmasks, reads the stream from the second column, deinterleaves, corrects each block, and parses the segments, ECI included.
- **Images, later.** `Decode`, `DecodeBytes`, and `DecodeAll` do not look for rMQR symbols yet; a finder plus a finder sub-pattern at the opposite corner is a follow-up milestone.

## 6. Milestones

Milestones land sequentially. **Checkpoint A** (after RQ5) round-trips every rMQR type and matches zint symbol for symbol. **Checkpoint B** (RQ6) is the `v0.17.0` release.

### RQ1 — Plan Doc `(S)`

- [x] This document and its Indonesian counterpart.

### RQ2 — Reference Doc `(S)`

- [x] `docs/theory/28-rmqr.md` (EN + ID): what rMQR is for; the 32 versions and their codeword, block, and remainder tables; modes and the per-version count widths; terminator and padding; error correction and interleaving; the layout, function pattern by function pattern; the two-copy format information with its BCH code and masks; placement from the second column; the fixed mask; rMQR against QR and Micro QR; and how the facts were checked. Plus the theory README entry and code-mapping row.

### RQ3 — Tables and Bit Stream `(L)`

- [x] `RMQRVersion`, the 32-version table (size, total and data codewords per level, block counts, remainder bits, alignment columns), and the count-width table.
- [x] The rMQR cost model for the segmentation DP; `encodeRMQRText`: version and level selection, segments (with ECI), terminator, padding, block split, Reed-Solomon, and interleaving (remainder bits come with placement in RQ4). One DP run under the narrowest count widths bounds every version's cost from below, so most versions are ruled out without a DP of their own.
- [x] A BCH(18,6) word function, tested against QR's version-information table (same generator) and against zint's 64 format words of each copy.
- [x] Tests: 70 final codeword streams read back from zint symbols match byte for byte, and our automatic choices match zint's in all 545 auto records.

### RQ4 — Rectangular Matrix, Layout, and Options `(L)`

- [ ] Row and column counts in `matrix`; the renderers, `Matrix`, `applyMask`, and `placeBits` use them, with QR and Micro output unchanged.
- [ ] The rMQR layout, the two format copies, placement from the second column, and the fixed mask.
- [ ] The options of section 5 and their validation.
- [ ] Golden tests: our encoder forced to zint's version and level reproduces every zint symbol module for module, and our automatic choices match zint's.

### RQ5 — Matrix Decoder and Round Trips `(M)`

- [ ] `decodeRMQRMatrix` and the `DecodeMatrix` dispatch by shape.
- [ ] Tests: decode every zint golden, round-trip our encoder across versions, levels, and modes, correct errors up to each block's capacity, and survive damage to one format copy.
- [ ] One-off check, reported in the commit: zxing-cpp reads our rendered rMQR PNGs.

### Checkpoint A — every rMQR type round-trips and matches zint.

### RQ6 — Polish & Release `(S)`

- [ ] CLI flags mirroring the options, README (encoding section, API table, scope, limitations, roadmap), and `examples/encode/rmqr`.
- [ ] `CHANGELOG.md` `v0.17.0` entry, left unstaged for the release branch.
- [ ] The theory code-map row for doc 28, without a planned mark.
- [ ] `go test -race ./...` clean, gofmt-clean.

## 7. Validation strategy

1. **Byte-exact goldens from zint.** Every version at both levels, with single-mode inputs spread across numeric, alphanumeric, byte, and Kanji, plus short inputs with pad codewords, an input whose terminator lands exactly on a codeword boundary, and an ECI input, stored as text in `qrgen/testdata/thirdparty/rmqr_symbols.txt` next to the QR and Micro fixtures.
2. **Decoding.** `DecodeMatrix` reads every zint fixture.
3. **Round trips** of our own encoder; necessary but never sufficient on its own.
4. **An independent decoder.** zxing-cpp reads our rendered PNGs, reported in the commit since Go tests cannot call Python.
5. **Format words.** The computed BCH(18,6) words with both masks equal zint's and zxing-cpp's 64-entry tables.
6. **Automatic choices.** Our version and level choices match zint's for inputs across the whole capacity range.

## 8. Proposed File Layout Delta

```
qrgen/
├── rmqr.go                # new — RMQRVersion, the version and count-width tables
├── rmqr_encode.go         # new — cost model, encodeRMQRText, blocks, interleaving
├── rmqr_matrix.go         # new — layout, format information, placement, mask
├── rmqr_decode.go         # new — decodeRMQRMatrix
├── rmqr_test.go           # new — tables, BCH, bit streams, round trips
├── rmqr_golden_test.go    # new — zint goldens, decoding, automatic choices
├── matrix.go, mask.go     # row and column counts; QR and Micro unchanged
├── render_png.go, render_svg.go, render_terminal.go, api.go   # rectangular output
├── options.go, encode.go  # rMQR options, validation, buildMatrix branch
├── decode.go              # DecodeMatrix dispatch by shape
└── testdata/thirdparty/rmqr_symbols.txt   # new — zint fixtures
cmd/qrgen/main.go          # rMQR flags
docs/
├── plan-rmqr.md / .id.md
└── theory/28-rmqr.md / .id.md
examples/encode/rmqr/main.go
```

## 9. Risks & Technical Notes

- **The self-consistency trap, again.** Placement that starts one column off, or a format copy written in the wrong order, would round-trip through our own decoder and fail everywhere else. The zint goldens and the zxing-cpp check are the acceptance criteria, and the two format copies use different layouts, so each is checked on its own.
- **The rectangular refactor touches QR and Micro rendering.** The third-party QR and Micro fixtures, and every existing renderer test, must stay byte-identical.
- **Segmentation ties.** Two optimal encoders can split mixed text differently; byte-exact goldens use single-mode inputs, and mixed inputs are checked by decoding.
- **The automatic policy has a subtle step** (H when it fits in the version M chose); it is pinned by tests against zint's choices rather than reasoned about.
- **rmqrcode's two block errors** are a reminder that a second transcription is evidence, not proof; agreement between zint and zxing-cpp, an encoder and a reader written independently, is what the tables rest on.

## 10. References

- ISO/IEC 23941:2022 — the rMQR specification; table numbers (Table 3 count widths, Table 8 error correction, Table C.1 format information, Table D.1 alignment columns) as cited in zint's and zxing-cpp's source.
- zint (backend/qr.c and qr.h, BSD-3-Clause), through zint-bindings 1.2.2 — the golden encoder.
- zxing-cpp (core/src/qrcode: QRVersion, QRFormatInformation, QRBitMatrixParser, QRCodecMode) — tables, and the independent reader.
- rmqrcode 0.3.2 (MIT) — a third transcription, with the two block deviations of section 3.
- `docs/theory/26-micro-qr.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `17-optimal-segmentation.md`, `20-eci-segments.md` — the QR and Micro counterparts this milestone parallels.

## 11. Open Questions

- **The public API.** The proposal is `WithRMQR`, `WithRMQRVersion` with constants named after the standard (`R7x43` to `R17x139`), and `WithRMQRHeight` for a fixed height. To confirm before RQ4.
- **Disagreeing format copies.** The proposal takes the nearer of the two decoded words; requiring both to agree would be stricter but would throw away the redundancy the second copy exists for. To settle in RQ5 against damaged-copy tests.
