# QR Generator — Micro QR Detection Plan

This document describes the implementation plan for **locating Micro QR Code symbols in images**, targeting the `v0.16.0` minor release. v0.15 shipped the Micro encoder and a matrix-level decoder, so a clean Micro grid already decodes through `DecodeMatrix`; but `Decode`, `DecodeBytes`, and `DecodeAll` still need the three finder patterns of a QR symbol, so they cannot find a Micro symbol in a photograph. This milestone closes that gap: one finder pattern, read together with the timing patterns and the format information, is enough to locate, orient, and size a Micro symbol, after which the existing matrix decoder takes over.

> Status: **draft / living document.** Milestones MD1..MD7 land incrementally on the `micro-qr-detection` branch; each is a focused commit (or small series) with tests, matching the cadence of prior milestones.

> Indonesian version: [docs/plan-micro-qr-detection.id.md](plan-micro-qr-detection.id.md).

---

## 1. Vision & Goals

- `Decode` and `DecodeBytes`, and through them the CLI's `-decode`, read Micro QR symbols from images with no new function or option, the same way `DecodeMatrix` already recognises Micro grids by their size.
- `DecodeAll` reports Micro symbols alongside the QR symbols found in the same image.
- Leave QR decoding exactly as it is. The Micro search runs only after the QR attempts on the same binarised image have failed, so an image that decodes as QR today never reaches the new code.
- Reuse the image pipeline rather than rebuild it: binarisation, the connected-component finder detector, corner extraction, the homography solvers, grid sampling, and the v0.15 matrix decoder all carry over.
- Validate against independent implementations: the zint symbols already stored as text in `qrgen/testdata/thirdparty/micro_symbols.txt`, rendered into images by the tests, and zxing-cpp's reader run on the same synthetic scenes.
- Keep **zero runtime dependencies** and no binary fixtures, and report the rotation, tilt, and scale band that actually holds.

## 2. What one finder pattern gives, and what it does not

The QR pipeline is built around three finders. Both the affine path (`findFinders`) and the projective path (`findFinderQuadTriple`) refuse an image with fewer than three, and the geometry comes from the triangle the three centres form. A Micro symbol has one finder, so neither path can start.

One finder still carries a lot. Its four outer corners, which the connected-component detector already extracts, are four point correspondences: exactly enough for a projective homography from module coordinates to pixels. What they cannot tell is which corner is which. The finder looks the same after a quarter turn, so the four corners fit the 7 by 7 square in four rotations, and the homography is known only up to that choice. The rest of the symbol resolves it:

- **Orientation.** The timing patterns run along row 0 and column 0, starting right after the separator, dark at even indices, so modules (row 0, column 8) and (row 8, column 0) are dark in every Micro version. In each of the three wrong rotations at least one of them lands in the quiet zone. Recognising which two sides of the finder face the quiet zone is recognising the orientation.
- **Format information.** All 15 format modules lie in the 9 by 9 block at the finder's corner (column 8 rows 1..8 and row 8 columns 1..7), so they can be read through the finder homography before the symbol's size is known. Decoded against all 32 Micro format words, they give the symbol number and therefore the version, the error-correction level, and the mask.
- **Version cross-check.** The timing pattern on row 0 alternates up to column n-1 and is followed by the quiet zone, so its length independently confirms the size the format information claims.
- **Geometry.** The finder spans 7 modules; an M4 symbol spans 17. Sampling the whole symbol through the finder homography means extrapolating it about two and a half times beyond the points that determine it, so any error in the corners grows on the way out. Whether that matters at the angles we care about is a measurement, not an assumption (section 5).

## 3. Prior art: zxing-cpp

zxing-cpp, already this project's independent Micro decoder, locates Micro symbols the same way in outline. Its `SampleMQR` (core/src/qrcode/QRDetector.cpp, master as read in November 2026) fits a homography to the corner quad of one finder in each of the four rotations, requires the two innermost timing modules (column 8 of row 0, and row 8 of column 0) to be dark, reads the 15 format bits and decodes them against all 32 Micro words in both bit orders (the second for mirrored symbols), and keeps the rotation with the lowest Hamming distance. The version comes from the symbol number. A quiet-zone check then rejects the candidate if more than 2n/3 of the 2n samples along row n and column n are dark, which is how it avoids reading the corner of a regular QR symbol as a Micro symbol. Finally it re-locates the top-right, bottom-left, and bottom-right corners with a local grid search before sampling. Its reader runs this on every finder pattern not used by a successfully decoded QR symbol, after the QR pass.

Two details of that design matter for ours. First, zxing-cpp does not trust the finder homography for the far corners: it finds them again. Second, its corner quad sits in the middle of the finder's outer ring (the centres of the corner modules), blended from fits to the ring's two edges; ours comes from the extreme pixels of the outer ring.

## 4. Approach, grounded in the code

A read of the image pipeline (decode_image.go, decode_finder_cc.go, decode_multi.go, decode_image_sauvola.go) and the Micro decoder (micro_decode.go, micro_matrix.go) sorts the work into three groups.

- **Reusable as-is:** the Otsu and Sauvola binarisers and their dispatch; `labelComponents`, `finderNestings`, and `extractQuadCorners`; `computeHomography` and `computeHomographyLS`; `sampleMatrix`, which already takes the grid size as a parameter; `decodeMicroMatrix`, which re-reads the format information for the sampled size, applies Reed-Solomon (detection only in M1), and parses the Micro segments; `microFormatInfo`; and the claim loop and `Result` of `DecodeAll`.
- **Micro-specific, new:** an accessor for every finder quad, since `detectFinderQuads` refuses fewer than three; ordering a quad's corners into a consistent winding, since `extractQuadCorners` returns them in arbitrary order; the per-rotation homography from one finder; the orientation gate; an image-side format reader that searches all 32 words; the timing cross-check; far-corner refinement if the measurements call for it; the Micro pass inside the connected-component attempt and inside the `DecodeAll` loop; and a field on `Result` for the Micro version.
- **Hidden assumptions to avoid:** the three-nesting minimum in `detectFinderQuads`; `homographyFromFinderQuads` and `estimateVersionQuad`, which take or produce a QR version; `Result.Version`, which is QR-only and must stay so (v0.15 made `MicroVersion` a separate type on purpose); `decodeImage` returning the affine error unconditionally; `decodeImageWhich` knowing only two paths; `readMicroFormatInfo` needing the version up front, which is exactly what the image side does not know yet; and corner pixels being pixel centres rather than the ring's outer edge, a half-pixel bias that is negligible at 8 pixels per module and large at 2.

## 5. Design decisions

- **Transparent, with no new single-symbol API.** `Decode` and `DecodeBytes` gain Micro support in place; their documentation and the note in `DecodeMatrix` about images are updated.
- **QR first, on every binarisation.** The affine QR pipeline runs first, unchanged, with its own Otsu-then-Sauvola dispatch. The connected-component attempt then labels each binarisation once (Otsu, then Sauvola when needed) and uses its finder quads twice: the best QR triple first, and only if that fails, each quad as a Micro finder, largest first, the first that decodes winning. Labelling is shared, a clean Micro image is found on the first binarisation, and no QR candidate on a bitmap is ever preempted by a Micro candidate on the same bitmap.
- **Orientation.** The four corners are put in the winding of an un-mirrored symbol and tried in their four rotations. A rotation passes the gate only if the innermost timing modules (row 0 column 8, row 8 column 0) are dark: doc 27 shows that in each wrong rotation at least one of them falls in the quiet zone, while the separator modules would pass in some wrong rotations and are left to the timing cross-check. Among the rotations that pass, the one whose format word is nearest a valid word wins. Mirrored symbols are out of scope, as they are for QR.
- **Format and version.** The 15 bits are decoded against all 32 Micro words within the same 3-bit budget as `readMicroFormatInfo`; the symbol number gives the version and the grid size n. Section 7 of doc 26 warned that an all-light strip is within budget of M4-L mask 1, and a random 15-bit word lies within budget of some Micro word more than half the time; that is why the format word alone never accepts a candidate.
- **Timing cross-check rather than a quiet-zone count.** For the claimed size, row 0 and column 0 from index 7 to n must read: light separator, timing alternating from dark at 8 to dark at n-1, then light quiet zone at n, within a small tolerance fixed by measurement in MD3. This rejects the corner of a QR symbol (whose row 0 beyond the finder is data, not timing) and a wrong size, using the structure the standard put there for that purpose. zxing-cpp's 2n/3 quiet-zone count stays in reserve in case the measurement shows a need.
- **Geometry, decided by measurement.** The first version samples through the finder homography at size n. MD5 measures where that fails under rotation, tilt, and small module sizes, and only then adds what is needed: the half-pixel corner correction, and re-locating the top-right and bottom-left corners from the last module of each timing pattern (index n-1 is even, so that module is always dark, and the quiet zone bounds it on two sides), refitting with `computeHomographyLS` over the finder corners plus those two. The Micro mask rule, which favours dark modules on the last row and column, makes the bottom-right corner findable too, if the measurement asks for it.
- **Reed-Solomon as the final arbiter.** A candidate is accepted only when `decodeMicroMatrix` succeeds on the sampled grid. M1 is the weak case: its two detection-only EC codewords let about 1 in 65,536 random words through, which is why the gates before it matter.
- **Error contract.** On total failure `Decode` keeps returning the affine attempt's error, so every existing error is unchanged, with one exception: when the affine attempt failed for lack of a finder triple (`ErrFinderNotFound`) and the Micro pass found a finder that convincingly belongs to a Micro symbol (a timing pattern that matches a Micro size exactly while the format information is unreadable, or every gate passed with only the data failing), the Micro error is returned instead (`ErrFormatUnreadable`, `ErrTooManyErrors`, or `ErrCorruptedPayload`), because a damaged Micro symbol is not a missing finder. Passing the orientation gate is not enough: MD4 measured that about a third of QR finders pass it.
- **`DecodeAll`.** After the QR claim loop, every unclaimed finder quad is tried as a Micro finder and claimed on success. `Result` gains `MicroVersion MicroVersion`, zero for QR symbols; for a Micro symbol `Version` is zero, `Finders[0]` is the finder centre and the other two entries are zero, and `Corners` comes from the homography at size n, as for QR. The change is additive and leaves QR results unchanged (open question 1).
- **Out of scope:** mirrored symbols, a line-scan (affine) path for Micro, a pure-image fast path, decoding a Micro symbol whose format information is unreadable, Micro in `DecodeStructuredAppend` (Micro has no structured append), and rMQR.

## 6. Milestones

Milestones land sequentially. **Checkpoint A** (after MD5) reads Micro symbols from images through `Decode` and `DecodeBytes`, validated against zint's symbols and zxing-cpp's reader. **Checkpoint B** (MD7) is the `v0.16.0` release.

### MD1 — Plan Doc `(S)`

- [x] This document and its Indonesian counterpart.

### MD2 — Reference Doc `(S)`

- [x] `docs/theory/27-micro-qr-detection.md` (EN + ID): why the QR pipeline cannot start on one finder; what a finder's four corners determine and the quarter-turn ambiguity they leave; orientation from the separator and timing modules; reading the format information before the size is known, and why a format word alone is weak evidence; the version cross-check; extrapolating from a 7-module finder and how the far corners can be found again; false positives and the gates that stop them; where the search plugs into the pipeline; prior art. Plus the theory README entry and code-mapping row.

### MD3 — One-Finder Geometry `(L)`

- [x] `allFinderQuads`: every finder quad, with no minimum; `detectFinderQuads` keeps its contract on top of it.
- [x] Winding normalisation and the per-rotation homography from one finder quad.
- [x] The orientation gate; image-side format reading over all 32 words; the version and the timing cross-check, with its tolerance measured.
- [x] `decodeMicroQuad(bm, quad)`: the above, then `sampleMatrix` at size n and `decodeMicroMatrix`, returning the text, the version, and the homography.
- [x] Tests on painted bitmaps at the four right-angle rotations, and on every zint symbol rendered upright.

### MD4 — Pipeline Integration `(M)`

- [x] The Micro pass in the connected-component attempt, after the QR triple, per binarisation; `decodeImageWhich` reports `"micro"`.
- [x] The error contract of section 5, and the `Decode` and `DecodeMatrix` doc comments.
- [x] The half-pixel corner correction, pulled forward from MD5: rendered at 3 pixels per module, every zint M4 symbol failed the timing cross-check without it.
- [x] Tests: `Decode` and `DecodeBytes` read every zint symbol and our own PNGs for all eight types; the CLI's `-decode` reads a Micro PNG; a lone finder pattern, a blank image, and damaged QR symbols produce no Micro text; existing QR fixtures still decode through the same path as before.

### MD5 — Robustness Measurement `(M)`

- [x] Informational sweeps over in-plane rotation, perspective tilt (`warpTilt`), module size, and quiet-zone width, each logging which stage fails.
- [x] The far-corner refinement plus two rounds of an edge fit of all four outer edges, which the sweeps showed are needed below 5 pixels per module (192 of 612 small rotated scenes decode with the finder homography alone, 502 with the full chain); the band that holds is asserted (the half-pixel correction landed in MD4).
- [x] One-off zxing-cpp check on the same 1,836 scenes, reported in the commit and in theory doc 27, section 10: the only scenes it reads and we do not are 5 at 2 pixels per module and 14 at 65 degrees of tilt.

### Checkpoint A — `Decode` reads Micro symbols from images, checked against zint symbols and zxing-cpp's reader.

### MD6 — Micro in DecodeAll `(M)`

- [ ] The Micro pass over unclaimed finder quads in `decodeMultipleFromBitmap`, and `Result.MicroVersion`.
- [ ] Tests on composited scenes: several Micro symbols of different versions, Micro and QR together, rotated tiles, and reading order; a one-off zxing-cpp check on one composite.

### MD7 — Polish & Release `(S)`

- [ ] README (decoding section, scope, limitations, roadmap) and `examples/decode/micro`.
- [ ] `CHANGELOG.md` `v0.16.0` entry, left unstaged for the release branch.
- [ ] The theory code-map row for doc 27, without a planned mark.
- [ ] `go test -race ./...` clean, gofmt-clean.

## 7. Validation strategy

1. **zint symbols as images.** The 34 zint symbols in `micro_symbols.txt` are rendered by the tests at several module sizes, quiet zones, and rotations, and `Decode` must return each one's text. They come from an encoder that shares no code with ours, so a mistake shared by our encoder and our detector cannot hide behind a round trip.
2. **Our own PNGs.** Every symbol type from `Encode(..., WithMicro())` decodes through `DecodeBytes`; necessary, never sufficient on its own.
3. **An independent reader.** zxing-cpp reads the same synthetic scenes, written to files in a scratch directory; a one-off check reported in the MD5 and MD6 commits, since Go tests cannot call Python.
4. **Negative cases.** A blank image, a lone finder pattern with no timing, the corners of QR symbols (whole, and with a finder wiped so that the QR attempt fails), and noise must never yield Micro text.
5. **No QR regression.** Every existing test passes unchanged, and QR fixtures decode through the same path (`affine` or `projective`) as before.

## 8. Proposed File Layout Delta

```
qrgen/
├── decode_micro.go         # new — winding, per-rotation homography, orientation gate, format and timing reads, decodeMicroQuad
├── decode_micro_test.go    # new — painted bitmaps, zint symbols as images, sweeps, negative cases
├── decode_finder_cc.go     # allFinderQuads; detectFinderQuads on top of it
├── decode_image.go         # Micro pass in the connected-component attempt, error contract, decodeImageWhich
├── decode_multi.go         # Micro pass over unclaimed quads, Result.MicroVersion
├── decode_multi_test.go    # Micro scenes for DecodeAll
└── decode.go               # Decode and DecodeMatrix doc comments
docs/
├── plan-micro-qr-detection.md / .id.md
└── theory/27-micro-qr-detection.md / .id.md
examples/decode/micro/main.go
```

## 9. Risks & Technical Notes

- **Extrapolation.** The finder homography is determined by a 7-module square and must reach 17 modules in M4, so corner noise is amplified about two and a half times at the far corner, more under perspective. Measured in MD5 before any refinement is added.
- **False positives.** The corner of a QR symbol contains a finder pattern of exactly the right shape. Running Micro only after QR fails on the same bitmap, the orientation gate, the timing cross-check, and Reed-Solomon stand between it and a bogus decode. M1, with two detection-only EC codewords and a 3-module timing pattern, is the weakest case and gets dedicated negative tests.
- **Small modules.** Corner pixels are pixel centres, half a pixel inside the true outer corner; at 2 pixels per module that is a quarter of a module before extrapolation. The module-size sweep decides whether to correct it.
- **The narrow quiet zone.** Micro needs only 2 modules of quiet zone, and the timing cross-check reads the first module past the symbol, so clutter 2 modules away is fine but clutter touching the symbol is not. Documented with the band.
- **Error contract.** The one change in returned errors affects only images where the affine attempt found no finder triple but a finder convincingly belongs to a Micro symbol; existing tests pin the rest, and QR symbols with a finder wiped out keep their errors (MD4).
- **Cost.** At most `maxCCCandidates` (64) quads, four rotations each, a few dozen module reads per rotation, and a full sample and decode only for the candidates that pass the gates: negligible next to labelling, which is shared with the QR attempt.

## 10. References

- ISO/IEC 18004:2015 — the Micro QR layout, timing patterns, format information, quiet zone, and the masking rule that keeps the far edges rich in dark modules.
- zxing-cpp, core/src/qrcode/QRDetector.cpp (`SampleMQR`, `DetectPureMQR`), QRReader.cpp, and QRFormatInformation.cpp (`DecodeMQR`), master as read in November 2026 — the prior art summarised in section 3; its reader is the independent check for validation.
- zint, through zxing-cpp 2.3.0 — the encoder behind the in-tree Micro symbols.
- `docs/theory/24-arbitrary-angle-detection.md`, `25-multi-symbol-decoding.md`, `26-micro-qr.md` — the connected-component detector and projective geometry this milestone reuses, the claim loop of `DecodeAll`, and the Micro symbol itself.

## 11. Open Questions

- **Micro in `Result`.** The proposal adds `MicroVersion MicroVersion` and leaves `Finders[1]` and `Finders[2]` zero for a Micro symbol. The alternative, a separate result type for Micro, would split every caller's loop in two. To confirm before MD6.
- **Timing tolerance.** Resolved in MD3: two. Clean symbols read with no disagreement; over every finder of 224 QR symbols (versions 1 to 7, every EC level and mask), the assignments that reached the cross-check disagreed in 1 to 14 modules, 8 at the median, and the three that fell within the tolerance were rejected by Reed-Solomon.
- **Far-corner refinement.** Resolved in MD5: needed, and not enough on its own. The finder homography alone failed at 3 to 5 pixels per module under rotation; the timing-end corners fixed most of that, and an edge fit of all four outer edges, which anchors the bottom-right corner through the dark modules the masking rule keeps on the last row and column, fixed the rest down to 3 pixels per module.
- **Error contract.** Resolved in MD4: yes, but only for convincing Micro evidence. Passing the orientation gate, which about a third of QR finders do, proved too weak; an exact timing pattern with unreadable format information, or a located symbol whose data fails, reaches fewer than one QR finder in a hundred, and none of 96 QR symbols with a finder wiped out changed its error.
