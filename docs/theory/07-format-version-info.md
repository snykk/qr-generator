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

The 15 bits are written into two locations:

- Around the **top-left finder** in an L shape.
- Across the **top-right** finder (row 8) and the **bottom-left** finder (column 8), split between the two so that damage to one corner does not destroy the metadata.

Order: bit 14 (most significant) first. The exact module positions per bit are tabulated in ISO/IEC 18004:2015 §8.9.

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

- ISO/IEC 18004:2015, §8.9 (Format information), §8.10 (Version information), Annexes C and D.
- Bose, R. C. and Ray-Chaudhuri, D. K., "On a Class of Error Correcting Binary Group Codes," *Information and Control*, 3(1), 1960, pp. 68–79.
