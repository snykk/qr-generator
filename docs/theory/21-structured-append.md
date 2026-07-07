# Structured Append

The v0.10 encoder and decoder add **structured append**, the spec's way of splitting one message across a set of 2..16 linked QR symbols and reassembling them back into the original text. It lifts the single-symbol size ceiling (a V40 symbol caps at ~2,953 bytes in byte mode at EC-L): a message too large for one symbol can be carried by several. This document records the structured-append header, the symbol sequence indicator, the parity byte, and the split-and-reassemble model.

> Indonesian version: [21-structured-append.id.md](21-structured-append.id.md).

## 1. Why structured append

- **One symbol has a hard capacity.** The largest symbol, V40 at EC level L, holds about 2,953 bytes in byte mode; higher EC levels hold less. A longer message simply does not fit.
- **Structured append chains symbols.** Up to 16 symbols can be linked so a scanner that reads all of them reconstructs the original message, in order, regardless of the order they were scanned. Each symbol is an ordinary, independently-decodable QR code that also declares where it sits in the set.
- **It is purely algorithmic.** Unlike Kanji (which needs a Shift-JIS table) or arbitrary ECI charsets (which need `golang.org/x/text`), structured append is just header bits, a message division, and an XOR parity — no data tables, so it keeps the zero-dependency rule.

## 2. The structured-append header

Each symbol in a set begins its data bit stream with a structured-append header:

```text
0011           symbol sequence indicator   parity
^mode (4 bits) ^8 bits                     ^8 bits
```

- The **mode indicator is `0011`** (4 bits).
- The **symbol sequence indicator** (8 bits, section 3) says which symbol this is and how many there are.
- The **parity byte** (8 bits, section 4) is shared across the set and lets a decoder confirm the symbols belong together.

The header has no character-count indicator; it is a fixed 20 bits. It precedes everything else in the symbol — if an ECI header is also present, the order is structured-append header, then ECI header, then the data mode segments. After the header, the symbol's own chunk of the message is encoded with the ordinary mixed-mode segmentation, followed by that symbol's own terminator and padding.

## 3. The symbol sequence indicator

The 8-bit sequence indicator packs two 4-bit nibbles:

| Bits | Field | Meaning |
|------|-------|---------|
| high nibble | symbol position `m` | 0-based index of this symbol in the set (0..15) |
| low nibble  | count minus one `n-1` | total number of symbols minus one (so 0..15 encodes 1..16) |

Because each field is 4 bits, a set holds **at most 16 symbols**. A structured-append set always has at least two, so `n` is 2..16 in practice.

Worked example — symbol 2 of 4: the position is `m = 1` (0-based), the count is `n = 4` so the count nibble is `n - 1 = 3`. The indicator byte is `0001 0011` = `0x13`. Symbol 1 of 4 is `0x03`, symbol 4 of 4 is `0x33`.

## 4. The parity byte

The parity byte is the XOR, byte by byte, of **all the byte values of the original message before it was divided** into symbols. Every symbol in the set carries the same value.

For this library the original message is a UTF-8 Go string, so the parity is the XOR of `[]byte(text)` over the whole input. For example the message `"AB"` (`0x41`, `0x42`) has parity `0x41 ^ 0x42 = 0x03`.

Its purpose is integrity, not error correction: on reassembly the decoder recomputes the XOR from the concatenated text and checks it equals the parity every symbol reported. A mismatch means a symbol from a different set was mixed in, or a symbol was misread — either way the set is not trustworthy. (A decoder that cross-checks against ZXing should compute parity over the same byte basis; the encoder and decoder here agree by construction, and an empirical gozxing check during implementation confirms the byte basis interoperates.)

## 5. Splitting and reassembly

**Splitting.** The message is divided into `parts` contiguous chunks on rune boundaries — a multi-byte rune is never cut, so no chunk holds invalid UTF-8. Each chunk is then encoded as its own symbol: structured-append header, then the chunk run through the ordinary mixed-mode segmenter, then that symbol's terminator and padding. All symbols in the set are sized to one common version (chosen to fit the largest chunk plus its header) so the set is visually uniform.

**Reassembly.** The decoder reads each symbol, extracts its position, count, and parity from the header, and decodes the rest as that symbol's chunk. It then orders the symbols by position and concatenates their chunks to recover the original message. Only the decoded chunk data is concatenated: each symbol's own terminator and padding are consumed during its decode and never appear in the joined result. A single symbol from a set still decodes on its own — the header is parsed and skipped, and that symbol's chunk is returned — so `DecodeBytes` on one part yields that part rather than an error.

## 6. Placement and interaction with other features

- **Header order.** The structured-append header is first in each symbol. An ECI header, when present, follows it; then the data segments. v0.10 encodes each chunk as default UTF-8 mixed-mode and does not combine structured append with an explicit ECI.
- **Per-symbol independence.** Each symbol is a complete, spec-valid QR code with its own version, mask, error correction, terminator, and padding. Structured append only adds the shared header and the coordination it implies; nothing about the underlying symbol changes.
- **Version selection per chunk.** Each chunk's data is its 20-bit structured-append header plus its segments, so the version-selection bit count includes the header; the common version is the largest needed across the chunks.

## 7. Implementation pointers

- `qrgen/structured_append.go` will host the sequence-indicator pack/unpack, the parity XOR, and the rune-boundary division. `ModeStructuredAppend` (`0011`) joins the mode indicators in `mode.go`.
- `EncodeStructuredAppend(text, parts, opts...)` in `qrgen/api.go` validates `parts` (2..16), divides the text, picks the common version, and emits each symbol's header plus chunk through the existing pipeline, returning one PNG per symbol.
- `decodeText` gains `case 0b0011`: read the sequence indicator and parity, then decode the remaining segments as the chunk. `DecodeStructuredAppend(symbols)` in `qrgen/decode.go` decodes each symbol, orders by position, concatenates, and verifies the shared parity and a consistent count.
- Validation is a split-then-reassemble round-trip (including symbols supplied out of order) plus a `TestRoundTripWithThirdPartyDecoder` case confirming gozxing reads each symbol's chunk and its sequence metadata.

## References

- ISO/IEC 18004:2015 — Structured Append mode indicator `0011`, the symbol sequence indicator (position and count nibbles), the parity-data byte and its XOR derivation, and the reassembly rules.
- `docs/theory/02-data-encoding.md` and `docs/theory/20-eci-segments.md` — the mode-indicator and header notes structured append extends; the structured-append header precedes an ECI header when both are present.
- ZXing / gozxing — structured-append handling in an independent decoder, used as the cross-check oracle and to confirm the parity byte basis interoperates.
