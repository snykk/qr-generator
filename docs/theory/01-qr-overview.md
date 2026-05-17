# QR Code Overview

## What a QR code is

A QR (Quick Response) code is a two-dimensional matrix barcode invented in 1994 by Masahiro Hara at Denso Wave for automotive part tracking. It encodes data in a square grid of dark and light modules and embeds redundancy via Reed–Solomon error correction so it can still be decoded under partial damage, dirt, or obstruction.

QR is standardised by ISO/IEC 18004 (current edition: 2015).

## Symbol structure

A QR symbol is composed of:

- **Finder patterns** — three large concentric squares in the top-left, top-right, and bottom-left corners. They allow a scanner to locate the symbol regardless of rotation.
- **Separators** — one-module-wide light borders around each finder, facing the data area.
- **Alignment patterns** — smaller concentric squares (present in version 2 and above) used to correct perspective distortion.
- **Timing patterns** — alternating dark/light modules on row 6 and column 6; they let the scanner measure module pitch across the symbol.
- **Format information** — 15 bits encoding the error-correction level and the chosen mask pattern, written into two redundant locations.
- **Version information** — 18 bits encoding the version number, present only in version 7 and above, also replicated in two locations.
- **Data and error-correction modules** — fill all remaining cells.
- **Quiet zone** — at least 4 modules of background around the symbol.

ASCII sketch of a version-1 (21×21) symbol:

```
F F F F F F F . . . . . . . F F F F F F F
F . . . . . F . . . . . . . F . . . . . F
F . X X X . F . . . . . . . F . X X X . F
F . X X X . F . . . . . . . F . X X X . F
F . X X X . F . . . . . . . F . X X X . F
F . . . . . F . . . . . . . F . . . . . F
F F F F F F F . T . . . . . F F F F F F F
. . . . . . . . . . . . . . . . . . . . .
. . . . . . T . . . . . . . . . . . . . .
. . . . . . . . D D D D D D D D D D D D D
. . . . . . . . D D D D D D D D D D D D D
...
```

Legend: `F` finder, `T` timing, `D` data area.

## Versions and sizes

QR comes in 40 versions. Version `v` has side length:

    side(v) = 21 + 4 * (v - 1)   modules

So version 1 is 21×21 and version 40 is 177×177.

## Error correction levels

Four levels of redundancy are supported:

| Level | Approx. recovery |
|:-----:|:-----------------|
|   L   | up to ~7%        |
|   M   | up to ~15%       |
|   Q   | up to ~25%       |
|   H   | up to ~30%       |

Higher levels mean fewer data codewords for the same version, because the codeword budget for a given (version) is fixed and split between data and error-correction codewords.

## Capacity

Capacity depends on the (version, EC level, encoding mode) triple. Two end-of-range examples:

- Version 1 / EC level M can hold up to 14 alphanumeric characters.
- Version 40 / EC level L can hold up to 7,089 numeric digits.

The exact tables live in ISO/IEC 18004:2015 Table 7 and will be encoded into `qrgen/version.go`.

## Lifecycle of an encode

1. **Mode selection** — pick the encoding mode that minimises bit count.
2. **Version selection** — choose the smallest version whose capacity holds the payload at the requested EC level.
3. **Bit stream construction** — mode indicator + character count indicator
   + payload bits + terminator + pad bytes.
4. **Reed–Solomon** — compute EC codewords per block and interleave them.
5. **Matrix construction** — place functional patterns, then weave data bits through the remaining cells in a zig-zag walk.
6. **Masking** — XOR data-area modules with whichever of the eight mask patterns yields the lowest penalty score.
7. **Format & version info** — write the BCH-encoded metadata into reserved modules.
8. **Render** — translate the bit matrix into a PNG bitmap with a quiet zone.

The remaining documents in this folder cover each of these steps in detail.

## References

- ISO/IEC 18004:2015, sections 6 (symbol structure) and 7 (encoding).
- Thonky, "Module Placement in Matrix" — <https://www.thonky.com/qr-code-tutorial/module-placement-matrix>
- Denso Wave, "History of QR Code" — <https://www.qrcode.com/en/history/>
