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

### Decoder robustness (v0.3.0)

14. [Adaptive Thresholding](14-adaptive-thresholding.md) — Otsu's failure modes on uneven lighting, Sauvola's formula and parameters, integral images for O(1) window queries, and the proactive-bimodality plus reactive-post-check dispatch that decides which binariser runs.

### Decoder robustness (v0.4.0)

15. [Rotation Handling](15-rotation-handling.md) — where the upright assumption lives in the v0.3 pipeline, why right-angle vertex detection is already rotation-invariant, the cross-product handedness identity that disambiguates top-right from bottom-left at any rotation, and the scope boundary at ~30 degrees set by the 1:1:3:1:1 scanner's tolerance.

### Output formats (v0.5.0)

16. [SVG Rendering](16-svg-rendering.md) — why SVG (lossless scaling, crisp edges, HTML embedding — not file size), the SVG document model for a QR symbol, single-path drawing versus one rect per module, the module-unit viewBox with pixel sizing, crispEdges and decodability, colour-to-hex with fill-opacity for alpha, and why the renderer is a sibling function rather than a Render interface.

### Encoder completeness (v0.6.0)

17. [Optimal Mode Segmentation](17-optimal-segmentation.md) — why single-mode greedy leaves bits on the table, the per-segment cost model, the dynamic program over positions and modes, a worked example, the version-group interplay, the UTF-8 rune-boundary rule, and the homogeneous-input identity guarantee.

### Payload conventions (v0.7.0)

18. [Payload Formats](18-payload-formats.md) — the scanner conventions the convenience builders produce (Wi-Fi, vCard, mailto, tel, SMS, geo): exact templates, escaping rules, citations, and worked examples, plus why string builders compose with every output format.

### Output formats (v0.8.0)

19. [Terminal Rendering](19-terminal-rendering.md) — why a terminal renderer (immediacy, pipeable text, scannable from a screen), the cell aspect ratio that motivates half-block packing, the module-pair to glyph mapping, the ASCII fallback, the polarity problem and the invert switch, the quiet-zone and odd-row-tail handling, and why the renderer is a sibling function returning a string.

### Encoding completeness (v0.9.0)

20. [ECI Segments](20-eci-segments.md) — Extended Channel Interpretation: the ECI header (mode `0111` plus the 1/2/3-codeword designator), the shortest-form encode versus the non-minimal-accepting decode, the assignment numbers and the ISO-8859-1-versus-UTF-8 default-charset nuance, ECI scope, and the zero-dependency transcoding boundary (UTF-8 and Latin-1 only).

### Structured append (v0.10.0)

21. [Structured Append](21-structured-append.md) — splitting a message across up to 16 linked symbols and reassembling them: the structured-append header (mode `0011` plus the position/count sequence indicator and the shared parity byte), the 16-symbol cap, the XOR parity over the original message, header placement before ECI, and the rune-boundary split with ordered reassembly.

### Encoding completeness (v0.11.0)

22. [Kanji Mode](22-kanji-mode.md) — the fourth data mode: encoding JIS X 0208 characters as 13-bit values under mode indicator `1000` (8/10/12 count widths), the Shift-JIS derivation, the generated committed table that keeps the runtime zero-dependency, how Kanji joins the DP segmenter, and the honest ~1.8x density versus byte mode.

### Output styling (v0.12.0)

23. [Logo Embedding](23-logo-embedding.md) — overlaying a centred logo while staying scannable: why Reed–Solomon recovery and block interleaving absorb the occlusion, the safe default size, the automatic EC-H bump, the background pad, PNG compositing versus SVG data-URI embedding, and the zero-dependency resampler.

### Decoder robustness (v0.13.0)

24. [Arbitrary-Angle Decoding](24-arbitrary-angle-detection.md) — reading symbols tilted well past where the affine pipeline fails: why the real blocker is the affine three-centroid homography rather than finder detection, why perspective needs a projective transform, extracting the finder-pattern corners via connected-component analysis, the twelve-corner least-squares homography, the corner-based version seed plus trial decode, the affine-first fallback that leaves upright decoding unchanged, and the synthetic-tilt validation with an honest band.

### Multi-symbol decoding (v0.14.0)

25. [Multi-Symbol Decoding](25-multi-symbol-decoding.md) — reading every QR symbol in one image: why the connected-component detector already enumerates all finders, grouping them into disjoint valid triples of a shared module pitch and right-isosceles geometry, why decode success (not geometry) must be the claim arbiter so a same-size grid's cross-symbol triples cannot steal real finders, reusing the per-symbol projective pipeline, deriving each symbol's outline from its homography, the reading-order and skip-per-symbol error semantics, and the honest limits.

### Micro QR Code (v0.15.0)

26. [Micro QR Code](26-micro-qr.md) — the small sibling of QR in the same standard: the eight symbol types M1 to M4 and their capacities, the shorter mode indicators and count widths, the terminator and padding rules including the 4-bit half codeword in M1 and M3, a single Reed-Solomon block (detection only in M1), the one-finder layout with timing on row 0 and column 0, the single format-information copy with its own XOR mask, placement without the column-6 skip, the four masks and their highest-edge-score rule, a step-by-step comparison with QR, and worked bit streams checked against zint.

### Micro QR detection (v0.16.0)

27. [Micro QR Detection](27-micro-qr-detection.md) — locating Micro symbols in images from a single finder pattern: what the finder's four corners determine and the quarter-turn ambiguity they leave, orientation from the innermost timing modules, reading the format information before the size is known and why a valid format word is weak evidence, cross-checking the size against the timing pattern, extrapolation error and finding the far corners again, the false-positive gates that end in Reed-Solomon, where the search joins the pipeline after QR, and zxing-cpp as prior art.

### rMQR Code (v0.17.0)

28. [rMQR Code](28-rmqr.md) — the rectangular member of the QR family (ISO/IEC 23941): the 32 versions from R7x43 to R17x139 with their codeword, block, remainder, and count-width tables, the 3-bit mode indicator, terminator and padding, error correction at levels M and H with QR's interleaving, the layout with its finder sub-pattern, corner patterns, edge timing, and alignment columns, the two 18-bit format copies under BCH(18,6) with their own masks, placement from the second column, the single fixed mask, a comparison with QR and Micro QR, and worked bit streams checked against zint.

### Symbol inspection and the web demo (v0.18.0)

29. [Symbol Inspection](29-symbol-inspection.md) — taking a finished symbol apart again with `Inspect`: the function and message layers, the census of every module role and the overlaps that decide it (alignment over timing, QR's format area, R7's finder), recording roles and stream bits in the encoder's own placement code, mapping stream bits to codewords including Micro QR's half codeword, the symbol before masking and the score of every candidate mask, the message's bit budget, locating each codeword in its block, and running the library in a browser through WebAssembly, with the cost of scanning camera frames.

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
| 11-rs-decoding.md                   | `qrgen/rs_decode.go`            |
| 12-image-processing.md              | `qrgen/decode_image.go`     |
| 13-decoder-pipeline.md              | `qrgen/decode.go`         |
| 14-adaptive-thresholding.md         | `qrgen/decode_image_sauvola.go` |
| 15-rotation-handling.md             | `qrgen/decode_image.go` (`orderFinderTriple`) |
| 16-svg-rendering.md                 | `qrgen/render_svg.go` |
| 17-optimal-segmentation.md          | `qrgen/segment.go` |
| 18-payload-formats.md               | `qrgen/payload.go` |
| 19-terminal-rendering.md            | `qrgen/render_terminal.go` |
| 20-eci-segments.md                  | `qrgen/eci.go` |
| 21-structured-append.md             | `qrgen/structured_append.go` |
| 22-kanji-mode.md                    | `qrgen/kanji.go` |
| 23-logo-embedding.md                | `qrgen/logo.go` |
| 24-arbitrary-angle-detection.md     | `qrgen/decode_finder_cc.go`, `qrgen/decode_image.go` |
| 25-multi-symbol-decoding.md         | `qrgen/decode_multi.go`, `qrgen/decode_finder_cc.go` |
| 26-micro-qr.md                      | `qrgen/micro.go`, `qrgen/micro_encode.go`, `qrgen/micro_matrix.go`, `qrgen/micro_decode.go` |
| 27-micro-qr-detection.md            | `qrgen/decode_micro.go`, `qrgen/decode_finder_cc.go`, `qrgen/decode_image.go`, `qrgen/decode_multi.go` |
| 28-rmqr.md                          | `qrgen/rmqr.go`, `qrgen/rmqr_encode.go`, `qrgen/rmqr_matrix.go`, `qrgen/rmqr_decode.go` |
| 29-symbol-inspection.md             | `qrgen/inspect.go`, `web/internal/bridge/bridge.go`, `web/site/inspect.js` |

If you change an algorithm, please update the corresponding document in this folder. The theory docs are the durable explanation of *why* the code looks the way it does.
