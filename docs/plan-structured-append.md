# QR Encoder — Structured Append Plan

This document describes the implementation plan for **structured append** targeting the `v0.10.0` minor release. It closes the listed limitation that a payload must fit in a single symbol (V40 caps at ~2,953 bytes) by letting the encoder split a long message across up to 16 linked QR symbols and the decoder reassemble them back into the original text.

> Status: **draft / living document.** Milestones SA1..SA5 land incrementally on the `structured-append` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M, D, T, R, S, MM, P, TR, and ECI milestones.

> Indonesian version: [docs/plan-structured-append.id.md](plan-structured-append.id.md).

---

## 1. Vision & Goals

- Implement **structured append** end to end: the encoder divides a message into a set of 2..16 symbols, each carrying a structured-append header so a decoder knows its position, the total count, and a parity byte; the decoder reassembles the symbols in order and verifies they belong to the same set.
- Keep the change **purely additive.** Structured append is new public functions (`EncodeStructuredAppend`, `DecodeStructuredAppend`); the single-symbol `Encode`/`Decode`/`Matrix` path is untouched and its output is byte-for-byte unchanged.
- Get the **header exactly right.** Each symbol begins with the mode indicator `0011`, an 8-bit symbol sequence indicator (4-bit position + 4-bit count-minus-one), and an 8-bit parity byte shared across the set. This is the only fiddly part and is exactly what table-driven tests are for.
- Keep **zero runtime dependencies.** Structured append is purely algorithmic — header bits, message division, and an XOR parity — with no character tables or external modules, unlike Kanji.
- Same philosophy as every prior milestone: pure Go, spec/reference-first with a bilingual doc, table-driven tests plus a split-then-reassemble round-trip and a gozxing per-symbol cross-check.

## 2. Design Principles

1. **New functions, additive.** `EncodeStructuredAppend(text string, parts int, opts ...Option) ([][]byte, error)` returns `parts` PNG symbols; `DecodeStructuredAppend(symbols [][]byte) (string, error)` reads them back. The zero structured-append case is simply not calling these — `Encode` and `DecodeBytes` keep their exact single-symbol behaviour.
2. **The header is the first thing in each symbol.** Mode indicator `0011` (4 bits) + symbol sequence indicator (8 bits: high nibble = 0-based position `m`, low nibble = total `n` minus one) + parity (8 bits). It precedes any ECI header and the data mode segments. The 4-bit count nibble caps a set at 16 symbols.
3. **Parity is an XOR over the whole original message.** The parity byte is the XOR of every byte of the original (undivided) UTF-8 data; every symbol in the set carries the same value. On reassembly the decoder recomputes it from the concatenated text and checks it matches, catching a symbol from the wrong set or a misread.
4. **Split at rune boundaries, as evenly as possible.** The message is divided into `parts` contiguous chunks on rune boundaries (never splitting a multi-byte rune), each independently run through the existing mixed-mode segmenter. The division is deterministic so the same input always yields the same set.
5. **One common version for the set.** All symbols are sized to a single version chosen to fit the largest chunk (plus its header), so the set is visually uniform, as structured-append sets conventionally are. A forced `WithVersion` applies to every symbol.
6. **Tests first**, including a split-then-reassemble round-trip (`EncodeStructuredAppend` -> `DecodeStructuredAppend` -> exact original) and a gozxing cross-check that each symbol decodes to its chunk with the right sequence metadata.

## 3. Scope

### In scope for v0.10.0

- `EncodeStructuredAppend(text string, parts int, opts ...Option) ([][]byte, error)` — `parts` in 2..16, PNG output, one common version.
- `DecodeStructuredAppend(symbols [][]byte) (string, error)` — decode each symbol, order by position, concatenate, verify the shared parity and a consistent total count; tolerate symbols supplied in any order.
- The structured-append header codec: mode `0011`, the position/count sequence indicator, and the XOR parity, emitted by the encoder and parsed by the decoder.
- Single-symbol decode resilience: `DecodeBytes`/`DecodeMatrix` on one structured-append symbol parse and skip the header and return that symbol's chunk rather than erroring.
- Reference doc `docs/theory/21-structured-append.md` (EN + ID), a runnable example, and a README usage section.

### Out of scope (still)

- **Auto-determining the symbol count.** v0.10 takes an explicit `parts`; splitting to the minimum count that fits a target version is a possible later convenience.
- **Structured append combined with ECI or Kanji.** The header supports preceding an ECI header structurally, but v0.10 encodes each chunk as default UTF-8 mixed-mode; SA + ECI is deferred.
- **SVG and terminal structured-append variants.** v0.10 returns PNG bytes; exposing the per-symbol matrices for the other renderers is a possible follow-up.
- **Micro QR structured append.** Only the standard 40-version family is supported, as elsewhere.
- **More than 16 symbols.** The spec's 4-bit count nibble caps a set at 16; larger messages are out of reach by design.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after SA3) gives an encoder that produces a valid, independently-readable set. **Checkpoint B** (SA5) is the `v0.10.0` release.

### SA1 — Plan Doc `(S)`

- [x] `docs/plan-structured-append.md` and its Indonesian counterpart covering vision, principles, scope, milestones SA1..SA5, file-layout delta, risks, references, and open questions.

### SA2 — Structured Append Reference Doc `(S)`

Goal: document the mechanism before any code lands.

- [x] `docs/theory/21-structured-append.md` — what structured append is and the limitation it lifts; the header layout (mode `0011`, the position/count sequence indicator, the parity byte) with bit-level detail and the 16-symbol cap; how parity is computed (XOR over the whole original message) and why it matters; where the header sits relative to ECI and the data segments; the message-division and reassembly model; and a worked example. Closes with implementation pointers.
- [x] Indonesian counterpart `docs/theory/21-structured-append.id.md`.
- [x] Updated `docs/theory/README.md` and `.id.md`: entry 21 plus a code-mapping row pointing at `qrgen/structured_append.go`.

### SA3 — Encoder + Header Codec `(M)`

Goal: the encoder side, producing a valid set.

- [x] `qrgen/structured_append.go` with the header codec (sequence indicator + parity), the message-division logic, and `ModeStructuredAppend` (`0011`) added to `mode.go`.
- [x] `EncodeStructuredAppend` in `qrgen/api.go`: validate `parts` (2..16), divide the text, pick a common version, and emit each symbol's header + chunk via the existing pipeline.
- [x] Tests: sequence-indicator encoding for representative positions/counts; parity equals the XOR of the original bytes; `parts` out of range errors; each symbol carries the header and the same parity; a chunk too large for V40 errors.

### Checkpoint A — the encoder produces a spec-valid structured-append set.

### SA4 — Decoder + Round-Trip `(M)`

Goal: parse the header, reassemble, and prove the round-trip including on an independent decoder.

- [x] `decodeText` gains `case 0b0011`: read the sequence indicator and parity, record them, and decode the remaining segments as the symbol's chunk (single-symbol decode returns the chunk).
- [x] `DecodeStructuredAppend` in `qrgen/decode.go`: decode each symbol, order by position, concatenate, and verify the shared parity and total count; return a typed error on a mismatched or incomplete set.
- [x] `TestStructuredAppendRoundTrip` splits a long payload into several symbols and confirms `DecodeStructuredAppend` recovers the exact original, including out-of-order input; `TestRoundTripWithThirdPartyDecoder` confirms gozxing reads each symbol's chunk.

### SA5 — Polish & Release `(S)`

Goal: cut `v0.10.0`.

- [x] README: a structured-append usage section; `EncodeStructuredAppend`/`DecodeStructuredAppend` rows in the API summary; the Limitations "No structured-append" bullet updated; Scope and Roadmap updated.
- [x] Runnable example `examples/encode/structured-append/main.go`.
- [x] `CHANGELOG.md` `v0.10.0` entry plus compare/tag anchors written; left unstaged in the working tree for the maintainer to commit with the release (mirroring v0.6 through v0.9).
- [x] `go test -race ./...` clean, gofmt-clean.
- [ ] Tag `v0.10.0` (left for the maintainer per the established git/release workflow; annotation recommended in the release conversation).

---

## 5. Proposed File Layout Delta

```
qrgen/
├── structured_append.go       # new — header codec, parity, message division
├── structured_append_test.go  # new — header + parity + round-trip
├── mode.go                     # +ModeStructuredAppend (0011)
├── api.go                      # +EncodeStructuredAppend
├── decode.go                   # +DecodeStructuredAppend
├── decode_matrix.go            # decodeText: case 0b0011 (parse header, return chunk)
├── encode.go                   # SA header emission ahead of the data segments
├── roundtrip_test.go           # +structured-append gozxing case
docs/
├── plan-structured-append.md       # this file
├── plan-structured-append.id.md    # Indonesian counterpart
└── theory/
    ├── 21-structured-append.md      # new
    └── 21-structured-append.id.md   # new
examples/encode/structured-append/
└── main.go                     # new — split a long payload, decode it back
```

## 6. Risks & Technical Notes

- **Single-symbol decode must not regress.** Today `decodeText` errors on the `0011` mode indicator. After SA4 it parses the header and returns the symbol's chunk, so existing single-symbol behaviour is unchanged for non-SA symbols and an SA symbol decodes gracefully rather than failing.
- **Parity convention.** The parity is the XOR of the original UTF-8 bytes; the encoder and decoder must agree, and the gozxing cross-check reads each symbol's chunk (gozxing surfaces sequence/parity metadata but leaves reassembly to the caller, as we do). A mismatch test guards against a wrong-set symbol.
- **The 16-symbol cap.** The 4-bit count nibble limits a set to 16 symbols; `parts` outside 2..16 is a clear caller error, and a message too large for 16 V40 symbols cannot be encoded.
- **Header overhead in version selection.** Each symbol's data is its 20-bit structured-append header plus its chunk's segments; the per-chunk version-selection bit count includes the header so a chunk that just fits cannot overflow.
- **Rune-boundary division.** Splitting must never cut a multi-byte rune, or a chunk would hold invalid UTF-8 and the reassembled text would differ. Division is on rune boundaries and the round-trip test exercises multi-byte content.

## 7. References

- ISO/IEC 18004:2015 — Structured Append mode indicator `0011`, the symbol sequence indicator (position + count), and the parity-data byte and its XOR derivation.
- `docs/theory/02-data-encoding.md` and `docs/theory/20-eci-segments.md` — the mode-indicator and header notes structured append extends; the SA header precedes an ECI header when both are present.
- ZXing / gozxing — structured-append handling in an independent decoder, used as the cross-check oracle.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Explicit `parts` vs auto-count.** `EncodeStructuredAppend(text, parts, opts...)` is proposed for its determinism; an auto-count variant that splits to the minimum symbols fitting a target version could be added later. Confirm the signature in SA3.
- **Common vs per-symbol version.** Proposed: one common version sized to the largest chunk, for a uniform set. The alternative (each symbol independently auto-sized) is smaller on average but visually uneven. Confirm in SA3.
- **Output format.** v0.10 returns `[][]byte` (PNG). Exposing the per-symbol matrices (for SVG/terminal) is deferred; confirm PNG-only is acceptable for the first cut.
- **Single-symbol decode return.** Proposed: `DecodeBytes` on one SA symbol returns that symbol's chunk (header parsed and ignored). The alternative is a typed "this is part of a set" error; the chunk-returning behaviour is friendlier and lossless. Confirm in SA4.
- **Parity over original vs per-chunk.** Proposed: XOR over the whole original message (spec-faithful, shared across symbols). Confirm the byte basis (UTF-8 bytes of the original string) in SA2.
