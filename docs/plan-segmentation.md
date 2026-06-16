# QR Encoder — Mixed-Mode Segmentation Plan

This document describes the implementation plan for **DP-optimal mixed-mode segmentation** in the encoder, targeting the `v0.6.0` minor release. It continues the encoder-breadth phase opened by the SVG renderer (v0.5.0) and closes the long-standing "greedy mode analyzer" limitation documented in the README.

> Status: **draft / living document.** Milestones MM1..MM6 land incrementally on the `encoder-segmentation` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M1..M11, D1..D14, T1..T6, R1..R6, and S1..S6.

> Indonesian version: [docs/plan-segmentation.id.md](plan-segmentation.id.md).

---

## 1. Vision & Goals

- Replace the encoder's **single-mode greedy analyzer** with a **dynamic-programming optimal segmentation** that splits the input into a sequence of numeric / alphanumeric / byte segments minimising the total encoded bit length. A payload like `"PHONE: 12345"` currently encodes entirely in byte mode (or alphanumeric if it qualifies); segmentation can encode the leading text as alphanumeric and the trailing digits as numeric, often shrinking the symbol — sometimes by a whole version.
- Keep the change **encoder-only with no public API change.** `Encode`, `EncodeToFile`, `EncodeSVG`, and `Matrix` keep their exact signatures; callers see smaller or equal symbols for the same input, never larger.
- **No decoder work needed.** The decoder's bit-stream parser already reads an arbitrary sequence of mode segments (it loops over segment headers until the terminator), so segmented output round-trips through our own decoder and through third-party decoders with zero changes. The existing round-trip test suite validates this the moment segmentation lands.
- Preserve the **same philosophy**: pure Go, zero runtime dependencies, spec-first with a bilingual theory doc, and golden / round-trip / property tests.
- **Guarantee no regression.** A pure-numeric, pure-alphanumeric, or pure-byte input must still produce a single segment whose bytes are identical to today's output, so existing golden fixtures and the gozxing round-trip stay green.

## 2. Design Principles

1. **DP over positions, keyed by mode.** The classic optimal-segmentation algorithm (Nayuki) walks the input left to right, tracking the minimum bit cost to encode the prefix ending in each of the three modes, with transitions for "extend the current segment" and "switch mode (paying a new header)". The cost of a segment is `4 (mode indicator) + mode.CharCountBits(v) + payloadBitLength(mode, segmentText)`.
2. **Segmentation is version-group-dependent.** `CharCountBits(v)` changes across the three version groups (1–9, 10–26, 27–40), so the optimal split can differ by group. Version selection therefore computes the optimal segmentation *for each candidate version* (or per group) and picks the smallest version whose segmented length fits. Forty iterations of an O(n) DP is cheap.
3. **Respect UTF-8 rune boundaries.** Numeric and alphanumeric characters are single-byte ASCII; any multi-byte rune can live only in a byte segment and contributes its full UTF-8 byte length to the byte-mode count. The DP must never split a multi-byte rune across a segment boundary. Operating the DP over runes (not raw bytes) while costing byte-mode segments in bytes keeps this correct.
4. **Subsume, do not special-case.** The DP must yield exactly one segment for a homogeneous input, byte-identical to the current single-mode path, so the greedy analyzer becomes a provable special case rather than a parallel path that can drift.
5. **No new public surface.** No new option, no new exported function or type. `segment` and `segmentText` are unexported. The greedy `analyzeMode` may stay as an internal helper (and for tests) but `encodeText` routes through the segmenter.
6. **Tests first.** Each milestone ships with table-driven Go tests: DP-cost correctness, optimality-vs-greedy on mixed payloads, the homogeneous-input identity invariant, UTF-8 boundary cases, and end-to-end round-trips.

## 3. Scope

### In scope for v0.6.0

- `segment` type (`{mode Mode, text string}`) and `segmentText(text string, v Version) []segment`, the DP-optimal segmenter, in a new `qrgen/segment.go`.
- A bit-length helper for a segmentation at a given version, reused by both the DP and version selection.
- `selectVersion` reworked to size the optimal segmentation per candidate version.
- `encodeText` reworked to write a sequence of `[mode indicator][char count][payload]` blocks, then the single shared terminator + bit padding + pad bytes exactly as today.
- Theory doc `docs/theory/17-optimal-segmentation.md` (EN + ID) covering the DP, the cost model, the version-group interplay, the UTF-8 boundary rule, and the homogeneous-input identity.
- Validation: optimality and identity tests, decoder + gozxing round-trips, and encoder benchmarks (the DP adds work to the encode hot path, so this must be measured).

### Out of scope (still)

- **ECI segments and Kanji mode.** Segmentation works within the existing three modes; ECI/Kanji remain separate roadmap items. (Kanji, once added, would become a fourth mode the DP could target — noted for the future.)
- **A public "force segmentation off" option.** Segmentation is strictly better-or-equal, so there is no reason to expose a toggle; revisit only if a real caller needs byte-for-byte parity with some other encoder.
- **Structured append.** Unchanged; a separate roadmap item.
- **Decoder changes.** None — it already parses multi-segment streams.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after MM4) gives a working segmented encoder validated by round-trips. **Checkpoint B** (MM6) is the `v0.6.0` release.

### MM1 — Plan Doc `(S)`

Goal: this document and its Indonesian counterpart, committed before any code or theory lands.

- [ ] `docs/plan-segmentation.md` and `docs/plan-segmentation.id.md` covering vision, principles, scope, milestones MM1..MM6, file-layout delta, risks, references, open questions.

### MM2 — Optimal-Segmentation Theory Doc `(S)`

Goal: cover the algorithm and its subtleties in `docs/theory/` before any code lands.

- [ ] `docs/theory/17-optimal-segmentation.md` — why greedy single-mode leaves bits on the table, the per-segment cost model (`4 + CharCountBits(v) + payload bits`), the DP formulation (state = position x ending-mode, transitions extend/switch, base case, traceback), worked example for a mixed payload showing greedy-vs-optimal bit counts, the version-group interplay and why selection recomputes per version, the UTF-8 rune-boundary rule and byte-mode byte-counting, and the homogeneous-input identity guarantee.
- [ ] Indonesian counterpart `docs/theory/17-optimal-segmentation.id.md`.
- [ ] Update `docs/theory/README.md` and `docs/theory/README.id.md` to add entry 17 under a new "Encoder completeness (v0.6.0)" subsection plus a code-mapping row pointing at `qrgen/segment.go`. Cross-link from doc 02 (data encoding), whose greedy-analyzer note should point forward to doc 17.

### MM3 — `segment` Type + DP Segmenter `(M)`

Goal: the segmenter itself, with no encoder integration yet.

- [ ] `qrgen/segment.go` with the `segment` struct and `segmentText(text string, v Version) []segment` implementing the DP, plus `segmentsBitLength(segs []segment, v Version) int`.
- [ ] Correctly handle the empty string (one empty segment, or an explicit empty result the encoder treats as a zero-length numeric segment, matching today's empty-input behaviour).
- [ ] UTF-8: iterate runes; ASCII digits/alnum are DP-eligible for numeric/alphanumeric; everything else forces byte mode; byte-mode cost counts UTF-8 bytes.
- [ ] Tests in `qrgen/segment_test.go`: homogeneous inputs return a single segment of the expected mode (identity invariant); `"PHONE: 12345"`-style mixed inputs return the expected alphanumeric+numeric split with a strictly smaller bit count than the single-mode encoding; the segmentation is never worse than greedy for a sweep of payloads; version-group boundary cases (same text, versions 9 vs 10 vs 27) recompute correctly; UTF-8 payloads keep multi-byte runes intact in byte segments.

### Checkpoint A — segmenter is correct and provably never worse than greedy.

### MM4 — Encoder Integration `(M)`

Goal: route the encoder through the segmenter.

- [ ] Rework `selectVersion` to pick the smallest version whose optimal segmentation fits (`4`-bit-per-segment headers included), using `segmentsBitLength`.
- [ ] Rework `encodeText` to compute the segmentation for the chosen version, write each segment's `[mode indicator][char count][payload]` via the existing `writeNumeric/Alphanumeric/Byte`, then the single shared terminator + bit padding + pad bytes. Adjust the `forceVersion` capacity check to size the segmentation rather than a single mode. Reconcile the `m Mode` return value (internal, currently discarded by `buildMatrix`) — either drop it from the signature or return a representative mode; note the choice in the doc.
- [ ] Keep `analyzeMode` as an internal helper (still referenced by tests) but stop using it for capacity decisions.
- [ ] Tests: end-to-end `Encode`/`Matrix` round-trips through our own `DecodeMatrix`/`DecodeBytes` for mixed payloads; the homogeneous-input byte-identical-output invariant against a few golden fixtures; `WithVersion`/`WithMask` still honoured; `ErrCapacityExceeded` still fires when even V40 cannot hold the (now smaller) segmentation.

### MM5 — Validation & Benchmarks `(M)`

Goal: prove the win and guard the hot path.

- [ ] Optimality assertions: a table of mixed payloads where the segmented version/bit-count is strictly better than the greedy single-mode baseline, with the exact expected numbers recorded.
- [ ] Cross-validation: extend the gozxing round-trip test (and our own decoder round-trip) with segmented payloads so an independent decoder confirms the segmented stream is spec-valid.
- [ ] No-regression: confirm pure-numeric / pure-alphanumeric / pure-byte inputs still match their existing golden outputs byte-for-byte.
- [ ] Benchmarks: the DP runs per candidate version, so measure `BenchmarkEncodeSmall` / `URL` / `MultiBlock` / `Large` against the v0.5 baseline and record the delta. If the per-version DP is measurably hot, cache the segmentation across versions within a group (same `CharCountBits`) as an optimisation.
- [ ] `go test -race ./...` clean.

### MM6 — Polish & Release `(S)`

Goal: cut `v0.6.0`.

- [ ] README: remove the **"Greedy mode analyzer"** bullet from `## Limitations`; mention optimal segmentation under Library usage or a short note; update `## Roadmap` (drop "mixed-mode segmentation" from the encoding-completeness bullet, leaving ECI + Kanji).
- [ ] Update the `analyzeMode` doc comment and `docs/theory/02-data-encoding.md` so they no longer describe segmentation as deferred.
- [ ] `CHANGELOG.md` `v0.6.0` entry under Added / Changed / Validated plus the compare/tag anchors.
- [ ] Tag `v0.6.0` (left for the maintainer per the established release workflow; annotation recommended in the release conversation).

---

## 5. Proposed File Layout Delta

```
qrgen/
├── encode.go            # existing — selectVersion + encodeText reworked for segments; analyzeMode kept as helper
├── segment.go           # new — segment type, segmentText DP, segmentsBitLength
├── segment_test.go      # new — DP correctness, optimality, identity, UTF-8 tests
├── encode_test.go       # existing — gains mixed-payload round-trip + identity-invariant tests
├── bench_test.go        # existing — re-measured; possible per-group segmentation cache
└── roundtrip_test.go    # existing gozxing test — gains segmented payloads
docs/
├── plan-segmentation.md          # this file
├── plan-segmentation.id.md       # Indonesian counterpart
└── theory/
    ├── 02-data-encoding.md        # greedy-analyzer note points forward to doc 17
    ├── 17-optimal-segmentation.md     # new
    └── 17-optimal-segmentation.id.md  # new
```

## 6. Risks & Technical Notes

- **Version/segmentation circularity.** Optimal segmentation depends on the version (via `CharCountBits`), but version selection depends on the encoded length, which depends on the segmentation. Resolved by computing the segmentation per candidate version inside the selection loop; correctness is unconditional, and a per-group cache removes any real cost.
- **UTF-8 correctness.** The single sharpest trap: a multi-byte rune must never be split, and byte-mode length is counted in bytes, not runes. The DP iterates runes and costs byte segments by `len(string(runes))`; tests include emoji / CJK payloads.
- **Identity invariant.** The biggest regression risk is changing the bytes of a homogeneous input. A dedicated test asserts byte-for-byte equality with the pre-segmentation output for pure-numeric / pure-alpha / pure-byte strings; this also keeps the v0.1 golden fixtures and the gozxing round-trip valid.
- **Hot-path cost.** Running an O(n) DP up to 40 times per encode is more work than the old O(n) single-mode scan. For typical payloads this is negligible, but the benchmarks must confirm it; a per-version-group cache (three computations instead of forty) is the fallback if needed.
- **`m Mode` return value.** `encodeText` currently returns a single mode that `buildMatrix` discards. Segmentation has no single mode; the signature should drop the return or return a representative value. Internal-only, but worth doing cleanly to avoid a misleading API.
- **Mask/penalty unaffected.** Segmentation changes the data bit stream, not matrix construction, masking, or rendering. Those stages and their tests are untouched.

## 7. References

- ISO/IEC 18004:2015 — clause 7.4 (data encoding, mode segments and mode indicators), clause 7.4.1 (mixing modes within a symbol).
- Project Nayuki — *Optimal text segmentation for QR Codes*: <https://www.nayuki.io/page/optimal-text-segmentation-for-qr-codes>. The dynamic-programming formulation adopted here.
- `docs/theory/02-data-encoding.md` — the existing mode/character-count notes, extended by doc 17.
- `docs/theory/09-data-tables.md` — `CharCountBits` per version group and the alphanumeric value table.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Drop or keep the `m Mode` return from `encodeText`?** Leaning toward dropping it since it is internal and discarded; a representative mode would be misleading for a multi-segment encode. Settle in MM4.
- **Per-version-group segmentation cache from the start, or only if benchmarks demand it?** Default: implement the simple per-version computation first, measure in MM5, and add the three-entry group cache only if the delta is material. Correctness first.
- **Empty-string handling.** Today an empty payload reports Numeric and encodes a zero-length numeric segment. Keep that exact behaviour through the segmenter so output is unchanged for the edge case.
- **Should `analyzeMode` be removed entirely?** It is subsumed by the DP for the single-segment case, but it is small, self-documenting, and used in tests; default to keeping it as an internal helper unless it becomes dead code.
