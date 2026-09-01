# Arbitrary-Angle Decoding

The v0.13 decoder reads a QR symbol photographed at a steep angle -- the tilt band where the previous pipeline failed. Getting there meant finding the stage that actually breaks under perspective, and it was not the one the roadmap assumed. This document records the real diagnosis, the projective fix built on it, and -- honestly -- how far it reaches.

> Indonesian version: [24-arbitrary-angle-detection.id.md](24-arbitrary-angle-detection.id.md).

## 1. What actually breaks under tilt

The intuition going in was that finder detection fails first: the 1:1:3:1:1 line scanner (docs 12, 15) measures a one-dimensional run ratio, and perspective foreshortening should distort that ratio past its tolerance. Measured against synthetic tilt fixtures, that intuition was wrong in two ways.

First, the line scanner is more robust than expected. Its per-module tolerance is +-50%, loose enough that it keeps finding all three finders up to roughly 55 degrees of tilt. So finder detection was never the first thing to fail.

Second, and decisively: **even when the finders are found, the decode fails from about 20 degrees up.** The reason is the geometry stage. [homographyFromFinders](../../qrgen/decode_image.go) builds the image-to-module transform from the three finder centres plus a parallelogram-completed fourth corner -- an **affine** map. Affine maps preserve parallelism, so they cannot represent the keystoning a slanted plane produces: the far edge of the symbol is imaged smaller than the near edge, and an affine transform has no term for that. The alignment-pattern refinement (v0.4) is supposed to supply a true fourth point, but under tilt the affine prediction of the alignment centre lands outside the small search window, so the refinement quietly falls back to the affine estimate. And there is a subtler cap: a finder's **centroid** is not a projective invariant -- a projective map sends the centre of a square to a point that is not the centroid of the imaged quad -- so even the three centre correspondences carry error that grows with the angle.

A control experiment settles it. Sampling the exact same tilted fixtures with the *ground-truth* projective homography decodes cleanly all the way to 70 degrees. The symbol is perfectly readable at a steep angle; the only thing missing was a transform that could undo perspective. That is the whole milestone.

## 2. The fix: a projective homography from finder corners

A projective homography (a full 3x3 with a non-trivial bottom row) is exactly the transform between a plane and its perspective image, so it can undo the keystoning. It has eight degrees of freedom and needs at least four point correspondences that are not projective-degenerate.

The three finder centres give only three points -- and imperfect ones, per section 1. But each finder is a **square**, and a square has four corners. Three finders give twelve corner correspondences, spread across the whole symbol, which over-determine the homography and pin down the perspective. This is the standard basis real decoders (ZXing, OpenCV) use. So the milestone's job is to find the finders, extract their corners, and solve.

## 3. Finding the finders: connected components

Corners come from the finder regions, so the first step is to segment them. The connected-component detector labels the binarised image with two-pass union-find (4-connectivity) and matches the finder's topological nesting -- a hollow, roughly square dark ring enclosing a concentric light ring enclosing a dark centre, with bounding-box sides in the 7:5:3 proportion. Nesting and concentricity survive any homography, so this locates finders at the same tilts the line scanner reaches (empirically ~55 degrees; beyond that both fail and nothing downstream can help). The labelling also returns a per-pixel component map, so the outer ring's own pixels can be revisited for corner extraction.

## 4. Extracting the four corners

For each finder's outer-ring component, its corners are the extreme vertices of a convex quad. A rotation-agnostic way to find them: take the point farthest from the ring's centroid (one corner), then the point farthest from *that* (the opposite corner), then the two points lying farthest to either side of the line joining them. This works at any orientation, unlike taking the axis-aligned extremes of `x+y` and `x-y`, which degrade toward 45 degrees of rotation.

## 5. Assigning module coordinates to corners

The homography solver needs each corner paired with its position in the symbol's module grid. The four corners come out in arbitrary order, so each is classified by projecting its offset from the finder centre onto the symbol's own axes: the top-left-to-top-right direction and the top-left-to-bottom-left direction, taken from the ordered finder triple. The signs of the two projections place the corner at one of the finder's four outer corners, which sit 3.5 modules from the finder centre on each axis. The top-left finder's corners map to module coordinates around (3, 3); the top-right's around (n-4, 3); the bottom-left's around (3, n-4).

## 6. Solving: least squares over twelve points

With more than four correspondences the system is over-determined, so it is solved in the total-least-squares sense via the normal equations `(A^T A) h = A^T b` with the h22 = 1 normalisation, reusing the existing 8x8 Gaussian solver. Averaging over twelve corners absorbs the per-corner extraction noise that any single four-point solve would inherit.

## 7. Version under perspective, and the trial decode

One problem remains: the grid size. The homography's far corners are placed at module (n-4, ...), so the transform itself needs the version n, and the module grid sampled from it is n by n. But version is read from the finder spacing in modules, and under tilt that spacing foreshortens while the module-pitch estimate from a rotated finder's bounding box inflates -- so the usual estimate lands out of range.

The fix has two parts. A better *seed*: estimate the module pitch from a finder quad's outer-edge lengths (each edge is 7 modules) rather than its axis-aligned bounding box, which rotation does not inflate. And a *trial*: because the seed can still be off by a few versions under strong perspective, the decoder tries candidate versions in order of proximity to the seed, fitting the homography and attempting a decode for each. Reed-Solomon and format-information validation make a wrong version fail fast and a right version decode, so the first success is the answer. The correct version is normally the seed or one of its immediate neighbours, so the trial ends quickly.

## 8. A fallback, so nothing regresses

The projective pipeline is strictly the second attempt. `decodeImage` first runs the unchanged affine pipeline -- the fast line scanner and the centre-based homography -- which is optimal for upright and rotated symbols and carries every case the previous decoder handled. Only when that fails to produce a decodable grid, the signature of perspective tilt, does it fall back to the connected-component projective pipeline. Upright symbols never pay for the component pass or the version trial, existing decodes are byte-for-byte unchanged, and the affine attempt's error is the one surfaced on total failure so error messages are stable.

## 9. Validation, and honest limits

There is no spec table to check a projective decoder against, so it is validated empirically. A test-only tilt warp renders a symbol upright and projects it through a chosen rotation-plus-tilt homography (each destination pixel inverse-mapped and bilinearly sampled), producing synthetic photos at a range of angles. The round-trip test decodes them and asserts the band that holds.

The honest result: across orientations the decoder reliably reads tilts to roughly 35 to 40 degrees, and up to about 55 degrees at favourable in-plane rotations; the exact ceiling depends on orientation because corner extraction and the bounding-box seed degrade as the finder squares foreshorten. Beyond ~55 to 60 degrees the finders themselves stop being detectable and no downstream stage can recover. Heavy blur, glare that breaks a finder's nesting, folds, and near-grazing angles still fail. Because the whole path is a fallback, none of that regresses the upright and rotated cases; it only adds decodes that previously failed outright.

## 10. Implementation pointers

- `qrgen/decode_finder_cc.go`: two-pass union-find labelling, the nesting matcher, corner extraction (`extractQuadCorners`, `farthestFrom`, `farthestAcross`), and `findFinderQuadTriple`, which orders the finder quads via the existing rotation-invariant `orderFinderTriple`.
- `qrgen/decode_image.go`: `homographyFromFinderQuads` (corner module-coordinate assignment) and `computeHomographyLS` (the least-squares solve); `decodeImageProjective` and `decodeProjectiveFromBitmap` (the version trial); and `decodeImage`, which sequences the affine and projective attempts.
- `qrgen/decode_tilt_test.go`: the test-only tilt warp and the arbitrary-angle round-trip, plus the upright-uses-affine regression guard.

## References

- ISO/IEC 18004:2015 -- the finder pattern's concentric 7x7 / 5x5 / 3x3 module structure and the quiet zone that isolates it.
- `docs/theory/12-image-processing.md` -- binarisation and the affine finder pipeline this milestone falls back from.
- `docs/theory/14-adaptive-thresholding.md` -- the try-then-fallback dispatch pattern reused here.
- `docs/theory/15-rotation-handling.md` -- the rotation-invariant finder ordering, reused unchanged for the quad triple.
- The Direct Linear Transform for homography estimation, and connected-component / contour analysis for finder location, are standard computer vision; ZXing and OpenCV QR detectors use the same corner-plus-homography basis as conceptual prior art (not a dependency).
