# Optimal Mode Segmentation

The encoder through v0.5 chooses **one** mode for the whole payload: the most restrictive mode that covers every character (numeric if all digits, else alphanumeric if all in the alphanumeric set, else byte). That single-mode greedy is simple and often fine, but it can leave bits on the table when a payload mixes character classes — a long digit run buried inside otherwise-byte text, for instance. v0.6 replaces the greedy choice with a **dynamic-programming optimal segmentation** that splits the input into a sequence of mode segments minimising the total encoded bit length. This document covers the cost model, the DP, the version-group interplay, the UTF-8 boundary rule, and the invariant that keeps homogeneous inputs byte-for-byte unchanged.

> Indonesian version: [17-optimal-segmentation.id.md](17-optimal-segmentation.id.md).

## 1. Why one mode leaves bits on the table

Each mode packs characters at a different density:

| Mode | Bits per unit | Effective bits/char |
| --- | --- | --- |
| Numeric | 10 bits per 3 digits | ~3.33 / digit |
| Alphanumeric | 11 bits per 2 chars | 5.5 / char |
| Byte | 8 bits per byte | 8 / byte |

Digits are dramatically cheaper in numeric mode (3.33 bits) than in alphanumeric (5.5) or byte (8). So when a payload contains a run of digits surrounded by characters that force a looser mode, encoding the whole thing in that looser mode wastes bits on the digits. Splitting the digit run into its own numeric segment recovers them — but each new segment pays a fresh header (a 4-bit mode indicator plus a character-count indicator), so a split only pays off when the run is long enough to recover more than the header costs.

The QR spec explicitly allows a single symbol to contain multiple mode segments back to back (ISO/IEC 18004 clause 7.4.1); the decoder reads segment headers until it hits the terminator. So mixed-mode output is fully spec-conformant and needs no decoder cooperation beyond what already exists.

## 2. The per-segment cost model

A segment encoding the substring `s` in mode `m` at version `v` costs:

```text
cost(m, s, v) = 4                       (mode indicator)
              + CharCountBits(m, v)      (character-count indicator, version-group dependent)
              + payloadBits(m, s)        (the data itself)
```

where `payloadBits` is the familiar per-mode packing:

```text
numeric(n digits):       floor(n/3)*10 + {0:0, 1:4, 2:7}[n mod 3]
alphanumeric(n chars):   floor(n/2)*11 + {0:0, 1:6}[n mod 2]
byte(n bytes):           n*8
```

The total cost of a segmentation is the sum over its segments. The terminator, bit padding, and pad bytes are added once at the very end (they do not vary with the segmentation), so the DP minimises only the sum of segment costs.

## 3. The dynamic program

Walk the input left to right. Let `dp[i][m]` be the minimum total cost to encode the first `i` characters such that the segment covering character `i-1` is in mode `m`. The character at position `i` can either **extend** the current segment (stay in mode `m`, paying only that character's incremental payload bits) or **start a new segment** in a different eligible mode (paying a fresh `4 + CharCountBits(m, v)` header). The base case seeds each mode at position 0 with its header cost; the answer is `min over m of dp[n][m]`, and a back-pointer per cell reconstructs the chosen segments.

Two implementation conveniences make this clean:

- Because `payloadBits` is not linear per character (numeric packs in groups of 3, alphanumeric in pairs), it is easiest to compute each candidate segment's cost with the closed-form `payloadBits` above rather than an incremental per-character delta. A simple correct formulation computes, for every cut, the cost of the segment ending at the cut in each mode — `O(n²)` in the worst case but trivially fast for QR-sized payloads. The Nayuki formulation achieves `O(n)` by tracking per-mode running costs; either is fine here, and the implementation note records which we chose.
- A character is *eligible* for a mode only if the mode can represent it: digits are eligible for all three modes, alphanumeric characters for alphanumeric and byte, everything else for byte only.

## 4. Worked example: `"Order #1234567890"`

Take `"Order #1234567890"` at V1 (character-count widths: numeric 10, alphanumeric 9, byte 8). The lowercase letters force byte mode under the greedy analyzer, so the whole 17-byte string encodes as one byte segment:

```text
greedy byte:  header 4 + 8 = 12,  payload 17*8 = 136,  total 148 bits
```

The optimal segmentation splits at the digit run — `"Order #"` stays byte (it contains lowercase and `#`, neither representable in the compact modes), and `"1234567890"` becomes numeric:

```text
byte "Order #":     header 12,  payload 7*8 = 56,   subtotal 68
numeric "1234567890": header 4 + 10 = 14,  payload floor(10/3)*10 + 4 = 34,  subtotal 48
optimal total: 68 + 48 = 116 bits
```

That is a 32-bit saving (148 → 116), which at the V1-M boundary can be the difference between fitting a smaller symbol and stepping up a version.

**A counter-example worth internalising:** `"PHONE: 12345"` is *fully alphanumeric* (`:` and space are in the alphanumeric set), so the greedy analyzer already encodes it as one alphanumeric segment — 13 + 66 = 79 bits at V1. Splitting `"12345"` into numeric would cost 13 + 39 (alpha `"PHONE: "`) + 14 + 17 (numeric) = 83 bits, which is *worse*. A 5-digit run is too short to recover the extra ~14-bit header. The break-even is roughly **7 digits** when pulling a run out of alphanumeric and roughly **4 digits** out of byte. The DP discovers this automatically and simply keeps the single alphanumeric segment — optimal segmentation never produces a worse result than greedy, by construction.

## 5. Version-group interplay

`CharCountBits(m, v)` depends on the version group:

| Version | Numeric | Alphanumeric | Byte |
| --- | --- | --- | --- |
| 1–9 | 10 | 9 | 8 |
| 10–26 | 12 | 11 | 16 |
| 27–40 | 14 | 13 | 16 |

Because the header cost of each segment changes across groups, the *optimal* segmentation can differ by group: a wider character-count indicator makes extra segments more expensive, sometimes tipping a marginal split back to a single segment. This creates a circularity — the optimal segmentation depends on the version, but the version we need depends on the encoded length, which depends on the segmentation.

We resolve it the same way the single-mode encoder already resolves the analogous dependency for the character-count width: iterate. For each candidate version from 1 upward, compute the optimal segmentation *for that version* and its total length, and pick the smallest version whose length fits the data capacity. Running the DP up to 40 times is cheap for QR-sized inputs; since the cost only actually changes at the two group boundaries (9→10 and 26→27), the result can be cached to three computations if benchmarks ever show it matters.

## 6. UTF-8 and the rune-boundary rule

Numeric and alphanumeric characters are single-byte ASCII. Any other character — an accented letter, a CJK ideograph, an emoji — is multi-byte UTF-8 and can live **only** in a byte segment, where it contributes its full UTF-8 byte length to the byte-mode character count (byte mode counts bytes, not runes). Two rules keep this correct:

- The DP iterates over **runes**, never splitting a multi-byte rune across a segment boundary. A multi-byte rune is eligible for byte mode only.
- A byte segment's `payloadBits` and character count are computed from the **byte** length of its substring (`len(string(runes))`), not the rune count.

This matches the encoder's existing byte-mode behaviour (UTF-8 passthrough) and keeps the round trip exact for arbitrary Unicode payloads.

## 7. The homogeneous-input identity guarantee

The most important correctness property: for an input that is entirely one mode — all digits, all alphanumeric, or any string that the greedy analyzer would have encoded as a single byte segment — the DP must return exactly **one** segment, byte-for-byte identical to the pre-segmentation output. This is true because a single segment incurs exactly one header, and adding any internal split adds at least one more header with no offsetting payload saving (the characters were already in their cheapest covering mode). The DP, minimising total cost, therefore keeps the single segment. This guarantee is what lets the v0.1 golden fixtures and the gozxing round-trip stay green unchanged — segmentation only ever changes the output of genuinely mixed payloads.

## 8. Implementation pointers

- `qrgen/segment.go` (new in v0.6) hosts the `segment` type, `segmentText(text string, v Version) []segment`, and `segmentsBitLength`.
- `qrgen/encode.go` `selectVersion` sizes the optimal segmentation per candidate version; `encodeText` writes each segment's `[mode indicator][char count][payload]` via the existing `writeNumeric/Alphanumeric/Byte`, then the single shared terminator + padding.
- `analyzeMode` stays as an internal helper and a documented special case (it equals the DP's output for homogeneous input), useful for tests and readability.
- The decoder is untouched: its bit-stream parser already loops over segment headers, so segmented output decodes with no change.

## References

- ISO/IEC 18004:2015 — clause 7.4 (data encoding), clause 7.4.1 (multiple mode segments within one symbol).
- Project Nayuki — *Optimal text segmentation for QR Codes*: <https://www.nayuki.io/page/optimal-text-segmentation-for-qr-codes>. The DP formulation adopted here, including the `O(n)` running-cost variant.
- `docs/theory/02-data-encoding.md` — the per-mode packing and character-count widths this builds on.
- `docs/theory/09-data-tables.md` — the full `CharCountBits` table and alphanumeric value map.
