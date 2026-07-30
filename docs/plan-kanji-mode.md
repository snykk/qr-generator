# QR Encoder — Kanji Mode Plan

This document describes the implementation plan for **Kanji mode** targeting the `v0.11.0` minor release. It closes the "No Kanji mode" limitation and completes the encoder's standard mode set: after numeric, alphanumeric, byte, mixed-mode segmentation, and ECI, Kanji is the last data mode defined by ISO/IEC 18004. Japanese text that currently falls through to byte mode (three UTF-8 bytes, 24 bits per BMP character) can instead be encoded at the spec's native 13 bits per character.

> Status: **draft / living document.** Milestones K1..K6 land incrementally on the `kanji-mode` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M, D, T, R, S, MM, P, TR, ECI, and SA milestones.

> Indonesian version: [docs/plan-kanji-mode.id.md](plan-kanji-mode.id.md).

---

## 1. Vision & Goals

- Implement **Kanji mode** end to end: the encoder recognises Shift-JIS (JIS X 0208) characters and encodes each as a 13-bit value under mode indicator `1000`; the decoder reverses it back to the original runes.
- Make it **part of the optimal segmentation.** Kanji joins numeric, alphanumeric, and byte as a fourth mode in the DP segmenter, so a mixed Japanese/ASCII payload splits across modes to minimise symbol size rather than dropping wholesale to byte mode.
- Keep **zero runtime dependencies.** The Unicode-to-Shift-JIS mapping is generated at development time into a committed, pure-Go table (`kanji_table.go`); the generator is a `//go:build ignore` program run via `go generate` and is the only thing that touches `golang.org/x/text`, which is already an indirect (test-only) module and never enters the library's runtime or normal test build.
- Be **honest about density.** Kanji mode is 13 bits per character versus 24 bits for a BMP kanji in UTF-8 byte mode — roughly 1.8x denser, not the "4x" the current README limitation claims. That figure is corrected as part of this milestone.
- Same philosophy as every prior milestone: pure Go, spec/reference-first with a bilingual doc, table-driven tests plus an encode-decode round-trip and a gozxing cross-check.

## 2. Design Principles

1. **A generated, committed mapping table.** `kanji_table.go` holds the Unicode-to-Shift-JIS and reverse maps for JIS X 0208. It is produced by `gen_kanji_table.go` (`//go:build ignore`, uses `x/text/encoding/japanese.ShiftJIS`) and checked in, so building or testing the library needs no external module. Regenerating is a documented `go generate` step, not part of CI.
2. **The Kanji codec lives in one tested place.** `qrgen/kanji.go` holds the rune-to-13-bit encode and the 13-bit-to-rune decode (the Shift-JIS offset arithmetic), plus eligibility (is a rune in the table). Both directions are unit-tested at the range boundaries.
3. **Kanji is a first-class segmentation mode.** The DP segmenter gains `ModeKanji` with per-character cost 13 bits and a Kanji-eligibility prefix (a rune counts as eligible only if the table maps it). A homogeneous non-Kanji input is unaffected, so existing golden fixtures stay byte-for-byte identical.
4. **Spec-faithful bit layout.** Mode indicator `1000`; character-count indicator 8 / 10 / 12 bits for version groups 1-9 / 10-26 / 27-40 (Kanji's widths, the narrowest of any mode); each character 13 bits via the standard Shift-JIS derivation (section below).
5. **Only JIS X 0208.** The single double-byte set the QR Kanji mode targets. Characters outside it (JIS X 0212, unmapped runes) are simply not Kanji-eligible and fall to byte mode as today; no error, just a mode choice.
6. **Tests first**, including a round-trip: encode Japanese text, decode, assert the exact runes return, plus a gozxing cross-check so an independent decoder agrees, and a table round-trip over every mapped code point.

## 3. The Kanji encoding (reference)

For a Shift-JIS two-byte code `C`:

- If `0x8140 <= C <= 0x9FFC`: `D = C - 0x8140`.
- If `0xE040 <= C <= 0xEBBF`: `D = C - 0xC140`.
- Then the 13-bit value is `(D >> 8) * 0xC0 + (D & 0xFF)`.

Decoding reverses it: from the 13-bit value `V`, `D = ((V / 0xC0) << 8) | (V % 0xC0)`, then add back `0x8140` or `0xC140` depending on which range `D` falls in. The library works in Unicode runes, so encode is rune -> Shift-JIS (table) -> 13-bit, and decode is 13-bit -> Shift-JIS -> rune (table).

## 4. Scope

### In scope for v0.11.0

- `qrgen/kanji.go`: the rune/Shift-JIS/13-bit codec and Kanji eligibility, plus `ModeKanji` (`1000`) in `mode.go` and its 8/10/12 character-count widths.
- `kanji_table.go` (generated, committed) and `gen_kanji_table.go` (`//go:build ignore` generator) with a `go generate` directive.
- Encoder: `ModeKanji` in the DP segmenter, `writeKanji`, and Kanji's contribution to the version-selection bit count.
- Decoder: `decodeText` gains `case 0b1000`, decoding a Kanji segment back to runes.
- Reference doc `docs/theory/22-kanji-mode.md` (EN + ID), a runnable example, and a README usage section plus the density correction.

### Out of scope (still)

- **Character sets other than JIS X 0208** (JIS X 0212, Shift-JIS extensions, other national sets). Runes outside JIS X 0208 stay in byte mode.
- **Kanji combined with a non-default ECI.** v0.11 uses Kanji under the default interpretation; pairing Kanji segments with an explicit ECI is deferred.
- **Micro QR Kanji.** Only the standard 40-version family, as elsewhere.
- **Shipping x/text as a runtime dependency.** It is used only by the ignored generator; the library and its normal tests never import it.

---

## 5. Milestones

Milestones land sequentially. **Checkpoint A** (after K4) gives an encoder that emits Kanji segments whose default path stays byte-identical. **Checkpoint B** (K6) is the `v0.11.0` release.

### K1 — Plan Doc `(S)`

- [ ] `docs/plan-kanji-mode.md` and its Indonesian counterpart covering vision, principles, the encoding reference, scope, milestones K1..K6, file-layout delta, risks, references, and open questions.

### K2 — Kanji Reference Doc `(S)`

Goal: document the mode and the table strategy before any code lands.

- [ ] `docs/theory/22-kanji-mode.md` — the `1000` mode indicator and 8/10/12 count widths; the Shift-JIS two-byte-to-13-bit derivation with a worked example; JIS X 0208 scope; the accurate density figure versus byte mode; and the generated-table strategy (why a committed table keeps runtime zero-dependency). Closes with implementation pointers.
- [ ] Indonesian counterpart `docs/theory/22-kanji-mode.id.md`.
- [ ] Updated `docs/theory/README.md` and `.id.md`: entry 22 plus a code-mapping row pointing at `qrgen/kanji.go`.

### K3 — Shift-JIS Table + Codec `(M)`

Goal: the mapping table and the standalone codec, fully covered.

- [ ] `gen_kanji_table.go` (`//go:build ignore`) generating `kanji_table.go` from `x/text/encoding/japanese.ShiftJIS`, with a `//go:generate` directive; the committed table is pure Go with no imports.
- [ ] `qrgen/kanji.go`: `kanjiEligible(rune) bool`, `kanjiValue(rune) (uint16, bool)` (rune -> 13-bit), and `kanjiRune(uint16) (rune, bool)` (13-bit -> rune).
- [ ] Tests: the offset arithmetic at both Shift-JIS ranges' boundaries; a full table round-trip (every mapped rune encodes and decodes back); a non-Kanji rune reports ineligible.

### K4 — Encoder Integration `(M)`

Goal: Kanji in the segmenter and the writer, default path unchanged.

- [ ] `ModeKanji` added to the DP segmenter's mode set with 13-bit per-character cost and a Kanji-eligibility prefix; `writeKanji` emits the 13-bit values; character-count handling extended for Kanji's 8/10/12 widths; version selection accounts for Kanji segments.
- [ ] Tests: a pure-Kanji string encodes in Kanji mode at the expected bit length; a mixed Japanese/ASCII string segments across Kanji and byte/alphanumeric optimally; a guard test asserting non-Kanji inputs are byte-for-byte identical to the pre-change encoder.

### Checkpoint A — the encoder emits spec-correct Kanji segments; non-Kanji output is byte-identical.

### K5 — Decoder + Round-Trip `(M)`

Goal: decode Kanji and prove the round-trip, including on an independent decoder.

- [ ] `decodeText` gains `case 0b1000`: read the Kanji character count, then each 13-bit value back to a rune via the table.
- [ ] `TestKanjiRoundTrip` encodes pure-Kanji and mixed Japanese/ASCII payloads and confirms `DecodeBytes` recovers the exact text; `TestRoundTripWithThirdPartyDecoder` gains a Kanji case; gozxing reads it identically.

### K6 — Polish & Release `(S)`

Goal: cut `v0.11.0`.

- [ ] README: a Kanji usage note; the Limitations "No Kanji mode" bullet replaced (and its inaccurate "4x" density figure corrected); Scope and Roadmap updated; a runnable example `examples/encode/kanji`.
- [ ] `CHANGELOG.md` `v0.11.0` entry plus compare/tag anchors written; left unstaged in the working tree for the maintainer (mirroring v0.6 through v0.10).
- [ ] `go test -race ./...` clean, gofmt-clean, and `go generate` reproduces the committed table byte-for-byte.
- [ ] Tag `v0.11.0` (left for the maintainer per the established git/release workflow; annotation recommended in the release conversation).

---

## 6. Proposed File Layout Delta

```
qrgen/
├── kanji.go                 # new — rune/Shift-JIS/13-bit codec + eligibility
├── kanji_table.go           # new, generated — the committed JIS X 0208 maps
├── gen_kanji_table.go       # new — //go:build ignore generator (uses x/text)
├── kanji_test.go            # new — codec boundaries + full table round-trip
├── mode.go                  # +ModeKanji (1000) + 8/10/12 count widths
├── segment.go               # +ModeKanji in the DP with eligibility + 13-bit cost
├── encode.go                # writeKanji; Kanji in version selection
├── decode_matrix.go         # decodeText: case 0b1000
├── roundtrip_test.go        # +Kanji gozxing case
docs/
├── plan-kanji-mode.md          # this file
├── plan-kanji-mode.id.md       # Indonesian counterpart
└── theory/
    ├── 22-kanji-mode.md         # new
    └── 22-kanji-mode.id.md      # new
examples/encode/kanji/
└── main.go                  # new — encode + decode Japanese text
```

## 7. Risks & Technical Notes

- **Table size and provenance.** JIS X 0208 has roughly 6,900 characters, so the generated maps are sizeable Go source. They are generated deterministically from `x/text`'s Shift-JIS codec and checked in; `go generate` must reproduce the committed file byte-for-byte, which a K6 check asserts. Regeneration is a rare, documented step.
- **Default path must not move.** Adding Kanji to the DP must not change the segmentation of non-Kanji input — a rune is Kanji-eligible only if the table maps it, and ASCII never is. A guard test compares non-Kanji output against known-good encodings, and the existing gozxing round-trip stays green.
- **Character-count table shape.** The current `charCountBitsTable` is a `[3][3]` indexed by the three data modes; Kanji's `[8, 10, 12]` widths and its mode value need either a fourth row with a remapped index or a small dedicated lookup. Whichever is chosen keeps `CharCountBits` returning 0 for the non-data modes (ECI, structured append).
- **Shift-JIS range gaps.** Not every value in the two Shift-JIS ranges is assigned; the codec must round-trip only through the table, never by blind arithmetic, so an unassigned code is never produced or accepted.
- **Density honesty.** The doc and README state 13 bits/char versus 24 for UTF-8 byte mode (~1.8x), replacing the README's incorrect "4x", consistent with the project's habit of measuring rather than guessing.

## 8. References

- ISO/IEC 18004:2015 — Kanji mode indicator `1000`, the character-count widths (8/10/12), and the Shift-JIS two-byte-to-13-bit derivation with its two offset ranges.
- JIS X 0208 — the double-byte character set QR Kanji mode encodes; Shift-JIS is its byte serialisation.
- `golang.org/x/text/encoding/japanese` — the Shift-JIS codec used by the ignored table generator only.
- `docs/theory/02-data-encoding.md`, `09-data-tables.md`, and `17-optimal-segmentation.md` — the mode, table, and DP notes Kanji extends.

## 9. Open Questions

To be answered before the corresponding milestone starts:

- **Table generation source.** Generate from `x/text`'s Shift-JIS codec (proposed, since x/text is already an indirect module) versus parsing an official mapping data file checked into the repo. The `//go:build ignore` generator keeps either out of the runtime graph. Confirm in K3.
- **Character-count table shape.** A fourth `charCountBitsTable` row with an index remap, or a dedicated Kanji width lookup. Confirm in K4.
- **DP cost of Kanji eligibility.** Building the Kanji-eligibility prefix means a table lookup per rune during segmentation; confirm the added cost is acceptable (it is O(n) with a map lookup, same order as the existing prefixes) or whether to gate Kanji segmentation behind the presence of any eligible rune.
- **Example content.** Which Japanese sample text to ship in the example and tests (kept short, checked for JIS X 0208 membership). Confirm in K5.
