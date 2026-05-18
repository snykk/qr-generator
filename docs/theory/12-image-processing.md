# Image Processing for QR Decoding

The decoder's image stage turns an arbitrary `image.Image` (a camera photo, scanned page, or PNG we generated ourselves) into a clean `[][]bool` module grid that the matrix-level decoder can consume. This document covers the four sub-stages used by `qrgen/decode_image.go` (built in milestones D8..D12): grayscale conversion, Otsu binarisation, finder-pattern detection, perspective transform, and module sampling.

> Indonesian version: [12-image-processing.id.md](12-image-processing.id.md).

## 1. Grayscale conversion

Source images can be `image.Gray`, `image.RGBA`, `image.NRGBA`, or other types. We collapse them to a single luminance channel using the standard ITU-R BT.601 weights:

```text
Y = 0.299 R + 0.587 G + 0.114 B
```

The Go standard library's `image/color.GrayModel` already implements this conversion; we just pull the `Y` byte for every pixel into a single `[]uint8` buffer of length `width * height`. Working in a flat row-major buffer makes the binarisation and scanning passes faster than going through the `image.Image` interface.

## 2. Otsu binarisation

Once we have grayscale, we need a single threshold `t` such that pixels with `Y ≥ t` are background and `Y < t` are foreground. **Otsu's method** picks the `t` that maximises *between-class variance*, i.e. the threshold that best separates the histogram into two clusters.

Algorithm:

```text
function otsuThreshold(pixels []uint8) uint8:
    hist[256] = histogram of pixels
    total = len(pixels)
    sumAll = Σ_{i=0..255} i * hist[i]

    sumBg = 0
    wBg   = 0
    best  = 0
    bestT = 0
    for t = 0..255:
        wBg += hist[t]
        if wBg == 0: continue
        wFg = total - wBg
        if wFg == 0: break
        sumBg += t * hist[t]
        meanBg = sumBg / wBg
        meanFg = (sumAll - sumBg) / wFg
        between = wBg * wFg * (meanBg - meanFg)^2
        if between > best:
            best, bestT = between, t
    return bestT
```

Otsu is global — one threshold for the whole image. It works perfectly for our own encoder output (which is monochrome PNG) and for most well-lit photos. For photos with strong gradients or uneven shadowing, a fallback to a local thresholding scheme (Sauvola, block-mean, etc.) is a candidate post-v0.2 enhancement.

## 3. Finder-pattern detection

A QR finder pattern in pixel space presents itself as a sequence of dark/light/dark/light/dark runs in the **1:1:3:1:1 ratio**. We scan every row from left to right, tracking the lengths of the last five consecutive runs of constant colour. Whenever the five runs fit the ratio (within tolerance) and the central pixel is dark, the centre column is recorded as a candidate.

```text
function scanRowFinders(row []bool, y int) []candidate:
    runs = [0]*5
    color = row[0]
    candidates = []
    for x = 1..len(row)-1:
        if row[x] == color:
            runs[4] += 1
        else:
            shift runs left by 1; runs[4] = 1; color = row[x]
            if center color is dark and runs ≈ k:k:3k:k:k (tolerance ±50%):
                centerX = x − runs[4] − runs[3] − runs[2]/2
                append candidate(centerX, y, k)
    return candidates
```

After the row scan we do a **vertical confirmation** at each candidate's centre column: walk up and down from `(centerX, y)`, checking that the vertical run lengths through that column also obey the 1:1:3:1:1 ratio. Candidates that fail the vertical check are dropped.

Surviving candidates are clustered (a single finder usually produces several candidate centres across consecutive rows). The three cluster centres with the most evidence become our top-left, top-right, and bottom-left finders, ordered by geometry: the right-angle vertex is closest to the top-left finder of the symbol.

If fewer than three valid finders survive, the decoder returns `ErrFinderNotFound`.

## 4. Geometry validation

Before trusting the three finder centres we sanity-check them:

- The triangle they form must be approximately a right angle.
- The two legs (top-left → top-right and top-left → bottom-left) must have similar lengths (the symbol is square).
- The estimated module size from each finder (`runs[2] / 3` from the 1:1:3:1:1 fit) must agree across the three finders within tolerance.

Failed validation also returns `ErrFinderNotFound`. This is what catches the common failure mode of a busy background that accidentally contains a 1:1:3:1:1 pattern.

## 5. Perspective transform

The image we received may be rotated, perspective-skewed, or non-uniformly scaled. We need a function `srcPixel(col, row)` that takes a module's `(col, row)` grid coordinates and returns the source-image pixel `(u, v)` where that module's centre lives.

Given four point correspondences `(col_i, row_i) ↔ (u_i, v_i)` we can compute a **3×3 homography matrix** `H` such that:

```text
[u_i]       [col_i]
[v_i] = H * [row_i]
[ 1 ]       [  1  ]
```

(In homogeneous coordinates: `[u, v, w] = H · [col, row, 1]`, then divide by `w`.)

Four correspondences give 8 equations; `H` has 9 entries but is scale-invariant so 8 unknowns — solvable via a small linear system.

The four correspondences for QR are:

- **Top-left finder centre** → `(3, 3)` (centre of a 7×7 finder pattern occupies modules 0..6, centred at module 3).
- **Top-right finder centre** → `(n − 4, 3)` where `n = 21 + 4·(v − 1)`.
- **Bottom-left finder centre** → `(3, n − 4)`.
- **Bottom-right alignment-pattern centre** → for V2+ this comes from finding the alignment pattern in the source image (D11); for V1 we extrapolate from the other three (the parallelogram is a rectangle).

The version `v` is estimated from the finder spacing first; see "Module pitch estimation" below.

## 6. Module pitch & version estimation

From the top-left and top-right finder centres we know the symbol's horizontal extent in pixels. Dividing by `(n − 7)` (the number of modules between the two finder centres) gives us the module pitch in pixels. Since `n` depends on `v` and we do not know `v` yet, the standard trick is:

1. Estimate the module size from the 1:1:3:1:1 fit at each finder (the "3" run is exactly 3 modules wide; divide by 3 to get pitch).
2. Compute `v = round( (distance / pitch − 7) / 4 + 1 )`.
3. Validate that the resulting `n` is in `[21, 177]` and consistent with the vertical and horizontal finders.

For V7+, the version-information BCH codeword later in the pipeline (D6 inside the matrix decoder) can double-check this estimate.

## 7. Alignment-pattern refinement

For V2 and above, the QR spec defines one or more alignment patterns. After computing the initial homography from the three finders, we refine it by searching for the bottom-right alignment pattern in the source image (the one nearest the `(col, row) = (n-7, n-7)` corner). Refining the bottom-right corner alone is usually enough; symbols with severe local distortion can refine every alignment pattern individually at the cost of more solve steps.

V1 has no alignment patterns and skips this step.

## 8. Module sampling

With `H` in hand, sampling is straightforward:

```text
function sample(col, row) -> bool:
    (u, v) = H · (col + 0.5, row + 0.5, 1)   # centre of the module
    return grayscale[v * width + u] < threshold
```

The `+ 0.5` shifts to the module's centre in grid coordinates. For robustness we can average a 3×3 patch around `(u, v)` and threshold the mean, but a single-pixel sample is the textbook approach.

The result is a `[][]bool` of side `n` × `n` — exactly what the matrix decoder consumes.

## Implementation pointers

- `qrgen/decode_image.go` hosts the bitmap struct, grayscale conversion, Otsu, finder scanning, geometry validation, homography, and sampling.
- Numerical work uses `float64` throughout; we promote `uint8` and `int` pixel coordinates only at the boundary.
- For testing, generate a PNG with our own encoder, then run it back through the decoder — the matrix that comes out must match the original `[][]bool` exactly. Synthetic rotated/scaled variants exercise the homography path.

## References

- ISO/IEC 18004:2015, §11 — reference decode algorithm.
- Otsu, N. — "A Threshold Selection Method from Gray-Level Histograms," IEEE Trans. Systems, Man, and Cybernetics, 1979.
- Hartley & Zisserman — *Multiple View Geometry in Computer Vision*, 2nd ed., §4 (homography estimation).
- ZXing — *open-source decoder reference*: <https://github.com/zxing/zxing>, especially the `FinderPatternFinder` and `PerspectiveTransform` classes.
