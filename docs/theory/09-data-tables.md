# Data Tables & Lookup Values

This document collects the static lookup tables and constants the encoder needs at runtime, alongside the algorithms that derive them where derivation is possible. It is the *data* counterpart to the algorithmic documents 02–08 in this folder.

> Indonesian version: [09-data-tables.id.md](09-data-tables.id.md).

**Sourcing & verification.** Values here are reproduced from ISO/IEC 18004:2015 (the normative source) and cross-checked against Project Nayuki's open-source *QR-Code-generator* reference data. Before relying on any single number in production code, verify it against at least one of those two sources or against the round-trip golden tests from milestone M10.

## 1. Mode indicators

The 4-bit mode indicator is the first thing in the bit stream of every segment:

| Mode               | Indicator |
|--------------------|:---------:|
| Numeric            | `0001`    |
| Alphanumeric       | `0010`    |
| Byte (8-bit)       | `0100`    |
| Kanji              | `1000`    |
| ECI                | `0111`    |
| Structured append  | `0011`    |
| Terminator         | `0000`    |

ECI, Kanji, and Structured append are out of scope for v0.1; their indicators are listed for completeness so that ECI segments can be added later without renumbering.

## 2. Alphanumeric character mapping

In alphanumeric mode each character maps to an integer `0..44`:

| Char | Val | Char | Val | Char | Val | Char | Val | Char | Val |
|:----:|:---:|:----:|:---:|:----:|:---:|:----:|:---:|:----:|:---:|
| `0`  | 0   | `9`  | 9   | `I`  | 18  | `R`  | 27  | (sp) | 36  |
| `1`  | 1   | `A`  | 10  | `J`  | 19  | `S`  | 28  | `$`  | 37  |
| `2`  | 2   | `B`  | 11  | `K`  | 20  | `T`  | 29  | `%`  | 38  |
| `3`  | 3   | `C`  | 12  | `L`  | 21  | `U`  | 30  | `*`  | 39  |
| `4`  | 4   | `D`  | 13  | `M`  | 22  | `V`  | 31  | `+`  | 40  |
| `5`  | 5   | `E`  | 14  | `N`  | 23  | `W`  | 32  | `-`  | 41  |
| `6`  | 6   | `F`  | 15  | `O`  | 24  | `X`  | 33  | `.`  | 42  |
| `7`  | 7   | `G`  | 16  | `P`  | 25  | `Y`  | 34  | `/`  | 43  |
| `8`  | 8   | `H`  | 17  | `Q`  | 26  | `Z`  | 35  | `:`  | 44  |

Note the implicit asymmetries that an analyzer must handle: lowercase letters are *not* part of the alphanumeric alphabet — a string containing any lowercase character must fall back to byte mode.

## 3. Character count indicator widths

For each (mode × version range) the count of characters is encoded in this many bits:

| Version range | Numeric | Alphanumeric | Byte | Kanji |
|:-------------:|:-------:|:------------:|:----:|:-----:|
| 1..9          | 10      | 9            | 8    | 8     |
| 10..26        | 12      | 11           | 16   | 10    |
| 27..40        | 14      | 13           | 16   | 12    |

(Source: ISO/IEC 18004:2015, Table 3.)

## 4. Padding bytes

After the terminator and bit padding to the next byte boundary, the remaining capacity is filled with these two bytes alternating, starting with `0xEC`:

| Hex   | Binary     |
|:-----:|:----------:|
| `0xEC`| `11101100` |
| `0x11`| `00010001` |

The pattern is fixed by the spec; both values are roughly balanced in dark/light bits, which helps keep the eventual mask penalty low when the tail of the message is mostly padding.

## 5. Total codeword count per version

This is the *total* codewords (data + EC) packed into each version's data area, before remainder bits. It is the same regardless of EC level — only the data/EC split changes.

| V  | Total | V  | Total | V  | Total | V  | Total |
|:--:|------:|:--:|------:|:--:|------:|:--:|------:|
| 1  | 26    | 11 | 404   | 21 | 1156  | 31 | 2323  |
| 2  | 44    | 12 | 466   | 22 | 1258  | 32 | 2465  |
| 3  | 70    | 13 | 532   | 23 | 1364  | 33 | 2611  |
| 4  | 100   | 14 | 581   | 24 | 1474  | 34 | 2761  |
| 5  | 134   | 15 | 655   | 25 | 1588  | 35 | 2876  |
| 6  | 172   | 16 | 733   | 26 | 1706  | 36 | 3034  |
| 7  | 196   | 17 | 815   | 27 | 1828  | 37 | 3196  |
| 8  | 242   | 18 | 901   | 28 | 1921  | 38 | 3362  |
| 9  | 292   | 19 | 991   | 29 | 2051  | 39 | 3532  |
| 10 | 346   | 20 | 1085  | 30 | 2185  | 40 | 3706  |

(Derivable from matrix geometry minus functional patterns; cross-check against ISO/IEC 18004:2015 Annex A.)

## 6. Data codewords per (version × EC level)

The *data* codeword count per version per EC level. Multiply by 8 to get bit-capacity for the data stream (before mode indicator, count indicator, terminator, and padding consumption).

| V  | L     | M     | Q     | H     |
|:--:|------:|------:|------:|------:|
| 1  | 19    | 16    | 13    | 9     |
| 2  | 34    | 28    | 22    | 16    |
| 3  | 55    | 44    | 34    | 26    |
| 4  | 80    | 64    | 48    | 36    |
| 5  | 108   | 86    | 62    | 46    |
| 6  | 136   | 108   | 76    | 60    |
| 7  | 156   | 124   | 88    | 66    |
| 8  | 194   | 154   | 110   | 86    |
| 9  | 232   | 182   | 132   | 100   |
| 10 | 274   | 216   | 154   | 122   |
| 11 | 324   | 254   | 180   | 140   |
| 12 | 370   | 290   | 206   | 158   |
| 13 | 428   | 334   | 244   | 180   |
| 14 | 461   | 365   | 261   | 197   |
| 15 | 523   | 415   | 295   | 223   |
| 16 | 589   | 453   | 325   | 253   |
| 17 | 647   | 507   | 367   | 283   |
| 18 | 721   | 563   | 397   | 313   |
| 19 | 795   | 627   | 445   | 341   |
| 20 | 861   | 669   | 485   | 385   |
| 21 | 932   | 714   | 512   | 406   |
| 22 | 1006  | 782   | 568   | 442   |
| 23 | 1094  | 860   | 614   | 464   |
| 24 | 1174  | 914   | 664   | 514   |
| 25 | 1276  | 1000  | 718   | 538   |
| 26 | 1370  | 1062  | 754   | 596   |
| 27 | 1468  | 1128  | 808   | 628   |
| 28 | 1531  | 1193  | 871   | 661   |
| 29 | 1631  | 1267  | 911   | 701   |
| 30 | 1735  | 1373  | 985   | 745   |
| 31 | 1843  | 1455  | 1033  | 793   |
| 32 | 1955  | 1541  | 1115  | 845   |
| 33 | 2071  | 1631  | 1171  | 901   |
| 34 | 2191  | 1725  | 1231  | 961   |
| 35 | 2306  | 1812  | 1286  | 986   |
| 36 | 2434  | 1914  | 1354  | 1054  |
| 37 | 2566  | 1992  | 1426  | 1096  |
| 38 | 2702  | 2102  | 1502  | 1142  |
| 39 | 2812  | 2216  | 1582  | 1222  |
| 40 | 2956  | 2334  | 1666  | 1276  |

The EC codewords for a given (V, level) are `total(V) − data(V, level)`, e.g. for V1-L: 26 − 19 = 7 EC codewords.

## 7. EC block structure per (version × EC level)

The data stream is split into one or two *groups* of blocks. Each row below gives `(ec_per_block, num_blocks_g1, data_per_block_g1, num_blocks_g2, data_per_block_g2)`. When `num_blocks_g2 = 0` there is only one group. The total EC codewords equal `(num_blocks_g1 + num_blocks_g2) * ec_per_block`.

Notation: `ec[g1×d1][g2×d2]`. Example: `18[2×15][2×16]` means 18 EC codewords per block, 2 blocks of 15 data codewords, 2 blocks of 16 data codewords.

| V  | L                  | M                  | Q                   | H                   |
|:--:|:------------------:|:------------------:|:-------------------:|:-------------------:|
| 1  | `7[1×19]`          | `10[1×16]`         | `13[1×13]`          | `17[1×9]`           |
| 2  | `10[1×34]`         | `16[1×28]`         | `22[1×22]`          | `28[1×16]`          |
| 3  | `15[1×55]`         | `26[1×44]`         | `18[2×17]`          | `22[2×13]`          |
| 4  | `20[1×80]`         | `18[2×32]`         | `26[2×24]`          | `16[4×9]`           |
| 5  | `26[1×108]`        | `24[2×43]`         | `18[2×15][2×16]`    | `22[2×11][2×12]`    |
| 6  | `18[2×68]`         | `16[4×27]`         | `24[4×19]`          | `28[4×15]`          |
| 7  | `20[2×78]`         | `18[4×31]`         | `18[2×14][4×15]`    | `26[4×13][1×14]`    |
| 8  | `24[2×97]`         | `22[2×38][2×39]`   | `22[4×18][2×19]`    | `26[4×14][2×15]`    |
| 9  | `30[2×116]`        | `22[3×36][2×37]`   | `20[4×16][4×17]`    | `24[4×12][4×13]`    |
| 10 | `18[2×68][2×69]`   | `26[4×43][1×44]`   | `24[6×19][2×20]`    | `28[6×15][2×16]`    |
| 11 | `20[4×81]`         | `30[1×50][4×51]`   | `28[4×22][4×23]`    | `24[3×12][8×13]`    |
| 12 | `24[2×92][2×93]`   | `22[6×36][2×37]`   | `26[4×20][6×21]`    | `28[7×14][4×15]`    |
| 13 | `26[4×107]`        | `22[8×37][1×38]`   | `24[8×20][4×21]`    | `22[12×11][4×12]`   |
| 14 | `30[3×115][1×116]` | `24[4×40][5×41]`   | `20[11×16][5×17]`   | `24[11×12][5×13]`   |
| 15 | `22[5×87][1×88]`   | `24[5×41][5×42]`   | `30[5×24][7×25]`    | `24[11×12][7×13]`   |
| 16 | `24[5×98][1×99]`   | `28[7×45][3×46]`   | `24[15×19][2×20]`   | `30[3×15][13×16]`   |
| 17 | `28[1×107][5×108]` | `28[10×46][1×47]`  | `28[1×22][15×23]`   | `28[2×14][17×15]`   |
| 18 | `30[5×120][1×121]` | `26[9×43][4×44]`   | `28[17×22][1×23]`   | `28[2×14][19×15]`   |
| 19 | `28[3×113][4×114]` | `26[3×44][11×45]`  | `26[17×21][4×22]`   | `26[9×13][16×14]`   |
| 20 | `28[3×107][5×108]` | `26[3×41][13×42]`  | `30[15×24][5×25]`   | `28[15×15][10×16]`  |
| 21 | `28[4×116][4×117]` | `26[17×42]`        | `28[17×22][6×23]`   | `30[19×16][6×17]`   |
| 22 | `28[2×111][7×112]` | `28[17×46]`        | `30[7×24][16×25]`   | `24[34×13]`         |
| 23 | `30[4×121][5×122]` | `28[4×47][14×48]`  | `30[11×24][14×25]`  | `30[16×15][14×16]`  |
| 24 | `30[6×117][4×118]` | `28[6×45][14×46]`  | `30[11×24][16×25]`  | `30[30×16][2×17]`   |
| 25 | `26[8×106][4×107]` | `28[8×47][13×48]`  | `30[7×24][22×25]`   | `30[22×15][13×16]`  |
| 26 | `28[10×114][2×115]`| `28[19×46][4×47]`  | `28[28×22][6×23]`   | `30[33×16][4×17]`   |
| 27 | `30[8×122][4×123]` | `28[22×45][3×46]`  | `30[8×23][26×24]`   | `30[12×15][28×16]`  |
| 28 | `30[3×117][10×118]`| `28[3×45][23×46]`  | `30[4×24][31×25]`   | `30[11×15][31×16]`  |
| 29 | `30[7×116][7×117]` | `28[21×45][7×46]`  | `30[1×23][37×24]`   | `30[19×15][26×16]`  |
| 30 | `30[5×115][10×116]`| `28[19×47][10×48]` | `30[15×24][25×25]`  | `30[23×15][25×16]`  |
| 31 | `30[13×115][3×116]`| `28[2×46][29×47]`  | `30[42×24][1×25]`   | `30[23×15][28×16]`  |
| 32 | `30[17×115]`       | `28[10×46][23×47]` | `30[10×24][35×25]`  | `30[19×15][35×16]`  |
| 33 | `30[17×115][1×116]`| `28[14×46][21×47]` | `30[29×24][19×25]`  | `30[11×15][46×16]`  |
| 34 | `30[13×115][6×116]`| `28[14×46][23×47]` | `30[44×24][7×25]`   | `30[59×16][1×17]`   |
| 35 | `30[12×121][7×122]`| `28[12×47][26×48]` | `30[39×24][14×25]`  | `30[22×15][41×16]`  |
| 36 | `30[6×121][14×122]`| `28[6×47][34×48]`  | `30[46×24][10×25]`  | `30[2×15][64×16]`   |
| 37 | `30[17×122][4×123]`| `28[29×46][14×47]` | `30[49×24][10×25]`  | `30[24×15][46×16]`  |
| 38 | `30[4×122][18×123]`| `28[13×46][32×47]` | `30[48×24][14×25]`  | `30[42×15][32×16]`  |
| 39 | `30[20×117][4×118]`| `28[40×47][7×48]`  | `30[43×24][22×25]`  | `30[10×15][67×16]`  |
| 40 | `30[19×118][6×119]`| `28[18×47][31×48]` | `30[34×24][34×25]`  | `30[20×15][61×16]`  |

(Source: ISO/IEC 18004:2015 Table 9. Always verify before shipping.)

## 8. Remainder bits per version

After the codeword stream is written into the matrix, some versions need extra zero bits appended so the data-area cell count is filled exactly.

| Version range | Remainder bits |
|:-------------:|:--------------:|
| 1             | 0              |
| 2..6          | 7              |
| 7..13         | 0              |
| 14..20        | 3              |
| 21..27        | 4              |
| 28..34        | 3              |
| 35..40        | 0              |

(Source: ISO/IEC 18004:2015 Table 1.)

## 9. Alignment pattern positions

For versions 2..40, alignment patterns sit at every pairing of the coordinates listed below, *except* positions that overlap with the three finder patterns at `(6, 6)`, `(6, n-7)`, and `(n-7, 6)` (where `n = 21 + 4·(v-1)` is the matrix side).

Version 1 has *no* alignment patterns.

| V  | Coordinates                                 |
|:--:|:--------------------------------------------|
| 2  | 6, 18                                       |
| 3  | 6, 22                                       |
| 4  | 6, 26                                       |
| 5  | 6, 30                                       |
| 6  | 6, 34                                       |
| 7  | 6, 22, 38                                   |
| 8  | 6, 24, 42                                   |
| 9  | 6, 26, 46                                   |
| 10 | 6, 28, 50                                   |
| 11 | 6, 30, 54                                   |
| 12 | 6, 32, 58                                   |
| 13 | 6, 34, 62                                   |
| 14 | 6, 26, 46, 66                               |
| 15 | 6, 26, 48, 70                               |
| 16 | 6, 26, 50, 74                               |
| 17 | 6, 30, 54, 78                               |
| 18 | 6, 30, 56, 82                               |
| 19 | 6, 30, 58, 86                               |
| 20 | 6, 34, 62, 90                               |
| 21 | 6, 28, 50, 72, 94                           |
| 22 | 6, 26, 50, 74, 98                           |
| 23 | 6, 30, 54, 78, 102                          |
| 24 | 6, 28, 54, 80, 106                          |
| 25 | 6, 32, 58, 84, 110                          |
| 26 | 6, 30, 58, 86, 114                          |
| 27 | 6, 34, 62, 90, 118                          |
| 28 | 6, 26, 50, 74, 98, 122                      |
| 29 | 6, 30, 54, 78, 102, 126                     |
| 30 | 6, 26, 52, 78, 104, 130                     |
| 31 | 6, 30, 56, 82, 108, 134                     |
| 32 | 6, 34, 60, 86, 112, 138                     |
| 33 | 6, 30, 58, 86, 114, 142                     |
| 34 | 6, 34, 62, 90, 118, 146                     |
| 35 | 6, 30, 54, 78, 102, 126, 150                |
| 36 | 6, 24, 50, 76, 102, 128, 154                |
| 37 | 6, 28, 54, 80, 106, 132, 158                |
| 38 | 6, 32, 58, 84, 110, 136, 162                |
| 39 | 6, 26, 54, 82, 110, 138, 166                |
| 40 | 6, 30, 58, 86, 114, 142, 170                |

(Source: ISO/IEC 18004:2015 Annex E.)

## 10. Reed–Solomon generator polynomials

For an EC block length of `n` codewords, the generator polynomial is:

    g(x) = ∏_{i=0}^{n-1} (x − αⁱ)

multiplied out over GF(256). We compute this once per `n` in code rather than hardcoding the coefficients:

```text
function genPoly(n):
    g = [1]                  # represents the polynomial "1"
    for i in 0..n-1:
        # multiply g by (x + α^i)  (subtraction equals addition in GF(2^8))
        g = polyMul(g, [1, exp[i]])
    return g                 # length n+1
```

The 13 EC block lengths that actually appear in the QR EC tables are `n ∈ { 7, 10, 13, 15, 16, 17, 18, 20, 22, 24, 26, 28, 30 }`. Coefficients are listed as α-exponents (so the byte value is `exp[exponent]`), highest degree first; the leading coefficient is always `α⁰ = 1`.

```text
n =  7: [  0,  87, 229, 146, 149, 238, 102,  21 ]
n = 10: [  0, 251,  67,  46,  61, 118,  70,  64,  94,  32,  45 ]
n = 13: [  0,  74, 152, 176, 100,  86, 100, 106, 104, 130, 218, 206, 140,  78 ]
n = 15: [  0,   8, 183,  61,  91, 202,  37,  51,  58,  58, 237, 140, 124,   5,  99, 105 ]
n = 16: [  0, 120, 104, 107, 109, 102, 161,  76,   3,  91, 191, 147, 169, 182, 194, 225, 120 ]
n = 17: [  0,  43, 139, 206,  78,  43, 239, 123, 206, 214, 147,  24,  99, 150,  39, 243, 163, 136 ]
n = 18: [  0, 215, 234, 158,  94, 184,  97, 118, 170,  79, 187, 152, 148, 252, 179,   5,  98,  96, 153 ]
n = 20: [  0,  17,  60,  79,  50,  61, 163,  26, 187, 202, 180, 221, 225,  83, 239, 156, 164, 212, 212, 188, 190 ]
n = 22: [  0, 210, 171, 247, 242,  93, 230,  14, 109, 221,  53, 200,  74,   8, 172,  98,  80, 219, 134, 160, 105, 165, 231 ]
n = 24: [  0, 229, 121, 135,  48, 211, 117, 251, 126, 159, 180, 169, 152, 192, 226, 228, 218, 111,   0, 117, 232,  87,  96, 227,  21 ]
n = 26: [  0, 173, 125, 158,   2, 103, 182, 118,  17, 145, 201, 111,  28, 165,  53, 161,  21, 245, 142,  13, 102,  48, 227, 153, 145, 218,  70 ]
n = 28: [  0, 168, 223, 200, 104, 224, 234, 108, 180, 110, 190, 195, 147, 205,  27, 232, 201,  21,  43, 245,  87,  42, 195, 212, 119, 242,  37,   9, 123 ]
n = 30: [  0,  41, 173, 145, 152, 216,  31, 179, 182,  50,  48, 110,  86, 239,  96, 222, 125,  42, 173, 226, 193, 224, 130, 156,  37, 251, 216, 238,  40, 192, 180 ]
```

The polynomial `g(x)` for `n = 7` reads as `α⁰x⁷ + α⁸⁷x⁶ + α²²⁹x⁵ + α¹⁴⁶x⁴ + α¹⁴⁹x³ + α²³⁸x² + α¹⁰²x + α²¹`. Compute these at build time via `genPoly(n)` and assert against the table above; do not hand-copy at runtime.

(Source: Project Nayuki, *QR Code generator library*, cross-checked against ISO/IEC 18004:2015 Annex A.)

## 11. Format information codewords

The 15-bit format codeword encoded as `BCH(15, 5)` then XOR-masked with `0x5412`. Indexed by `(EC level, mask 0..7)` = 32 entries. The full enumeration is precomputed once at build time:

```text
function formatInfo(ecBits, mask):
    payload = (ecBits << 3) | mask         # 5 bits total
    data = payload << 10
    rem = data
    for i in 14..10:                       # reduce by g(x) = 0b10100110111 = 0x537
        if (rem >> i) & 1:
            rem ^= 0x537 << (i - 10)
    code = (data | (rem & 0x3FF)) ^ 0x5412
    return code                            # 15 bits
```

EC level bits are `L=01`, `M=00`, `Q=11`, `H=10` (a deliberately non-monotonic mapping from the spec). The full 4×8 precomputed table, indexed `[ecLevel][mask]`:

| EC | mask 0 | mask 1 | mask 2 | mask 3 | mask 4 | mask 5 | mask 6 | mask 7 |
|:--:|:------:|:------:|:------:|:------:|:------:|:------:|:------:|:------:|
| L  | 0x77C4 | 0x72F3 | 0x7DAA | 0x789D | 0x662F | 0x6318 | 0x6C41 | 0x6976 |
| M  | 0x5412 | 0x5125 | 0x5E7C | 0x5B4B | 0x45F9 | 0x40CE | 0x4F97 | 0x4AA0 |
| Q  | 0x355F | 0x3068 | 0x3F31 | 0x3A06 | 0x24B4 | 0x2183 | 0x2EDA | 0x2BED |
| H  | 0x1689 | 0x13BE | 0x1CE7 | 0x19D0 | 0x0762 | 0x0255 | 0x0D0C | 0x083B |

Verify by running the algorithm above for every `(ecBits, mask)` pair and matching against this table; the table itself was reproduced from ISO/IEC 18004:2015 Annex C.

## 12. Version information codewords

The 18-bit version codeword encoded as `BCH(18, 6)`. Indexed by version 7..40 → 34 entries. Like format info, this is precomputed once at build time:

```text
function versionInfo(version):
    data = version << 12
    rem = data
    for i in 17..12:                       # reduce by g(x) = 0b1111100100101 = 0x1F25
        if (rem >> i) & 1:
            rem ^= 0x1F25 << (i - 12)
    return data | (rem & 0xFFF)            # 18 bits
```

No XOR mask is applied. Full table for versions 7..40:

| V  | Codeword  | V  | Codeword  | V  | Codeword  | V  | Codeword  |
|:--:|:---------:|:--:|:---------:|:--:|:---------:|:--:|:---------:|
|  7 | 0x07C94   | 16 | 0x10B78   | 25 | 0x191E1   | 34 | 0x228BA   |
|  8 | 0x085BC   | 17 | 0x1145D   | 26 | 0x1AFAB   | 35 | 0x2379F   |
|  9 | 0x09A99   | 18 | 0x12A17   | 27 | 0x1B08E   | 36 | 0x24B0B   |
| 10 | 0x0A4D3   | 19 | 0x13532   | 28 | 0x1CC1A   | 37 | 0x2542E   |
| 11 | 0x0BBF6   | 20 | 0x149A6   | 29 | 0x1D33F   | 38 | 0x26A64   |
| 12 | 0x0C762   | 21 | 0x15683   | 30 | 0x1ED75   | 39 | 0x27541   |
| 13 | 0x0D847   | 22 | 0x168C9   | 31 | 0x1F250   | 40 | 0x28C69   |
| 14 | 0x0E60D   | 23 | 0x177EC   | 32 | 0x209D5   |    |           |
| 15 | 0x0F928   | 24 | 0x18EC4   | 33 | 0x216F0   |    |           |

Verify by running `versionInfo(v)` for `v ∈ 7..40` and matching against this table; the table itself was reproduced from ISO/IEC 18004:2015 Annex D.

## 13. Verification checklist

Before relying on these tables in production code:

- [ ] Cross-check §6 (data codewords) against ISO/IEC 18004:2015 Annex A.
- [ ] Cross-check §7 (EC block structure) against ISO/IEC 18004:2015 Table 9.
- [ ] Cross-check §9 (alignment positions) against ISO/IEC 18004:2015 Annex E.
- [ ] Compute §10 generator polynomials at runtime and assert match for `n=7` and `n=10` against published α-exponent sequences.
- [ ] Compute §11 and §12 BCH codes at runtime and assert at least the first three entries match ISO Annex C / Annex D.
- [ ] Add a round-trip golden test: encode a known string with a third-party reference (Nayuki) and assert the matrix matches our output bit-for-bit.

When in doubt, prefer the computed value over the tabulated one. Tables exist for speed, not for authority — the *algorithms* in §10–§12 are authoritative.

## References

- ISO/IEC 18004:2015, Tables 1, 3, 7, 9; Annexes A, C, D, E.
- Project Nayuki, *QR Code generator library* — `EccBlock` and capacity arrays in the reference Java/Python/Rust sources.
- Thonky, "Error Correction Coding" and "Format and Version String Tables" — <https://www.thonky.com/qr-code-tutorial/>
