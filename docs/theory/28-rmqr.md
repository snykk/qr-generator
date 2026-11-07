# rMQR Code

rMQR, the Rectangular Micro QR Code of ISO/IEC 23941:2022, is the third member of the QR family. It keeps QR's data modes, Reed-Solomon code, and zigzag placement and Micro QR's small quiet zone, but drops the square: an rMQR symbol is 7 to 17 modules tall and 27 to 139 wide. This document walks through each step as doc 26 did for Micro QR, says how rMQR differs from both relatives, and closes with worked examples checked against zint.

> Indonesian version: [28-rmqr.id.md](28-rmqr.id.md).

## 1. Why rMQR exists

QR and Micro QR are square, so in a narrow space a square symbol must shrink until it is no taller than the space, and its capacity shrinks with it. rMQR keeps the height small and grows the width instead. R7x139 holds 44 data codewords at level M in a strip 7 modules tall; the largest Micro QR symbol holds 16 in a 17 by 17 square. A long thin symbol needs a way to find both of its ends, so rMQR adds a finder sub-pattern at the far corner and small corner patterns at the other two, and it spreads timing along every edge.

## 2. Versions and capacities

There are 32 versions, named R*h*x*w* after their height and width in modules: heights 7, 9, 11, 13, 15, and 17, widths 43, 59, 77, 99, and 139, and also width 27 for heights 11 and 13. They are numbered 1 to 32 in the order below, and the format information carries the number minus one. Each version has two error-correction levels, M and H; there is no L or Q.

| No. | Version | Codewords | M: data / blocks | H: data / blocks | Remainder bits | Count widths N / A / B / K |
|---|---|---|---|---|---|---|
| 1 | R7x43 | 13 | 6 / 1 | 3 / 1 | 0 | 4 / 3 / 3 / 2 |
| 2 | R7x59 | 21 | 12 / 1 | 7 / 1 | 3 | 5 / 5 / 4 / 3 |
| 3 | R7x77 | 32 | 20 / 1 | 10 / 1 | 5 | 6 / 5 / 5 / 4 |
| 4 | R7x99 | 44 | 28 / 1 | 14 / 1 | 6 | 7 / 6 / 5 / 5 |
| 5 | R7x139 | 68 | 44 / 1 | 24 / 2 | 1 | 7 / 6 / 6 / 5 |
| 6 | R9x43 | 21 | 12 / 1 | 7 / 1 | 2 | 5 / 5 / 4 / 3 |
| 7 | R9x59 | 33 | 21 / 1 | 11 / 1 | 3 | 6 / 5 / 5 / 4 |
| 8 | R9x77 | 49 | 31 / 1 | 17 / 2 | 1 | 7 / 6 / 5 / 5 |
| 9 | R9x99 | 66 | 42 / 1 | 22 / 2 | 4 | 7 / 6 / 6 / 5 |
| 10 | R9x139 | 99 | 63 / 2 | 33 / 3 | 5 | 8 / 7 / 6 / 6 |
| 11 | R11x27 | 15 | 7 / 1 | 5 / 1 | 2 | 4 / 4 / 3 / 2 |
| 12 | R11x43 | 31 | 19 / 1 | 11 / 1 | 1 | 6 / 5 / 5 / 4 |
| 13 | R11x59 | 47 | 31 / 1 | 15 / 2 | 0 | 7 / 6 / 5 / 5 |
| 14 | R11x77 | 67 | 43 / 1 | 23 / 2 | 2 | 7 / 6 / 6 / 5 |
| 15 | R11x99 | 89 | 57 / 2 | 29 / 2 | 7 | 8 / 7 / 6 / 6 |
| 16 | R11x139 | 132 | 84 / 2 | 42 / 3 | 6 | 8 / 7 / 7 / 6 |
| 17 | R13x27 | 21 | 12 / 1 | 7 / 1 | 4 | 5 / 5 / 4 / 3 |
| 18 | R13x43 | 41 | 27 / 1 | 13 / 1 | 1 | 6 / 6 / 5 / 5 |
| 19 | R13x59 | 60 | 38 / 1 | 20 / 2 | 6 | 7 / 6 / 6 / 5 |
| 20 | R13x77 | 85 | 53 / 2 | 29 / 2 | 4 | 7 / 7 / 6 / 6 |
| 21 | R13x99 | 113 | 73 / 2 | 35 / 3 | 3 | 8 / 7 / 7 / 6 |
| 22 | R13x139 | 166 | 106 / 3 | 54 / 4 | 0 | 8 / 8 / 7 / 7 |
| 23 | R15x43 | 51 | 33 / 1 | 15 / 2 | 1 | 7 / 6 / 6 / 5 |
| 24 | R15x59 | 74 | 48 / 1 | 26 / 2 | 4 | 7 / 7 / 6 / 5 |
| 25 | R15x77 | 103 | 67 / 2 | 31 / 3 | 6 | 8 / 7 / 7 / 6 |
| 26 | R15x99 | 136 | 88 / 2 | 48 / 4 | 7 | 8 / 7 / 7 / 6 |
| 27 | R15x139 | 199 | 127 / 3 | 69 / 5 | 2 | 9 / 8 / 7 / 7 |
| 28 | R17x43 | 61 | 39 / 1 | 21 / 2 | 1 | 7 / 6 / 6 / 5 |
| 29 | R17x59 | 88 | 56 / 2 | 28 / 2 | 2 | 8 / 7 / 6 / 6 |
| 30 | R17x77 | 122 | 78 / 2 | 38 / 3 | 0 | 8 / 7 / 7 / 6 |
| 31 | R17x99 | 160 | 100 / 3 | 56 / 4 | 3 | 8 / 8 / 7 / 6 |
| 32 | R17x139 | 232 | 152 / 4 | 76 / 6 | 4 | 9 / 8 / 8 / 7 |

Codewords counts every codeword in the symbol; the error-correction codewords are the codewords minus the data codewords, shared equally among the blocks. The remainder bits are the data modules left over after the last codeword (section 8), and the count widths are the character-count field widths for numeric, alphanumeric, byte, and Kanji (section 3).

## 3. Modes, indicators, and character counts

Every rMQR version uses a 3-bit mode indicator:

| Indicator | Mode |
|---|---|
| `000` | terminator |
| `001` | numeric |
| `010` | alphanumeric |
| `011` | byte |
| `100` | Kanji |
| `101`, `110` | FNC1 in first and second position |
| `111` | ECI |

The payload encodings are QR's own: ten bits per three digits, eleven bits per two alphanumeric characters, eight bits per byte, thirteen bits per Kanji character (docs 02 and 22). An ECI header is the indicator `111` followed by the same 8, 16, or 24-bit designator as in QR (doc 20).

The character-count widths are given per version in the table of section 2. As in Micro QR and unlike QR, they change with every version rather than with groups of versions, and they follow the capacity: R7x43 counts digits in 4 bits, while R15x139 and R17x139 use 9.

## 4. From bits to codewords

After the last segment comes a 3-bit terminator, truncated if the capacity runs out first. Zero bits then fill to the next codeword boundary, and the pad codewords `11101100` and `00010001` alternate through the remaining data codewords, exactly as in QR. Every codeword is a full 8 bits; rMQR has nothing like the 4-bit final codeword of Micro QR's M1 and M3.

## 5. Error correction

The Reed-Solomon code is QR's: GF(256) with the primitive polynomial `0x11D` (docs 03 and 04). Small symbols use one block; the largest, R17x139 at level H, uses six. When the data codewords do not divide evenly, the first blocks are one codeword shorter, and the blocks are interleaved as in QR: the first data codeword of every block, then the second, and so on, the longer blocks contributing their last codeword at the end, followed by the error-correction codewords in the same column order. R9x77 at level H, for instance, splits its 17 data codewords into blocks of 8 and 9 and sends D1, D9, D2, D10, …, D8, D16, D17.

## 6. Symbol layout

- **Finder pattern.** QR's 7 by 7 finder in the top-left corner, with a light separator in column 7 and, from height 9 on, in row 7. In R7 symbols the finder fills the whole height and there is no separator below it.
- **Finder sub-pattern.** A 5 by 5 pattern in the bottom-right corner: a dark ring, a light ring, and a single dark centre module. It marks the far end of the symbol.
- **Corner finder patterns.** At the top-right, the modules (0, w - 2), (0, w - 1), and (1, w - 1) are dark around a light (1, w - 2). At the bottom-left, the modules (h - 1, 0), (h - 1, 1), and (h - 1, 2) are dark, and from height 11 on, so is (h - 2, 0), next to a light (h - 2, 1).
- **Timing patterns.** Row 0 and row h - 1 are dark at even columns, and columns 0 and w - 1 dark at even rows, wherever no other pattern sits.
- **Alignment patterns.** A 3 by 3 pattern, a dark ring around a light centre, at the top edge (rows 0 to 2) and the bottom edge (rows h - 3 to h - 1) of each alignment column, joined by a vertical timing pattern dark at even rows. The columns depend on the width only: 21 for width 43; 19 and 39 for 59; 25 and 51 for 77; 23, 49, and 75 for 99; 27, 55, 83, and 111 for 139; none for 27.
- **Format information.** Two 18-module areas, beside the finder and beside the sub-pattern (section 7).

R11x43, with F the finder, s its separator, S the sub-pattern, C the corner patterns, T timing, A alignment, f format information, and `.` data:

```
FFFFFFFsTTTTTTTTTTTTAAATTTTTTTTTTTTTTTTTTCC
FFFFFFFsffff........AAA..................CC
FFFFFFFsffff........AAA...................T
FFFFFFFsffff.........T....................T
FFFFFFFsfff..........T....................T
FFFFFFFsfff..........T.............ffffff.T
FFFFFFFs.............T.............fffSSSSS
ssssssss.............T.............fffSSSSS
T...................AAA............fffSSSSS
CC..................AAA............fffSSSSS
CCCTTTTTTTTTTTTTTTTTAAATTTTTTTTTTTTTTTSSSSS
```

## 7. Format information

The format information is 6 data bits: the level in the top bit (0 for M, 1 for H) above the 5-bit version number minus one. They are protected by BCH(18,6) with the generator `x¹² + x¹¹ + x¹⁰ + x⁹ + x⁸ + x⁵ + x² + 1` (`0x1F25`), the same code that protects QR's version information (doc 07); the 12 check bits follow the 6 data bits. The code's minimum distance is 8, so a copy is decodable through up to three bit errors.

There are two copies, each XORed with its own mask: `0x1FAB2` for the copy beside the finder and `0x20A7B` for the copy beside the sub-pattern. With bit 0 the least significant and `i` the bit index:

| Copy | Bits 0–14 at `(row, column)` | Bits 15, 16, 17 at |
|---|---|---|
| Beside the finder | `(1 + i mod 5, 8 + ⌊i / 5⌋)` | `(1, 11)`, `(2, 11)`, `(3, 11)` |
| Beside the sub-pattern | `(h − 6 + i mod 5, w − 8 + ⌊i / 5⌋)` | `(h − 6, w − 5)`, `(h − 6, w − 4)`, `(h − 6, w − 3)` |

Both copies are filled column by column, five bits per column from the top, and the last three bits run in a line of their own: down column 11 beside the finder, along row h - 6 beside the sub-pattern. Compared with its relatives, rMQR's format information carries the version, which QR keeps in separate version information and Micro QR folds into a symbol number, and it carries no mask at all (section 9).

## 8. Data placement

Placement is QR's zigzag: two-module-wide columns filled from the bottom, upward first, reversing direction at each edge and skipping function modules. rMQR changes where it starts. The rightmost column holds only timing, corner, and sub-pattern modules, so the first column pair is w - 2 and w - 3, and the pairs continue left to columns 1 and 0, with no column skipped. Starting from the rightmost column, as QR does, would pair every column with the wrong neighbour and still round-trip through a decoder that made the same mistake.

After the last codeword, the remaining 0 to 7 data modules are remainder bits, set to zero before masking.

## 9. Masking

rMQR has a single mask, QR's mask 4: with `i` the row and `j` the column, a data or remainder module is flipped when `(⌊i / 2⌋ + ⌊j / 3⌋) mod 2 = 0`. There is no mask selection and no mask number in the format information.

## 10. rMQR, QR, and Micro QR at a glance

| Step | QR Code | Micro QR Code | rMQR Code |
|---|---|---|---|
| Shape | square, 21 to 177 | square, 11 to 17 | rectangle, 7 to 17 by 27 to 139 |
| Versions | 40 | 4 | 32 |
| Quiet zone | 4 modules | 2 modules | 2 modules |
| Finding patterns | 3 finders | 1 finder | 1 finder, 1 sub-pattern, 2 corner patterns |
| Timing | row 6, column 6 | row 0, column 0 | all four edges, and down each alignment column |
| Alignment | grid, from version 2 | none | top and bottom edges, at fixed columns |
| Mode indicator | 4 bits | 0 to 3 bits | 3 bits |
| Count widths | by version group | by version | by version |
| Terminator | 4 bits | 3, 5, 7, 9 bits | 3 bits |
| EC levels | L, M, Q, H | detection (M1); L, M; L, M, Q (M4) | M, H |
| RS blocks | 1 to 81 | 1 | 1 to 6 |
| Format information | 15 bits, 2 copies: level, mask | 15 bits, 1 copy: symbol number, mask | 18 bits, 2 copies: level, version |
| Version information | 18 bits from version 7 | none | none |
| Masks | 8, lowest penalty | 4, highest edge score | 1, fixed |
| ECI | yes | no | yes |
| Structured append | yes | no | no |

## 11. Worked examples

Each stream below was derived by hand from the rules above and matches zint's symbol exactly, error-correction codewords included. Spaces separate fields.

**R7x43 at level H, `"123"`: the smallest symbol.**
Indicator `001`, count `0011` (3), `123` → `0001111011`, terminator `000`: 20 bits, then four padding bits to the 24-bit capacity, exactly three codewords.
Data `26 3D 80`; error correction `A9 7F E5 56 03 84 46 AC 4C DF`. No remainder bits.

**R11x27 at level M, `"HELLO"`: pad codewords and remainder bits.**
Indicator `010`, count `0101` (5), `HE` → `01100001011`, `LL` → `01111000110`, `O` → `011000`, terminator `000`: 38 bits, padding `00` to 40, then two pad codewords to fill the seven data codewords.
Data `4A C2 DE 33 00 EC 11`; error correction `F8 99 00 E4 B0 CB 71 17`; two remainder bits.

**R11x27 at level M, `"12345678901234"`: the terminator cut short.**
Indicator `001`, count `1110` (14), `123`, `456`, `789`, `012` → four 10-bit groups, `34` → `0100010`: 54 bits of the 56-bit capacity, so the terminator shrinks to `00`.
Data `3C 3D B9 18 A8 18 88`; error correction `43 4E CF F5 DB F2 66 E0`.

## 12. How these facts were checked

ISO/IEC 23941:2022 was not consulted directly. The facts come from three implementations that share no code, and each was checked against the others rather than trusted. zint (through zint-bindings 1.2.2) and zxing-cpp, an encoder and a reader written independently, agree on every size, codeword count, block structure, count width, and alignment column. The third, rmqrcode 0.3.2, agrees with them except in two block definitions, R13x27 at M and R17x43 at M, where it is wrong. The 64 format words of each copy were recomputed from the BCH generator and the two masks and match both zint's and zxing-cpp's tables.

The layout of section 6 was built as a model and compared with zint's symbols for all 32 versions at both levels: every function module and both format copies agree. Reading each of those 64 symbols back through the model, with section 8's placement and section 9's mask, recovers data codewords identical to an encoding by the rules of sections 3 and 4, and error-correction codewords identical to an independent Reed-Solomon implementation. As with Micro QR, agreement with implementations that share none of our code is the only check that means anything; a round trip through our own encoder and decoder would pass with the placement started one column off.

## 13. Implementation pointers

Planned in RQ3 to RQ5 (see `docs/plan-rmqr.md`):

- `qrgen/rmqr.go`: `RMQRVersion` and the tables of section 2.
- `qrgen/rmqr_encode.go`: the cost model for the segmentation DP, `encodeRMQRText`, padding, and block interleaving.
- `qrgen/rmqr_matrix.go`: the layout, the two format copies, placement, and the mask.
- `qrgen/rmqr_decode.go`: `decodeRMQRMatrix`, reached from `DecodeMatrix` by the grid's shape.
- `qrgen/testdata/thirdparty/rmqr_symbols.txt`: zint symbols for byte-exact and decoding tests.

## References

- ISO/IEC 23941:2022 — rMQR Code bar code symbology specification; its Table 3 (count widths), Table 8 (error correction), Table C.1 (format information), and Table D.1 (alignment columns) as cited in zint's and zxing-cpp's source.
- zint, backend/qr.c and qr.h (BSD-3-Clause), through zint-bindings 1.2.2 — the reference encoder for the worked examples and goldens.
- zxing-cpp, core/src/qrcode (QRVersion, QRFormatInformation, QRBitMatrixParser, QRCodecMode) — a second transcription of the tables, and an independent reader.
- rmqrcode 0.3.2 — a third transcription, with the two block deviations of section 12.
- `docs/theory/02-data-encoding.md`, `04-reed-solomon.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `20-eci-segments.md`, `26-micro-qr.md` — the QR and Micro QR counterparts of each step.
