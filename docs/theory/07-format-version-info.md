# Format & Version Information

Two short messages — one about EC level and mask, the other about version — are written into reserved modules so a decoder can configure itself before reading the payload. Both are BCH codes, and the format-info codeword has a fixed XOR mask applied at the end.

## Format information — BCH(15, 5)

### Payload

5 bits, in order:

- 2 bits: EC level (`L=01`, `M=00`, `Q=11`, `H=10`).
- 3 bits: mask pattern index `0..7`.

So for EC level `M` and mask `5` the payload is `00 101` = `00101`.

### BCH encoding

Compute `payload · x¹⁰` modulo `g(x) = x¹⁰ + x⁸ + x⁵ + x⁴ + x² + x + 1` (binary `10100110111`, hex `0x537`). Concatenate `payload (5 bits) || remainder (10 bits)` to get a 15-bit codeword.

### Mask

XOR the 15-bit codeword with `101010000010010` (`0x5412`). The XOR prevents an all-zero payload from producing an all-zero format string, which would violate the spec's requirement that the format area never be uniform.

### Placement

The 15 bits are written twice so that damage to one region does not destroy the metadata. Bit 0 is the least significant bit of the masked codeword and bit 14 the most significant; positions are `(row, column)` with `(0, 0)` at the top-left and `n` the symbol side.

- **First copy, around the top-left finder.** Bits 0–5 run down column 8 in rows 0–5, bit 6 sits at `(7, 8)` just past the timing row, bit 7 at the corner `(8, 8)`, bit 8 at `(8, 7)` just past the timing column, and bits 9–14 run left along row 8 so that bit 14 lands at `(8, 0)`.
- **Second copy, split between the other two finders.** Bits 0–7 run right to left along row 8 from `(8, n−1)` to `(8, n−8)`, and bits 8–14 run down column 8 from `(n−7, 8)` to `(n−1, 8)`. The module at `(n−8, 8)` just above them is the always-dark module, not part of the copy.

| Bit | 0–5 | 6 | 7 | 8 | 9–14 |
|---|---|---|---|---|---|
| First copy | `(i, 8)` | `(7, 8)` | `(8, 8)` | `(8, 7)` | `(8, 14−i)` |
| Second copy | `(8, n−1−i)` | `(8, n−7)` | `(8, n−8)` | `(n−7, 8)` | `(n−15+i, 8)` |

Both copies therefore carry the same 15-bit word, which is what lets a reader recover the format from either one. These positions follow ISO/IEC 18004:2015 section 7.9.1 and match ZXing's `TYPE_INFO_COORDINATES`, segno, and zint's output; `qrgen/testdata/thirdparty` pins them with symbols from independent encoders.

Before v0.14.1 this library wrote and read the first copy bit-reversed, putting bit `i` where the table puts bit `14−i`. Because the encoder and decoder shared the mistake, every internal round trip passed, and other readers still scanned our symbols through the intact second copy, so it went unnoticed; but the redundancy was gone, and the decoder could not read 14 of the 32 format words written by any other encoder. The lesson generalises: test a decoder on symbols it did not produce.

## Version information — BCH(18, 6)

Present only for **version 7 and above**.

### Payload

6 bits encoding the version number `7..40`, in unsigned binary, most-significant bit first.

### BCH encoding

Compute `payload · x¹²` modulo `g(x) = x¹² + x¹¹ + x¹⁰ + x⁹ + x⁸ + x⁵ + x² + 1` (binary `1111100100101`, hex `0x1F25`). Concatenate `payload (6) || remainder (12)` to get the 18-bit codeword.

No XOR mask is applied.

### Placement

Two 6×3 blocks:

- A `6 × 3` block directly above the bottom-left finder, in rows `n − 11..n − 9` and columns `0..5`.
- A `3 × 6` block directly to the left of the top-right finder, in rows `0..5` and columns `n − 11..n − 9`.

The same 18 bits are written into both blocks but in transposed orientation. The exact bit-to-module mapping is given in ISO/IEC 18004:2015 Annex D.

## Pre-computed lookup tables

The full enumeration is small: 32 format-info codewords (4 EC levels × 8 masks) and 34 version-info codewords (versions 7 through 40). We precompute both tables once and look them up at runtime, which keeps the encode path free of polynomial arithmetic for these tiny codes.

## Implementation pointers

- `qrgen/formatinfo.go`: precomputed `formatInfo[ecLevel][mask] uint16` and `versionInfo[version] uint32` tables, plus the placement functions that write them into the matrix.

## References

- ISO/IEC 18004:2015, section 7.9 (Format information), section 7.10 (Version information), Annexes C and D. Older references, including JIS X 0510:2004 and the ZXing sources, number these sections 8.9 and 8.10.
- Bose, R. C. and Ray-Chaudhuri, D. K., "On a Class of Error Correcting Binary Group Codes," *Information and Control*, 3(1), 1960, pp. 68–79.
