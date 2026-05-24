# QR Decoder — Rotation Handling Plan

This document describes the implementation plan for the **rotation handling** enhancement that targets the `v0.4.0` minor release. It is the second of two robustness branches that split out of the v0.2.0 Roadmap (the first, **adaptive thresholding**, shipped as `v0.3.0` from the `decoder-thresholding` branch).

> Status: **draft / living document.** Milestones R1..R6 land incrementally on the `decoder-rotation` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M1..M11, D1..D14, and T1..T6.

> Indonesian version: [docs/plan-decoder-rotation.id.md](plan-decoder-rotation.id.md).

---

## 1. Vision & Goals

- Lift the **"No rotated-image decoding"** limitation called out in README under `## Limitations` and `## Roadmap` so the decoder can recover QR codes from images that are rotated 90 / 180 / 270 degrees (a phone held sideways while scanning a printed page is the canonical case) plus soft tilts up to roughly 30 degrees off-axis.
- Keep the public API surface identical: callers continue to invoke `qrgen.Decode` / `qrgen.DecodeBytes` / `qrgen.DecodeMatrix` unchanged. All new behaviour is internal to the finder-ordering stage.
- Stay theory-first and bilingual: write `docs/theory/15-rotation-handling.md` (EN + ID) before any code lands, in the same spirit as the v0.3 sequence (plan -> theory -> code).
- Stay pure Go: no new third-party dependencies, no extra allocation on the hot path.

## 2. Design Principles

1. **Find the structural bug, not the surface symptom.** Inspection of [qrgen/decode_image.go:367-371](../qrgen/decode_image.go#L367-L371) shows that `orderFinderTriple` already identifies the right-angle vertex (top-left) via "vertex opposite to the longest side" — that step is rotation-invariant. The only rotation-broken step is the very last `if tr.y > bl.y` discriminator that decides which of the remaining two finders is top-right. Replacing that single check with a cross-product handedness test is enough to unlock rotation handling.
2. **Trust the homography.** `homographyFromFinders` already solves for a general 3x3 projective transform from four anchor-point correspondences. Once `orderFinderTriple` returns the correct labels, the homography absorbs the rotation, tilt, and perspective without any change. We do not need to touch the linear-solver path.
3. **Finder detection itself is rotation-symmetric.** The 1:1:3:1:1 row scan picks up the finder pattern at any orientation because the pattern is a concentric square — a horizontal line through its centre always crosses dark/light/dark/light/dark in the 1:1:3:1:1 ratio (within the ±50% module tolerance). The same is true vertically. Axis-aligned rotations (90 / 180 / 270) and soft tilts (~0–30 degrees) need no scanner changes.
4. **No new public knob in v0.4.** No functional option, no new sentinel error. Tilts beyond ~30 degrees and arbitrary 0..360 rotation remain future work because they would require either a contour-based finder detector or a more flexible 1:1:3:1:1 scanner — both significant rewrites that do not belong in a minor release.
5. **Tests first.** Every milestone ships with table-driven Go tests and at least one round-trip case that fails on `master` (`Decode` returns `ErrFinderNotFound`) and passes on the branch (`Decode` round-trips the original payload).

## 3. Scope

### In scope for v0.4.0

- **Axis-aligned rotations:** 90, 180, and 270 degrees both directions.
- **Soft tilts:** up to roughly 30 degrees off axis (limited by the existing ±50% tolerance inside `fitsFinderRatio`).
- **`orderFinderTriple` rewrite:** swap the upright-image `y`-coordinate discriminator for a cross-product handedness test that works at any rotation.
- **Theory doc** `15-rotation-handling.md` (EN + ID) covering right-angle vertex detection, cross-product handedness, the proof sketch that the homography handles the rest, and the explicit scope statement (axis-aligned + soft tilts in v0.4, arbitrary 0..360 deferred).
- **Synthetic in-memory rotation fixtures** built procedurally inside test files: encode `"HELLO"` with the existing API, render the matrix into an `*image.Gray` rotated by 0 / 90 / 180 / 270 plus soft tilts (e.g. 15, 30 degrees) using straight Go arithmetic, and assert `DecodeBytes` round-trip.
- **Documentation updates:** remove `**No rotated-image decoding**` from `## Limitations`; update `## Roadmap` to focus the open work on arbitrary-angle decoding rather than the whole rotation category; add a CHANGELOG `v0.4.0` entry.

### Out of scope (still)

- **Arbitrary rotation in [30, 90) degrees.** The 1:1:3:1:1 scanner can produce false negatives at angles where module edges hit row pixels at oblique angles; recovering this band would require either contour-tracing or a fan-of-orientations search. Deferred to a future minor.
- **Mirrored QR symbols.** The cross-product check rejects mirrored handedness on purpose because real QR codes are never mirrored.
- **Multi-symbol detection.** Same as v0.3: not in scope.
- **Combined rotation + heavy uneven lighting.** The v0.3 Sauvola fallback and the v0.4 ordering fix compose naturally, but no new fixture explicitly stress-tests the combination — recoverability is the union of each milestone's coverage.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after R2) gives a working rotation-invariant finder ordering whose unit tests pass on synthetic coordinates. **Checkpoint B** (R6) is the `v0.4.0` release.

### R1 — Rotation Handling Theory Doc `(S)`

Goal: cover the geometry and the algorithm in `docs/theory/` before any code lands.

- [ ] `docs/theory/15-rotation-handling.md` — Why the existing `orderFinderTriple` fails for rotated symbols (the `tr.y > bl.y` shortcut), why "vertex opposite to the longest side" already nails the top-left at any rotation, the cross-product handedness identity `(TR - TL) x (BL - TL) > 0` and a worked sign analysis at 0 / 90 / 180 / 270 degrees in image coordinates (with `y` growing downward), why the same identity rejects mirrored symbols cleanly, and a short proof sketch that the homography stage absorbs rotation once the labels are right.
- [ ] Indonesian counterpart `docs/theory/15-rotation-handling.id.md`.
- [ ] Update `docs/theory/README.md` and `docs/theory/README.id.md` to add entry 15 with a one-line summary under a new subsection "Decoder robustness (v0.4.0)", plus a row in the "How these notes relate to the code" table pointing at `qrgen/decode_image.go` (the `orderFinderTriple` neighbourhood).

### R2 — Rotation-Invariant `orderFinderTriple` `(S)`

Goal: replace the upright-image y-discriminator with a cross-product handedness check so finder labelling works at any rotation.

- [ ] Keep the existing "longest side opposite the right-angle vertex" path that already picks the top-left correctly.
- [ ] Replace the `if tr.y > bl.y || (math.Abs(tr.y - bl.y) < 1 && tr.x < bl.x) { tr, bl = bl, tr }` block with `if cross((tr - tl), (bl - tl)) < 0 { tr, bl = bl, tr }` so the labelling holds at any rotation. (Sign convention is image-coordinate cross product with `y` growing downward, so the un-mirrored, real-QR case sits on the positive side.)
- [ ] Keep the existing right-angle and leg-ratio sanity checks unchanged; they were already rotation-invariant.
- [ ] Unit tests in `qrgen/decode_image_test.go` that build three synthetic `finderCandidate` triples at 0 / 90 / 180 / 270 degrees and at a 30-degree tilt, then assert `orderFinderTriple` produces the same `(tl, tr, bl)` identities each time.

### Checkpoint A — Rotation-invariant ordering compiles and passes coordinate-level tests.

### R3 — Synthetic Rotation Fixtures `(M)`

Goal: lock in end-to-end recovery coverage for axis-aligned rotations and soft tilts via in-memory image generation.

- [ ] Add a helper `rotateImage(src image.Image, angleDeg float64) *image.Gray` inside `qrgen/decode_rotation_test.go` that renders the source image into a new gray buffer with bilinear sampling, using a destination rectangle large enough to hold the rotated content plus the existing quiet zone. Background fill is the source's quiet-zone colour.
- [ ] Fixtures `TestRotation90`, `TestRotation180`, `TestRotation270`: encode `"HELLO"`, rotate by the matching angle, assert `DecodeBytes` recovers the payload and `decodeImageDebug` reports `binariserOtsu` (rotation should not perturb the binariser dispatch).
- [ ] Fixtures `TestRotationSoftTilt15` and `TestRotationSoftTilt30`: same shape but at 15 and 30 degrees; soft tilts are inside the ±50% ratio tolerance and should round-trip.
- [ ] One explicit negative fixture `TestRotationSoftTiltOutOfBand` at 45 degrees that asserts `ErrFinderNotFound` (no rotation), documenting the v0.4 boundary inside the test suite itself.
- [ ] Keep all fixtures in-process, V1 only, so the rotation batch stays under 200 ms on a laptop.

### R4 — Documentation Polish `(S)`

Goal: align README and CHANGELOG with what shipped.

- [ ] README `## Limitations`: remove the `**No rotated-image decoding**` bullet; replace with `**Limited arbitrary-angle decoding**` recording that 90 / 180 / 270 and tilts up to ~30 degrees work but the 30..90-degree band is out of reach until the scanner is updated. Stays honest about scope.
- [ ] README `## Roadmap`: narrow the decoder robustness bullet from "arbitrary rotations" (now partially done) to "arbitrary-angle decoding for the 30..90-degree band, contour-based finder detection".
- [ ] README `## Decoding QR codes`: add one sentence acknowledging axis-aligned rotation support, pointing at `docs/theory/15-rotation-handling.md`.
- [ ] CHANGELOG `v0.4.0` entry under `### Added` (cross-product handedness in `orderFinderTriple`, theory doc 15, plan doc, synthetic rotation fixtures), `### Validated` (R3 fixtures, `go test -race` clean, no encoder regression).
- [ ] Plan checklist for R1..R6 ticked.

### R5 — Benchmarks & Regression Guard `(S)`

Goal: confirm the ordering change is allocation-neutral and within run-to-run noise of v0.3.

- [ ] Re-run `BenchmarkDecodeImageSmall`, `BenchmarkDecodeImageMultiBlock`, `BenchmarkDecodeImageURL`, `BenchmarkDecodeImageFromPNGDecode`, `BenchmarkDecodeImageSauvolaFallback` against the v0.3.0 tag and the branch HEAD. The cross-product is a single multiply-subtract-compare, so the regression budget is the same as v0.3 (within ~1% of baseline).
- [ ] Optionally add `BenchmarkDecodeImageRotated90` that runs the rotation fixture through the full pipeline to publish the cost of axis-aligned rotation.
- [ ] `go test -race ./...` remains clean.

### R6 — Polish & Release `(S)`

Goal: cut `v0.4.0`.

- [ ] No public API change; nothing to add to the API summary tables.
- [ ] Tag `v0.4.0` after the first push to GitHub so the tag lands on the commit the remote sees: `git tag -a v0.4.0 -F -` with a subject line `QR rotation handling release` followed by a paragraph derived from the CHANGELOG. Left for the user to run manually.

---

## 5. Proposed File Layout Delta

Rotation handling lands as a minimal patch to the existing image stage; no new package directories, only one new test file and the theory + plan docs.

```
qrgen/
├── decode_image.go              # existing — only orderFinderTriple is touched
├── decode_image_test.go         # existing — gains rotation-invariant ordering unit tests
├── decode_rotation_test.go      # new — synthetic rotation fixtures + rotateImage helper
└── decode_bench_test.go         # existing — optional BenchmarkDecodeImageRotated90
docs/
├── plan-decoder-rotation.md     # this file
├── plan-decoder-rotation.id.md  # Indonesian counterpart
└── theory/
    ├── 15-rotation-handling.md
    └── 15-rotation-handling.id.md
```

## 6. Risks & Technical Notes

- **The 1:1:3:1:1 scanner at oblique angles.** Row scans hit rotated finder modules at angles other than 0 or 90 degrees, so module widths in the scan-line projection differ from the actual module size. The existing ±50% tolerance in `fitsFinderRatio` absorbs tilts up to roughly 30 degrees comfortably; beyond that the ratio starts drifting outside the tolerance band. The v0.4 scope deliberately stops at the tolerance boundary; broader coverage would require a new contour-based or fan-of-orientations finder detector.
- **Bilinear vs nearest-neighbour rotation in fixtures.** Rotating the source image with bilinear sampling produces some intermediate-grey pixels along edges that did not exist in the encoder output. The Otsu fast path still binarises these correctly because the ink and paper modes stay well-separated, but extreme angles produce noisier edges and can challenge the strict 1:1:3:1:1 ratio. Fixtures stay inside the safe band by design.
- **Interaction with the v0.3 Sauvola dispatch.** The rotation change is orthogonal to the binariser dispatch. We expect `binariserOtsu` to fire for clean rotated PNGs and the Sauvola path to fire for rotated-and-shadowed inputs. R3 fixtures assert the Otsu branch on clean rotations; a follow-up after v0.4 can optionally cross-test the combined case.
- **Module-pitch estimation at extreme tilts.** `crossCheckVertical` averages horizontal and vertical module-size estimates. For a 30-degree tilt these two estimates differ by ~15%; the averaging biases the homography slightly. Acceptable inside v0.4's scope.
- **Mirrored symbols.** The cross-product check returns the "wrong" sign for a mirrored QR. Real QR codes are never mirrored, so we treat the mirror case as an explicit detection failure rather than auto-flipping the labels — that keeps the failure mode loud rather than silently decoding a synthetic mirrored input as if it were valid.

## 7. References

- ISO/IEC 18004:2015 — §11.2 (Locator pattern detection) and §11.3 (Image sampling). Spec references for the assumption that the symbol is "approximately right-side-up".
- Hartley & Zisserman — *Multiple View Geometry in Computer Vision*, 2nd ed., §4 (homography estimation). Confirms that a 3x3 projective transform absorbs rotation, scale, translation, and perspective given correct corner correspondences.
- ZXing project — *open-source decoder reference*: <https://github.com/zxing/zxing>, especially the `FinderPatternFinder.orderBestPatterns` method, which uses the same cross-product handedness trick we adopt here.
- Project Nayuki — *QR Code generator library, decoder companion notes*.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Rotation fixture angle list.** The R3 sketch lists 90 / 180 / 270 / 15 / 30 plus a negative 45. Worth adding 60 and 75 to document where the soft-tilt band ends, or leave that to v0.5?
- **Mirror detection error.** Currently a mirrored input fails as `ErrFinderNotFound`. Worth adding a dedicated `ErrMirroredSymbol` sentinel, or stay quiet and treat it as the existing "not found" failure? Default: stay quiet, keep the public API stable.
- **Rotation as a public input.** Should callers be able to hint a rotation angle to skip the cross-product check on devices that already know the orientation? Not in v0.4; revisit if real callers ask.
