# QR Multi-Symbol Decoding Plan

This document describes the implementation plan for **multi-symbol decoding** targeting the `v0.14.0` minor release: reading every QR symbol present in one image, not just the first. It builds directly on the v0.13 connected-component finder detector, which already enumerates every finder pattern in the image; the milestone generalises the single-symbol selection into disjoint grouping and adds a small public surface (`DecodeAll`).

> Status: **draft / living document.** Milestones MS1..MS6 land incrementally on the `multi-symbol-decoding` branch; each is a focused commit (or small series) with tests, matching the cadence of prior milestones.

> Indonesian version: [docs/plan-multi-symbol-decoding.id.md](plan-multi-symbol-decoding.id.md).

---

## 1. Vision & Goals

- Decode **all** QR symbols in an image — a page of codes, a sheet of tickets, a structured-append set photographed together — through one call, without the caller cropping each symbol first.
- Reuse, don't rebuild. The v0.13 CC detector (`finderNestings`) already finds every finder ring in the image; the single-symbol path merely collapses to one best triple at `bestFinderTripleIdx`. The milestone generalises that one selection into "all disjoint valid triples" and decodes each through the existing per-symbol geometry pipeline unchanged.
- Keep the existing API **byte-for-byte**. `Decode`, `DecodeBytes`, `DecodeMatrix` stay single-symbol. `DecodeAll` / `DecodeAllBytes` are purely additive.
- Keep **zero runtime dependencies**. Grouping is pure Go over the existing candidate list; the multi-symbol test fixtures are composited in-tree from rendered QRs, cross-checked against the `gozxing` multi reader that is already a test dependency.
- Be **honest about limits**. This reads what it can from a scene of independent codes; damaged, cropped, or overlapping symbols are skipped, not recovered, and the documented behaviour says so.

## 2. Approach, grounded in the current code

The v0.13 projective path is: `detectFinderQuads(bm)` enumerates every finder ring as a `finderQuad` (centre + four ring corners); `findFinderQuadTriple` caps to `maxCCCandidates` and then `bestFinderTripleIdx` picks the single lowest-`finderTripleCost` triple; `decodeProjectiveFromBitmap` runs a version trial (`estimateVersionQuad` -> `candidateVersions` -> `homographyFromFinderQuads` -> `sampleMatrix` -> `DecodeMatrix`) and returns the first Reed-Solomon-valid text.

Multi-symbol changes exactly two things and reuses everything else:

1. **Grouping** replaces the single argmin. A new `groupFinderTriples` enumerates candidate triples (the same O(N^3) ceiling as `bestFinderTripleIdx`, N <= `maxCCCandidates`), keeps those that pass the existing validity gate (`orderFinderTriple`: leg ratio <= 1.5, hypotenuse residual <= 0.15) and a measured cost ceiling, sorts them best-first, and greedily claims disjoint triples. A finder is claimed **only when a triple containing it actually decodes** — decode success, not geometry alone, is the final arbiter, which is what stops a plausible-but-bogus triple from stealing a real symbol's finder.

2. **Per-symbol decode is factored out.** The body of `decodeProjectiveFromBitmap` becomes `decodeOneQuadTriple(bm, tq) (text string, v Version, hg homography, err error)` — surfacing the winning version and homography, which the public `Result` needs. The single-symbol path (`decodeProjectiveFromBitmap`) becomes `findFinderQuadTriple` + `decodeOneQuadTriple` and stays behaviourally identical.

Base the multi path on the **CC/projective detector, not the affine line scanner**: `finderNestings` already yields all finders, whereas the affine `findFinders` collapses to top-3-by-weight and would blend two symbols' finders into a garbage triple. The consequence — a blurry upright symbol the affine path would catch but whose rings do not cleanly label may be missed by `DecodeAll` — is documented, not silently ignored (section 6, risk 6).

## 3. Scope

### In scope for v0.14.0

- `groupFinderTriples` / `allFinderQuadTriples`: disjoint greedy grouping with claim-on-decode-success, index-based triple->quad mapping (no float-equality lookup).
- `decodeOneQuadTriple` extracted from `decodeProjectiveFromBitmap`, returning text + version + homography.
- Public `DecodeAll(image.Image) ([]Result, error)` and `DecodeAllBytes([]byte) ([]Result, error)`; a `Result{Text, Corners [4]image.Point, Finders [3]image.Point, Version}` with a `Bounds()` helper. Symbol corners are derived from the winning homography, not the finder-ring corners.
- Deterministic reading-order sort; the documented error contract (section 4).
- Reference doc `docs/theory/25-multi-symbol-decoding.md` (EN + ID) and a README/limitations/roadmap update.
- Tests: an in-tree composite-image helper, a `gozxing` multi-reader cross-check, and the single-symbol-equivalence guard.

### Out of scope (still)

- **Structured-append reassembly from one image.** `DecodeAll` returns raw per-symbol text; it does not stitch an SA set back into one message. `DecodeStructuredAppend` (separate images) stays the path for that. `Result` omits SA metadata for now (extensible later without breaking callers).
- **A multi-symbol affine fast path.** `DecodeAll` is CC/projective only; the affine `findFinders` stays single-symbol.
- **Recovering cropped symbols.** All three finder rings of a symbol must be fully within the frame; a symbol running off the edge is dropped.
- **`DecodeAllMatrix`.** A `[][]bool` grid is by construction one symbol; it would only duplicate `DecodeMatrix`.

---

## 4. Design decisions (resolving the cross-design contradictions)

These lock the interfaces the three exploratory designs left inconsistent, per the completeness critique.

- **Per-triple decode signature.** `decodeOneQuadTriple(bm *bitmap, tq finderTripleQuad) (text string, v Version, hg homography, err error)`. Both the grouper (needs success/failure) and `DecodeAll` (needs version + homography for `Result`) consume this one signature.
- **`Result.Corners` are the symbol's outer corners, derived from the winning homography** — not the finder-ring corners (`finderQuad.corners`), which only cover the 7x7 finders. With module extent `n = v.Size()`, the four symbol corners are `hg.apply(-0.5,-0.5)`, `hg.apply(n-0.5,-0.5)`, `hg.apply(n-0.5,n-0.5)`, `hg.apply(-0.5,n-0.5)` (module outer edges), each rounded (not truncated) to `image.Point`. Order is `[TL, TR, BR, BL]`.
- **Error contract.** `DecodeAll` returns `err != nil` only for a whole-image failure: `DecodeAllBytes` surfaces `image.Decode` errors, and **zero successful decodes** — whether from fewer than three finders or from finders that never RS-decode — returns `ErrFinderNotFound`, for parity with `Decode` and `errors.Is`. Any success returns `(results, nil)`; failed regions are simply absent. Never a partial slice together with a non-nil error.
- **Single-symbol path unchanged.** `Decode` keeps calling `findFinderQuadTriple` (global argmin), NOT `allFinderQuadTriples`-then-first, because greedy-below-threshold can pick a different triple than the argmin on a noisy single-symbol image. A test asserts `DecodeAll` on a one-symbol image returns exactly one `Result` whose `Text` equals `Decode`'s output.
- **Index-based mapping.** The grouper carries each candidate's index through the triple; the quad is recovered by index, not by `finderCandidate` float-equality, removing that footgun.
- **Reading order.** A deterministic total order: band symbols whose `Corners[0].Y` fall within a tolerance (a fraction of median symbol height) into rows, sort rows top-to-bottom and symbols left-to-right by `Corners[0].X`, with an exact `(Y, X, stable-index)` final tie-break so output is reproducible. Documented as best-effort for grid/document layouts.

## 5. Milestones

Milestones land sequentially. **Checkpoint A** (after MS5) decodes composited multi-symbol images through the public `DecodeAll`, agreeing with the `gozxing` multi reader. **Checkpoint B** (MS6) is the `v0.14.0` release.

### MS1 — Plan Doc `(S)`

- [x] `docs/plan-multi-symbol-decoding.md` and its Indonesian counterpart: vision, approach grounded in the code, scope, design decisions, milestones MS1..MS6, file-layout delta, risks, references, open questions.

### MS2 — Reference Doc `(S)`

- [ ] `docs/theory/25-multi-symbol-decoding.md` (EN + ID): why the CC detector already enumerates all finders; the grouping problem (disjoint valid triples of a shared module pitch and right-isosceles geometry); why decode-success must be the claim arbiter; the per-symbol geometry reuse; the symbol-corner derivation from the homography; the reading-order and error semantics; and the honest limits. Plus the theory README entry 25 and code-mapping row.

### MS3 — Grouping + Per-Triple Refactor `(L)`

Goal: the grouping engine and the reusable per-symbol decode, tested on constructed candidate sets.

- [ ] Extract `decodeOneQuadTriple(bm, tq) (string, Version, homography, error)`; re-express `decodeProjectiveFromBitmap` in terms of it with no behaviour change.
- [ ] `groupFinderTriples(quads) [][3]int`: enumerate, gate by `orderFinderTriple` + a **measured** cost ceiling, sort best-first, greedy disjoint claim (claim-on-decode-success happens in the wrapper). Index-based.
- [ ] `allFinderQuadTriples(bm) ([]finderTripleQuad, error)` and the decode wrapper that loops `decodeOneQuadTriple`, claiming finders on success, bounded by an **N-scaled** decode-attempt cap (not a fixed 32 — a same-size grid produces many cross-symbol triples that pass geometry; a fixed cap would silently drop real symbols).
- [ ] **Calibrate tolerances from data:** measure `finderTripleCost` across the existing `decode_tilt_test.go` / `decode_rotation_test.go` corpus and set the cost ceiling with margin above the worst currently-decodable symbol, so no v0.13 decode regresses. Consider a narrowed per-triple version window for the multi path (a proximity window around `estimateVersionQuad`) to bound the version-trial fan-out; keep the trade-off documented.
- [ ] Unit tests on hand-built candidate sets: two disjoint triples grouped; a shared/false finder not double-claimed; a same-pitch cross-symbol junk triple rejected by decode failure.

### MS4 — Public API `(M)`

Goal: the user-facing multi-symbol surface.

- [ ] `Result` struct + `Bounds()`; symbol-corner derivation from the winning homography (rounded); `Finders` from the ordered triple centres; `Version`.
- [ ] `DecodeAll(img) ([]Result, error)` and `DecodeAllBytes([]byte) ([]Result, error)`: binarise via the existing Otsu/Sauvola dispatch, `allFinderQuadTriples` -> per-triple `decodeOneQuadTriple`, collect successes, apply the reading-order sort, map zero results to `ErrFinderNotFound`.

### MS5 — Multi-Symbol Round-Trip Tests `(M)`

Goal: decode real composited scenes end to end.

- [ ] An in-tree composite helper: `draw.Draw` several `Encode(...)` outputs onto one canvas at chosen offsets (reuse `warpTilt` for tilted tiles). No binary fixtures.
- [ ] Cases: (a) 2+ upright same-size, (b) different-size (per-symbol pitch survives), (c) two different tilts, (d) zero symbols -> `ErrFinderNotFound`, (e) exactly one -> equals `Decode`, (f) adjacent/overlapping (honest partial), (g) a same-size grid stressing the attempt cap.
- [ ] A `gozxing` `multi/qrcode.QRCodeMultiReader` cross-check on a composite (the strongest available oracle; already a test dependency), asserting the same payload set.

### Checkpoint A — the public DecodeAll reads composited multi-symbol images and agrees with gozxing.

### MS6 — Polish & Release `(S)`

Goal: cut `v0.14.0`.

- [ ] README: a decoding subsection on `DecodeAll` and `Result`; the Limitations and Roadmap updated (multi-symbol done; note the CC-only asymmetry and the cropped/overlapping limits); Scope updated; a runnable `examples/decode/multi`.
- [ ] `CHANGELOG.md` `v0.14.0` entry plus anchors, left **unstaged** for the release branch (per the established convention: the feature branch does not define the tag).
- [ ] `go test -race ./...` clean, gofmt-clean.

---

## 6. Risks & Technical Notes

- **Cost-gate calibration is the highest technical risk.** The single-symbol path takes the global argmin with no absolute threshold, so there is no existing evidence real (especially tilted) symbols score below any fixed cost. `finderTripleCost` sums squared-distance residuals that inflate under foreshortening. Mitigation: `orderFinderTriple` remains the primary validity gate (already calibrated and shipping), and any additional cost ceiling is set from a measurement of the existing corpus with margin — never a round number — so no v0.13 decode regresses.
- **Attempt cap vs same-size grids.** In a regular grid of same-size codes, cross-symbol triples (one finder from A, two from B) are right-isosceles and share a pitch, so they pass geometry and are only rejected at RS. A fixed attempt cap could exhaust before real symbols are reached and drop them. Mitigation: scale the cap with N, and rely on claim-on-success plus cost-ascending order so the cheapest real triples are tried first.
- **Version-trial fan-out is the real cost.** Each `decodeOneQuadTriple` may trial up to 40 versions (`sampleMatrix` + RS each), so the honest worst case is `attemptCap x 40` matrix decodes, dwarfing the O(N^3) cost arithmetic. Mitigation: proximity-ordered versions try the right one early; consider a narrowed window for the multi path and document the bound.
- **CC-only asymmetry.** `DecodeAll` uses only the CC/projective detector, so a blurry/low-res upright symbol the affine line scan decodes but whose rings do not cleanly label may be found by `Decode` and missed by `DecodeAll`. Documented as a known limitation; the affine path is not generalised to multi.
- **Cropped, overlapping, and different-tilt symbols.** All three finder rings must be fully in frame (a border-touching ring does not label as a nesting, so that symbol is dropped, no crash — `sampleMatrix` guards out-of-bounds). Abutting symbols with merged quiet zones may lose an edge finder. Different tilts each get their own homography, which the per-triple path already handles, but tighten the cost-gate calibration to pass both a frontal and a steep symbol in one image.
- **`maxCCCandidates = 64` truncation-by-largest-module** could drop a small symbol's finders in a scene mixing one large and many small codes. Flag as an edge of the different-size case; revisit the cap if it bites.

## 7. Proposed File Layout Delta

```
qrgen/
├── decode_finder_cc.go        # + groupFinderTriples, allFinderQuadTriples (index-based)
├── decode_image.go            # extract decodeOneQuadTriple; binarise dispatch reused
├── decode_multi.go            # new — Result, Bounds, DecodeAll, DecodeAllBytes, reading order
├── decode_multi_test.go       # new — composite helper, cases (a)-(g), 1-symbol equivalence
├── roundtrip_multi_test.go    # new — gozxing multi/qrcode cross-check
docs/
├── plan-multi-symbol-decoding.md      # this file
├── plan-multi-symbol-decoding.id.md   # Indonesian counterpart
└── theory/
    ├── 25-multi-symbol-decoding.md     # new
    └── 25-multi-symbol-decoding.id.md  # new
examples/decode/multi/main.go          # new — runnable demo
```

## 8. References

- ISO/IEC 18004:2015 — finder-pattern structure and the quiet zone the detector relies on.
- `docs/theory/24-arbitrary-angle-detection.md` — the CC finder detector and projective per-symbol pipeline this milestone reuses.
- `docs/theory/12-image-processing.md`, `15-rotation-handling.md` — binarisation and the rotation-invariant finder ordering (`orderFinderTriple`) reused as the grouping validity gate.
- ZXing `MultiDetector` / `MultiFinderPatternFinder.selectMultipleBestPatterns` — prior art for grouping finders into multiple symbols; `gozxing`'s `multi/qrcode.QRCodeMultiReader` (already a test dependency) is the cross-check oracle.

## 9. Open Questions

To be answered before the corresponding milestone:

- **Cost ceiling value.** Set empirically in MS3 from the tilt/rotation corpus; the plan commits to measuring, not to a number.
- **Version window for the multi path.** Full 1..40 proximity sweep (max robustness) versus a narrowed window around `estimateVersionQuad` (bounded cost). Decide in MS3 against the attempt-cap and grid cases.
- **Attempt-cap policy.** `c x N`, or "until the candidate pool is exhausted"? Pick the simplest that never drops a real symbol in the grid stress case (MS5 case g).
- **Result metadata.** Ship the lean `Result` (Text, Corners, Finders, Version); add EC level / mask / SA header later behind demand, without breaking callers.
