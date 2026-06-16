# Payload Formats for Convenience Helpers

A QR symbol just carries a string; what makes a phone offer to "join this Wi-Fi network" or "add this contact" is a **convention on that string** that scanner apps recognise. These conventions are not part of ISO/IEC 18004 — they sit on top of it. This document records the exact string templates and escaping rules the v0.7 payload builders (`qrgen/payload.go`) produce, with a citation and a worked example for each. The builders only format and escape; they do not validate the input.

> Indonesian version: [18-payload-formats.id.md](18-payload-formats.id.md).

## 1. Wi-Fi network join

Template:

```text
WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;
```

- `T` — authentication type: `WPA` (covers WPA/WPA2/WPA3), `WEP`, or `nopass` for an open network. For `nopass` the `P` field is empty or omitted.
- `S` — the SSID (network name).
- `P` — the password.
- `H` — `true` if the SSID is hidden; omitted otherwise.
- The payload ends with a double semicolon `;;`.

**Escaping.** Inside `S` and `P`, the characters `\ ; , : "` are special and must be backslash-escaped (`\;`, `\,`, `\:`, `\\`, `\"`). A leading-hex SSID convention also exists but is rarely needed and is out of scope. There is no formal RFC; the de-facto format is the one the ZXing `WifiResultParser` reads and the Android/iOS camera apps implement.

Worked example — SSID `Cafe; Wifi`, WPA, password `p:w,d\1`:

```text
WIFI:T:WPA;S:Cafe\; Wifi;P:p\:w\,d\\1;;
```

## 2. vCard contact

The builder emits vCard 3.0:

```text
BEGIN:VCARD
VERSION:3.0
FN:<name>
N:<family>;<given>;;;
ORG:<org>
TITLE:<title>
TEL:<number>          (one line per phone)
EMAIL:<address>       (one line per email)
URL:<url>
ADR:;;<address>;;;;
NOTE:<note>
END:VCARD
```

Lines are separated by CRLF (`\r\n`). Only the fields the caller fills are emitted; empty optionals are skipped. `N` is a structured field with five semicolon-separated components — the builder fills the first two (family, given) and leaves additional/prefix/suffix empty. `ADR` has seven components (post-office-box, extended, street, city, region, postcode, country); a free-form address string is placed in the street component and the rest are left empty. Phones and emails are emitted untyped (one `TEL:` / `EMAIL:` line each), since the input carries no type information.

**Escaping** (RFC 6350 §3.4, which vCard 3.0 follows in practice): within a text value, escape `\` as `\\`, `;` as `\;`, `,` as `\,`, and a newline as `\n`. The structural semicolons that separate `N`/`ADR` components are *not* escaped — only literal semicolons inside a component value are.

vCard 3.0 also defines line folding at 75 octets; the builder emits unfolded lines because every common scanner tolerates them and folding adds parsing fragility. This is a deliberate simplification.

Worked example — name "Ada Lovelace", org "Analytical Engine, Ltd":

```text
BEGIN:VCARD
VERSION:3.0
FN:Ada Lovelace
N:Lovelace;Ada;;;
ORG:Analytical Engine\, Ltd
END:VCARD
```

## 3. Email (`mailto:`)

Template (RFC 6068):

```text
mailto:<addr>?subject=<subject>&body=<body>
```

- `<addr>` is the recipient; the `?subject=…&body=…` query is omitted entirely when both are empty.
- `subject` and `body` are percent-encoded as query components. Spaces become `%20` (not `+`, which is a form-encoding convention `mailto:` does not use), and `&`, `=`, and non-ASCII bytes are percent-encoded.

Worked example — to `ada@example.com`, subject `Hello there`, body `Hi & bye`:

```text
mailto:ada@example.com?subject=Hello%20there&body=Hi%20%26%20bye
```

## 4. Phone (`tel:`)

Template (RFC 3966):

```text
tel:<number>
```

The number is emitted as given (typically in international `+CC...` form). No escaping is needed for the digit/`+`/`-` characters phone numbers use; the builder does not reformat or validate.

Worked example:

```text
tel:+15551234567
```

## 5. SMS

Template (the widely-supported `SMSTO:` scheme that the ZXing `SMSMMSResultParser` reads):

```text
SMSTO:<number>:<message>
```

The number and message are separated by a colon; the message runs to the end of the payload. The RFC 5724 `sms:<number>?body=<message>` form is an alternative some apps prefer, noted here but not the default.

Worked example — number `+15551234567`, message `on my way`:

```text
SMSTO:+15551234567:on my way
```

## 6. Geo location (`geo:`)

Template (RFC 5870):

```text
geo:<lat>,<lon>
```

Latitude and longitude are decimal degrees. The builder formats each with the shortest exact decimal representation (`strconv.FormatFloat(v, 'f', -1, 64)`), which avoids scientific notation (a `geo:` parser would reject `1e2`) and trailing zeros.

Worked example — 37.422°, -122.084°:

```text
geo:37.422,-122.084
```

## 7. Why these compose, not encode

Every builder returns a `string`. That string is an ordinary QR payload, so it flows through the existing pipeline unchanged: `Encode(WiFiPayload(cfg))` for PNG, `EncodeSVG(WiFiPayload(cfg))` for SVG, `Matrix(...)` for a raw grid. The digit-heavy ones (`tel`, `SMSTO`, `geo`) also benefit automatically from the v0.6 optimal segmentation, which packs their long digit runs into numeric segments. There is no payload-specific encoding path; the helpers are pure string construction tested both as golden strings and by a build-encode-decode round trip.

## Implementation pointers

- `qrgen/payload.go` hosts the `WiFi` and `VCard` config structs, the six builders, and the `escapeWiFi` / `escapeVCard` helpers; `mailto:`/query percent-encoding uses `net/url`.
- Each escaper is unit-tested against every special character its convention lists; each builder has golden-string tests and a round-trip test through `Encode`/`DecodeBytes`.
- Empty optional fields are omitted from the output rather than emitted blank, so there are no dangling separators.

## References

- Wi-Fi `WIFI:` scheme — ZXing `WifiResultParser` (de-facto; no RFC). <https://github.com/zxing/zxing>
- vCard 3.0 — RFC 2426; text-value escaping per RFC 6350 §3.4.
- `mailto:` — RFC 6068.
- `tel:` — RFC 3966.
- SMS `SMSTO:` — ZXing `SMSMMSResultParser`; `sms:` per RFC 5724.
- `geo:` — RFC 5870.
