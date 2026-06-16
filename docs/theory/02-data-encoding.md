# Data Encoding

This document covers how an input string is turned into the bit stream that feeds the Reed–Solomon stage. It corresponds to ISO/IEC 18004:2015 §7.3.

## Encoding modes

ISO/IEC 18004 defines four standard modes; we implement the first three:

| Mode         | Mode indicator | Symbol set                                     |
|--------------|:--------------:|------------------------------------------------|
| Numeric      | `0001`         | `0–9` (10 symbols)                             |
| Alphanumeric | `0010`         | `0–9 A–Z $ % * + - . / : space` (45 symbols)   |
| Byte         | `0100`         | Any 8-bit value (we emit raw UTF-8)            |
| Kanji        | `1000`         | Shift-JIS double-byte (out of scope for v0.1)  |

ECI (mode indicator `0111`) and structured-append (`0011`) are also out of scope for v0.1.

### Numeric encoding

Group digits in threes, then encode each group as a 10-bit unsigned integer. A trailing partial group of two digits is encoded as 7 bits; a single digit as 4 bits.

Worked example for `"01234567"`:

- Groups: `012 | 345 | 67`.
- Encoded: `0000001100 | 0101011001 | 1000011` → 27 bits total.

### Alphanumeric encoding

Each character maps to an integer `0..44` (`A`=10, `B`=11, …, `space`=36, `$`=37, etc.). Process the string in pairs; each pair packs into 11 bits as `value(a) * 45 + value(b)`. A trailing single character is packed into 6 bits.

Worked example for `"HELLO WORLD"` (11 characters): 5 pairs plus 1 single → 5 × 11 + 6 = **61 bits** for the payload.

### Byte encoding

Each byte of the (UTF-8) input becomes 8 bits, written most-significant bit first. We assume UTF-8 and do **not** emit an ECI segment to declare it; modern decoders usually guess UTF-8 correctly, but this is a known non-conformance and is documented in the README.

## Character count indicator

After the 4-bit mode indicator we emit a *character count indicator* whose width depends on the version range and the chosen mode:

| Version | Numeric | Alphanumeric | Byte |
|:-------:|:-------:|:------------:|:----:|
|  1–9    |   10    |      9       |   8  |
| 10–26   |   12    |     11       |  16  |
| 27–40   |   14    |     13       |  16  |

Source: ISO/IEC 18004:2015, Table 3.

## Choosing the smallest version

Given the payload and a requested EC level, the smallest version is the minimum `v` whose data-codeword capacity (multiplied by 8 to get bits) is at least the encoded bit length. The encoded length depends on `v` itself through the character count indicator width, so the search is iterative: start at `v = 1`, compute the encoded length with `v`'s indicator width, and advance while capacity is insufficient.

## Terminator and padding

Once the payload bit stream is built, three steps finish the codeword stream:

1. **Terminator** — append up to 4 zero bits, truncated if fewer remain before the byte-capacity boundary.
2. **Bit padding** — pad with zeros to the next byte boundary.
3. **Pad bytes** — fill the remaining capacity with the alternating bytes `0xEC 0x11` (`11101100`, `00010001`).

## Why these choices

- The mode-specific encoders exploit the limited alphabets: numeric averages 3.33 bits per character, alphanumeric averages 5.5 bits, byte costs 8 bits.
- Character count indicator widths are sized so the maximum count per version range fits exactly into the indicator's bit width — the spec balances payload overhead against capacity span.
- Pad bytes `0xEC 0x11` are specified by the spec because their bit patterns (`11101100` and `00010001`) are roughly balanced in dark/light modules, which reduces the chance of high mask-penalty scores from monotone tails.

## Implementation pointers

- `qrgen/mode.go` will host the per-mode encoders and the mode analyzer.
- `qrgen/version.go` will host the capacity tables and the smallest-version search.
- The v0.1 mode analyzer was a single-segment greedy: pick the most restrictive mode that covers all input characters. As of v0.6 the encoder uses DP-optimal mixed-mode segmentation instead, which splits the payload into a sequence of mode segments minimising the total bit length; the greedy choice is now a provable special case for homogeneous input. See [17-optimal-segmentation.md](17-optimal-segmentation.md).

## References

- ISO/IEC 18004:2015, §7.3 and Tables 2–7.
- Thonky, "Data Encoding" — <https://www.thonky.com/qr-code-tutorial/data-encoding>
- Project Nayuki, *QR Code generator library* — pseudo-code for the mode analyzer.
