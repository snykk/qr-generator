# Theory & References

This folder contains the literature review and theoretical foundation for the `qrgen` library. Each document focuses on one component of the QR code encoder and is intended both as a learning resource for contributors and as a record that the implementation is grounded in published algorithms — not folklore.

## Contents

1. [QR Code Overview](01-qr-overview.md) — history, symbol anatomy, version system, error correction levels, lifecycle of an encode.
2. [Data Encoding](02-data-encoding.md) — encoding modes, character count indicators, version selection, terminator and padding.
3. [Galois Field GF(256)](03-galois-field.md) — field arithmetic that underpins Reed–Solomon.
4. [Reed–Solomon Error Correction](04-reed-solomon.md) — generator polynomial, per-block encoding, interleaving, remainder bits.
5. [Matrix Construction](05-matrix-construction.md) — finder, alignment, timing patterns, reserved areas, the zig-zag data walk.
6. [Masking](06-masking.md) — the 8 mask patterns and the four-rule penalty score.
7. [Format & Version Information](07-format-version-info.md) — BCH(15, 5) and BCH(18, 6) codes plus their fixed bit placements.
8. [Rendering](08-rendering.md) — matrix → PNG, module sizing, quiet zone, colour choices.
9. [Data Tables & Lookup Values](09-data-tables.md) — every static table needed by the encoder: mode indicators, alphanumeric mapping, capacity, EC block structure, alignment positions, generator polynomials, format/version codewords.
10. [Worked Example: `HELLO WORLD`](10-worked-example.md) — end-to-end encode at EC-M with every intermediate value, ready to use as a golden fixture.

### Decoder side (v0.2.0)

11. [Reed–Solomon Decoding](11-rs-decoding.md) — syndromes, Berlekamp–Massey, Chien search, Forney's algorithm. The inverse of doc 04 but algorithmically different.
12. [Image Processing](12-image-processing.md) — grayscale, Otsu binarisation, finder-pattern scanning, homography, module sampling.
13. [Decoder Pipeline](13-decoder-pipeline.md) — end-to-end stage diagram, what each stage can fail on, and the error-handling philosophy.

## Primary references

- **ISO/IEC 18004:2015** — *Information technology — Automatic identification and data capture techniques — QR code bar code symbology specification.* The normative source.
- **Thonky QR Code Tutorial** — practical walk-through of the encoding process, useful when the spec is too dense. <https://www.thonky.com/qr-code-tutorial/>
- **Project Nayuki** — *QR Code generator library*, including step-by-step reference implementations in multiple languages, useful as a cross-check oracle. <https://www.nayuki.io/page/qr-code-generator-library>

Additional references appear at the end of each document.

## Conventions used in these documents

- Bit strings are written left-to-right with the most significant bit first.
- Matrix coordinates use `(row, column)` with `(0, 0)` at the **top-left**, matching the orientation we use when rendering to images.
- Hex literals use the `0x` prefix.
- Polynomial coefficients are listed from highest degree to lowest unless explicitly noted.
- "Spec ref." pointers refer to ISO/IEC 18004:2015 sections.

## How these notes relate to the code

| Theory doc                          | Primary implementation file        |
|-------------------------------------|------------------------------------|
| 02-data-encoding.md                 | `qrgen/mode.go`, `qrgen/version.go`|
| 03-galois-field.md                  | `qrgen/gf256.go`                   |
| 04-reed-solomon.md                  | `qrgen/reedsolomon.go`             |
| 05-matrix-construction.md           | `qrgen/matrix.go`                  |
| 06-masking.md                       | `qrgen/mask.go`                    |
| 07-format-version-info.md           | `qrgen/formatinfo.go`              |
| 08-rendering.md                     | `qrgen/render_png.go`              |
| 09-data-tables.md                   | `qrgen/version.go`, `qrgen/formatinfo.go`, `qrgen/matrix.go` |
| 10-worked-example.md                | golden test fixtures under `qrgen/testdata/` |
| 11-rs-decoding.md                   | `qrgen/rs_decode.go` (planned, D3)            |
| 12-image-processing.md              | `qrgen/decode_image.go` (planned, D8–D12)     |
| 13-decoder-pipeline.md              | `qrgen/decode.go` (planned, D7 + D12)         |

If you change an algorithm, please update the corresponding document in this folder. The theory docs are the durable explanation of *why* the code looks the way it does.
