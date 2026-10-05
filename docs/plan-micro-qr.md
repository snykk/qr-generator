# QR Generator — Micro QR Code Plan

This document describes the implementation plan for **Micro QR Code** (versions M1 to M4) targeting the `v0.15.0` minor release. Micro QR is the last symbology in ISO/IEC 18004 this library does not produce. The milestone delivers the encoder for all eight Micro symbol types plus a matrix-level decoder, so Micro symbols round-trip through `DecodeMatrix`; locating Micro symbols in photographs is left to a follow-up milestone.

> Status: **draft / living document.** Milestones MQ1..MQ6 land incrementally on the `micro-qr` branch; each is a focused commit (or small series) with tests, matching the cadence of prior milestones.

> Indonesian version: [docs/plan-micro-qr.id.md](plan-micro-qr.id.md).

---

## 1. Vision & Goals

- Encode Micro QR M1, M2, M3, and M4 at every error-correction level they define: eight symbol types in all.
- Make the existing API produce Micro symbols through options, the way ECI, logos, and terminal output were added, so every encode function and renderer gains Micro support at once.
- Decode Micro symbols at the matrix level: `DecodeMatrix` recognises the Micro sizes (11, 13, 15, 17) on its own, since they can never be QR sizes (21 + 4k).
- Validate against **independent** implementations, never only against ourselves. v0.14.1 just fixed a format-information bug that survived thirteen releases because our decoder only ever read our own encoder's output; Micro has a single format strip, so the same kind of mistake would make every Micro symbol unreadable by real scanners.
- Keep **zero runtime dependencies**, and keep test fixtures as text matrices.
- Be honest about scope: no image-level Micro detection in this milestone.

## 2. What Micro QR is

The numbers below come from the ISO/IEC 18004:2015 tables as transcribed in segno 1.6.6, and were cross-checked against zint on 69 symbols (57 byte-identical; the other 12 differ only in padding, where segno deviates from the standard; see section 3).

| Symbol number | Type | Size | Total codewords | Data codewords | EC codewords | Data bits |
|---|---|---|---|---|---|---|
| 0 | M1 (error detection only) | 11 | 5 | 3 (last is 4 bits) | 2 | 20 |
| 1 | M2-L | 13 | 10 | 5 | 5 | 40 |
| 2 | M2-M | 13 | 10 | 4 | 6 | 32 |
| 3 | M3-L | 15 | 17 | 11 (last is 4 bits) | 6 | 84 |
| 4 | M3-M | 15 | 17 | 9 (last is 4 bits) | 8 | 68 |
| 5 | M4-L | 17 | 24 | 16 | 8 | 128 |
| 6 | M4-M | 17 | 24 | 14 | 10 | 112 |
| 7 | M4-Q | 17 | 24 | 10 | 14 | 80 |

Every type uses a single Reed-Solomon block. There is no H level, and no ECI, structured append, or FNC1.

- **Modes.** The mode indicator is 0, 1, 2, or 3 bits wide for M1 to M4, with values numeric 0, alphanumeric 1, byte 2, Kanji 3. M1 supports numeric only; M2 adds alphanumeric; M3 and M4 support all four modes. Character-count widths are numeric 3/4/5/6, alphanumeric -/3/4/5, byte -/-/4/5, and Kanji -/-/3/4 for M1 to M4.
- **Terminator.** 3, 5, 7, or 9 zero bits for M1 to M4, truncated if the capacity is reached first.
- **Padding.** After the terminator, zero bits pad to the next codeword boundary (none if already on one), then `11101100` and `00010001` alternate through the remaining full data codewords. In M1 and M3 the final data codeword is only 4 bits long and pads as `0000`. For Reed-Solomon that half codeword is a byte with its 4 bits in the high nibble; only those 4 bits are placed in the symbol.
- **Layout.** One finder pattern at the top-left, separated from the data only on its right and bottom (an 8 by 8 block). Timing patterns run along row 0 and column 0 from index 8, dark at even indices. There is no alignment pattern, no version information, and no dark module.
- **Format information.** A single 15-bit copy: column 8 rows 1..8 hold bits 0..7, and row 8 columns 1..7 hold bits 14..8. The word is BCH(15,5) over the 3-bit symbol number and the 2-bit mask, with the same generator as QR and XOR mask `0x4445`.
- **Data placement.** The QR zigzag in two-module columns from the right, first pair upward, but without QR's column-6 skip: column 0 is timing, and column 6 below the finder holds data.
- **Masks.** Four patterns, Micro masks 0..3 equal QR masks 1, 4, 6, and 7. Selection scores the dark modules in the last column (SUM1) and the last row (SUM2), each excluding the timing module: `SUM1*16 + SUM2` if `SUM1 <= SUM2`, otherwise `SUM2*16 + SUM1`. The **highest** score wins, the opposite of QR's lowest penalty; segno breaks ties toward the lowest index.

## 3. Oracles, and what they taught us

gozxing, this library's test oracle so far, has no Micro QR support, so this milestone brings in reference implementations used offline during development and never as module dependencies.

- **zint**, reached through zxing-cpp 2.3.0's `create_barcode`, is a spec-careful encoder and becomes the **golden source**. It produces all eight types: `ec_level` `"1"`, `"2"`, and `"3"` select L, M, and Q, and `"4"` is rejected with zint's own "Error correction level H not available". It chooses the version and mask itself; both are read back from the symbol's format information.
- **zxing-cpp's reader** decodes Micro QR images: an independent decoder for our rendered output.
- **segno** transcribes the spec tables correctly, but its padding deviates in two places. It zero-fills all padding in M1 and M3 instead of alternating the pad codewords. And when the terminator lands exactly on a codeword boundary it still appends eight zero bits: `"1234"` in M4-Q is exactly 32 bits, and segno shifts every pad codeword by one. Decoders ignore pad bits, so segno's symbols scan, but goldens taken from segno would have made a correct encoder look wrong. The tables are cross-checked with segno; the goldens come from zint.
- **Segmentation ties.** Two optimal encoders can split mixed-mode text differently and both be right. Byte-exact goldens therefore use single-mode inputs; mixed-mode inputs are checked by decoding instead.

## 4. Approach, grounded in the code

A read of the encoder (mode.go, version.go, segment.go, encode.go, matrix.go, mask.go, formatinfo.go, options.go, api.go, the renderers, and the matrix decoder) sorts the work into three groups.

- **Reusable as-is:** `bitBuffer` (its zero-filled trailing partial byte is exactly the half-codeword RS convention), the payload writers, GF(256) and `encodeBlock`, the matrix primitives and `placeSingleFinder` (its edge clipping already yields the Micro separator), `maskCondition` through the mapping 0→1, 1→4, 2→6, 3→7, `applyMask`, `ECLevel`, all three renderers (they read only `m.size` and `m.modules`), the option pattern, and the decoder's payload readers and `rsDecode`.
- **Micro-specific, in parallel:** a `MicroVersion` type and its tables; mode-indicator, count-width, and terminator tables; a cost model injected into the segmentation DP; `encodeMicroText` with Micro padding; a bit-exact data-plus-EC stream (36 / 80 / 132 / 192 bits); timing and format reservation; a single-copy format writer and reader; four-mask, highest-score selection; `microMatrixFromData`; option flags and validation; and `decodeMicroMatrix`.
- **Hidden assumptions to avoid:** capacity computed as codewords times 8; the fixed 4-bit mode indicator and terminator; the DP's assumption that byte mode is always available (M1 and M2 can make an input infeasible, which must surface as "does not fit" rather than a bogus segment); `Version(0).Size()` returning 17 (Micro never goes into the `Version` type); the column-6 skip; the 4-module default quiet zone; the EC default having no "unset" state; and mask selection's direction.

## 5. Design decisions

- **API by options.** `WithMicro()` selects the smallest Micro symbol that fits; `WithMicroVersion(v MicroVersion)` forces one, with `type MicroVersion int` and constants `M1`..`M4`. One branch in `buildMatrix` gives Micro to `Encode`, `EncodeToFile`, `EncodeSVG`, `EncodeSVGToFile`, `EncodeTerminal`, and `Matrix`. `validate` rejects, with specific errors, `WithVersion` together with Micro, `WithECI`, `WithLogo` (it would force the H level Micro lacks), EC level H, EC level Q below M4, a mask above 3, and `EncodeStructuredAppend` with Micro.
- **EC policy, including M1.** The options gain a flag recording whether the EC level was set explicitly. When it was not, the smallest symbol wins and, within a version, M is preferred over L: the order is M1, M2-M, M2-L, M3-M, M3-L, M4-M, M4-L, never Q. That is exactly zint's automatic choice, measured on probes such as nine digits (M2-L, not M3-M) and 35 digits (M4-L). An explicit level selects only the types that offer it: L gives M2-L to M4-L, M gives M2-M to M4-M, Q gives M4-Q only, and H is an error. M1 provides error detection only, so it is never chosen for an explicit level. zint does pick M1 for an explicit L; that is the one deliberate divergence from it, pinned in the tests.
- **No silent fallback.** If the data does not fit M4 at the requested level, the encoder returns a Micro-specific capacity error instead of quietly producing a full QR symbol.
- **Quiet zone.** Micro defaults to the spec's 2-module quiet zone; an explicit `WithQuietZone` still wins. This needs a "set" flag, since a negative sentinel would break the existing rejection of negative quiet zones.
- **Empty input.** Valid in M1 under the spec (a zero count), and the one case where M1 has a full pad codeword. zint refuses empty input, so this case is checked against a hand-derived bit stream and our own round trip rather than a golden.
- **Matrix decoding.** `decodeMatrixSA` routes 11/13/15/17 grids to `decodeMicroMatrix` before the QR size check. It reads the single format copy with a 3-bit budget and requires the decoded symbol number to match the grid size. M1 is detection-only: any non-zero syndrome fails the decode rather than being corrected.
- **Images, later.** `Decode`, `DecodeBytes`, and `DecodeAll` keep needing three finder patterns. Finding a Micro symbol (one finder plus timing patterns) is a follow-up milestone.

## 6. Milestones

Milestones land sequentially. **Checkpoint A** (after MQ5) round-trips every Micro type and matches zint byte for byte. **Checkpoint B** (MQ6) is the `v0.15.0` release.

### MQ1 — Plan Doc `(S)`

- [x] This document and its Indonesian counterpart.

### MQ2 — Reference Doc `(S)`

- [x] `docs/theory/26-micro-qr.md` (EN + ID): symbol types and capacities, modes and bit widths, terminator and padding including the half codeword, layout and format information, placement, masks and their scoring, how Micro differs from QR at each step, and the hand-derived bit streams that pin the encoding. Plus the theory README entry and code-mapping row.

### MQ3 — Tables, Segmentation, and Bit Stream `(L)`

- [x] `MicroVersion`, the symbol-type table, and the mode, count-width, and terminator tables.
- [x] Inject the cost model (allowed modes and header bits) into the segmentation DP, keeping `segmentText` as the QR wrapper, and report infeasibility.
- [x] `encodeMicroText`: version and EC selection, segments, terminator, padding with the half codeword, and the bit-exact data-plus-EC stream.
- [x] Tests: the hand-derived bit streams, and data codewords read from zint symbols.

### MQ4 — Matrix, Masks, and Options `(L)`

- [x] The Micro matrix: finder, separator, timing, format reservation, placement without the column-6 skip, single-copy format writer, and four-mask selection.
- [x] `WithMicro`, `WithMicroVersion`, the EC-set and quiet-zone-set flags, and validation.
- [x] Golden tests: our encoder pinned to zint's type and mask reproduces every zint symbol module for module, and our automatic choices of type and mask match zint's for the default EC level.

### MQ5 — Matrix Decoder and Round Trips `(M)`

- [x] `decodeMicroMatrix`: format read, unmasking, the bit-exact stream, RS (detection-only for M1), and Micro segment parsing; `DecodeMatrix` dispatch by size.
- [x] Tests: decode every zint fixture, round-trip our encoder across modes and types, and adjust the existing test that uses 17 as an invalid QR size.
- [x] One-off check, reported in the commit: zxing-cpp reads our rendered Micro PNGs.

### Checkpoint A — every Micro type round-trips and matches zint.

### MQ6 — Polish & Release `(S)`

- [x] README (encoding section, scope, limitations, roadmap), `examples/encode/micro`, and CLI flags.
- [x] `CHANGELOG.md` `v0.15.0` entry, left unstaged for the release branch.
- [x] Drop the planned mark from the doc 26 row of the theory code map, so it does not go stale the way rows 11 to 25 did.
- [x] `go test -race ./...` clean, gofmt-clean.

## 7. Validation strategy

1. **Byte-exact goldens from zint.** All eight types, including capacity-filling inputs, short inputs with pad codewords, the exact-boundary case (`"1234"` in M4-Q), and M1/M3 half-codeword cases, stored as text in `qrgen/testdata/thirdparty/micro_symbols.txt` next to the QR fixtures added in v0.14.1.
2. **Decoding.** `DecodeMatrix` reads every zint fixture.
3. **Round trips** of our own encoder, across modes and types; necessary but never sufficient on its own.
4. **An independent decoder.** zxing-cpp reads our rendered PNGs, once per type, reported in the commit since Go tests cannot call Python.
5. **Hand-derived bit streams.** Five cases already checked bit for bit against the rules: M1 `"1"` and `"12345"` (full capacity, no terminator), M2-L `"0123456789"` (terminator truncated to one bit), M4-Q `"1234"`, and M3-M `"HELLO WORLD"` (ending in the half codeword `1010`).
6. **Format words.** The computed BCH table matches segno's 32-entry `FORMAT_INFO_MICRO`.

## 8. Proposed File Layout Delta

```
qrgen/
├── micro.go                # new — MicroVersion, symbol-type and bit-width tables
├── micro_encode.go         # new — cost model, encodeMicroText, padding, bit stream
├── micro_matrix.go         # new — layout, format information, masks
├── micro_decode.go         # new — decodeMicroMatrix
├── micro_test.go           # new — tables, bit streams, round trips
├── micro_golden_test.go    # new — zint goldens and decoding
├── segment.go              # cost model injection; QR behaviour unchanged
├── options.go, encode.go   # Micro options, flags, validation, buildMatrix branch
├── decode.go               # DecodeMatrix dispatch by size
└── testdata/thirdparty/micro_symbols.txt   # new — zint fixtures
docs/
├── plan-micro-qr.md / .id.md
└── theory/26-micro-qr.md / .id.md
examples/encode/micro/main.go
```

## 9. Risks & Technical Notes

- **The self-consistency trap.** Our own round trips prove nothing about conformance; the zint goldens and the zxing-cpp check are the real acceptance criteria.
- **Refactoring the segmentation DP touches the QR path.** The QR third-party fixtures from v0.14.1 must stay byte-identical, alongside the existing tests.
- **Mask ties.** zint's tie-breaking may differ from segno's lowest-index rule. Pinned goldens are unaffected; the automatic-choice check will reveal it, and any difference gets documented rather than papered over.
- **Kanji in M3/M4.** zint must actually choose Kanji mode for a Kanji input for a byte-exact golden; if it does not, that case becomes decode-only.
- **New option flags.** The EC-set and quiet-zone-set flags must leave QR defaults and behaviour exactly as they are.
- **M1 semantics.** Detection-only means the decoder must refuse to correct, which `rsDecode` does not do by default.

## 10. References

- ISO/IEC 18004:2015 — the Micro QR tables, layout, format information, masking, and padding rules.
- segno 1.6.6 source (`consts.py`, `encoder.py`) — the tables, cross-checked; its two padding deviations are recorded in section 3.
- zint, through zxing-cpp 2.3.0 — the golden encoder; zxing-cpp's reader as the independent decoder.
- `docs/theory/05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `17-optimal-segmentation.md`, `22-kanji-mode.md` — the QR counterparts this milestone parallels.

## 11. Open Questions

All four were resolved during the milestone:

- **Constant names.** Resolved: `M1`..`M4`, reading naturally as `qrgen.M2`.
- **Empty input.** Resolved: follow the spec. An empty string encodes as M1 with a zero count; zint refuses to generate it, but zxing-cpp decodes our symbol without error to empty content.
- **CLI shape.** Resolved in MQ6: `-micro` plus `-micro-version M1..M4`, mirroring `WithMicro` and `WithMicroVersion`; `-version` stays QR-only. `-ec` and `-quiet-zone` now default to unset, so Micro's defaults can apply while QR output stays byte-identical.
- **Image detection.** Still open: locating Micro symbols in photographs (one finder plus timing patterns) is a candidate milestone after v0.15.0.
