# QR Encoder — Convenience Payload Helpers Plan

This document describes the implementation plan for **convenience payload helpers** targeting the `v0.7.0` minor release. It continues the encoder/output-breadth phase (SVG in v0.5, optimal segmentation in v0.6) by making the most common real-world QR payloads — Wi-Fi join, contact card, email, phone, SMS, geo — a one-call formatting step instead of hand-built strings.

> Status: **draft / living document.** Milestones P1..P5 land incrementally on the `convenience-helpers` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M, D, T, R, S, and MM milestones.

> Indonesian version: [docs/plan-convenience-helpers.id.md](plan-convenience-helpers.id.md).

---

## 1. Vision & Goals

- Provide **payload builder functions** that turn structured input into the exact, correctly-escaped string a phone camera expects for well-known QR conventions: Wi-Fi network join, vCard contact, `mailto:` email, `tel:` phone, SMS, and `geo:` location.
- Make them **composable with every output format.** Each builder returns a `string`, so the caller pipes it into the existing `Encode` (PNG), `EncodeSVG` (SVG), or `Matrix` ( `[][]bool`). There is deliberately no `EncodeWiFi`-returns-PNG wrapper: that would lock the helper to one output format right after v0.5 added a second, and `Encode(WiFiPayload(cfg), opts...)` reads just as cleanly.
- Keep the change **purely additive and engine-free.** Builders are string formatting; they touch neither the encoder pipeline nor the decoder. No existing API changes.
- Get the **escaping right.** The only real complexity is character escaping: Wi-Fi escapes `\ ; , : "`, vCard escapes `\ ; ,` and newlines, and `mailto:`/`geo:` query parts are percent-encoded. This is exactly what table-driven tests are for.
- Same philosophy as every prior milestone: pure Go, zero runtime dependencies (percent-encoding uses `net/url` from the standard library), spec/reference-first with a bilingual doc, and table-driven + round-trip tests.

## 2. Design Principles

1. **Builders return strings, not bytes.** `WiFiPayload(cfg) string`, `VCardPayload(cfg) string`, `MailtoPayload(...) string`, `TelPayload(...) string`, `SMSPayload(...) string`, `GeoPayload(...) string`. Composable, minimal, and unit-testable without encoding anything.
2. **Config structs for the rich formats, plain params for the simple ones.** Wi-Fi and vCard have many optional fields, so they take a struct (`WiFi`, `VCard`) for readable call sites and forward compatibility. `tel`, `mailto`, `sms`, `geo` are a couple of arguments each and take plain parameters.
3. **Escaping lives in one tested place per format.** A small `escapeWiFi` and `escapeVCard` plus `net/url` for percent-encoding. Each escaper is unit-tested against the characters its spec calls out.
4. **Spec-faithful output.** Follow the de-facto conventions every major scanner implements: the `WIFI:` scheme, vCard 3.0 (`BEGIN:VCARD`…`END:VCARD`), RFC 6068 `mailto:`, RFC 3966 `tel:`, the `SMSTO:` scheme, and RFC 5870 `geo:`. The reference doc records each with citations.
5. **No validation theatre.** Builders format what they are given; they do not reject "invalid" phone numbers or emails (formats vary worldwide and the library is not a validator). They only guarantee the output is correctly *escaped* so the structure parses.
6. **Tests first**, including a round-trip: build → `Encode` → `DecodeBytes` → assert the decoded text equals the built payload, plus a gozxing cross-check so an independent decoder agrees.

## 3. Scope

### In scope for v0.7.0

- `WiFiPayload(cfg WiFi) string` — `WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;` with escaping; supports WPA/WEP/nopass and the hidden flag.
- `VCardPayload(cfg VCard) string` — vCard 3.0 with the common fields (formatted + structured name, org, title, one or more tels and emails, url, address, note), RFC 6350 escaping, CRLF line breaks.
- `MailtoPayload(addr, subject, body string) string` — RFC 6068 `mailto:` with percent-encoded `subject`/`body`.
- `TelPayload(number string) string` — RFC 3966 `tel:`.
- `SMSPayload(number, message string) string` — the widely-supported `SMSTO:<number>:<message>` scheme.
- `GeoPayload(lat, lon float64) string` — RFC 5870 `geo:lat,lon` with sensible coordinate formatting.
- Reference doc `docs/theory/18-payload-formats.md` (EN + ID) documenting each scheme and its escaping, with citations.
- A runnable example and a README usage section.

### Out of scope (still)

- **`EncodeWiFi`-style PNG wrappers.** Rejected per principle 1 (composability with SVG/Matrix).
- **MeCard contact format.** vCard is the chosen contact format; MeCard could be a future addition if asked.
- **Payload validation** (phone/email correctness). Builders escape, they do not validate.
- **Calendar (VEVENT), crypto-address, or EPC payment schemes.** Possible future helpers; not in v0.7.
- **A `URLPayload` helper.** A URL is already its own payload — `Encode(url)` suffices — so a dedicated builder would add surface with no value.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after P3) gives the builders with full escaping coverage. **Checkpoint B** (P5) is the `v0.7.0` release.

### P1 — Plan Doc `(S)`

- [ ] `docs/plan-convenience-helpers.md` and its Indonesian counterpart covering vision, principles, scope, milestones P1..P5, file-layout delta, risks, references, open questions.

### P2 — Payload-Formats Reference Doc `(S)`

Goal: document each payload scheme and its escaping rules before any code lands.

- [x] `docs/theory/18-payload-formats.md` — for each of Wi-Fi, vCard, mailto, tel, SMS, geo: the exact string template, which characters must be escaped and how, the relevant spec/convention with a citation, and a worked example. Opens by stating these are scanner conventions layered on top of the QR symbol, not part of ISO/IEC 18004, and closes with the compose-not-encode rationale plus implementation pointers. Pins the open questions from P1: vCard 3.0 with the FN/N/ORG/TITLE/TEL/EMAIL/URL/ADR/NOTE field set and unfolded lines, `SMSTO:` scheme, `geo:` shortest-exact float formatting.
- [x] Indonesian counterpart `docs/theory/18-payload-formats.id.md`.
- [x] Updated `docs/theory/README.md` and `.id.md`: entry 18 under a new "Payload conventions (v0.7.0)" subsection plus a code-mapping row pointing at `qrgen/payload.go`.

### P3 — Payload Builders + Escaping `(M)`

Goal: the builders themselves, with escaping fully covered.

- [x] `qrgen/payload.go` with the `WiFi` and `VCard` config structs, the `WiFiSecurity` type (`WiFiWPA`/`WiFiWEP`/`WiFiNoPass`), and the six builders, plus unexported `escapeWiFi`/`escapeVCard` (via `strings.Replacer`) and `mailtoEscape` (via `net/url`, converting `+` to `%20`). The vCard set was leaned to `Name`, `FamilyName`, `GivenName`, `Org`, `Title`, `Phones`, `Emails`, `URL`, `Address`, `Note` (untyped `TEL`/`EMAIL`, free-form `ADR` in the street component, `N` filling family/given) — doc 18's vCard template was reconciled to match.
- [x] Zero-value behaviour: empty optionals are omitted (no dangling separators); Wi-Fi defaults to `WiFiWPA` and omits the password for `nopass`/empty.
- [x] Tests in `qrgen/payload_test.go`: table-driven golden strings for every builder; Wi-Fi escaping of `; , : \ "`; vCard comma/newline escaping, multiple phones/emails, and omitted optionals; mailto `%20`-not-`+` plus `&`→`%26`; geo shortest-exact formatting with a no-scientific-notation case; direct `escapeWiFi`/`escapeVCard` tests. gofmt-clean, race-clean.

### Checkpoint A — builders produce correctly-escaped, spec-faithful payloads.

### P4 — Round-Trip Validation + Example `(S)`

Goal: prove the built payloads encode and decode intact, including on an independent decoder.

- [x] `TestPayloadRoundTrip` builds all six payloads (Wi-Fi with escaped specials + hidden, a full vCard, mailto, tel, SMS, geo) and asserts `DecodeBytes(Encode(...))` returns the exact built string; the digit-heavy ones exercise v0.6 segmentation in passing.
- [x] `TestRoundTripWithThirdPartyDecoder` gained four representative payloads (Wi-Fi, vCard, mailto, geo); the independent gozxing decoder reads each exactly, confirming the escaping/format is sound.
- [x] Runnable example `examples/encode/payloads/main.go` writes a Wi-Fi join QR to PNG and a vCard QR to SVG (showing the same builder composing with both outputs) and prints the tel/geo payloads; verified with `go run`.

### P5 — Polish & Release `(S)`

Goal: cut `v0.7.0`.

- [ ] README: a "Payload helpers" usage section with a Wi-Fi and a vCard example showing composition with both `Encode` and `EncodeSVG`; API-summary rows for the six builders and the `WiFi`/`VCard` structs; update Scope; drop "convenience helpers" from the Roadmap.
- [ ] `CHANGELOG.md` `v0.7.0` entry plus compare/tag anchors.
- [ ] `go test -race ./...` clean.
- [ ] Tag `v0.7.0` (left for the maintainer per the established git/release workflow; annotation recommended in the release conversation).

---

## 5. Proposed File Layout Delta

```
qrgen/
├── payload.go            # new — WiFi/VCard structs + six builders + escapers
├── payload_test.go       # new — golden + escaping + round-trip tests
docs/
├── plan-convenience-helpers.md     # this file
├── plan-convenience-helpers.id.md  # Indonesian counterpart
└── theory/
    ├── 18-payload-formats.md        # new
    └── 18-payload-formats.id.md     # new
examples/encode/payloads/
└── main.go               # new — Wi-Fi + vCard demo to PNG and SVG
```

## 6. Risks & Technical Notes

- **Escaping is the whole game.** Wi-Fi (`\ ; , : "`) and vCard (`\ ; ,` + newline) escaping must be exact, or a scanner mis-parses the structure (e.g. a `;` in a password ends the field early). Each escaper is unit-tested against every special character its spec lists.
- **Percent-encoding scope.** `mailto:` query components use `net/url`'s query escaping, but the local-part/address should not be over-encoded. The reference doc records exactly which parts are encoded.
- **vCard line folding.** vCard 3.0 technically folds long lines at 75 octets. Most scanners tolerate unfolded lines, and folding adds parsing fragility; v0.7 emits unfolded lines and the doc notes this as a deliberate simplification.
- **No false validation.** Deliberately not validating phone/email/coordinate ranges; the builders escape and format only. Documented so callers do not expect rejection of malformed input.
- **Composition, not new output paths.** Because builders return strings, there is zero interaction with the encoder, segmenter, renderers, or decoder. The only test surface is string output plus a confidence round-trip through the existing pipeline.

## 7. References

- Wi-Fi QR convention — the `WIFI:` URI scheme as implemented by Android/iOS camera apps (no formal RFC; the de-facto format documented by the ZXing project's `WifiResultParser`).
- vCard 3.0 — RFC 2426; escaping per RFC 6350 §3.4 (vCard 4.0, same escaping rules in practice).
- `mailto:` — RFC 6068.
- `tel:` — RFC 3966.
- SMS — the `SMSTO:` scheme (ZXing `SMSMMSResultParser`); `sms:` per RFC 5724 noted as an alternative.
- `geo:` — RFC 5870.
- ISO/IEC 18004:2015 — the QR symbology itself; payload conventions sit above it.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **vCard vs MeCard.** Default to vCard 3.0 (matches the roadmap's "EncodeVCard"); MeCard is simpler and QR-native but less expressive. Revisit only if MeCard is explicitly wanted.
- **vCard field set.** Which fields make the first cut? Proposed: FN, N (family/given/additional/prefix/suffix), ORG, TITLE, TEL (multiple, with type), EMAIL (multiple), URL, ADR, NOTE. Confirm in P3.
- **SMS scheme.** `SMSTO:number:message` (chosen, widest support) vs `sms:number?body=message` (RFC 5724). Default `SMSTO:`; document the alternative.
- **Geo precision.** How many decimal places for lat/lon? Default to `strconv.FormatFloat(..., 'f', -1, 64)` (shortest exact), which avoids scientific notation and trailing zeros.
