# SVG Rendering

The v0.5 renderer adds a **scalable vector** output alongside the original PNG raster. Where `renderPNG` writes a fixed grid of pixels, `renderSVG` writes a text document that describes the symbol geometrically, so it stays crisp at any zoom and is tiny on disk for typical payloads. This document records the SVG document model used for a QR symbol, the path-data drawing approach, the coordinate system, colour handling, and why the renderer is a sibling function rather than something hidden behind an interface.

> Indonesian version: [16-svg-rendering.id.md](16-svg-rendering.id.md).

## 1. Why SVG

- **Lossless scaling.** A QR symbol is pure geometry — squares on a grid. A raster fixes that geometry at one resolution; an SVG describes it once and lets the viewer scale it to any size with no blur and no resampling artefacts. Print pipelines and high-DPI displays get a perfect symbol at any dimension.
- **Small files.** For most payloads the vector description is smaller than the equivalent PNG, and it gzip-compresses well because the path data is highly repetitive.
- **Embeddability.** SVG drops straight into HTML, can be styled or themed by the host document, and is understood by every design tool.
- **Crisp edges.** With one instruction the renderer can tell viewers not to anti-alias module boundaries, which keeps the symbol decodable (see section 5).

## 2. The SVG document model for a QR symbol

A rendered QR symbol is three things stacked: a canvas, a light background, and the dark modules. In SVG that maps to:

```text
<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg"
     width="W" height="H"
     viewBox="0 0 D D"
     shape-rendering="crispEdges">
  <rect width="D" height="D" fill="#FFFFFF"/>     <!-- background incl. quiet zone -->
  <path d="M3 3h1v1h-1z M5 3h1v1h-1z ..." fill="#000000"/>  <!-- every dark module -->
</svg>
```

The root `<svg>` carries the SVG namespace, the nominal pixel size in `width`/`height`, the logical coordinate box in `viewBox`, and the crisp-edges hint. A single `<rect>` paints the whole canvas — including the quiet zone — in the background colour. A single `<path>` paints every dark module in the foreground colour. That is the entire document; there is no per-module element.

## 3. Path data: one path, not many rects

The naive way to draw the dark modules is one `<rect>` per module. A V1 symbol has up to a few hundred dark modules; a V40 has tens of thousands. Hundreds or thousands of `<rect>` elements bloat the file and slow down parsers.

Instead we emit a single `<path>` whose `d` attribute is a sequence of small closed subpaths, one square per dark module:

```text
M x y h 1 v 1 h -1 z
```

read as: **m**ove to the module's top-left corner `(x, y)`, draw a **h**orizontal line one unit right, a **v**ertical line one unit down, a **h**orizontal line one unit left, then **z** close the square back up. Concatenating one such subpath per dark module yields a single `<path>` element that fills all of them in one `fill` operation. This is the approach Project Nayuki's `toSvgString` uses and is the de-facto standard for SVG QR output.

A further optimisation merges horizontal runs of adjacent dark modules into a single wider rectangle (`h 3` instead of three separate `h 1` squares), shrinking the path further. v0.5 ships the simple per-module form for clarity and correctness; run-length merging is a future size-focused tweak.

## 4. Coordinate system: module units that scale

The `viewBox` is expressed in **module units**, not pixels: `viewBox="0 0 D D"` where `D = n + 2q` (the matrix side `n` plus a quiet zone of `q` modules on each side). Inside that box, dark module `(row, col)` is the unit square at `(col + q, row + q)`. Working in module units keeps the path data small and integer-valued, and — because `viewBox` defines a logical coordinate system independent of the rendered size — the symbol scales to any physical dimension cleanly.

The nominal rendered size is then set by `width` and `height`, both defaulting to `moduleSize * D` pixels. This makes an SVG and a PNG produced with the same options describe the same nominal dimensions: a `WithModuleSize(8)` V1 symbol is `8 * (21 + 8) = 232` px wide either way. A viewer is free to override the rendered size — the symbol still maps cleanly because the `viewBox` carries the true proportions.

## 5. crispEdges and decodability

SVG viewers anti-alias by default: a shape edge that falls between two device pixels is blended into intermediate greys to look smooth. For a QR symbol that is harmful — a decoder samples each module's centre and thresholds dark vs light, and blurred boundaries push edge pixels toward the threshold, eroding the margin a binariser relies on. The `shape-rendering="crispEdges"` attribute on the root `<svg>` tells the viewer to disable anti-aliasing and snap edges to the pixel grid, preserving the hard black/white transitions QR decoding assumes. This mirrors the PNG renderer's deliberate choice to draw modules as crisp squares with no anti-aliasing (doc 08).

The trade-off is that at non-integer scale factors `crispEdges` can make module widths slightly uneven (some modules a pixel wider than others as the rasteriser rounds). That unevenness is far less damaging to decoding than the grey-fringe blur it replaces, so it is the right default; the module-unit `viewBox` is kept regardless because scalability is the whole point of choosing SVG.

## 6. Colour handling

`WithColors` accepts any `color.Color`. Go's `color.Color.RGBA()` returns four 16-bit channels, alpha-premultiplied, in the range `[0, 0xFFFF]`. To emit a CSS-style `#RRGGBB` hex string we take each channel down to 8 bits by dividing by `0x101` (= 65535/255) rather than shifting right by 8, which rounds correctly across the full range instead of truncating.

Alpha needs care. SVG 1.1 does not portably accept an 8-digit `#RRGGBBAA` hex, so when a colour is not fully opaque the renderer emits a separate `fill-opacity` attribute carrying the fractional alpha (`alpha / 0xFFFF`). Fully opaque colours — the overwhelmingly common case, including the black-on-white default — omit `fill-opacity` entirely so the document stays minimal. Because `RGBA()` is premultiplied, the hex channels are divided back out by alpha before emission to recover the true colour the caller passed.

## 7. Sibling function, not an interface

An earlier version of doc 08 claimed that "other formats can be added later behind the same `Render` interface." No such interface ever existed, and v0.5 deliberately does not add one. There are exactly two renderers, `renderPNG` and `renderSVG`, they share the identical signature `func(m *matrix, opts renderOptions) ([]byte, error)`, and neither is ever selected through a variable at runtime — `Encode` calls `renderPNG` directly and `EncodeSVG` calls `renderSVG` directly. An interface would add an abstraction with exactly one dispatch site per implementation and no polymorphic caller, which is precisely the situation where YAGNI applies. This is the same reasoning that kept the v0.3 Sauvola dispatch as a straight `if` rather than a strategy interface (doc 14, section 7). Doc 08 has been corrected to describe sibling render functions sharing `renderOptions` instead of a fictional interface.

Choosing the format by which function you call — `Encode` for PNG bytes, `EncodeSVG` for SVG bytes — also keeps each function's return contract unambiguous, which a single `Encode` plus a `WithFormat` enum would muddy (what does the byte slice contain? depends on a hidden option).

## 8. Implementation pointers

- `qrgen/render_svg.go` hosts `renderSVG(m *matrix, opts renderOptions) ([]byte, error)` and the `colorToHex` helper. It reuses the existing `renderOptions` struct and its `withDefaults` method unchanged.
- The emitter builds the document with `strings.Builder` and `fmt`; there is no XML library dependency because the structure is fixed and all attribute values are numbers or controlled colour strings.
- Tests parse the output with `encoding/xml` to guarantee well-formedness, count the `M` commands in the path to confirm the dark-module total matches the matrix, and check `viewBox` / `width` / `height` against the option math.
- `EncodeSVG` / `EncodeSVGToFile` in `qrgen/api.go` run the same `resolveOptions -> validate -> buildMatrix` front-half as `Encode`, then call `renderSVG` instead of `renderPNG`.

## References

- W3C — *Scalable Vector Graphics (SVG) 1.1 (Second Edition)*: <https://www.w3.org/TR/SVG11/>. Path data grammar (section 8.3), `shape-rendering` (section 11.2), basic shapes and the `rect` element.
- Project Nayuki — *QR Code generator library*: its `toSvgString` renders the whole symbol as a single path, the approach adopted here. <https://www.nayuki.io/page/qr-code-generator-library>
- `docs/theory/08-rendering.md` — the PNG rendering notes; the colour-model and crisp-edges decisions there carry over to SVG, and its `Render`-interface sentence is corrected as part of this milestone.
- Go standard library — `image/color` (`color.Color.RGBA` semantics) and `encoding/xml` (used by the tests).
