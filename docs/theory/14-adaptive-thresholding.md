# Adaptive Thresholding for the Decoder

The v0.2 decoder uses **Otsu's method** as its only binariser, picking one global threshold for the entire image. That works perfectly for our own encoder output and for evenly-lit photos, but it falls apart on real-world captures with gradients, soft shadows, vignettes, or low overall contrast. This document explains why Otsu fails on those inputs, what local adaptive thresholding does instead, why we chose **Sauvola's algorithm** over Niblack / Bernsen / Adaptive Gaussian as the fallback in v0.3, how integral images keep Sauvola linear in pixel count, and how the decoder decides at runtime which binariser to use.

> Indonesian version: [14-adaptive-thresholding.id.md](14-adaptive-thresholding.id.md).

## 1. Why Otsu fails on uneven lighting

Otsu's method maximises the *between-class variance* of the grayscale histogram. Concretely it picks the threshold `t` that maximises:

```text
σ²_B(t) = w_0(t) * w_1(t) * (μ_0(t) - μ_1(t))²
```

where `w_0, w_1` are the cumulative weights of the two classes and `μ_0, μ_1` their means. The choice has one crucial assumption: the histogram is approximately **bimodal**, with a clean valley between the dark-ink peak and the bright-paper peak. Three real-world conditions break that assumption.

- **Linear gradients** (one side of the image lit, the other in shadow) blend the two peaks into a continuum. The histogram becomes broad and flat. Otsu still returns a threshold, but pixels on the dark side that *should* be paper end up below it, and pixels on the lit side that *should* be ink end up above it.
- **Vignettes and drop shadows** create a low-frequency offset across the image. Otsu sees a histogram whose peaks have been smeared apart and picks a threshold that satisfies the global average while sacrificing one corner of the symbol.
- **Low overall contrast** (faint print, dim lighting, washed-out scans) leaves only a single peak. Otsu's between-class variance has no clear maximum and the chosen `t` lands almost arbitrarily, often producing a binarisation where one class is empty.

In all three cases, the symbom is that the binarised image has finder patterns destroyed in at least one corner, and `findFinders` either returns fewer than three candidates or returns geometrically invalid ones. That is exactly the failure mode this milestone addresses.

## 2. Niblack's idea: localise the threshold

The fix is to stop picking one threshold for the whole image. **Niblack (1986)** proposed computing a separate threshold for every pixel based on the statistics of a small window centred on that pixel:

```text
T_Niblack(x, y) = m(x, y) + k * s(x, y)
```

where `m(x, y)` is the local mean and `s(x, y)` the local standard deviation of pixel intensities in a `w x w` window around `(x, y)`, and `k` is a tunable constant (typically `k = -0.2` for dark-on-light text). The threshold tracks lighting: in a dark region the mean is low, so the threshold is low; in a bright region the mean is high, so the threshold is high. Gradients no longer matter because the threshold rides with the gradient.

Niblack works on documents but has two well-known weaknesses. In uniform regions (no ink, just paper or just shadow) the standard deviation is tiny and the threshold collapses onto the mean, classifying random noise as ink. And the parameter `k` is brittle: a value tuned for one document fails on the next.

## 3. Sauvola's refinement

**Sauvola and Pietikainen (2000)** introduced a normalisation that suppresses the uniform-region noise problem:

```text
T_Sauvola(x, y) = m(x, y) * (1 + k * (s(x, y) / R - 1))
```

The key change is the `(s / R - 1)` term, where `R` is a normalising range — for 8-bit images the standard choice is `R = 128`, half the dynamic range. Inside this term:

- In a **high-variance** window (real text edge), `s` is close to `R`, the bracket approaches `1 + k * 0 = 1`, and the threshold sits near the local mean. Ink and paper separate cleanly.
- In a **low-variance** window (uniform paper, uniform shadow), `s` is far below `R`, the bracket goes negative, and the threshold drops well below the local mean. Random noise no longer crosses the threshold and the region stays as one class.

The classical parameter pair is `k = 0.2` (paper) or `k ≈ 0.2..0.5`, with `w = 25` for documents printed at typical DPI. We adopt the textbook defaults and treat them as internal constants in v0.3; if real-world fixtures demand tuning, a public option can come later.

## 4. Integral images: making Sauvola O(width * height)

A naive computation of the local mean and standard deviation costs `O(w²)` per pixel, so the full image costs `O(width * height * w²)` — for `w = 25` that is 625 extra reads per pixel. **Integral images** (Shafait, Keysers, Breuel 2008) reduce that to `O(1)` per pixel regardless of window size.

An integral image `S` is the running sum of pixel values along both axes:

```text
S[y][x] = Σ_{i ≤ y, j ≤ x} pixel(i, j)
```

It can be filled in one linear pass via:

```text
S[y][x] = pixel(y, x) + S[y-1][x] + S[y][x-1] - S[y-1][x-1]
```

Given `S`, the sum of pixels inside any rectangle `[(x0, y0), (x1, y1)]` becomes a constant-time corner-arithmetic query:

```text
sum_rect = S[y1][x1] - S[y0-1][x1] - S[y1][x0-1] + S[y0-1][x0-1]
```

Dividing by the rectangle area gives the local mean.

For Sauvola we also need the **local variance**, which requires the running sum of *squared* pixel values. We build a second integral image `S²` with `S²[y][x] = Σ pixel²` and recover the variance via the standard identity:

```text
mean   = sum_rect  / area
mean²  = sum²_rect / area
var    = mean² - mean²
std    = sqrt(var)
```

Both integral images are filled in one linear pass over the grayscale buffer, and each Sauvola threshold then costs four loads from each integral image — eight loads plus a handful of multiplications and one `sqrt`. The total cost of Sauvola binarisation is `O(width * height)`, the same asymptotic complexity as Otsu.

A practical caveat: a 4096x4096 image of 8-bit values sums to about `4.3 * 10^9` for `S` and `1.1 * 10^12` for `S²`. Both fit comfortably in `uint64`, blow `uint32`, so we use `uint64` throughout.

## 5. Why Sauvola over the alternatives

| Algorithm | Idea | Strength | Weakness |
| --- | --- | --- | --- |
| Otsu | One global threshold maximising between-class variance | Fast, parameter-free, perfect on clean bimodal histograms | Fails on gradients, shadows, low contrast |
| Niblack | Local mean + `k * std` | Tracks lighting | Noise in uniform regions; brittle `k` |
| **Sauvola** | Niblack with `(s/R - 1)` normalisation | Tracks lighting AND suppresses uniform-region noise; standard for document binarisation | Slightly slower than Niblack; needs integral images for speed |
| Wolf | Sauvola with global min/max normalisation | Slightly better on very low contrast | Loses Sauvola's locality benefit; extra global pass |
| Bernsen | Use local midrange between min and max instead of mean+std | Tracks local contrast directly | Very noisy in uniform regions; outlier-sensitive |
| Adaptive Gaussian (OpenCV) | Threshold = weighted Gaussian mean of the window minus a constant `C` | Smooth; available in image libs | Equivalent to Niblack flavours under the hood; tunable `C` and window have the same brittleness |

Sauvola is the standard choice for document binarisation precisely because it inherits Niblack's locality and fixes Niblack's uniform-region noise. The qr-generator decoder needs exactly that: QR symbols are documents that happen to be square, and the failure modes we want to repair (gradients, shadows, low contrast) are document-binarisation territory.

## 6. The runtime dispatch: when do we use which?

Even when Sauvola is available, we do not want to pay its integral-image cost on every decode. Synthetic PNGs and well-lit photos are common, and Otsu is provably faster on them. The decoder therefore uses a **two-stage gate** to decide which binariser runs.

**Stage 1 — proactive bimodality gate.** Otsu already computes the maximum between-class variance `σ²_B` and the total variance `σ²_T` of the histogram. Their ratio:

```text
η = σ²_B / σ²_T   ∈ [0, 1]
```

is the standard *separability measure* that Otsu's original paper defines. `η` close to 1 means the histogram is well-bimodal and Otsu's threshold is meaningful; `η` close to 0 means the histogram is essentially unimodal and Otsu cannot do its job. We pick `η_min = 0.5` as the default cutoff — when `η < η_min`, we know Otsu's binarisation will be unhealthy *before* we run it, so we skip the Otsu binarisation step entirely and go straight to Sauvola. This costs only a histogram pass (which Otsu does anyway to compute the threshold) and saves a wasted finder-detection pass on the unhealthy Otsu output.

**Stage 2 — reactive post-check.** If `η ≥ η_min` we run Otsu's binarisation and finder detection as usual. Two things can still go wrong: the foreground ratio of the binarised image might be degenerate (less than 5% or more than 95%, meaning effectively one class only), or finder detection might still fail for a different reason. In either case, the pipeline rebinarises the grayscale image with Sauvola and re-runs finder detection. Only when the Sauvola pass also fails does the decoder return `ErrFinderNotFound`.

This combination keeps the common case (clean PNG, well-lit photo) on the Otsu fast path, lets a clearly bad histogram skip straight to Sauvola, and keeps Sauvola as a safety net for cases where the histogram looked fine but the spatial structure still defeated Otsu (e.g. one corner shadowed but the histogram still bimodal globally).

The three runtime states are visible to tests via an unexported `binariserUsed` hook: `binariserOtsu`, `binariserSauvolaProactive` (Stage 1 fired), and `binariserSauvolaReactive` (Stage 2 fired). The hook never appears in the public API.

## 7. Implementation pointers

- `qrgen/decode_image.go` hosts the existing `otsuThreshold` and `binarise`. The Otsu function gains a return value for `η` (the separability measure) without changing its threshold output, so callers can read both with one call.
- `qrgen/decode_image_sauvola.go` (new in v0.3) hosts `sauvolaBinarise`, the two integral images, and the `windowMeanStd` helper. The function returns a `bitmap` with the same `p <= t` convention as Otsu so finder detection downstream sees no difference.
- The dispatch logic is a small `if η < ηMin { ... } else { ... }` block inside `decodeImage`, plus a reactive `else if !found || !healthy { ... }` fallback. We deliberately keep it as straight code instead of a strategy interface — there are only two algorithms and YAGNI applies.
- Defaults are unexported constants in `decode_image_sauvola.go`: `sauvolaWindow = 25`, `sauvolaK = 0.2`, `sauvolaR = 128.0`, `etaMin = 0.5`. The theory doc explains the numbers; the constants document where they live.

## References

- Otsu, N. — "A threshold selection method from gray-level histograms," *IEEE Trans. Systems, Man, and Cybernetics*, 9(1):62–66, 1979. Defines the separability measure `η` reused in our proactive gate.
- Niblack, W. — *An Introduction to Digital Image Processing*, Prentice-Hall, 1986. Sauvola's predecessor.
- Sauvola, J., Pietikainen, M. — "Adaptive document image binarization," *Pattern Recognition*, 33(2):225–236, 2000. The formula and the `k = 0.2`, `R = 128` defaults.
- Shafait, F., Keysers, D., Breuel, T. M. — "Efficient implementation of local adaptive thresholding techniques using integral images," *Document Recognition and Retrieval XV*, SPIE, 2008. The integral-image trick that keeps Sauvola linear.
- Wolf, C., Jolion, J.-M. — "Extraction and recognition of artificial text in multimedia documents," *Pattern Analysis and Applications*, 6(4):309–326, 2003. Wolf's variant on Sauvola.
- Viola, P., Jones, M. — "Rapid object detection using a boosted cascade of simple features," *CVPR*, 2001. Popularised integral images in computer vision (Section 2).
