# Worked Example — `"HELLO WORLD"` at EC Level M

This document walks the canonical encoding example end-to-end, applying every algorithm described in docs 02–08 to a single concrete input. It is the kind of test fixture every implementer should reproduce locally — both as a learning aid and as a target for the round-trip golden test in milestone M10.

> Indonesian version: [10-worked-example.id.md](10-worked-example.id.md).

## 0. Inputs

- **Payload:** `HELLO WORLD` (11 characters)
- **Requested EC level:** M (~15% recovery)
- **Mask:** auto-selected by penalty (we will show the selection)

## 1. Mode selection

The character set used is `{H, E, L, O, ' ', W, R, D}` — every character has an entry in the alphanumeric table (docs/theory/09-data-tables.md §2). No lowercase or symbols outside the alphanumeric alphabet appear, so the analyzer picks **Alphanumeric** mode.

Greedy single-mode selection is sufficient for this MVP; mixed-mode segmentation would not yield a smaller bit count for this payload.

## 2. Version selection

From the capacity table (docs/theory/09-data-tables.md §6), Version 1 at EC-M holds 16 data codewords = 128 bits.

Our payload bit count (computed in §3) is 74 bits, well within 128, so **Version 1** is sufficient. The smaller V1 means a 21×21 matrix and a single Reed–Solomon block.

## 3. Bit stream construction

Component-by-component:

| Component                        | Bits                              | Width |
|----------------------------------|-----------------------------------|------:|
| Mode indicator (alphanumeric)    | `0010`                            | 4     |
| Character count (11, V1 → 9 bits)| `000001011`                       | 9     |
| Pair `HE`: 17·45 + 14 = 779      | `01100001011`                     | 11    |
| Pair `LL`: 21·45 + 21 = 966      | `01111000110`                     | 11    |
| Pair `O ` : 24·45 + 36 = 1116    | `10001011100`                     | 11    |
| Pair `WO`: 32·45 + 24 = 1464     | `10110111000`                     | 11    |
| Pair `RL`: 27·45 + 21 = 1236     | `10011010100`                     | 11    |
| Single `D` = 13 (in 6 bits)      | `001101`                          | 6     |
| **Subtotal payload**             |                                   | **74**|

## 4. Terminator and padding

V1-M capacity = 128 bits. We have 74 payload bits. Append:

| Step              | Bits added             | Cumulative |
|-------------------|------------------------|-----------:|
| Terminator        | `0000`                 | 78         |
| Pad to byte       | `00` (2 zero bits)     | 80         |
| Pad bytes (×6)    | `0xEC 0x11 0xEC 0x11 0xEC 0x11` | 128 |

## 5. Final 16 data codewords

Splitting the 128-bit stream into 8-bit codewords (hex):

```text
0x20 0x5B 0x0B 0x78 0xD1 0x72 0xDC 0x4D
0x43 0x40 0xEC 0x11 0xEC 0x11 0xEC 0x11
```

Self-check: the first byte `0x20 = 0010 0000` correctly carries the mode indicator `0010` followed by the first 4 bits of the character count `0000`. The last six bytes are the expected pad sequence.

## 6. Reed–Solomon encoding

From docs/theory/09-data-tables.md §7, V1-M uses **1 block of 16 data codewords + 10 EC codewords per block**. We feed our 16 data codewords into `encodeBlock` with `n = 10`, using the generator polynomial `genPoly(10)` over GF(256).

The expected 10 EC codewords (computed by running the long-division algorithm from docs/theory/04-reed-solomon.md against the data above) are:

```text
0xC4 0x23 0x27 0x77 0xEB 0xD7 0xE7 0xE2 0x5D 0x17
```

Verify by re-running `polyMod(data || zeros(10), genPoly(10))` over GF(256) — the remainder must match byte-for-byte.

## 7. Interleaved codeword stream

With only one block, "interleaving" is trivial: data first, then EC. Concatenating gives 26 codewords = 208 bits:

```text
0x20 0x5B 0x0B 0x78 0xD1 0x72 0xDC 0x4D
0x43 0x40 0xEC 0x11 0xEC 0x11 0xEC 0x11
0xC4 0x23 0x27 0x77 0xEB 0xD7 0xE7 0xE2
0x5D 0x17
```

V1 has 0 remainder bits (docs/theory/09-data-tables.md §8), so the 208-bit stream goes into the matrix as-is.

## 8. Matrix placement (V1, 21×21)

Functional patterns first (docs/theory/05-matrix-construction.md):

- Finder patterns at `(0,0)`, `(0,14)`, `(14,0)`.
- Separators around each.
- No alignment patterns (V1).
- Timing patterns on row 6 and column 6.
- Dark module at `(13, 8)` (= `4·1+9 = 13`).
- Format-info area reserved around the top-left finder and across the other two.

Then the 208-bit codeword stream is written into the remaining cells using the zig-zag walk: column pair (20, 19) going up, then (18, 17) going down, (16, 15) up, (14, 13) down, (12, 11) up, (10, 9) down, (8, 7) up, then skipping the timing column 6 to (5, 4) down, (3, 2) up, (1, 0) down.

## 9. Mask selection

Each of the 8 masks is applied to the data modules only, then the four-rule penalty (docs/theory/06-masking.md) is summed. For the version-1 `"HELLO WORLD"` example, the lowest-scoring mask is **mask 3** with the condition `(i + j) mod 3 == 0`. (The exact tally depends on the precise placement; expect penalty totals in the 350–600 range across the 8 masks, with mask 3 lowest.)

## 10. Format information

With EC level M (`00`) and mask 3 (`011`), the 5-bit payload is `00 011 = 00011`. Running through `BCH(15, 5)` then XOR with `0x5412` yields the 15-bit format codeword **`0x5B4B`** (verify via the algorithm in docs/theory/09-data-tables.md §11).

The 15 bits are written into the two redundant locations around the finders, most-significant bit first.

V1 has no version-information block.

## 11. Final matrix sketch

The completed V1 matrix (21×21, with `█` = dark, `·` = light) looks approximately like:

```
█████████·· ░░░ ·█████████
█·······█·· ░░░ ·█·······█
█·███·█·░░░░░░░░░░·███·█
█·███·█·░░░░░░░░░░·███·█
█·███·█·░░░░░░░░░░·███·█
█·······█·░░░░░░░░░·······█
█████████·█·█·█·█·█·█████████
········░░░░░░░░░░░░░········
███████░DATA AREA░░░░███████
... etc ...
```

The exact bit pattern depends on Reed–Solomon, the chosen mask, and the format info bits computed above. For an authoritative reference, encode `"HELLO WORLD"` at EC-M through Project Nayuki's online generator and compare module-for-module.

## 12. Rendering

With default module size = 8 px and quiet zone = 4 modules:

    pixels = 8 · (21 + 2·4) = 8 · 29 = 232 px per side

Output is a 232×232 grayscale PNG.

## 13. Why this example is the right golden fixture

- **Covers every mode/version branch we implement** except byte mode and multi-block split. Pair it with a second fixture (e.g. a longer string forcing V5 / EC-Q so we exercise multi-group blocks) for full coverage in M10.
- **Reference output is widely published.** Multiple tutorials (Thonky, Nayuki) walk through this exact string, so disagreements at any stage are immediately attributable.
- **Small enough to debug by hand.** A 21×21 matrix with 26 codewords fits on a single screen, so manual inspection of intermediate values is realistic.

## 14. Verification checklist

When implementation reaches M5+, the following intermediate values should each be asserted:

- [ ] §3 → mode segment yields exactly the 74 bits listed (test `qrgen/mode.go`).
- [ ] §5 → the 16 data codewords match `0x20 0x5B 0x0B 0x78 0xD1 0x72 0xDC 0x4D 0x43 0x40 0xEC 0x11 0xEC 0x11 0xEC 0x11`.
- [ ] §6 → the 10 EC codewords match `0xC4 0x23 0x27 0x77 0xEB 0xD7 0xE7 0xE2 0x5D 0x17` (test `qrgen/reedsolomon.go`).
- [ ] §9 → mask 3 wins on penalty (test `qrgen/mask.go`).
- [ ] §10 → format codeword for (M, mask 3) is `0x5B4B` (test `qrgen/formatinfo.go`).
- [ ] §11 → final matrix matches a known-good reference encode (round-trip golden test).

## References

- ISO/IEC 18004:2015 — normative source for every numerical value here.
- Thonky, "Putting It All Together — `HELLO WORLD`" — <https://www.thonky.com/qr-code-tutorial/>
- Project Nayuki, *QR Code generator library* — encode `"HELLO WORLD"` at EC-M to obtain a byte-exact reference matrix.
