# Micro QR Code

Micro QR Code is the small sibling of QR Code defined in the same standard, ISO/IEC 18004. It keeps QR's building blocks (the same data modes, the same Galois field and Reed-Solomon code, the same BCH-protected format word, the same zigzag placement) but changes the parameters at almost every step, and drops the parts that only pay off in large symbols. This document walks through each step, says exactly how it differs from QR, and closes with worked examples that pin the encoding bit for bit.

> Indonesian version: [26-micro-qr.id.md](26-micro-qr.id.md).

## 1. Why Micro QR exists

The smallest QR symbol, version 1, is 21 by 21 modules and needs a 4-module quiet zone: 29 by 29 in all. That is a lot of area for a five-digit code. Micro QR's smallest symbol, M1, is 11 by 11 with a 2-module quiet zone, 15 by 15 in all, about a quarter of the area, and still carries five digits.

The savings come from removing redundancy that small symbols do not need. There is one finder pattern instead of three, so orientation comes from that finder plus the timing patterns. There is one copy of the format information instead of two, and a single Reed-Solomon block. The trade-offs follow from the same choices: fewer error-correction levels (no H at all, and only error *detection* in M1), smaller capacities, and no ECI, structured append, or FNC1.

## 2. Symbol types and capacities

A Micro symbol of version M*k* is `2k + 9` modules square. Each combination of version and error-correction level has a 3-bit **symbol number** that the format information carries in place of QR's separate version and EC fields.

| Symbol number | Type | Size | Data codewords | EC codewords | Data bits | Numeric | Alphanumeric | Byte | Kanji |
|---|---|---|---|---|---|---|---|---|---|
| 0 | M1 | 11 | 3 (last is 4 bits) | 2 (detection only) | 20 | 5 | — | — | — |
| 1 | M2-L | 13 | 5 | 5 | 40 | 10 | 6 | — | — |
| 2 | M2-M | 13 | 4 | 6 | 32 | 8 | 5 | — | — |
| 3 | M3-L | 15 | 11 (last is 4 bits) | 6 | 84 | 23 | 14 | 9 | 6 |
| 4 | M3-M | 15 | 9 (last is 4 bits) | 8 | 68 | 18 | 11 | 7 | 4 |
| 5 | M4-L | 17 | 16 | 8 | 128 | 35 | 21 | 15 | 9 |
| 6 | M4-M | 17 | 14 | 10 | 112 | 30 | 18 | 13 | 8 |
| 7 | M4-Q | 17 | 10 | 14 | 80 | 21 | 13 | 9 | 5 |

The character capacities follow from the data bits and the header widths in section 3; all 25 were checked against zint's limits.

## 3. Modes, indicators, and character counts

Micro QR supports the four data modes of QR, but its mode indicator is shorter, and not every version supports every mode.

| | M1 | M2 | M3 | M4 |
|---|---|---|---|---|
| Mode indicator width | 0 bits | 1 bit | 2 bits | 3 bits |
| Numeric count width | 3 | 4 | 5 | 6 |
| Alphanumeric count width | — | 3 | 4 | 5 |
| Byte count width | — | — | 4 | 5 |
| Kanji count width | — | — | 3 | 4 |
| Terminator | 3 bits | 5 bits | 7 bits | 9 bits |

The indicator values are numeric `0`, alphanumeric `1`, byte `2`, Kanji `3`, written in the indicator's width: M2's single bit can only say numeric or alphanumeric, which is exactly the set M2 supports. M1 has no indicator at all, because numeric is its only mode. The payload encodings themselves are unchanged from QR: ten bits per three digits, eleven bits per two alphanumeric characters, eight bits per byte, thirteen bits per Kanji character (docs 02 and 22).

The terminator is a run of zero bits exactly as long as the mode indicator plus the numeric count field (3, 5, 7, 9 = 0+3, 1+4, 2+5, 3+6). Since numeric's indicator value is `0`, a reader that meets it parses "numeric mode, zero characters" and stops. Like QR's, it is truncated when the capacity runs out first.

## 4. From bits to codewords

After the last segment and the terminator, the bit stream is extended to fill the data capacity in two steps, as in QR:

1. **Padding bits.** Zero bits up to the next codeword boundary. If the stream already ends on a boundary, nothing is added.
2. **Pad codewords.** `11101100` and `00010001`, alternating, through the remaining data codewords.

The difference is the **half codeword**. In M1 and M3 the data capacity (20, 84, or 68 bits) is not a multiple of 8, and the final data codeword is only 4 bits long. Its codeword boundary is therefore 4 bits after the previous one, and when it is a pad position it is filled with `0000` rather than a pad codeword.

Two independent encoders disagree on exactly these rules, which is a useful warning. segno fills all of M1 and M3's padding with zeros, and appends eight zero bits even when the terminator already ends on a codeword boundary. zint follows the rules above. Decoders ignore pad bits, so both produce symbols that scan; but only one of them matches the standard, and an encoder tested against the other would look wrong while being right.

## 5. Error correction

Every Micro type uses a single Reed-Solomon block over the same GF(256) field as QR (primitive polynomial `0x11D`, docs 03 and 04), with as many EC codewords as the table in section 2 lists. With one block there is no interleaving.

For the half codeword, Reed-Solomon sees a full byte: the 4 data bits in the high nibble and zeros in the low nibble. Only the 4 data bits are placed in the symbol. The final bit stream is the data codewords (the half counted as 4 bits) followed by the EC codewords: 36, 80, 132, or 192 bits for M1 to M4, exactly the number of data modules, so Micro symbols never need remainder bits.

M1's two EC codewords are for **error detection only**. In principle two check symbols can correct one error, but then nothing is left to notice a wrong correction, so the standard specifies M1 as detection-only: a decoder rejects an M1 symbol whose syndromes are not all zero instead of correcting it.

## 6. Symbol layout

- **Finder.** One 7 by 7 finder pattern in the top-left corner, with its separator only on the right (column 7) and bottom (row 7), since the other two sides are the symbol's edge: an 8 by 8 block in all.
- **Timing.** Along row 0 and column 0, from index 8 to the edge, dark at even indices. In QR the timing patterns run along row and column 6, between finders; Micro has no second finder to run between, so they move to the outer edge.
- **Format information.** 15 modules in an L just outside the separator: column 8 rows 1 to 8, and row 8 columns 1 to 7 (section 7).
- **No** alignment patterns, version information, or dark module.

Function patterns take `63 + 2n` modules of an `n`-module symbol (64 for the finder block, `2(n − 8)` for timing, 15 for format), which leaves `n² − 2n − 63` data modules: 36, 80, 132, and 192.

## 7. Format information

The 5-bit payload is the 3-bit symbol number followed by the 2-bit mask number. It is protected exactly as in QR, a BCH(15,5) code with generator `x¹⁰ + x⁸ + x⁵ + x⁴ + x² + x + 1` (`0x537`), and then XORed with `0x4445` instead of QR's `0x5412`.

| Symbol | Mask 0 | Mask 1 | Mask 2 | Mask 3 |
|---|---|---|---|---|
| M1 | `4445` | `4172` | `4E2B` | `4B1C` |
| M2-L | `55AE` | `5099` | `5FC0` | `5AF7` |
| M2-M | `6793` | `62A4` | `6DFD` | `68CA` |
| M3-L | `7678` | `734F` | `7C16` | `7921` |
| M3-M | `06DE` | `03E9` | `0CB0` | `0987` |
| M4-L | `1735` | `1202` | `1D5B` | `186C` |
| M4-M | `2508` | `203F` | `2F66` | `2A51` |
| M4-Q | `34E3` | `31D4` | `3E8D` | `3BBA` |

There is a single copy. With bit 0 the least significant:

| Bit | 0–7 | 8–14 |
|---|---|---|
| Module `(row, column)` | `(i + 1, 8)` | `(8, 15 − i)` |

Bits 0 to 7 run down column 8 from row 1 to row 8, and bits 8 to 14 run left along row 8 so that bit 14 lands at `(8, 1)`. This is the same L as QR's first copy (doc 07), with the same corner: bit 7 at `(8, 8)` and bit 8 at `(8, 7)`. It differs in two ways. Its outer ends stop one module short, at row 1 and column 1, because row 0 and column 0 hold Micro's timing patterns. And it has no gaps to step over, because there is no timing pattern on row or column 6. It is easy to get this orientation backwards and still pass every self-made round trip, as this library's QR code did until v0.14.1; with only one copy, a Micro symbol written that way would be unreadable by every other reader.

A decoder has no second copy to fall back on, so it can spend the whole BCH budget of 3 errors on this one. That budget makes one more check essential: the lowest-weight word in the table, M4-L with mask 1 (`1202`), has only 3 one-bits, so a format strip that reads as all light is within the budget and decodes as M4-L. The decoder must require the symbol number to agree with the grid size before trusting it.

## 8. Data placement

Data placement is QR's zigzag: the bit stream fills two-module-wide columns, starting at the bottom-right and moving upward, reversing direction at each edge, and skipping function modules. Micro drops one detail: QR steps over column 6 because its vertical timing pattern lives there, but Micro's column 6 below the finder holds data, and the column pairs simply run from the right edge to column 1, leaving column 0 to the timing pattern. The half codeword needs no special handling at this stage; the stream is placed bit by bit, whatever its codeword boundaries.

## 9. Masking

Micro uses four of QR's eight mask patterns, applied to data modules only. With `i` the row and `j` the column:

| Micro mask | QR mask | Dark module flipped when |
|---|---|---|
| 0 | 1 | `i mod 2 = 0` |
| 1 | 4 | `(⌊i / 2⌋ + ⌊j / 3⌋) mod 2 = 0` |
| 2 | 6 | `((i·j) mod 2 + (i·j) mod 3) mod 2 = 0` |
| 3 | 7 | `((i + j) mod 2 + (i·j) mod 3) mod 2 = 0` |

The selection rule is entirely different from QR's four penalty rules (doc 06). Count the dark modules on the last column (SUM1) and on the last row (SUM2), each excluding the timing module at index 0. The score is `SUM1 × 16 + SUM2` when `SUM1 ≤ SUM2`, and `SUM2 × 16 + SUM1` otherwise, and the **highest** score wins. The two edges it looks at are the ones with no finder or timing pattern, and multiplying the smaller sum by 16 lets the edge with fewer dark modules dominate the score; a reasonable reading is that the rule keeps the symbol's far boundary visible to a reader. On a tie, segno keeps the lowest-numbered mask. Unlike in QR, the format information never touches the scored edges, so it plays no part in the choice.

## 10. Micro versus QR at a glance

| Step | QR Code | Micro QR Code |
|---|---|---|
| Sizes | 21 to 177, step 4 | 11, 13, 15, 17 |
| Quiet zone | 4 modules | 2 modules |
| Finder patterns | 3 | 1 |
| Timing | row 6, column 6 | row 0, column 0 |
| Mode indicator | 4 bits | 0 to 3 bits |
| Terminator | 4 bits | 3, 5, 7, 9 bits |
| Codewords | all 8 bits | last data codeword 4 bits in M1, M3 |
| RS blocks | 1 to 81, interleaved | 1 |
| EC levels | L, M, Q, H | M1 detection only; L, M (M2, M3); L, M, Q (M4) |
| Format information | 2 copies, mask `0x5412` | 1 copy, mask `0x4445`, symbol number |
| Version information | 18 bits for v7+ | none |
| Masks | 8, lowest penalty | 4, highest edge score |
| ECI, structured append, FNC1 | yes | no |

## 11. Worked examples

Each bit stream below was derived by hand from the rules above and matches zint's symbol exactly. Spaces separate fields; `|` marks codeword boundaries in the final line.

**M1, `"12345"`: full capacity, so no terminator.**
Count `101` (5), digits `123` → `0001111011`, digits `45` → `0101101`: 3 + 10 + 7 = 20 bits, exactly M1's capacity.
`10100011 | 11011010 | 1101` (the last is the half codeword).

**M1, `"1"`: padding bits run into the half codeword.**
Count `001`, digit `1` → `0001`, terminator `000`: 10 bits; padding bits to the codeword boundary at 16; then the half codeword is a pad position, `0000`.
`00100010 | 00000000 | 0000`.

**M2-L, `"0123456789"`: the terminator is cut to one bit.**
Indicator `0`, count `1010` (10), `012` → `0000001100`, `345` → `0101011001`, `678` → `1010100110`, `9` → `1001`: 39 bits, one short of the 40-bit capacity, so the 5-bit terminator shrinks to `0`.
`01010000 | 00011000 | 10101100 | 11010100 | 11010010`.

**M4-Q, `"1234"`: the terminator ends exactly on a codeword boundary.**
Indicator `000`, count `000100` (4), `123` → `0001111011`, `4` → `0100`, terminator `000000000`: 32 bits, four whole codewords, so no padding bits; six pad codewords follow.
`00000010 | 00001111 | 01101000 | 00000000 | 11101100 | 00010001 | 11101100 | 00010001 | 11101100 | 00010001`. segno inserts an extra `00000000` before the pad codewords here.

**M3-M, `"HELLO WORLD"`: ending in a data-carrying half codeword.**
Indicator `01`, count `1011` (11), the pairs `HE`, `LL`, `O ` (O and a space), `WO`, `RL` as 11-bit values and the final `D` as 6 bits: 67 bits of the 68-bit capacity, so the terminator is a single `0`.
`01101101 | 10000101 | 10111100 | 01101000 | 10111001 | 01101110 | 00100110 | 10100001 | 1010`.

**M2-L, `"01234567"`, the standard's own example, with error correction.**
Indicator `0`, count `1000` (8), `012`, `345`, `67` → 32 bits, terminator `00000`, padding `000` to the 40-bit capacity.
Data codewords `40 18 AC C3 00`; Reed-Solomon over GF(256) gives the five EC codewords `86 0D 22 AE 30`.

## 12. How these facts were checked

The tables come from the standard as transcribed in segno 1.6.6, and were cross-checked against zint (reached through zxing-cpp 2.3.0) on 69 symbols: 57 byte-identical, the remaining 12 differing only in padding, where segno departs from the standard as section 4 describes. An independent Reed-Solomon implementation reproduces zint's EC codewords for 20 symbols covering all eight types, which pins the half-codeword convention. The character capacities in section 2 match zint's limits in all 25 cases, and the format table in section 7 matches both a BCH computation and segno's precomputed table. The lesson from the format-information bug fixed in v0.14.1 applies here with more force: with a single format copy, the only meaningful check is agreement with implementations that share none of our code.

## 13. Implementation pointers

Planned in MQ3 to MQ5 (see `docs/plan-micro-qr.md`):

- `qrgen/micro.go`: `MicroVersion`, the symbol-type table, and the bit-width tables.
- `qrgen/micro_encode.go`: the Micro cost model for the segmentation DP, `encodeMicroText`, padding with the half codeword, and the bit-exact data-plus-EC stream.
- `qrgen/micro_matrix.go`: the layout, the single-copy format writer, and four-mask selection.
- `qrgen/micro_decode.go`: `decodeMicroMatrix`, reached from `DecodeMatrix` by grid size.
- `qrgen/testdata/thirdparty/micro_symbols.txt`: zint symbols for byte-exact and decoding tests.

## References

- ISO/IEC 18004:2015 — Micro QR Code symbol characteristics, data encoding, error correction, layout, format information, and masking.
- segno 1.6.6, `consts.py` and `encoder.py` — a transcription of the tables, with the padding deviations noted in section 4.
- zint, through zxing-cpp 2.3.0 — the reference encoder for the worked examples and goldens; zxing-cpp's reader as an independent decoder.
- `docs/theory/02-data-encoding.md`, `04-reed-solomon.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `22-kanji-mode.md` — the QR counterparts of each step.
