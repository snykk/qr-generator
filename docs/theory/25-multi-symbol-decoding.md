# Multi-Symbol Decoding

The v0.14 decoder reads every QR symbol present in one image, not just the first. A page of codes, a strip of tickets, or a structured-append set photographed together decodes in a single `DecodeAll` call. This document explains why the detector already sees all the symbols, what the genuinely new problem is (grouping finders into symbols), and why decode success — not geometry — has to be the final arbiter.

> Indonesian version: [25-multi-symbol-decoding.id.md](25-multi-symbol-decoding.id.md).

## 1. The detector already sees every symbol

Finding a QR symbol starts with its three finder patterns. The v0.13 connected-component detector (doc 24) labels the binarised image and matches every dark-ring / light-ring / dark-centre nesting with 7:5:3 bounding-box sides. That matcher is not limited to one symbol: `finderNestings` returns **every** finder ring in the image. A photo with four QR codes yields (about) twelve finder candidates.

The single-symbol pipeline then throws most of that away. `bestFinderQuadTriple` keeps the candidates, and `bestFinderTripleIdx` selects the one triple with the lowest geometric cost — the single best right-isosceles arrangement — and decodes it. Everything needed to read all four codes was already computed; the pipeline simply collapsed to one at the last selection step. Multi-symbol decoding is therefore not a new detector. It is a different *selection*: instead of the single best triple, find **all disjoint valid triples**, and decode each through the unchanged per-symbol pipeline.

## 2. The grouping problem

Given N finder candidates, partition them into zero or more symbols, where each symbol is a triple of three finders that

- share a module pitch within tolerance (a symbol's three finders are the same size),
- form a right-isosceles arrangement — two roughly equal legs meeting at a right angle at the top-left finder, the shape of a square symbol seen under any similarity transform, and
- do not reuse a finder: each finder belongs to at most one symbol.

Two oracles for this already exist and are reused unchanged. `finderTripleCost` scores a triple — the sum of a module-spread residual and two geometry residuals, zero for a perfect symbol, lower is better. `orderFinderTriple` validates a triple — it assigns top-left/top-right/bottom-left rotation-invariantly and rejects an implausible arrangement (leg ratio above 1.5, hypotenuse residual above 0.15). The single-symbol path uses these to take the global minimum. Multi-symbol grouping enumerates candidate triples (the same O(N^3) ceiling, with N capped), keeps those that pass `orderFinderTriple` and a cost ceiling, sorts them best-first, and greedily claims disjoint triples.

## 3. Decode success is the arbiter

Geometry alone is not enough to group correctly, and this is the subtle heart of the milestone. Consider a regular grid of same-size codes. Take one finder from symbol A and two from symbol B: those three centres can form a perfect right-isosceles triangle at the shared module pitch. Such a **cross-symbol triple** passes every geometric gate. If grouping trusted geometry and claimed those three finders, it would corrupt both A and B — stealing a real finder from each and decoding neither.

The defence is to make **a finder claimed only when a triple containing it actually Reed-Solomon decodes**. A cross-symbol triple samples a grid that is not a real symbol, so `DecodeMatrix` fails its format-information and RS checks, and the triple is discarded with none of its finders consumed — they remain in the pool for the real triples that do decode. Decode success, not geometric plausibility, is the final grouping arbiter. This also gives implicit backtracking with no bookkeeping: a failed triple leaves its finders free, so the next-best overlapping triple still gets its turn, bounded only by an attempt cap.

That attempt cap must **scale with the finder count**. A grid of many same-size codes produces many plausible cross-symbol triples sorted among the real ones; a small fixed cap could exhaust on junk before the real symbols are reached, silently dropping them. The bound grows with N so the real symbols are always reachable.

## 4. Reusing the per-symbol pipeline

Once a triple is grouped and ordered, decoding it is exactly what the single-symbol projective path already does: estimate the version from the finder-quad edge length, trial candidate versions, fit the twelve-corner projective homography, sample the module grid, and Reed-Solomon decode (doc 24). The milestone factors that body out of the single-symbol projective path into a helper, `decodeOneQuadTriple`, so both the single-symbol path and the multi-symbol loop call the identical per-symbol code. The helper surfaces the winning version and homography, which the single-symbol path discarded but the public result needs (section 5).

Because each symbol carries its own finder pitch into its own homography and version trial, symbols of different sizes and different tilts coexist in one image with no interference: grouping keeps each triple's three finders at one pitch, and everything downstream is per-triple.

The single-symbol `Decode` is deliberately left on the global-argmin selection, not the grouper's first result. Greedy-below-threshold can pick a different triple than the strict minimum on a noisy one-symbol image, so keeping `Decode` on `bestFinderTripleIdx` preserves its exact behaviour; a test asserts `DecodeAll` on a one-symbol image returns exactly what `Decode` does.

## 5. Locating each symbol

A multi-symbol result is only useful if the caller knows where each symbol is. Two pieces of geometry are already in hand once a symbol decodes:

- **The symbol's four outer corners**, derived from the winning homography. The homography maps module coordinates to pixels, so applying it to the module-grid corners `(-0.5, -0.5)`, `(n-0.5, -0.5)`, `(n-0.5, n-0.5)`, `(-0.5, n-0.5)` — the outer edges of a symbol of `n = version.Size()` modules — gives the true symbol outline in pixels, rotated and sheared with the symbol. This is *not* the finder-ring corners, which only bound the 7x7 finders; the symbol corners come from the homography and cover the whole code.
- **The three finder centres**, from the ordered triple, which pin the orientation (which edge is up).

An axis-aligned bounding rectangle is offered only as a derived convenience (`Bounds()`), not the primary geometry: for a tilted symbol the true outline is the corner quad, and an axis-aligned box would roughly double its area and overlap neighbours — exactly wrong in a scene of several codes at odd angles.

Results are returned in a deterministic reading order: symbols are banded into rows by their top-left corner's vertical position within a tolerance, rows run top to bottom and symbols left to right within a row, with an exact final tie-break so the order is reproducible. This is best-effort for grid and document layouts; scattered placements still get a stable, deterministic order.

## 6. Error and result semantics

Multi-symbol decoding exists to read what it can from a messy scene, so one unreadable symbol must not fail the batch. A symbol whose finders, format info, or Reed-Solomon blocks cannot be recovered is simply skipped, and the symbols that did decode are still returned. `DecodeAll` returns an error only for a whole-image failure: a malformed image byte stream, or **zero** successful decodes — whether from fewer than three finders or from finders that never decode — which maps to `ErrFinderNotFound` for parity with single `Decode`. It never returns a partial result slice together with an error.

Reassembling a structured-append set from one composite image is out of scope: `DecodeAll` returns raw per-symbol text, and `DecodeStructuredAppend` remains the path that stitches separate images into one message.

## 7. Honest limits

- **Connected-component only.** `DecodeAll` uses the CC detector, not the affine line scanner, because only the CC path enumerates all finders. A blurry or low-resolution upright symbol that the 1:1:3:1:1 line scan would decode, but whose rings do not cleanly label as a nesting, may be read by `Decode` and missed by `DecodeAll`.
- **Finders must be fully in frame.** A finder ring touching the image border does not label as a hollow nesting, so a symbol running off the edge is dropped rather than partially recovered. There is no crash: out-of-bounds sampling is guarded.
- **Touching symbols may lose an edge finder.** Two abutting symbols whose light quiet zones merge into one component can break the light-ring test for a finder on the shared edge.
- **Cost of the version trial.** Each grouped triple may trial multiple versions, each a full sample-and-decode, so the honest bound on a noisy image is the attempt cap times the version window; proximity-ordered versions keep the common case cheap.

## 8. Implementation pointers

Planned in MS3/MS4 (see `docs/plan-multi-symbol-decoding.md`):

- `qrgen/decode_finder_cc.go`: `groupFinderTriples` (enumerate, gate by `orderFinderTriple` and a measured cost ceiling, sort, index-based disjoint claim) and `allFinderQuadTriples`.
- `qrgen/decode_image.go`: `decodeOneQuadTriple(bm, tq) (text, Version, homography, error)`, extracted from the single-symbol projective path with no behaviour change.
- `qrgen/decode_multi.go`: the `Result` type with `Bounds()`, the symbol-corner derivation from the homography, and the public `DecodeAll` / `DecodeAllBytes`.
- Tests composite several rendered symbols onto one canvas and cross-check the `gozxing` `multi/qrcode` reader.

## References

- ISO/IEC 18004:2015 — the finder-pattern structure and quiet zone the detector relies on.
- `docs/theory/24-arbitrary-angle-detection.md` — the connected-component finder detector and the projective per-symbol pipeline reused here unchanged.
- `docs/theory/15-rotation-handling.md` — the rotation-invariant finder ordering (`orderFinderTriple`) reused as the grouping validity gate.
- ZXing `MultiDetector` / `MultiFinderPatternFinder.selectMultipleBestPatterns` — prior art for grouping finders into several symbols; `gozxing`'s `multi/qrcode.QRCodeMultiReader` is the cross-check oracle.
