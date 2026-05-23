# QR Decoder — Adaptive Thresholding Plan

This document describes the implementation plan for the **adaptive thresholding** enhancement that targets the `v0.3.0` minor release. It is the first of two robustness branches that split out of the v0.2.0 Roadmap (the other branch, **rotation handling**, is tracked separately in its own plan and lands as `v0.4.0`).

> Status: **draft / living document.** Milestones T1..T6 land incrementally on the `decoder-thresholding` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M1..M11 and D1..D14.

> Indonesian version: [docs/plan-decoder-thresholding.id.md](plan-decoder-thresholding.id.md).

---

## 1. Vision & Goals

- Lift the **"No local thresholding"** limitation called out in README under `## Limitations` and `## Roadmap` so the decoder can recover QR codes from photos with uneven lighting, soft shadows, or strong illumination gradients.
- Keep the existing **Otsu global threshold** as the default fast path so synthetic PNGs and evenly-lit captures pay zero new cost. Adaptive thresholding only kicks in when Otsu's output is provably bad.
- Preserve the public API: callers continue to invoke `qrgen.Decode` / `qrgen.DecodeBytes` / `qrgen.DecodeMatrix` unchanged. All new behaviour is internal to the image stage.
- Stay theory-first and bilingual: write `docs/theory/14-adaptive-thresholding.md` (EN + ID) before any code lands, in the same spirit as docs 11..13.
- Stay pure Go: no new third-party dependencies. Sauvola's integral-image computation is straightforward over `[]uint8`.

## 2. Design Principles

1. **Otsu first, Sauvola as fallback.** Most inputs are clean; we do not want to pay Sauvola's window-scan cost on every decode. The pipeline tries Otsu, runs finder detection, and only falls back to Sauvola when that detection fails or when Otsu produces a binarisation with a degenerate foreground-to-background ratio.
2. **No new public surface for v0.3.** No new functional option, no new sentinel error. The strategy decision is hidden inside `decodeImage` so the API stays minimal until real users ask for control.
3. **Integral images for speed.** A naive Sauvola pass costs `O(width · height · w²)` for window size `w`. We precompute integral images for `sum(x)` and `sum(x²)` so each window mean and variance becomes `O(1)` per pixel and the total cost stays `O(width · height)`.
4. **Tunable but defaulted.** Sauvola's window size and `k` parameter sit as unexported package-level constants chosen via the QR-Code Wikipedia reference (`w = 25`, `k = 0.2`); we resist exposing them until a real-world image set tells us a different default works better.
5. **Tests first.** Every milestone ships with table-driven Go tests and at least one round-trip case that fails on `master` (`Decode` returns `ErrFinderNotFound`) and passes on the branch (Decode round-trips the original payload).

## 3. Scope

### In scope for v0.3.0

- **Sauvola adaptive thresholding** with integral-image acceleration.
- **Otsu -> Sauvola fallback heuristic** wired into `decodeImage`, gated on Otsu-path failure.
- **Theory doc** `14-adaptive-thresholding.md` (EN + ID) covering Otsu's limitations, Sauvola's formula, integral images, and the trade-off vs Niblack / Bernsen / Adaptive Gaussian.
- **Synthetic test fixtures** for uneven illumination: linear gradient, radial gradient, vignette, and soft drop-shadow; rendered procedurally inside tests so we keep `testdata/` free of binary blobs.
- **Benchmarks** that prove the Otsu-only hot path has not regressed (`BenchmarkDecodeImageSmall`, `BenchmarkDecodeImageURL` must stay within 1% of v0.2.0 baseline).
- **Documentation updates**: remove "No local thresholding" from Limitations; update Roadmap; add a CHANGELOG `v0.3.0` entry.

### Out of scope (still)

- **Rotation handling.** Tracked in a parallel plan and branch (`decoder-rotation`); lands as v0.4.0.
- **Multi-symbol detection.** No change.
- **Public knobs for Sauvola parameters.** Defaults only; revisit when callers complain or real images need different settings.
- **Adaptive thresholding for the *encoder*.** Encoding does not see images.
- **Non-Otsu alternatives** beyond Sauvola (Niblack, Bernsen, Wolf, Adaptive Gaussian). Discussed in theory doc, not implemented.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after T3) gives a working Sauvola fallback that recovers at least one fixture image which fails on `master`. **Checkpoint B** (T6) is the `v0.3.0` release.

### T1 — Adaptive Thresholding Theory Doc `(S)`

Goal: cover the new algorithm and the fallback heuristic in `docs/theory/` before any code lands.

- [x] `docs/theory/14-adaptive-thresholding.md` — Otsu recap and failure modes (gradient, shadow, low contrast), Niblack as Sauvola's ancestor, Sauvola's formula `T(x, y) = mean(x, y) * (1 + k * (std(x, y) / R - 1))` with the standard `R = 128`, `k = 0.2`, `w = 25` window defaults, integral-image construction for O(1) window queries, and a comparison table vs Niblack / Bernsen / Adaptive Gaussian explaining why Sauvola wins for documents and printed material like QR symbols. Also documents the runtime two-stage dispatch (proactive bimodality `η < η_min` plus reactive post-check) so the implementation milestones T2 and T3 only need to point at named sections.
- [x] Indonesian counterpart `docs/theory/14-adaptive-thresholding.id.md`.
- [x] Update `docs/theory/README.md` and `docs/theory/README.id.md` to add entry 14 with a one-line summary under a new subsection "Decoder robustness (v0.3.0)", plus a row in the "How these notes relate to the code" table pointing at `qrgen/decode_image_sauvola.go` (planned, T2 + T3).

### T2 — Sauvola Binariser `(M)`

Goal: a `sauvolaBinarise(img image.Image) *bitmap` that returns the same `bitmap` shape produced by the existing `binarise`. Lives in `qrgen/decode_image_sauvola.go`.

- [x] Build two integral images: `sum` for pixel values and `sum2` for squared values, both flattened `[]uint64` of size `(w+1) * (h+1)` so corner-arithmetic queries do not need bounds checks. The running-row-sum recurrence keeps the build to one linear pass.
- [x] Query helper `windowMeanStd(sum, sum2, w, h, x, y, half)` that returns `(mean, std)` for the centred window of half-extent `half`, clipped at the image bounds; guards against tiny negative variances from float rounding.
- [x] Apply the Sauvola formula per pixel and emit `bool` into the same `bitmap` struct used by Otsu. Constants `sauvolaWindow = 25`, `sauvolaK = 0.2`, `sauvolaR = 128.0` are unexported package-level values.
- [x] Mirror Otsu's `p <= t` convention so finder detection downstream stays unchanged.
- [x] Split `sauvolaBinariseFromGray` out from `sauvolaBinarise` so the T3 dispatch can reuse the grayscale buffer the Otsu pass already computed without re-walking the image.
- [x] Tests in `qrgen/decode_image_sauvola_test.go`: hand-checked integral-image values on a 3x2 fixture; `windowMeanStd` cross-validated against a naive O(w^2) reference over a 12x10 pseudo-random buffer for several half-extents; uniform image stays all-light (the property the proactive gate at T3 will lean on); two-illumination-region fixture where Sauvola classifies every ink and paper sample point correctly AND the same fixture proves Otsu fails in at least one direction so the test cannot pass vacuously; image smaller than the window does not panic; zero-sized image returns an empty bitmap.

### T3 — Fallback Heuristic in `decodeImage` `(S)`

Goal: invoke Sauvola only when Otsu's output looks unhealthy, and skip Otsu's binarisation pass entirely when the histogram already proves it will be unhealthy.

- [x] **Pre-check (proactive, free):** `otsuThreshold` now returns `(threshold, η)` where `η = σ²_B / σ²_T` is the standard separability measure in `[0, 1]`. The dispatch in `decodeImage` reads η and, when it falls below `etaMin`, skips the Otsu binarisation step entirely and goes straight to Sauvola — saving one full finder-detection pass in the failure case.
- [x] **Post-check (reactive, defense-in-depth):** when Otsu binarisation runs, the dispatch treats its output as unhealthy if either (a) `findFinders` returns `ErrFinderNotFound`, or (b) `foregroundRatio(bm)` falls outside `[foregroundLo, foregroundHi]` (degenerate single-class output). In either case it rebinarises the grayscale image with Sauvola via the shared grayscale buffer and re-runs finder detection. Only when the Sauvola pass also fails does the dispatch return `ErrFinderNotFound`.
- [x] Default `etaMin = 0.5`, `foregroundLo = 0.05`, `foregroundHi = 0.95` per Otsu's original paper; empirical note recorded in tests: textbook Otsu gives η ≈ 0.64 for a Gaussian and 0.75 for any uniform distribution, so the proactive gate only fires for genuinely degenerate (monochrome / single-delta) inputs. The reactive gate carries most of the v0.3 fallback weight. Tuning waits for T4 fixtures.
- [x] Introduced `binariserUsedState` (unexported) with values `binariserOtsu`, `binariserSauvolaProactive`, `binariserSauvolaReactive` and a `String()` method, plus `decodeImageDebug` — the package-internal sibling of `decodeImage` that returns the binariser state alongside the text. `decodeImage` is now a thin wrapper that throws the state away; nothing leaks to the public API.
- [x] Tests in `qrgen/decode_image_sauvola_test.go` cover all three branches: clean encoded PNG asserts `binariserOtsu` plus payload round-trip; a monochrome 80x80 image (η=0 by the variance-collapse branch) asserts `binariserSauvolaProactive`; a brightness-compression mutation of a clean QR that lifts right-side ink above left-side paper asserts `binariserSauvolaReactive` after first verifying that Otsu alone really fails on the fixture and that η stays above `etaMin` so the proactive branch could not have fired. End-to-end decode recovery on the reactive fixture is deferred to T4 where curated synthetic fixtures land.
- [x] Helpers `otsuBinariseFromGray` and `foregroundRatio` are extracted so the dispatch can run Otsu and Sauvola off the same grayscale pass without re-walking the image.

### Checkpoint A — Sauvola fallback recovers at least one image where v0.2 fails.

### T4 — Synthetic Uneven-Lighting Fixtures `(S)`

Goal: lock in regression coverage across the lighting failure modes the fallback was designed for.

- [ ] Render fixtures procedurally inside `qrgen/decode_image_sauvola_test.go` using the encoder to build a clean QR, then mutate the gray channel with one of: linear horizontal gradient (left dark, right bright), radial darkening (vignette), diagonal gradient, and a soft drop-shadow rectangle covering one quadrant.
- [ ] Assert each fixture decodes back to the original payload via `DecodeBytes` and that the Sauvola branch was hit.
- [ ] Add a low-contrast variant where global grey min/max stay within a 60-value band; confirm Sauvola still resolves modules even when Otsu picks a marginal threshold.
- [ ] Keep all fixtures in-process and small (V1..V3 only) so the test stays under 300 ms on a laptop.

### T5 — Benchmarks & Regression Guard `(S)`

Goal: prove the Otsu-only path has not regressed and quantify Sauvola overhead.

- [ ] Re-run `BenchmarkDecodeImageSmall`, `BenchmarkDecodeImageMultiBlock`, `BenchmarkDecodeImageURL`, `BenchmarkDecodeImageFromPNGDecode` and confirm allocations and ns/op stay within 1% of master's baseline (record both before/after numbers in the commit message).
- [ ] Add `BenchmarkDecodeImageSauvolaFallback` that forces the fallback path (gradient fixture) so the Sauvola cost is visible in `go test -bench`.
- [ ] `go test -race ./...` remains clean.

### T6 — Polish & Release `(S)`

Goal: cut `v0.3.0`.

- [ ] README updates: remove `**No local thresholding.**` bullet from `## Limitations` and the matching `local thresholding` clause from `## Roadmap`. Replace with a one-line "Adaptive thresholding (Sauvola fallback)" mention under `## Decoding QR codes` or in a new sub-paragraph under that section explaining the fallback is automatic.
- [ ] `CHANGELOG.md` `v0.3.0` entry under `### Added` (Sauvola binariser, fallback heuristic, theory doc 14, benchmark) and `### Validated` (synthetic uneven-lighting fixtures across linear, radial, diagonal, drop-shadow, low-contrast variants; Otsu hot path within 1% of v0.2 baseline).
- [ ] Bump module-level version constant only if we add one; otherwise just the CHANGELOG and tag carry the v0.3.0 marker.
- [ ] Tag `v0.3.0` after the first push to GitHub so the tag lands on the commit the remote sees: `git tag -a v0.3.0 -m "Adaptive thresholding release" && git push origin v0.3.0`.

---

## 5. Proposed File Layout Delta

All new code lands inside `qrgen/` next to the existing decoder image stage. No new package directories.

```
qrgen/
├── decode_image.go              # existing — gains the Otsu-or-Sauvola dispatch
├── decode_image_sauvola.go      # new — sauvolaBinarise + integral image helpers
├── decode_image_sauvola_test.go # new — Sauvola unit tests + fallback integration tests
└── decode_bench_test.go         # existing — gains BenchmarkDecodeImageSauvolaFallback
docs/
├── plan-decoder-thresholding.md     # this file
├── plan-decoder-thresholding.id.md  # Indonesian counterpart
└── theory/
    ├── 14-adaptive-thresholding.md
    └── 14-adaptive-thresholding.id.md
```

## 6. Risks & Technical Notes

- **Integer overflow in integral images.** A 4096x4096 image of 8-bit grey values sums to at most `4096 * 4096 * 255 = ~4.3 * 10^9` for `sum` and `~1.1 * 10^12` for `sum2`. Both fit easily into `uint64` but blow `uint32`. We use `uint64` throughout.
- **Sauvola window size at small symbols.** For V1 at 4x module-size rendering, the symbol is about 84 px wide, so a 25 px window covers roughly 7 modules; that is a reasonable local neighbourhood. For very large module sizes the window may shrink in relative terms; the theory doc records this caveat without changing the default.
- **False positives in finder detection after Sauvola.** Sauvola can introduce small black speckles in uniform regions; finder pattern detection's 1:1:3:1:1 ratio check and right-angle geometry validation should reject these, but we keep a regression test that throws gaussian-noise fixtures at the pipeline to confirm.
- **Floating-point reproducibility.** Sauvola's `std` calculation uses `sqrt`; we keep it in `float64` and accept that exact thresholds are platform-stable to the precision Go's standard library guarantees (no `cgo`, no SIMD vectorisation surprises).
- **Branch divergence with rotation work.** The rotation branch (`decoder-rotation`) will land later and may touch `decodeImage` in the same region. We minimise conflict surface by isolating Sauvola behind a single helper and keeping the dispatch logic in `decodeImage` to a small `if !found || !healthy` block.

## 7. References

- Sauvola, J., Pietikainen, M. — "Adaptive document image binarization," *Pattern Recognition*, 33(2):225–236, 2000. The canonical paper for the formula and the `k = 0.2`, `R = 128` defaults.
- Niblack, W. — *An Introduction to Digital Image Processing*, Prentice-Hall, 1986. Sauvola's predecessor, included in the theory doc as motivation.
- Shafait, F., Keysers, D., Breuel, T. M. — "Efficient implementation of local adaptive thresholding techniques using integral images," *Document Recognition and Retrieval XV*, SPIE, 2008. The integral-image trick that keeps Sauvola O(width * height).
- Otsu, N. — "A threshold selection method from gray-level histograms," *IEEE Trans. Systems, Man, and Cybernetics*, 9(1):62–66, 1979. Already cited in doc 12; relisted here for the failure-mode discussion.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Hard-coded vs configurable Sauvola parameters.** Default to hard-coded `w = 25`, `k = 0.2`, `R = 128`. Revisit if real-world fixtures need tuning, but do not add a public option in v0.3.
- **Always-Sauvola escape hatch.** Should we expose an internal "force Sauvola" hook for users who know their inputs are always uneven? Defer: a single `WithBinarisation(strategy)` option could come in v0.4 alongside the rotation work if there is demand.
- **Otsu-failure detection precision.** The current plan combines a proactive bimodality gate (`η < η_min` skips Otsu) with reactive post-checks (finder detection failure OR foreground ratio outside `[0.05, 0.95]`). Default `η_min = 0.5`; the exact value, and whether bimodality alone is enough to retire the post-checks, are worth measuring on the T4 synthetic fixture set before locking.
- **Theory doc placement.** Should doc 14 live as a standalone entry or as a new subsection under doc 12? Going with standalone to keep doc 12 frozen as the v0.2 record.
