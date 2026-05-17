# Rendering

Once we have a finished `[][]bool` matrix, rendering it to a bitmap is straightforward. This document records the choices for the PNG renderer.

## Module → pixel mapping

Each QR module becomes a square block of `s × s` pixels, where `s` is the configurable *module size* (default 8 px in our CLI). The output image side length in pixels is:

    pixels = s · (n + 2·q)

where `n` is the matrix side in modules and `q` is the quiet-zone width in modules (default 4, per the spec).

## Quiet zone

ISO/IEC 18004:2015 §6.3.8 requires at least 4 modules of background on all sides of the symbol. We always render the quiet zone explicitly into the output; we do **not** rely on the caller to leave whitespace around the image. This makes the output safely embeddable in arbitrary backgrounds.

## Colour model

The default renderer outputs an 8-bit grayscale image (`image.Gray`), with dark modules at `0x00` and light modules at `0xFF`. When the caller passes custom colours via `WithColors`, we switch to `image.RGBA` and write the provided RGBA values directly. There is no anti-aliasing — modules are rendered as crisp squares because QR decoders rely on hard edges.

## Encoding

We encode through `image/png` from the Go standard library. The encoder's defaults are fine; we do not need transparency or interlacing.

## Contrast warning

A QR scanner needs sufficient contrast between dark and light modules. The renderer will, by default, refuse a foreground/background pair whose luminance contrast ratio is below 4:1 (the threshold WCAG 2.1 uses for normal text). This is a soft check; callers can opt out via `WithSkipContrastCheck(true)` if they know what they're doing.

## Why PNG first

- PNG is loss-less, so module edges stay crisp — important for decodability.
- The standard library encoder is already in scope; no new dependencies.
- Other formats (SVG, terminal, JPEG) can be added later behind the same `Render` interface without breaking the public API.

## Implementation pointers

- `qrgen/render_png.go`: `renderPNG(m *Matrix, opts renderOpts) ([]byte, error)`.
- The renderer should accept the *unrendered* matrix plus the option struct, not a partially-prepared image, so it stays trivially testable.

## References

- ISO/IEC 18004:2015, §6.3.8 (Quiet zone), §11 (Reference decode algorithm).
- W3C, *Web Content Accessibility Guidelines (WCAG) 2.1*, §1.4.3 — contrast ratio formula used for the soft check.
- Go standard library, `image` and `image/png` documentation.
