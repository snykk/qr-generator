# Logo Embedding

The v0.12 renderers can overlay a centred logo on a QR symbol — the familiar branded-QR look — while keeping it scannable. Nothing about the encoding changes: the logo is drawn onto the finished raster (PNG) or embedded in the document (SVG), and the symbol still decodes because Reed–Solomon error correction recovers the modules the logo covers. This document records why that works, how big a logo is safe, the automatic error-correction bump, the background pad, and the zero-dependency compositing.

> Indonesian version: [23-logo-embedding.id.md](23-logo-embedding.id.md).

## 1. Why a centred logo still scans

A logo occludes some modules; to a decoder those modules are wrong. QR survives that through **Reed–Solomon error correction**, which recovers a fraction of the codewords in each block. The four EC levels recover roughly L 7%, M 15%, Q 25%, and H 30% of the data codewords. A logo that covers about a fifth of the symbol corrupts about a fifth of the modules, which sits inside the H budget with margin — so a modest centred logo at EC level H reads fine.

Two structural facts make it work better than the raw percentage suggests:

- **Interleaving spreads the damage.** A QR symbol interleaves its Reed–Solomon blocks across the whole matrix (doc 04), so a single contiguous patch in the centre does not wipe out one block — it takes a slice out of many blocks, and each block's own correction handles its slice. Error correction is per block, so spreading a fixed amount of damage across all blocks is the best case.
- **The centre is the safe place.** The three finder patterns sit in the corners and the timing patterns run along row and column 6; a centred logo touches none of them. The decoder needs the finders and timing to locate and sample the grid, so the logo must stay clear of them — which a centred, modestly-sized logo does automatically.

## 2. Size and the automatic EC-H bump

The safe size is a trade between looking good and staying inside the recovery budget. The default here covers about **20% of the symbol's width** (roughly 4% of its area by the square, but the logo's bounding box occludes closer to 20% of the modules along the centre band), which is comfortably inside EC-H.

Because the budget only exists at a high EC level, requesting a logo **forces the error-correction level to at least H**. This happens in option resolution, after all options are applied, so it is order-independent and a caller who explicitly asks for H (or is already there) is unaffected. A caller can still make the logo too large — past roughly a quarter of the width the symbol starts to fail — so the size ceiling is documented rather than silently enforced; the default is deliberately conservative.

## 3. The background pad

The logo is drawn on a small **background pad** — a white, slightly rounded rectangle a little larger than the logo. It does two jobs: it gives the logo a clean local quiet zone so it reads as a distinct element rather than merging into the surrounding modules, and it turns any transparent regions of the logo into white rather than letting dark modules show through and confuse the eye (the decoder already treats the whole patch as damage regardless). The pad is part of the occluded area, so it counts toward the recovery budget; the default keeps pad-plus-logo within the safe band.

## 4. PNG compositing versus SVG embedding

The two renderers reach the same result by different means:

- **PNG** draws the logo directly onto the rendered image before encoding. The pad rectangle is filled, then the logo — scaled to the target box (section 5) — is composited over it with `image/draw` using `draw.Over`, so the logo's own alpha blends onto the white pad. The result is a single raster with the logo baked in.
- **SVG** embeds the logo as a `<image>` element with a base64 `data:` URI, positioned and sized over a background `<rect>`, after the module path. Because the image is a data URI rather than an external `href`, the SVG stays a single self-contained file, consistent with the v0.5 renderer's goals (doc 16). SVG scales the `<image>` itself, so no resampling is needed on this path.

## 5. Zero-dependency resampling

The PNG path has to fit an arbitrary logo into a box whose size depends on the symbol's pixel dimensions, which depend on the version, so the library must scale the image. The standard library's `image/draw` composites but cannot resize, and `golang.org/x/image/draw` — which can — is an external module the zero-dependency rule rejects. So the milestone ships a small in-tree resampler:

- **Downscaling** (the common case, a large logo shrunk to fit) uses a **box filter**: each destination pixel averages the source pixels that map into it, which avoids the harsh aliasing nearest-neighbour would produce on a shrunk photo or icon.
- **Upscaling** (a small logo enlarged) uses **nearest-neighbour**, which is adequate and cheap.

A caller who needs Lanczos-grade quality can pre-scale the logo themselves and pass it at its final size; the resampler is a no-op when the source already matches the target box.

## 6. Implementation pointers

- `qrgen/logo.go` hosts the `Logo` config (`Image`, `Scale`, `Background`, padding), `WithLogo`, the resampler, and the PNG compositing (pad plus scaled logo). `renderOptions` gains a `logo` field that `Encode` threads through; `render_png.go` draws the logo after the modules, `render_svg.go` emits the `<image>` data URI.
- `resolveOptions` raises the EC level to H when a logo is present and the level is lower, so the recovery budget is always there.
- Everything is gated on `WithLogo`; a no-logo encode is byte-for-byte identical, guarded by a test and the intact golden fixtures.
- The headline test encodes text with a default logo and confirms `DecodeBytes` still returns the original text — the occluded modules become Reed–Solomon errors that correction recovers — with a gozxing cross-check that an independent decoder agrees.

## References

- ISO/IEC 18004:2015 — the Reed–Solomon error-correction structure, the block interleaving, and the four EC levels whose recovery capacity a logo consumes.
- `docs/theory/04-reed-solomon.md` — the per-block correction and interleaving that make a centred occlusion recoverable.
- `docs/theory/08-rendering.md` and `docs/theory/16-svg-rendering.md` — the PNG and SVG renderers the logo overlay extends.
- Go standard library — `image`, `image/draw` (`draw.Over` compositing), `image/color`, and `encoding/base64` (the SVG data URI); deliberately not `golang.org/x/image`.
