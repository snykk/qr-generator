# QR Encoder — ECI Segments Plan

This document describes the implementation plan for **ECI (Extended Channel Interpretation) segments** targeting the `v0.9.0` minor release. It closes the first listed limitation — byte-mode payloads are currently emitted as raw UTF-8 with no charset declaration, and the decoder treats byte segments as UTF-8 implicitly — by adding the spec's mechanism for declaring the character set of a byte segment on both the encoder and the decoder.

> Status: **draft / living document.** Milestones ECI1..ECI5 land incrementally on the `eci-segments` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M, D, T, R, S, MM, P, and TR milestones.

> Indonesian version: [docs/plan-eci-segments.id.md](plan-eci-segments.id.md).

---

## 1. Vision & Goals

- Implement the **ECI segment** end to end: the encoder can prepend an ECI designator that declares the character set of the following byte data, and the decoder parses ECI segments, tracks the active character set, and decodes subsequent byte segments accordingly.
- Make the change **opt-in and byte-identical by default.** With no ECI requested, the encoder emits exactly the same bit stream it does today, so every existing golden fixture and the third-party gozxing round-trip are unchanged. ECI is something the caller asks for, never an automatic behaviour change.
- Get the **designator encoding exactly right.** The ECI assignment number is encoded in 1, 2, or 3 bytes with a self-describing length prefix; this is the only genuinely fiddly part and is exactly what table-driven tests are for.
- Keep **zero runtime dependencies.** Only character sets transcodable with the standard library are supported: UTF-8 (Go strings are already UTF-8) and ISO-8859-1 / Latin-1 (every rune at most `0xFF` maps to one byte). Arbitrary code pages such as Shift-JIS would require `golang.org/x/text` and are deliberately out of scope.
- Same philosophy as every prior milestone: pure Go, spec/reference-first with a bilingual doc, table-driven tests plus an encode-decode round-trip and a gozxing cross-check so an independent decoder agrees.

## 2. Design Principles

1. **ECI is opt-in via a typed option.** A new `ECI` type with constants for the supported assignments (`ECIUTF8` = 26, `ECILatin1` = 3) plus `WithECI(ECI) Option`. The zero value means "no ECI declared", which preserves today's implicit-UTF-8 behaviour and the existing bit-for-bit output.
2. **The designator codec lives in one tested place.** A small `eci.go` holds the `ECI` type, the 1/2/3-byte designator encode and decode, and the stdlib transcoding helpers. The encoder and decoder both call into it; the byte layout is unit-tested against the spec's three length classes at their boundaries.
3. **The mechanism is general; the supported charsets are bounded.** The decoder parses *any* ECI designator (so it never chokes on a valid symbol), but only ECI 3 and 26 carry a transcoder. The byte payload itself is always a Go string round-tripped through the declared charset; numeric and alphanumeric segments are charset-independent and untouched.
4. **Version selection accounts for the ECI header.** The ECI segment adds `4 + 8/16/24` bits ahead of the data, so the needed-bits computation used by `selectVersion` and the force-version capacity check includes the ECI overhead. Otherwise a payload that just fits could overflow once the designator is prepended.
5. **Spec-faithful placement.** The ECI header (mode indicator `0111` followed by the designator — the spec's term, looser "ECI segment" notwithstanding) is emitted once at the head of the data, before the mixed-mode segment sequence, per ISO/IEC 18004. Per clause 15.2 an ECI governs the interpretation of all subsequent data; its visible effect lands on byte data only as a practical consequence, since numeric and alphanumeric encode the same fixed ASCII subsets under every common charset.
6. **Tests first**, including a round-trip: encode with an ECI, decode, assert the recovered text equals the input for both UTF-8 and Latin-1, plus a gozxing cross-check confirming an independent decoder reads the ECI-tagged symbol identically.

## 3. Scope

### In scope for v0.9.0

- `ECI` type with `ECIUTF8` (26) and `ECILatin1` (3); `WithECI(ECI) Option`.
- Encoder: emit the ECI segment (`0111` + 1/2/3-byte designator) when an ECI is requested, transcode the byte payload to the declared charset (UTF-8 passthrough, Latin-1 via per-rune narrowing with an error on a rune above `0xFF`), and include the ECI overhead in version selection.
- Decoder: parse ECI segments in `decodeText` (`case 0b0111`), read the designator, set the active charset, and decode following byte segments through it; ECI 3 → Latin-1, 26 → UTF-8.
- The 1/2/3-byte designator encode/decode with full boundary coverage (127/128, 16383/16384).
- Reference doc `docs/theory/20-eci-segments.md` (EN + ID), a runnable example, and a README usage section.

### Out of scope (still)

- **Arbitrary code pages** (Shift-JIS, the ISO-8859-* family beyond Latin-1, Windows code pages). These need `golang.org/x/text`, which would break the zero-runtime-dependency rule. The mechanism supports them structurally; only the transcoders are omitted.
- **Kanji mode.** A separate roadmap item; ECI is its prerequisite but not coupled to it in v0.9.
- **Automatic ECI.** The encoder never inserts an ECI on its own; UTF-8 without a declaration stays the default so output is unchanged for callers who do not ask.
- **Per-segment ECI switching mid-payload.** v0.9 emits a single ECI at the head. Multiple ECI switches within one symbol are valid spec but not needed for the common cases and are deferred.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after ECI3) gives a spec-correct encoder whose default path is byte-identical. **Checkpoint B** (ECI5) is the `v0.9.0` release.

### ECI1 — Plan Doc `(S)`

- [x] `docs/plan-eci-segments.md` and its Indonesian counterpart covering vision, principles, scope, milestones ECI1..ECI5, file-layout delta, risks, references, and open questions.

### ECI2 — ECI Reference Doc `(S)`

Goal: document the ECI mechanism and the bounded charset scope before any code lands.

- [ ] `docs/theory/20-eci-segments.md` — what ECI is and the problem it solves (implicit-UTF-8 non-conformance); the `0111` mode indicator; the 1/2/3-byte designator encoding with its `0` / `10` / `110` length prefixes and the value ranges; the common assignment numbers (3 = ISO-8859-1, 26 = UTF-8); where the ECI segment sits in the data stream; the zero-dependency transcoding boundary (why only UTF-8 and Latin-1); and a worked example. Closes with implementation pointers.
- [ ] Indonesian counterpart `docs/theory/20-eci-segments.id.md`.
- [ ] Updated `docs/theory/README.md` and `.id.md`: entry 20 plus a code-mapping row pointing at `qrgen/eci.go`.

### ECI3 — Encoder + Designator Codec `(M)`

Goal: the encoder side, with the default path provably unchanged.

- [ ] `qrgen/eci.go` with the `ECI` type, `ECIUTF8`/`ECILatin1`, `appendECIDesignator`/`readECIDesignator` (1/2/3-byte), and `transcodeTo`/`transcodeFrom` helpers (UTF-8 passthrough, Latin-1 narrowing). `ModeECI` (`0111`) added to `mode.go` for symmetry.
- [ ] `WithECI(ECI)` in `qrgen/options.go` (zero value = none); `encodeText` emits the ECI segment when set and transcodes byte payloads; `selectVersion`/`segmentsBitLength` add the ECI overhead.
- [ ] Tests: designator codec at the 127/128 and 16383/16384 boundaries; an all-numeric input with `WithECI` still selects the right version; a Latin-1 payload with a rune above `0xFF` returns a clear error; and a guard test asserting the no-ECI output is byte-identical to the pre-change encoder for representative inputs.

### Checkpoint A — encoder emits spec-correct ECI; the default no-ECI path is byte-identical.

### ECI4 — Decoder + Round-Trip `(M)`

Goal: parse ECI on decode and prove the round-trip, including on an independent decoder.

- [ ] `decodeText` gains `case 0b0111`: read the designator, set the active charset, and decode subsequent byte segments through it (3 → Latin-1, 26 → UTF-8). Decide and document the unknown-ECI behaviour (see open questions).
- [ ] `TestECIRoundTrip` encodes UTF-8 (ECI 26) and Latin-1 (ECI 3) payloads, decodes them, and asserts the exact text returns; designator-class coverage via payloads that force 1- and 2-byte designators where practical.
- [ ] `TestRoundTripWithThirdPartyDecoder` gains an ECI-tagged UTF-8 case; gozxing reads it identically, confirming spec conformance.

### ECI5 — Polish & Release `(S)`

Goal: cut `v0.9.0`.

- [ ] README: an ECI usage note; a `WithECI` row plus the `ECI` constants in the API summary; the Limitations "No ECI segment" bullet updated to reflect the new opt-in support and its bounded charset scope; Scope and Roadmap updated.
- [ ] Runnable example `examples/encode/eci/main.go` (explicit-UTF-8 and a Latin-1 payload).
- [ ] `CHANGELOG.md` `v0.9.0` entry plus compare/tag anchors written; left unstaged in the working tree for the maintainer to commit with the release (mirroring v0.6, v0.7, and v0.8).
- [ ] `go test -race ./...` clean, gofmt-clean.
- [ ] Tag `v0.9.0` (left for the maintainer per the established git/release workflow; annotation recommended in the release conversation).

---

## 5. Proposed File Layout Delta

```
qrgen/
├── eci.go                # new — ECI type, designator codec, stdlib transcoders
├── eci_test.go           # new — designator boundaries, encode/decode round-trip
├── mode.go               # +ModeECI (0111)
├── options.go            # +WithECI (+field)
├── encode.go             # emit ECI segment, account overhead in version selection
├── decode_matrix.go      # decodeText: case 0b0111, active-charset tracking
├── roundtrip_test.go     # +ECI gozxing case
docs/
├── plan-eci-segments.md      # this file
├── plan-eci-segments.id.md   # Indonesian counterpart
└── theory/
    ├── 20-eci-segments.md     # new
    └── 20-eci-segments.id.md  # new
examples/encode/eci/
└── main.go               # new — explicit-UTF-8 + Latin-1 demo
```

## 6. Risks & Technical Notes

- **Default output must not move.** The whole change is gated on `WithECI`; the no-ECI path emits the identical bit stream. A guard test compares no-ECI output against known-good encodings so a regression is caught immediately, and the existing gozxing round-trip stays green untouched.
- **Designator encoding is the fiddly part.** The 1/2/3-byte form with `0` / `10` / `110` prefixes must be exact, and the boundaries (127 vs 128, 16383 vs 16384) are where off-by-one bugs hide. Both directions are unit-tested at those boundaries.
- **Version-selection accounting.** Forgetting the `4 + 8/16/24` ECI bits would let a payload that just fits overflow once the designator is prepended. The needed-bits path adds the overhead before choosing or validating a version.
- **Zero-dependency transcoding limit.** Only UTF-8 (native) and Latin-1 (per-rune narrowing) are transcodable without a charset library. A Latin-1 request with a rune above `0xFF` is a clear caller error, not a silent mangle. The doc states plainly that other code pages are unsupported by design.
- **Unknown ECI on decode.** The decoder must still parse the designator of an ECI it has no transcoder for, rather than misreading the bits as data. What it does with the following bytes (typed error vs best-effort UTF-8) is settled in the open questions and documented.
- **Decoder must accept non-minimal designators.** The encoder emits the shortest 1/2/3-codeword form, but ISO/IEC 18004 (Table 4, clause 8.4.1.1) allows a low assignment number to be encoded in a longer form — the per-class ranges overlap and all start at zero, the shortest is merely preferred. So the decoder reads the codeword count from the prefix bits (`0`/`10`/`110`) and accepts any length, never assuming the minimal one. The 999999 ceiling is a six-decimal-digit cap, not a bit-width cap.

## 7. References

- ISO/IEC 18004:2015 — ECI mode (mode indicator `0111`), the 1/2/3-byte ECI designator encoding, and segment structure.
- AIM ITS/04-001 — *Extended Channel Interpretations* assignment registry: 3 = ISO-8859-1, 26 = UTF-8, 20 = Shift-JIS, etc.
- `docs/theory/02-data-encoding.md` and `docs/theory/09-data-tables.md` — the existing mode-indicator and character-count notes that ECI extends.
- Go standard library — `unicode/utf8` (UTF-8 validity) and the trivial Latin-1 narrowing; deliberately not `golang.org/x/text`.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Option shape.** `WithECI(ECI)` with an `ECI` type and `ECIUTF8`/`ECILatin1` constants (proposed) versus a string-charset option. The typed form is unambiguous and keeps the supported set explicit.
- **Unknown-ECI decode behaviour.** A typed `ErrUnsupportedECI` (honest, fails loudly) versus best-effort UTF-8 decoding of the following bytes (lenient, may produce mojibake). Proposed: parse and skip the designator, then decode as UTF-8 best-effort with the situation documented, because a hard error would reject a symbol the rest of which is readable. Confirm in ECI4.
- **Emit ECI only when byte segments exist.** A pure-numeric payload gains nothing from an ECI. Proposed: emit when requested regardless (simplest, spec-valid), and note the no-byte-segment optimisation as a possible later tweak.
- **Latin-1 transcoding direction.** Encode narrows each rune to one byte (error above `0xFF`); decode widens each byte to a rune. Confirm this is the full extent of Latin-1 support and that no normalisation is attempted.
