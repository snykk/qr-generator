# Decoder Pipeline

This document is the bird's-eye view of how `qrgen.Decode(image.Image) (string, error)` works end-to-end. Individual algorithms are detailed in [11-rs-decoding.md](11-rs-decoding.md) and [12-image-processing.md](12-image-processing.md); here we explain how they compose, what each stage can fail on, and what kind of error the caller sees in each case.

> Indonesian version: [13-decoder-pipeline.id.md](13-decoder-pipeline.id.md).

## End-to-end flow

```text
image.Image
   │
   ▼ D8: grayscale + Otsu binarisation
bitmap (width × height bools)
   │
   ▼ D9: finder-pattern scan + geometry validation
3 finder centres + estimated module pitch
   │
   ▼ D10: version estimation + homography from 4 corner correspondences
3×3 homography matrix
   │
   ▼ D11: optional alignment-pattern refinement (V2+)
refined homography
   │
   ▼ D12: module sampling at each grid centre
[][]bool matrix (n × n)
   │
   ▼ D4: read format-info, BCH error-correct → (EC level, mask)
(ECLevel, mask)
   │
   ▼ D5: reverse zig-zag walk, XOR mask, strip remainder bits
interleaved [data ‖ EC] byte stream
   │
   ▼ D6: deinterleave per block, RS decode each block
corrected data codeword stream
   │
   ▼ D7: parse mode + count indicators, decode payload per mode
final decoded text
```

The horizontal split in the diagram corresponds neatly to our public API split: stages D8..D12 turn an image into a matrix, and stages D4..D7 turn that matrix into text. `DecodeMatrix` is the second half exposed on its own; `Decode` runs both halves.

## Stage responsibilities & failure modes

| Stage | Input | Output | Error returned on failure |
|------:|------|--------|---------------------------|
| D8    | `image.Image` | bitmap | none (binarisation never fails) |
| D9    | bitmap | 3 finder centres | `ErrFinderNotFound` |
| D10   | finders | homography | `ErrInvalidVersion` if estimated version is out of range |
| D11   | homography | refined homography | silently falls back to unrefined H |
| D12   | refined H | `[][]bool` matrix | none (sampling cannot fail once H is computed) |
| D4    | matrix | (EC, mask) | `ErrFormatUnreadable` if both copies are too corrupted |
| D5    | matrix + (EC, mask) | interleaved bytes | none |
| D6    | interleaved + version + EC | corrected data bytes | `ErrTooManyErrors` if any block exceeds RS capacity |
| D7    | data bytes | text | `ErrCorruptedPayload` for impossible mode indicators or character counts |

Every error type is exported from the package so callers can branch with `errors.Is` rather than parsing error messages.

## Why the split matters

Keeping image processing and matrix processing in two separate stages lets us:

1. **Test them independently.** `DecodeMatrix` is fed by our own encoder via round-trip tests — no images involved, no homography numerical noise. Bugs in RS decoding or mask reversal surface clearly.
2. **Expose `DecodeMatrix` as a public entry point.** Callers that already have a clean matrix (perhaps from a different image pipeline or a custom rasteriser) get to skip the heavy image stage.
3. **Iterate on image robustness without touching the rest.** Adding local thresholding, multi-finder candidates, or more aggressive geometry recovery only changes stages D8–D11.

## Error-handling philosophy

The decoder must **fail loudly or succeed correctly** — there is no acceptable "return some plausible-looking text" mode. Concretely:

- If syndromes are non-zero and Berlekamp–Massey produces a locator polynomial of degree greater than `n/2`, the block is uncorrectable and we abort with `ErrTooManyErrors`.
- If Chien search returns fewer error positions than the BM degree, the syndrome data is internally inconsistent (more than `n` errors), and we also abort.
- If the format-info BCH decoder cannot find any of the 32 valid codewords within 3 bit-flips of either copy, `ErrFormatUnreadable`.
- If the mode indicator we read is not in `{0001, 0010, 0100}` (or `0000` terminator), `ErrCorruptedPayload`.

Silent fallback paths invite the worst kind of bug: code that "decoded" the wrong text because the matrix was actually unreadable.

## What the decoder does *not* do

- It does not search the image for *multiple* QR symbols. The first valid finder triple wins.
- It does not auto-rotate the image as a preprocessing step. The finder-pattern detector is rotation-aware enough to find finders regardless of orientation, so a separate rotation pass is unnecessary.
- It does not handle inverted (light-on-dark) QR symbols in v0.2. Most scanners do; we can add it later by trying both colour polarities at the binarisation stage.
- It does not parse ECI segments. Byte-mode payloads are interpreted as UTF-8 to mirror the encoder's convention.

## Implementation pointers

- `qrgen/decode.go` exposes `Decode`, `DecodeMatrix`, `DecodeBytes`. It is a thin façade over the staged helpers below.
- `qrgen/decode_image.go` covers D8..D12.
- `qrgen/decode_matrix.go` covers D5 and D7.
- `qrgen/rs_decode.go` covers D3 and D6 internals.
- `qrgen/format_decode.go` covers D4.
- Errors live alongside their producing stage but are documented in `qrgen/decode.go` for discoverability.

## References

See [11-rs-decoding.md](11-rs-decoding.md) and [12-image-processing.md](12-image-processing.md) for stage-specific references. The overall pipeline structure follows ISO/IEC 18004:2015 §11 and is informed by the ZXing reference implementation.
