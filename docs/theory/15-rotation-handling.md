# Rotation Handling in the Decoder

The v0.3 decoder assumes the source image is approximately right-side-up: the top-left finder sits at the top-left of the image, top-right is to its right, and bottom-left is below it. That assumption breaks the moment a user holds their phone sideways while scanning a printed page or sticks a QR on a label that the camera then frames upside down. This document explains where exactly the upright assumption is encoded in the v0.3 pipeline, why every other stage of the image pipeline is already rotation-invariant, and how a single cross-product handedness test unlocks rotation handling for the axis-aligned cases (90 / 180 / 270 degrees) plus soft tilts up to about 30 degrees.

> Indonesian version: [15-rotation-handling.id.md](15-rotation-handling.id.md).

## 1. Where the upright assumption lives

The v0.3 image pipeline has five stages: grayscale conversion, binarisation (Otsu with a Sauvola fallback as of v0.3), finder-pattern scanning, finder ordering, and homography-driven module sampling. Walking through them:

- **Grayscale and binarisation** depend only on per-pixel intensity. Rotation has no effect.
- **Finder-pattern scanning** uses the `1:1:3:1:1` row-and-column scan in `scanRowForFinders` and `crossCheckVertical`. The QR finder pattern is a concentric square (`XXXXXXX / X.....X / X.XXX.X / X.XXX.X / X.XXX.X / X.....X / XXXXXXX`), so any straight line through its centre crosses dark-light-dark-light-dark with the same `1:1:3:1:1` ratio regardless of orientation. The scanner picks up rotated finders at any angle inside the `±50%` per-module tolerance of `fitsFinderRatio`.
- **Finder ordering** lives in `orderFinderTriple`. This is where the upright assumption hides. The function identifies the right-angle vertex by "the finder farthest from the longest side" (rotation-invariant), then disambiguates the remaining two as top-right vs bottom-left with `if tr.y > bl.y { swap }` — the upright shortcut. For a rotated image, the labels come out swapped, which causes downstream stages to misread the matrix.
- **Homography** in `homographyFromFinders` builds a general 3x3 projective transform from four anchor-point correspondences. A projective transform already absorbs rotation, scale, translation, and perspective. Given correct labels from `orderFinderTriple`, the homography reproduces the rotation automatically — no code change needed.
- **Module sampling** evaluates the homography at every grid centre and reads the binarised pixel. Also rotation-agnostic.

So the entire fix lives in `orderFinderTriple`. Everything else already works.

## 2. The right-angle vertex is rotation-invariant

A QR symbol places its three finder patterns at three of the four corners of a square: top-left, top-right, and bottom-left. Connect those three centres and the result is a right triangle whose right angle sits at the top-left corner. The hypotenuse of that triangle is the segment between the top-right and bottom-left finders.

```text
                  (TR)
                  /|
                 / |
                /  |
            sqrt2 a|
              /    |a
             /     |
            /      |
           /-------|
        (TL)   a   (BL)
```

where `a` is the symbol's side length in pixels. The hypotenuse is `a * sqrt(2)` and is always longer than either leg.

Identifying the right-angle vertex therefore reduces to one rule: of the three pairwise distances, the longest one connects two finders that are NOT the top-left; the remaining finder is the top-left. This rule survives any rigid motion of the symbol because rigid motions preserve distances. Rotation 90, 180, 270 — same identity. Soft tilt — the leg lengths stretch slightly, but the longest pair is still the hypotenuse. The existing `orderFinderTriple` already does this and we keep it unchanged.

## 3. The hard part: telling top-right from bottom-left

Once we know which finder is the top-left, two finders remain. Each could be the top-right or the bottom-left. We need a rotation-invariant rule that picks the right one.

The trick is the **cross product**. Treat the two vectors from `TL` to each of the remaining finders as 2D vectors and compute their cross product. In a right-side-up image:

```text
v_tr = TR - TL    (points right)
v_bl = BL - TL    (points down)

cross = v_tr.x * v_bl.y - v_tr.y * v_bl.x
```

For an upright QR in image coordinates (where `y` grows downward):

```text
v_tr  = (a, 0)
v_bl  = (0, a)
cross = a * a - 0 * 0 = a²    (positive)
```

A real (un-mirrored) QR symbol always has the same handedness regardless of rotation, so the sign of `cross((TR - TL), (BL - TL))` is the same at every angle. To assign the labels correctly, we tentatively pick either of the two remaining finders as `TR`, compute the cross product, and swap `TR` and `BL` if the sign comes out negative.

## 4. Worked sign analysis at 0 / 90 / 180 / 270 degrees

To convince ourselves the cross product stays positive at every axis-aligned rotation, walk through each case. We place the symbol's TL at the corner indicated by the rotation, with side length `a` in image pixels:

| Rotation | TL    | TR    | BL    | `TR - TL` | `BL - TL` | Cross product |
| --- | --- | --- | --- | --- | --- | --- |
| 0 (upright) | `(10, 10)` | `(10 + a, 10)` | `(10, 10 + a)` | `(a, 0)` | `(0, a)` | `a²` |
| 90 (CW) | `(10 + a, 10)` | `(10 + a, 10 + a)` | `(10, 10)` | `(0, a)` | `(-a, 0)` | `a²` |
| 180 | `(10 + a, 10 + a)` | `(10, 10 + a)` | `(10 + a, 10)` | `(-a, 0)` | `(0, -a)` | `a²` |
| 270 (CW) | `(10, 10 + a)` | `(10, 10)` | `(10 + a, 10 + a)` | `(0, -a)` | `(a, 0)` | `a²` |

Every case gives `+a²`. Soft tilts that interpolate between two adjacent rotations preserve the sign because the cross product is a continuous function of the finder positions and never crosses zero for a non-degenerate triangle (a triangle whose vertices are collinear would give a zero cross product, but the right-angle plus leg-ratio validation already rules those out).

## 5. Mirrored symbols

A mirrored QR symbol — one whose ink pattern is reflected — would give the opposite cross-product sign. Real QR codes never occur mirrored: the encoder generates a fixed handedness, and printed or screen-displayed symbols preserve that handedness. So we are not in the business of rejecting mirrored handedness; we are simply observing that real input always has one sign and our algorithm relies on that fact to assign labels.

If a mirrored image were somehow fed to the decoder (e.g. a synthetic test image with the matrix horizontally flipped), the cross-product check would still produce a "consistent" labeling that the homography would happily decode, but the resulting matrix would fail downstream — format-info BCH would not validate, or Reed-Solomon would exceed its correction budget. The failure is loud and the public API surface stays clean; we accept this trade-off rather than add a dedicated `ErrMirroredSymbol` sentinel that no real-world caller can hit.

## 6. Why the homography handles the rest

Once `orderFinderTriple` returns the correct `(TL, TR, BL)` triple, `homographyFromFinders` builds the 3x3 projective transform `H` such that:

```text
H * (3, 3, 1)     = TL (in pixel coords)
H * (n - 4, 3, 1) = TR
H * (3, n - 4, 1) = BL
H * (n - 4, n - 4, 1) = BR (extrapolated by parallelogram completion)
```

where `n` is the side length in modules and the module coords `(3, 3)`, `(n-4, 3)`, `(3, n-4)`, `(n-4, n-4)` are the centres of the four finder regions (with the bottom-right slot occupied by the alignment pattern or its parallelogram-completed estimate).

A 3x3 homography is the most general invertible projective transform in 2D and is closed under composition with rotations and translations. Concretely, if the symbol is rotated by `theta` and translated, the homography we solve for absorbs both into a single matrix — we never need to apply a "derotation" step before sampling. Module sampling then walks the grid module by module and the homography maps each module centre to the right source pixel regardless of orientation.

Mathematically: any homography `H` can be decomposed into `H = T * R * K * P`, where `T` is translation, `R` is rotation, `K` is scale/shear, and `P` is the projective part. Our four-point linear solve in `solveLinear8` recovers `H` without ever needing to separate these factors.

## 7. The scope boundary at 30 degrees

Why does v0.4 stop at soft tilts of about 30 degrees rather than going all the way to 90? The reason lies in stage 3 of the v0.3 pipeline — the `1:1:3:1:1` row scanner. A horizontal scan through a finder rotated by angle `theta` hits each module's "horizontal" projection. For axis-aligned rotations the projection equals the module size exactly. For a tilt of `theta`, the run lengths inside the finder become `cos(theta) * module + tan(theta) * jitter`, and the ratio drifts away from the ideal `1:1:3:1:1` as `theta` grows.

`fitsFinderRatio` accepts each run within `±50%` of the expected size. Plugging numbers in: at `theta = 30°`, the drift is roughly `1 / cos(30°) - 1 ≈ 15%`. At `theta = 45°`, the drift jumps to roughly `1 / cos(45°) - 1 ≈ 41%` and starts approaching the tolerance band's edge with no headroom for the additional jitter introduced by bilinear interpolation in rotated PNGs. At `theta = 60°`, the row scan effectively walks the diagonal of the finder pattern and the ratio breaks entirely.

So v0.4 ships axis-aligned rotation + soft tilts up to about 30 degrees and explicitly leaves the `[30°, 90°)` band as future work. Lifting that bound requires either a wider tolerance in the scanner (risky — false positives jump on noisy backgrounds) or a different finder detector altogether (contour tracing, fan-of-orientations search). Both are bigger than a minor release.

## 8. Implementation pointers

- `qrgen/decode_image.go` hosts `orderFinderTriple`. The change in v0.4 replaces the final `if tr.y > bl.y { swap }` block with the cross-product handedness check; everything above it (right-angle vertex detection, leg-ratio sanity, hypotenuse check) stays unchanged because it was already rotation-invariant.
- The cross product is a single multiply-subtract-compare, so the v0.4 change is allocation-neutral and adds zero detectable cost to the Otsu fast path.
- `homographyFromFinders` in the same file is untouched in v0.4. Verifying this experimentally is part of R5's benchmark sweep.
- `qrgen/decode_rotation_test.go` (new in v0.4) hosts the `rotateImage` helper that builds the synthetic fixture set and the round-trip assertions. The fixtures are deterministic and stay in-memory; no PNG blobs land in `testdata/`.

## References

- ZXing project — *open-source decoder reference*: <https://github.com/zxing/zxing>. Their `FinderPatternFinder.orderBestPatterns` uses the same cross-product handedness trick we adopt here; this doc is in part a transcription of that algorithm into qrgen's conventions and notation.
- Hartley, R., Zisserman, A. — *Multiple View Geometry in Computer Vision*, 2nd ed., Cambridge University Press, 2004, §4. Foundations for the claim that a 3x3 homography absorbs rotation given correct corner correspondences.
- ISO/IEC 18004:2015 — §11.2 (locator pattern detection) and §11.3 (image sampling). The spec assumes "the symbol is approximately right-side-up" without elaborating; this document fills in the geometry that lets us drop that assumption for axis-aligned cases.
- Project Nayuki — *QR Code generator library, decoder companion notes*: <https://www.nayuki.io/page/qr-code-generator-library>. Useful cross-check on finder geometry for rotated symbols.
