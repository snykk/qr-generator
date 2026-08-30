# Arbitrary-Angle Finder Detection

The v0.13 decoder locates a QR symbol tilted or rotated well past the ~30-degree band the row-scanning finder detector handles. It does this by fixing the single stage that fails at steep angles — finding the three finder patterns — with a connected-component detector that is invariant to any projective transform, and hands the finder centres to the existing homography pipeline unchanged. This document records why the line scanner fails, why a topological approach survives, how the component detector works, and — honestly — how far it actually reaches.

> Indonesian version: [24-arbitrary-angle-detection.id.md](24-arbitrary-angle-detection.id.md).

## 1. Rotation is fine; perspective is the problem

The existing detector scans image rows for five runs of dark/light/dark/light/dark whose lengths match the **1:1:3:1:1** ratio, then cross-checks the column (docs 12, 15). A finder pattern is concentric squares, so a straight line through its centre crosses that ratio at any *in-plane rotation* — which is exactly why v0.4 already decodes symbols rotated up to ~30 degrees off-axis without special handling.

What defeats it is not rotation but **perspective foreshortening**. When the symbol is photographed at a slant, the projection compresses distances unevenly across the finder: the far side of the 7-module square is imaged smaller than the near side, so the five crossing lengths no longer read 1:1:3:1:1. Past roughly 30 degrees of tilt the measured ratio drifts outside `fitsFinderRatio`'s +-50%-per-module tolerance and no run of five qualifies. Widening the tolerance is not a fix: it would accept ordinary dark-light-dark texture as finders and flood the detector with false positives.

## 2. The topological insight

A projective transform can stretch, shear, and foreshorten a shape, but it cannot change what is *inside* what. The finder pattern is three nested regions — a solid dark centre, inside a light ring, inside a dark ring — and that nesting is preserved under any camera angle. So instead of measuring a one-dimensional ratio, detect the **structure**:

- a dark region,
- enclosing a light region,
- enclosing a dark region,
- with the three centres roughly coincident, and
- with areas in roughly the `3x3 : 5x5 : 7x7` proportion the finder's modules define.

Nesting and concentricity are topological and affine-ish invariants; they hold at 10 degrees and at 80. Detecting them requires segmenting the image into regions, which is what connected-component labelling does.

## 3. Connected-component labelling

The detector runs a standard **two-pass union-find** labelling over the binarised bitmap:

1. **First pass**, scanning in reading order: give each pixel a provisional label from its already-labelled up and left neighbours of the same colour (dark or light), and when those two neighbours carry different labels, union them in a disjoint-set structure.
2. **Second pass**: replace each provisional label with its set's representative, so every maximal same-colour region has one label.

For each labelled component the detector accumulates a bounding box, a pixel area, and a centroid. This is O(pixels) with near-linear union-find, run only on the fallback path (section 5), so it does not slow the common case.

## 4. The finder signature

With components in hand, the detector looks for the nested triple. A finder's outer dark ring is a dark component; the light ring it encloses is a light component whose centroid sits at the same place; the dark centre is a dark component inside that. The detector accepts a candidate when:

- the three centroids coincide within a small fraction of the outer box (concentricity), and
- the nested areas descend in roughly the finder's `49 : 25 : 9` module-area proportion, within tolerance (the ratio need not be exact — perspective changes absolute areas but the *ordering* and rough proportion survive), and
- the outer box is roughly square-ish (a strongly elongated blob is not a finder even under tilt, within reason).

Each surviving candidate contributes its centre. Three mutually-consistent centres then flow into the unchanged `orderFinderTriple` (rotation-invariant via the cross-product handedness test, doc 15), `estimateVersion`, `homographyFromFinders`, and `refineHomography`, which fit a full 3x3 projective transform — so once the three centres are found at a steep angle, the rest of the decode simply works. A stray non-finder candidate is filtered here: it will not form a geometrically consistent triple with two real finders.

## 5. A fallback, not a replacement

The component pass costs a full O(pixels) segmentation, more than a handful of row scans, so it runs **only when the line scanner fails to return three finders** — the same shape as the Otsu -> Sauvola fallback (doc 14). Upright and mildly-tilted symbols are found by the fast scanner and never pay for the component pass; steeply-tilted ones fall through to it. This keeps the common case at its current speed and guarantees no regression: if the line scanner still succeeds, its result is used exactly as before.

## 6. Validation, and honest limits

There is no spec table to check a CV detector against, so it is validated empirically. A test-only **tilt warp** renders a QR upright and projects it through a chosen rotation-plus-tilt homography (each destination pixel inverse-mapped and sampled from the source), producing synthetic photos at 30, 45, 60, and steeper degrees. The round-trip test decodes each and records the band that passes; the README and this doc state the band that was actually cleared, not an aspiration.

The honest expectation: this widens the reliable band substantially beyond ~30 degrees, but it is best-effort computer vision. Near-90-degree grazing angles (where the symbol is almost edge-on and finders collapse to slivers), heavy blur, or glare that breaks a finder's nesting can still fail. Because the detector is a fallback, none of that regresses the upright and mildly-tilted cases; it only ever adds decodes that previously failed outright.

## 7. Implementation pointers

- `qrgen/decode_finder_cc.go` hosts the two-pass union-find labelling, the per-component accumulation, and the nesting/concentricity/area-ratio matcher that emits `finderCandidate` centres.
- `decodeImageDebug` in `qrgen/decode_image.go` calls it as a fallback after the line scanner returns no triple, then proceeds through the unchanged geometry pipeline; the debug state records which detector succeeded, which the tests assert.
- The tilt warp and the arbitrary-angle round-trip live in `qrgen/decode_tilt_test.go`; the warp is test-only scaffolding, never part of the library.

## References

- ISO/IEC 18004:2015 — the finder pattern's concentric 7x7 / 5x5 / 3x3 module structure and the quiet zone that separates it from the data.
- `docs/theory/12-image-processing.md` — the binarisation and the existing row/column finder scan this milestone falls back from.
- `docs/theory/14-adaptive-thresholding.md` — the try-then-fallback dispatch pattern reused here.
- `docs/theory/15-rotation-handling.md` — the rotation-invariant finder ordering and the ~30-degree scope boundary this milestone extends.
- Connected-component labelling (two-pass, union-find) — the standard image-segmentation algorithm, implemented in pure Go; ZXing and OpenCV QR detectors use component/contour analysis as conceptual prior art.
