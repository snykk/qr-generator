# QR Generator — rMQR Detection Plan

This document describes the implementation plan for **locating rMQR Code symbols in images**, targeting the `v0.19.0` minor release. v0.17 shipped the rMQR encoder and a matrix-level decoder, so a clean rMQR grid already decodes through `DecodeMatrix`; but `Decode`, `DecodeBytes`, and `DecodeAll` cannot find an rMQR symbol in a photograph, and the web demo's Scan view cannot read one. This milestone closes that gap, the way v0.16 did for Micro QR: the single finder pattern gives the orientation and the format information, the format information gives the size, and the finder sub-pattern at the far corner anchors the other end of a symbol up to 139 modules long.

> Status: **draft / living document.** Milestones RD1..RD7 land incrementally on the `rmqr-detection` branch; each is a focused commit (or small series) with tests, matching the cadence of prior milestones.

> Indonesian version: [docs/plan-rmqr-detection.id.md](plan-rmqr-detection.id.md).

---

## 1. Vision & Goals

- `Decode`, `DecodeBytes`, the CLI's `-decode`, and `DecodeAll` read rMQR symbols in images, at every one of the 32 versions, upright, rotated, and under moderate perspective.
- QR and Micro QR decoding stay exactly as they are: rMQR is searched only after both have had their turn on the same finders, and every existing test passes unchanged.
- `DecodeAll` reports rMQR symbols next to QR and Micro QR ones, with their version.
- Validate against **independent** material, never only against ourselves: the 70 zint symbols of v0.17 rendered into images, and zxing-cpp's reader on the same scenes.
- Measure the working band (scale, rotation, tilt) rather than assume it, and say plainly where it ends.

## 2. What makes rMQR detection different

The rMQR layout is described in theory doc 28; these are the facts detection leans on.

- **One finder, as in Micro QR.** A 7 by 7 finder at the top-left, with a light separator in column 7 and, from height 9, in row 7. The connected-component detector of v0.13 already finds it as a nested ring, and v0.16's Micro search already turns a lone finder into a homography up to a quarter turn.
- **A long symbol.** Up to 139 modules wide and 7 to 17 tall. A homography fixed by one finder, 7 modules across, is extrapolated up to 20 times its own size, so a small error in the finder's corners becomes several modules at the far end. Micro QR, at most 17 modules, needed a refinement for the same reason (doc 27, section 6); rMQR cannot work without a second anchor.
- **A second anchor: the finder sub-pattern.** A 5 by 5 pattern in the bottom-right corner, a dark ring around a light ring around a single dark module, shaped exactly like a QR alignment pattern. Its dark ring has no separator and touches format and data modules, but its light ring is always a closed component around a one-module dark centre.
- **Orientation and size from the format information.** Row 0 next to the finder holds the top timing pattern, dark at even columns, which picks the quarter turn. The 18-bit format copy beside the finder, at rows 1 to 5 of columns 8 to 11, names the version and so both dimensions; a second copy beside the sub-pattern names them again.
- **Long, straight edges.** The top and bottom rows are timing patterns broken only by alignment and corner patterns, so both long edges are full of modules known to be dark.

## 3. Prior art

zxing-cpp 2.3.0 reads rMQR symbols in images with `SampleRMQR` (core/src/qrcode/QRDetector.cpp), read for this plan. For each finder not used by a QR symbol it tries the four quarter turns, keeps those whose top-edge modules, row 0 in columns 8 to 11, read dark, light, dark, light, reads the finder-side format copy, and keeps the turn with the nearest valid word. It then looks for the sub-pattern with its alignment-pattern locator around the point the finder homography predicts, builds a four-point transform from the finder and the sub-pattern, and samples the grid. It reads only the finder-side format copy and does no refinement along the symbol; the code is marked "TODO: this is a WIP". It is the reader we compare against, and its orientation test and format read are the same first steps this plan takes.

## 4. Approach, grounded in the code

- **Reusable as-is:** the binarisation dispatch and `labelComponents`; `allFinderQuads` and the finder corners; from v0.16, `outerRingCorners` (the half-pixel correction), `windCorners`, the finder homography per quarter turn, `moduleDark`, the edge-point march and line fit, and `computeHomographyLS`; from v0.17, the rMQR format positions, the 64 format words, and `decodeRMQRMatrix`; and from v0.14, the claim loop of `DecodeAll`.
- **New:** the rMQR orientation and format gates; sub-pattern candidates from the light components that `labelComponents` already produces; pairing a finder with a sub-pattern by predicted position and module size; the eight-point homography; an edge fit along the long edges; sampling a rectangular grid; the second format copy; the stage-based error contract; and `Result.RMQRVersion`.
- **Hidden assumptions to avoid:** `sampleMatrix` samples squares only; `checkAlignmentAt` samples along the image axes and fails on a pattern turned 45 degrees; Micro's timing gates expect timing down column 0, which an R7 symbol does not have; a QR alignment pattern looks exactly like a sub-pattern, so candidates are plentiful; Micro symbols pass the rMQR orientation test, since their row 0 is timing too; and a format word within 3 bits of one of 64 valid words is weak evidence (about one random word in four qualifies).

## 5. Design decisions

- **Order.** On each binarisation, the projective QR triple first, then the Micro search, then the rMQR search, all on the same finder quads; in `DecodeAll`, QR triples, then Micro, then rMQR on the finders still unclaimed. rMQR never preempts a symbol the earlier searches decode.
- **Gates.** A quarter turn survives if its top-edge modules, row 0 in columns 7 to 11, read light, dark, light, dark, light: the separator, then the start of the timing pattern. A survivor's finder-side format copy, within 3 bits of one of the 64 words, names the version; a sub-pattern candidate must then sit where that version puts it, within a tolerance that grows with distance, with a module size close to the finder's.
- **Geometry.** A least-squares homography over the finder's four outer corners and the four outer corners of the sub-pattern's light ring, then an edge fit over the top and bottom edges (and the short left and right ones); the variants are tried best first and Reed-Solomon decides, as in v0.16.
- **Both format copies.** Once the symbol is located, the sub-pattern-side copy must agree with the version; when the finder-side copy is unreadable, each version whose sub-pattern position holds a candidate is tried with the sub-pattern-side copy, so either copy alone is enough, as it is for `DecodeMatrix`.
- **Errors.** As for Micro QR: the search records how far each finder got (oriented, format read, located), and only a located rMQR symbol, a finder and a matching sub-pattern, may replace "finder patterns not found" with its own error.
- **API.** `Result` gains `RMQRVersion`, zero for QR and Micro QR; an rMQR result has `Version` and `MicroVersion` zero, its outline in `Corners`, and its finder in `Finders[0]`.
- **The web demo** reports rMQR symbols in the Scan view with no change beyond labelling them.

## 6. Milestones

Milestones land sequentially. **Checkpoint A** (after RD4) reads every zint symbol in clean, rotated, and tilted images. **Checkpoint B** (RD7) is the `v0.19.0` release.

### RD1 — Plan Doc `(S)`

- [x] This document and its Indonesian counterpart.

### RD2 — Reference Doc `(S)`

- [x] `docs/theory/30-rmqr-detection.md` (EN + ID): what one finder leaves open in a long symbol; orientation from the top timing; the finder-side format copy and how weak a match is; the sub-pattern's signature and why its light ring, not its dark ring, is the one to find; pairing by predicted position; the eight-point homography and the edge fit; the second format copy; where the search joins the pipeline; and zxing-cpp as prior art. Plus the theory README entry and code-mapping row.

### RD3 — Locate `(L)`

- [x] Orientation and format gates, sub-pattern candidates, pairing, the eight-point homography, rectangular sampling, and a decoder for one finder quad.
- [x] Tests: every zint symbol, rendered upright at several scales and at every quarter turn, decodes through the new search.

### RD4 — Refine and Confirm `(M)`

- [x] The edge fit and the variants; the sub-pattern-side format copy as confirmation and as fallback; the stages.
- [x] Tests: rotation and tilt bands, a damaged finder-side format copy, and what each refinement adds.

### Checkpoint A — every zint rMQR symbol reads from rotated and tilted images.

### RD5 — Pipeline `(M)`

- [ ] The rMQR pass in `Decode` and `DecodeBytes` after the Micro pass, with the error contract; the rMQR pass in `DecodeAll` and `Result.RMQRVersion`; `examples/decode/rmqr`.
- [ ] Tests: composites of QR, Micro QR, and rMQR symbols; no QR or Micro finder decodes as rMQR, and no rMQR symbol as Micro QR.

### RD6 — Measurement and the Web Demo `(M)`

- [ ] Sweeps of scale, rotation, tilt, and quiet zone over the zint symbols, and zxing-cpp on the same scenes, reported in the commit.
- [ ] The web demo's Scan view labels rMQR symbols; a browser run with an rMQR symbol through a file and the camera.

### RD7 — Polish & Release `(S)`

- [ ] README (decoding section, API table, scope, limitations, roadmap) and the theory code map.
- [ ] GitHub Actions bumped to their Node 24 releases (checkout, setup-go, configure-pages, upload-pages-artifact, deploy-pages), with runs free of the Node 20 warning.
- [ ] `CHANGELOG.md` `v0.19.0` entry, left unstaged for the release branch.
- [ ] `go test -race ./...` clean, gofmt-clean.

## 7. Validation strategy

1. **zint symbols in images.** All 70 rMQR fixtures of v0.17, rendered at several scales, quarter turns, in-plane rotations, perspective tilts, and quiet zones, decode to their text.
2. **An independent reader.** zxing-cpp reads the same scenes in a scratch environment; we compare who reads what, and never accept a different text.
3. **No false positives.** No finder of a QR or Micro QR symbol decodes as rMQR, no rMQR symbol decodes as Micro QR, and composites report each symbol once, as its own kind.
4. **Refinements measured.** Each geometric step must read at least as many scenes as the one before it.
5. **Damage.** A symbol with its finder-side format copy destroyed still reads through the other copy.

## 8. Proposed File Layout Delta

```
qrgen/
├── decode_rmqr.go            # new — orientation, format, sub-pattern, homography, refinement
├── decode_rmqr_test.go       # new — goldens in images, bands, damage, false positives
├── decode_image.go           # the rMQR pass after the Micro pass; rectangular sampling
├── decode_multi.go           # the rMQR pass in DecodeAll; Result.RMQRVersion
├── decode_finder_cc.go       # sub-pattern candidates from the light components
web/internal/bridge/bridge.go # rMQR results in the Scan view
web/site/                     # labels
examples/decode/rmqr/main.go  # new
.github/workflows/*.yml       # Node 24 action releases
docs/
├── plan-rmqr-detection.md / .id.md
└── theory/30-rmqr-detection.md / .id.md
```

## 9. Risks & Technical Notes

- **Extrapolation over a long symbol.** The finder alone cannot place the far end; the pairing tolerance must grow with distance, and the measured error decides how much.
- **Many look-alikes.** QR alignment patterns share the sub-pattern's shape, so a candidate is accepted only where the version says it must be, with a matching module size, and Reed-Solomon has the last word.
- **Micro and rMQR share a first gate.** Micro symbols pass the rMQR orientation test and rMQR symbols of height 9 can pass Micro's, so the order of the searches and Reed-Solomon keep them apart; tests check both directions.
- **Small sub-patterns.** The sub-pattern's centre is one module, so at very small scales its light ring may not resolve; the band below which rMQR stops reading is measured and documented, not hidden.
- **QR and Micro must not change.** The rMQR pass runs last, so their decode results cannot change; the existing suites guard it.

## 10. References

- ISO/IEC 23941:2022 — the rMQR layout, as cited through zint's and zxing-cpp's source (theory doc 28).
- zxing-cpp 2.3.0, core/src/qrcode/QRDetector.cpp (`SampleRMQR`, `DetectPureRMQR`) and QRReader.cpp — prior art, and the independent reader for validation.
- `docs/theory/24-arbitrary-angle-detection.md`, `25-multi-symbol-decoding.md`, `27-micro-qr-detection.md`, `28-rmqr.md` — the detector, the claim loop, the single-finder search, and the symbol this milestone builds on.

## 11. Open Questions

- **The pairing tolerance.** How far the finder homography's prediction of the sub-pattern drifts, by length, scale, and rotation, is measured in RD3 and sets the tolerance.
- **Small symbols.** Whether an R7 or R9 symbol, whose sub-pattern sits close to the finder, needs the edge fit at all, or only the eight-point homography.
