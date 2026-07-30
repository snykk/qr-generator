# Kanji Mode

The v0.11 encoder and decoder add **Kanji mode**, the last of the four data modes ISO/IEC 18004 defines and the one that encodes Japanese text compactly. Where the byte mode spends three UTF-8 bytes (24 bits) on a typical kanji, Kanji mode spends 13 bits — roughly 1.8x denser. This document records the mode's bit layout, the Shift-JIS to 13-bit derivation, the JIS X 0208 scope, how the Unicode mapping stays zero-dependency, and how Kanji fits the mixed-mode segmenter.

> Indonesian version: [22-kanji-mode.id.md](22-kanji-mode.id.md).

## 1. Why Kanji mode

- **Byte mode is wasteful for Japanese.** A BMP kanji is three bytes in UTF-8, so a byte-mode segment spends 24 bits per character. Kanji mode encodes the same character in 13 bits, a little under half the size.
- **It completes the mode set.** Numeric, alphanumeric, byte, and Kanji are the four data modes in the standard; with Kanji the encoder covers all of them (ECI and structured append are headers, not data modes).
- **It is a segmentation choice, not a switch.** Kanji joins the DP segmenter as a fourth mode, so a payload mixing Japanese with ASCII splits into Kanji, alphanumeric, and byte runs at whatever boundary minimises the total — a run of kanji becomes a Kanji segment, the ASCII around it stays numeric/alphanumeric/byte.

## 2. Bit layout

A Kanji segment is the ordinary mode-segment shape: a 4-bit mode indicator, a character-count indicator, then the payload.

- **Mode indicator: `1000`** (4 bits).
- **Character-count indicator:** 8 bits for versions 1-9, 10 bits for 10-26, 12 bits for 27-40. These are the narrowest counts of any mode, because a Kanji symbol holds fewer characters than a numeric one of the same size.
- **Payload:** 13 bits per character (section 3). A version-1 Kanji segment of `n` characters is therefore `4 + 8 + 13n` bits.

## 3. The 13-bit encoding

Each character is encoded from its Shift-JIS two-byte code `C`:

1. If `0x8140 <= C <= 0x9FFC`, subtract `0x8140`: `D = C - 0x8140`.
2. If `0xE040 <= C <= 0xEBBF`, subtract `0xC140`: `D = C - 0xC140`.
3. The 13-bit value is `(D >> 8) * 0xC0 + (D & 0xFF)`.

Decoding reverses it: from the 13-bit value `V`, `D = ((V / 0xC0) << 8) | (V % 0xC0)`, then add back `0x8140` when `D` is below the first range's span and `0xC140` otherwise, recovering `C`.

Worked example — the character 亜 (U+4E9C), Shift-JIS `0x889F`:

```text
C = 0x889F                         (in the 0x8140..0x9FFC range)
D = 0x889F - 0x8140 = 0x075F
V = (0x07 * 0xC0) + 0x5F = 1344 + 95 = 1439 = 0x59F
  -> 13 bits: 0 0101 1001 1111
```

The library holds Unicode runes, not Shift-JIS bytes, so the real pipeline is rune -> Shift-JIS (via the table, section 4) -> 13-bit on encode, and 13-bit -> Shift-JIS -> rune on decode.

## 4. JIS X 0208 and the generated table

QR Kanji mode targets exactly one character set, **JIS X 0208**, serialised as Shift-JIS. A rune is Kanji-eligible only if it has a JIS X 0208 code; anything else (JIS X 0212, emoji, most Latin) is not eligible and stays in byte mode. There is no error for an unmapped rune — it is simply a different mode choice.

Mapping a Unicode rune to its Shift-JIS code needs a table of roughly 6,900 entries. To keep the library at zero runtime dependencies, that table is **generated at development time and committed** as a pure-Go source file (`kanji_table.go`) with no imports. The generator (`gen_kanji_table.go`) carries a `//go:build ignore` tag and is run with `go generate`; it is the only code that imports `golang.org/x/text/encoding/japanese`, which is already an indirect (test-only) module. Building or testing the library never compiles the generator and never imports `x/text`. Regenerating the table is a rare, documented step, and CI asserts the committed file is byte-for-byte what `go generate` produces. The codec always round-trips through the table, never by blind arithmetic, so an unassigned Shift-JIS code is never produced or accepted.

## 5. Segmentation interaction

Kanji is added to the dynamic-programming segmenter (doc 17) as a fourth mode alongside numeric, alphanumeric, and byte. A rune is eligible for a Kanji segment only when the table maps it, exactly the way a rune is eligible for numeric mode only when it is a digit. The DP compares the cost of encoding a run of eligible kanji as a Kanji segment (13 bits each, plus the header) against splitting it differently, and picks the minimum.

Because ASCII is never Kanji-eligible and a non-Japanese payload contains no eligible runes, adding Kanji cannot change the segmentation of any input that has no JIS X 0208 characters — every existing golden fixture encodes byte-for-byte as before. Kanji only ever wins where it is genuinely cheaper, which for a run of kanji it always is (13 bits versus 24 in byte mode).

## 6. Density, honestly

Kanji mode is **13 bits per character**; the same BMP kanji in UTF-8 byte mode is three bytes, **24 bits** — about 1.8x. (An earlier README limitation claimed "4x"; that is wrong and is corrected with this milestone, in keeping with the project's habit of measuring rather than guessing, doc 16's file-size note being the precedent.) The gain is real and worth the mode, but it is a payload-density gain of under half, not a fourfold one.

## 7. Implementation pointers

- `qrgen/kanji.go` hosts `kanjiEligible(rune) bool`, `kanjiValue(rune) (uint16, bool)` (rune -> 13-bit), and `kanjiRune(uint16) (rune, bool)` (13-bit -> rune), all backed by the generated table. The Shift-JIS offset arithmetic lives here and is unit-tested at both range boundaries.
- `ModeKanji` (`1000`) joins `mode.go` with its 8/10/12 character-count widths; the DP segmenter in `segment.go` gains a Kanji-eligibility prefix and the 13-bit cost; `writeKanji` in `encode.go` emits the values and version selection accounts for Kanji segments.
- `decodeText` gains `case 0b1000`, reading the Kanji count then each 13-bit value back to a rune via the table.
- Validation is a rune-for-rune table round-trip over every mapped code point, an encode-decode round-trip for pure-Kanji and mixed Japanese/ASCII payloads, and a gozxing cross-check on a Kanji symbol.

## References

- ISO/IEC 18004:2015 — Kanji mode indicator `1000`, the 8/10/12 character-count widths (Table 3), and the Shift-JIS two-byte-to-13-bit derivation with its two offset ranges.
- JIS X 0208 — the double-byte character set QR Kanji mode encodes; Shift-JIS is its byte serialisation.
- `golang.org/x/text/encoding/japanese` — the Shift-JIS codec used only by the ignored table generator.
- `docs/theory/09-data-tables.md` and `docs/theory/17-optimal-segmentation.md` — the mode/count tables and the DP segmenter Kanji extends.
