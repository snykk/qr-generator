# QR Encoder — Logo Embedding Plan

This document describes the implementation plan for **logo embedding** targeting the `v0.12.0` minor release. It lets a caller overlay a centred logo on a QR symbol — the common branded-QR look — while keeping the symbol scannable by leaning on the error-correction budget. It is the first milestone after the encoder's mode set was completed (Kanji in v0.11), turning to output character rather than encoding.

> Status: **draft / living document.** Milestones L1..L5 land incrementally on the `logo-embedding` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for the M, D, T, R, S, MM, P, TR, ECI, SA, and K milestones.

> Indonesian version: [docs/plan-logo-embedding.id.md](plan-logo-embedding.id.md).

---

## 1. Vision & Goals

- Let a caller pass a logo image with `WithLogo` and have it composited into the centre of the rendered symbol, on both the PNG and SVG outputs, with no other change to how they call `Encode`/`EncodeSVG`.
- Keep the symbol **scannable** by relying on Reed–Solomon recovery: a logo occludes some modules, which become errors the decoder corrects, so the milestone's core is choosing a safe default logo size and a high error-correction level rather than any new encoding.
- Make it **automatic where it matters**: requesting a logo bumps the error-correction level to H (the ~30% recovery level) unless the caller already asked for at least that, so the occluded area stays within budget without the caller reasoning about it.
- Keep **zero runtime dependencies**: scaling the logo to fit uses a small in-tree resampler (the standard library has no image scaler), and the SVG path embeds the logo as a data URI so the document stays self-contained.
- Same philosophy as every prior milestone: pure Go, spec/reference-first with a bilingual doc, and tests — here a decode-still-works round-trip (through our decoder and gozxing) that proves the logo does not break scanning.

## 2. Design Principles

1. **A logo is a render-time overlay, not an encoding change.** The matrix is built exactly as today (at a high EC level); the logo is drawn onto the finished raster or embedded in the SVG. Encoding, masking, and the decoder are untouched.
2. **One option, both formats.** `WithLogo(Logo)` carries the image and its presentation (scale, background, padding). `Encode` composites it onto the PNG and `EncodeSVG` embeds it in the SVG; `EncodeTerminal` ignores it (text output has nowhere to put an image), the same way `WithModuleSize` is a no-op there.
3. **Safe defaults, tunable.** The default logo covers ~20% of the symbol's width and sits on a white rounded background pad (a local quiet zone so it reads cleanly and does not merge into the modules). Both are adjustable through the `Logo` struct.
4. **Automatic EC-H.** `resolveOptions` raises the EC level to H whenever a logo is present and the caller set a lower level, so the recovery budget covers the occlusion. This is order-independent (applied after all options).
5. **Zero-dependency scaling.** A logo is resampled to its target box with an in-tree box-filter downscale (and nearest-neighbour upscale), since `image/draw` cannot scale and `golang.org/x/image` would be a new dependency. Alpha is composited over the background pad with `image/draw`.
6. **Tests prove it still scans.** The headline test encodes text with a default logo, decodes it, and asserts the original text returns — the occluded modules become Reed–Solomon errors that correction recovers — plus a gozxing cross-check.

## 3. Scope

### In scope for v0.12.0

- `Logo` config struct (`Image image.Image`, `Scale float64`, `Background color.Color`, `Padding` / corner-radius) and `WithLogo(Logo) Option`.
- PNG: composite the scaled logo (over a background pad) onto the rendered image before encoding, via a `logo` field threaded through `renderOptions`.
- SVG: embed the logo as a centred `<image>` with a base64 data URI, over a background `<rect>`.
- Automatic EC-H bump when a logo is present.
- In-tree logo resampling (box-filter downscale, nearest-neighbour upscale).
- Reference doc `docs/theory/23-logo-embedding.md` (EN + ID), a runnable example, and a README usage section.

### Out of scope (still)

- **Guaranteeing scannability for oversized logos.** The library picks safe defaults and EC-H, but a caller who sets a very large `Scale` can exceed the recovery budget; the doc states the guidance and the milestone does not attempt an exact per-symbol occlusion solver.
- **Artistic QR** (per-module recolouring, dot styles, gradient fills, image-in-the-modules). A centred logo overlay only.
- **Terminal logos.** `EncodeTerminal` ignores `WithLogo`.
- **High-quality photographic resampling.** The in-tree resampler is good enough for logos; callers wanting Lanczos-quality output can pre-scale the image themselves.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after L3) gives a scannable logo on the PNG output. **Checkpoint B** (L5) is the `v0.12.0` release.

### L1 — Plan Doc `(S)`

- [x] `docs/plan-logo-embedding.md` and its Indonesian counterpart covering vision, principles, scope, milestones L1..L5, file-layout delta, risks, references, and open questions.

### L2 — Logo Embedding Reference Doc `(S)`

Goal: document the recovery-budget reasoning and the compositing approach before code.

- [x] `docs/theory/23-logo-embedding.md` — why a centred logo scans (Reed–Solomon recovery, the ~30% budget at EC-H, why the centre avoids the finder and timing patterns); the safe default size and why; the automatic EC-H bump; the background pad; PNG compositing versus SVG data-URI embedding; and the zero-dependency resampling choice. Closes with implementation pointers.
- [x] Indonesian counterpart `docs/theory/23-logo-embedding.id.md`.
- [x] Updated `docs/theory/README.md` and `.id.md`: entry 23 plus a code-mapping row pointing at `qrgen/logo.go`.

### L3 — WithLogo + PNG Compositing `(M)`

Goal: a scannable logo on the PNG output.

- [x] `qrgen/logo.go` with the `Logo` struct, `WithLogo`, the resampler, and the PNG compositing (background pad + scaled logo over the image). `renderOptions` gains a `logo` field; `Encode` passes it through.
- [x] `resolveOptions` raises EC to H when a logo is present and the level is lower.
- [x] Tests: the decode-still-works round-trip through our decoder at the default size; a too-large logo still decodes at a documented smaller default or is flagged; the resampler downscales and upscales correctly; a no-logo encode is byte-for-byte unchanged.

### Checkpoint A — a default-size logo composites onto the PNG and the symbol still decodes.

### L4 — SVG Embedding + Round-Trip + Example `(S)`

Goal: the SVG path and independent validation.

- [x] `EncodeSVG` embeds the logo as a centred `<image>` (base64 data URI) over a background rect; the SVG stays self-contained and well-formed.
- [x] `TestRoundTripWithThirdPartyDecoder` gains a with-logo case (PNG); gozxing reads it, confirming the logo does not defeat an independent decoder.
- [x] Runnable example `examples/encode/logo/main.go` writing a logo QR to PNG and SVG.

### L5 — Polish & Release `(S)`

Goal: cut `v0.12.0`.

- [x] README: a logo usage section; a `WithLogo` row and the `Logo` type in the API summary; the Roadmap "Logo embedding" item marked done; Scope updated.
- [x] `CHANGELOG.md` `v0.12.0` entry plus compare/tag anchors written; left unstaged in the working tree for the maintainer (mirroring v0.6 through v0.11).
- [x] `go test -race ./...` clean, gofmt-clean.
- [ ] Tag `v0.12.0` (left for the maintainer per the established git/release workflow; annotation recommended in the release conversation).

---

## 5. Proposed File Layout Delta

```
qrgen/
├── logo.go                # new — Logo struct, WithLogo, resampler, compositing
├── logo_test.go           # new — decode-still-works round-trip, resampler tests
├── options.go             # +logo field; auto EC-H in resolveOptions
├── render_png.go          # draw the logo onto the image before encoding
├── render_svg.go          # embed the logo <image> data URI
├── api.go                 # thread the logo through to the renderers
├── roundtrip_test.go      # +with-logo gozxing case
docs/
├── plan-logo-embedding.md      # this file
├── plan-logo-embedding.id.md   # Indonesian counterpart
└── theory/
    ├── 23-logo-embedding.md     # new
    └── 23-logo-embedding.id.md  # new
examples/encode/logo/
└── main.go                # new — logo QR to PNG and SVG
```

## 6. Risks & Technical Notes

- **Recovery budget is the whole game.** A logo scans only because error correction recovers the occluded modules. EC-H recovers about 30% of codewords; a ~20% logo stays comfortably inside that, but a larger logo, a busy background, or a logo overlapping a finder pattern (it must not) breaks it. The default centres the logo, keeps it small, and forces EC-H; the doc states the ceiling.
- **No stdlib scaler.** `image/draw` cannot resize, so the milestone ships a small resampler. A box-filter downscale avoids the aliasing nearest-neighbour would cause on the common case (a large logo shrunk to fit); upscaling a small logo uses nearest-neighbour. Adding `golang.org/x/image` would break the zero-dependency rule and is rejected.
- **Default output must not move.** Everything is gated on `WithLogo`; a no-logo encode is byte-for-byte identical, guarded by a test and the intact golden fixtures.
- **Alpha and the background pad.** A logo with transparency is drawn `draw.Over` a white pad, so transparent regions read as the local quiet zone rather than showing modules through. The pad also gives the logo visual separation from the code.
- **SVG self-containment.** The logo is embedded as a base64 data URI, not an external `href`, so the SVG remains a single portable file, consistent with the v0.5 renderer's goals.

## 7. References

- ISO/IEC 18004:2015 — the Reed–Solomon error-correction structure and the four EC levels (L/M/Q/H) whose recovery capacity a centred logo consumes.
- `docs/theory/04-reed-solomon.md` and `docs/theory/08-rendering.md` / `16-svg-rendering.md` — the error-correction and rendering notes logo embedding builds on.
- Common practice for branded QR codes — a centred logo at 15–25% with a high EC level and a background pad, the de-facto approach this milestone follows.
- Go standard library — `image`, `image/draw` (compositing), `image/color`, and `encoding/base64` (SVG data URI); deliberately not `golang.org/x/image`.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Option shape.** `WithLogo(Logo)` with a config struct (proposed, room for scale/background/padding) versus a bare `WithLogo(image.Image)` plus separate tuning options. The struct keeps the call site tidy for the several knobs.
- **Default scale.** 0.2 (20% of the symbol width) is proposed as a safe default under EC-H; confirm against a decode test, and decide whether to clamp or warn above a threshold.
- **EC bump policy.** Force at least H whenever a logo is present (proposed) versus only defaulting to H and letting an explicit lower level stand. Forcing at least H is the safe choice; confirm in L3.
- **Background pad shape.** A white rounded-rectangle pad (proposed) versus a plain square versus none. Confirm the default and whether the radius is exposed.
- **Resampler quality.** Box-filter downscale plus nearest-neighbour upscale (proposed) versus nearest-neighbour only. Confirm the box filter is worth the extra code for logo quality.
