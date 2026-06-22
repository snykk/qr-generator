# ECI Segments

The v0.9 encoder and decoder add **Extended Channel Interpretation (ECI)**, the spec's mechanism for declaring the character set of the data in a symbol. Until now byte-mode data has been emitted as raw UTF-8 with no declaration, and the decoder has read byte segments as UTF-8 implicitly — convenient, and what most modern scanners assume, but a known non-conformance because the standard's default byte-mode interpretation is ISO-8859-1, not UTF-8. ECI closes that gap by letting the caller state the character set explicitly. This document records what an ECI header is, how its designator is encoded, the assignment numbers and the default-charset nuance, the scope of an ECI, and the zero-dependency transcoding boundary that bounds which character sets this library supports.

> Indonesian version: [20-eci-segments.id.md](20-eci-segments.id.md).

> A terminology note up front: the standard calls the `0111`-indicator-plus-designator construct an **ECI header**, and reserves "segment" for a mode segment (mode indicator + character-count indicator + data). The roadmap and this milestone use the looser name "ECI segments"; in this doc the precise term is *ECI header*.

## 1. Why ECI

- **Byte mode carries no character set of its own.** A byte segment is just 8-bit values. How those bytes map to characters is a separate decision, and ISO/IEC 18004 says that without an ECI the default is **ISO-8859-1** (Latin-1) in the current 2006/2015 editions. (The original 2000 edition specified JIS X 0201; the change is why real-world decoder behaviour is inconsistent.)
- **This library assumes UTF-8 instead.** Go strings are UTF-8, modern scanners overwhelmingly assume UTF-8, and the encoder/decoder have always treated byte mode as UTF-8. That is pragmatic but is a deliberate deviation from the spec default — the first item in the README's limitations.
- **ECI makes the charset explicit and conformant.** Declaring ECI 26 (UTF-8) up front means a strict decoder no longer has to guess; declaring ECI 3 (ISO-8859-1) lets a caller emit genuine Latin-1. The data bytes are unchanged; only an explicit label is added.
- **Opt-in, never automatic.** The encoder emits an ECI header only when asked (`WithECI`). With no ECI the bit stream is byte-for-byte what it was before, so existing fixtures and the gozxing round-trip are untouched and the library keeps reading no-ECI byte data as UTF-8.

## 2. The ECI header

An ECI header is the 4-bit ECI **mode indicator `0111`** followed immediately by the ECI **designator**. Unlike a mode segment it has **no character-count indicator** — the designator is self-delimiting (section 3). In the common single-ECI case the header sits at the very **start of the data bit stream**, before the first mode segment:

```text
0111  <designator>  <mode segment> <mode segment> ...  0000(terminator)
^ECI  ^charset      ^e.g. 0100 byte: indicator + count + data
```

The standard's worked ordering is exactly this: `0111 / designator / 0100 / count / data`.

## 3. The ECI designator

The designator encodes the ECI assignment number in **1, 2, or 3 codewords** (a codeword is 8 bits). The length is self-describing: it is the count of leading `1` bits before the first `0`.

| Codewords | Bits | Template | Value bits | Shortest-form range |
|-----------|------|----------|-----------|---------------------|
| 1 | 8  | `0bbbbbbb`                     | 7  | 0 – 127        |
| 2 | 16 | `10bbbbbb bbbbbbbb`            | 14 | 128 – 16383    |
| 3 | 24 | `110bbbbb bbbbbbbb bbbbbbbb`   | 21 | 16384 – 999999 |

`b…b` is the plain binary ECI assignment number, right-aligned in the value bits.

Two subtleties matter for a correct implementation:

1. **Encoder emits the shortest form; the decoder must accept any form.** The ranges above are the *preferred/canonical* output an encoder should produce. The spec's own ranges are **overlapping and all start at zero** (1cw = 0–127, 2cw = 0–16383, 3cw = 0–999999): clause 8.4.1.1 says "the lower numbered ECI assignments may be encoded in multiple ways, but the shortest way is preferred." So a conformant decoder must read the length from the prefix bits and accept a non-minimal encoding (for example ECI 5 legally written in the 2- or 3-codeword form), even though our encoder will never produce one.
2. **The 999999 maximum is a decimal cap, not a bit-width cap.** The 21 value bits could physically hold up to 2,097,151; the standard limits the assignment number to six decimal digits, so 999999 is the ceiling.

Worked examples (the assignment numbers this library supports both fit in one codeword):

```text
ECI 26 (UTF-8):       0111  00011010              -> mode 0111, then 0 + 0011010 (=26)
ECI 3  (ISO-8859-1):  0111  00000011              -> mode 0111, then 0 + 0000011 (=3)
ECI 128 (2-codeword): 0111  10000000 10000000     -> 10 prefix + 14-bit 128
```

## 4. Assignment numbers and the default charset

The assignment numbers come from the AIM ITS/04-001 ECI registry. The three relevant here:

| ECI | Character set |
|-----|---------------|
| 000003 | ISO-8859-1 (Latin-1) |
| 000026 | UTF-8 |
| 000020 | Shift-JIS (JIS8 / JIS X 0208) |

The **default** interpretation of byte mode when no ECI is present is **ISO-8859-1** (equivalent to ECI 3) in the current standard — explicitly *not* UTF-8. To be standards-conformant, UTF-8 must be declared with ECI 26.

This library makes a deliberate, documented choice that differs from the spec default: with no ECI it continues to treat byte mode as **UTF-8** on both encode and decode, because that matches Go strings, real-world QR codes, and the library's prior behaviour, and because adopting the Latin-1 default would silently break every UTF-8 symbol already in the wild. ECI then lets a caller be explicit either way: `ECIUTF8` declares the assumption the library already makes, and `ECILatin1` selects genuine Latin-1. Because no-ECI decode and ECI-26 decode are both UTF-8, an ECI-26 symbol round-trips trivially; an ECI-3 symbol decodes through Latin-1.

## 5. The scope of an ECI

Per clause 15.2 an ECI reinterprets the byte values of **all subsequent data**, until end-of-data or a new `0111` header — it is not scoped to byte mode, and clause 8.4.1 even allows ECI data to be carried in numeric, alphanumeric, byte, or kanji mode (whichever encodes the byte values most compactly). So "an ECI only affects byte segments" is a practical observation, not a spec rule: numeric (digits) and alphanumeric (a fixed 45-character ASCII subset) encode the same characters under every common code-page ECI, so a character-set ECI's *visible* effect lands on the bytes that differ from ASCII, which in practice travel in byte mode. v0.9 emits a single ECI at the head and applies the declared charset when reconstructing byte-mode text, which is correct for this character-set use; per-segment ECI switching mid-payload is valid spec but deferred.

## 6. Zero-dependency transcoding

This library keeps zero runtime dependencies, which bounds the supported character sets to the two that transcode with the standard library alone:

- **UTF-8 (ECI 26) — passthrough.** Go strings are already UTF-8 byte sequences, so the byte payload is the string's bytes with no transcoding. One caveat: a Go string is formally an arbitrary immutable byte sequence and is not *guaranteed* valid UTF-8 (`string([]byte{0xFF})` is legal and invalid), so a strict encoder may validate with `unicode/utf8.ValidString` before declaring ECI 26.
- **ISO-8859-1 (ECI 3) — an exact bijection, no imports.** Latin-1 maps byte `0xXX` to Unicode code point `U+00XX` across the whole range `0x00..0xFF` with no gaps (this is why Unicode's first 256 code points were seeded from Latin-1). Encoding is per-rune: emit `byte(r)` when `r <= 0xFF`, otherwise it is a caller error (for example `é` U+00E9 encodes to `0xE9`, but the euro sign `€` U+20AC — which lives in Windows-1252/ISO-8859-15, not Latin-1 — and any astral rune correctly fail). Decoding is `rune(b)` for every byte.
- **Everything else is out of scope.** Shift-JIS (ECI 20) and the other code pages need a mapping table, which in Go means `golang.org/x/text/encoding` — an external module, not part of the standard library (the `golang.org/x/text` packages that ship with the toolchain are internal vendored copies user code cannot import). Pulling it in would break the zero-dependency rule, so the ECI mechanism here parses any designator but only carries transcoders for 3 and 26.

## 7. Implementation pointers

- `qrgen/eci.go` will host the `ECI` type (`ECIUTF8` = 26, `ECILatin1` = 3), the designator encode (shortest form) and decode (prefix-driven, accepting non-minimal lengths), and the two transcoders. `ModeECI` (`0111`) joins the existing mode indicators in `mode.go`.
- The encoder (`encodeText`) emits the ECI header at the head of the stream when `WithECI` is set and transcodes byte payloads to the declared charset; `selectVersion` and the force-version capacity check add the `4 + 8/16/24`-bit header overhead so a payload that just fits cannot overflow once the designator is prepended.
- The decoder (`decodeText`) gains `case 0b0111`: read the designator by its prefix, set the active charset, and decode following byte segments through it (3 → Latin-1, 26 and the no-ECI default → UTF-8). An ECI whose number has no transcoder is parsed and skipped, with the following byte data read as UTF-8 best-effort rather than failing a symbol that is otherwise readable.
- Validation is a round-trip (`Encode` with an ECI → `DecodeBytes` → exact text) for both UTF-8 and Latin-1, plus a gozxing cross-check on an ECI-26 symbol, and a guard test asserting the no-ECI output is byte-identical to the pre-change encoder.

## References

- ISO/IEC 18004:2015 (and :2000 / :2006) — ECI mode indicator `0111` (Table 2), the 1/2/3-codeword designator encoding (Table 4, clause 8.4.1.1), the ECI header structure and placement (clause 8.4), and ECI scope on decode (clause 15.2). The Latin-1 byte-mode default (current editions; JIS X 0201 in 2000) is specified here.
- AIM ITS/04-001 — *Extended Channel Interpretations* assignment registry: 3 = ISO-8859-1, 20 = Shift-JIS, 26 = UTF-8.
- `docs/theory/02-data-encoding.md` and `docs/theory/09-data-tables.md` — the mode-indicator and character-count notes the ECI header extends.
- Go standard library — `unicode/utf8` for UTF-8 validity and the trivial Latin-1 narrowing; deliberately not `golang.org/x/text`, which is an external module.
