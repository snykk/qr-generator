# qr-generator

[![Go Reference](https://pkg.go.dev/badge/github.com/snykk/qr-generator/qrgen.svg)](https://pkg.go.dev/github.com/snykk/qr-generator/qrgen)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)
[![CI](https://github.com/snykk/qr-generator/actions/workflows/ci.yml/badge.svg)](https://github.com/snykk/qr-generator/actions/workflows/ci.yml)

A pure-Go QR code generator **and decoder** implemented from scratch, following **ISO/IEC 18004:2015**, with no runtime dependencies beyond the Go standard library.

The encoder produces scannable PNG output for every QR version (1–40), every error-correction level (L/M/Q/H), and the three text-mode payloads (numeric, alphanumeric, byte). The decoder closes the loop: given a PNG, a JPEG, or a raw `image.Image`, it runs binarisation, finder-pattern detection, perspective transform, alignment refinement, Reed–Solomon error correction, and segment parsing to return the original string. The library is shipped together with a thin CLI (`cmd/qrgen`).

## Contents

- [Install](#install)
- [Library usage](#library-usage)
- [Decoding QR codes](#decoding-qr-codes)
- [CLI usage](#cli-usage)
- [API summary](#api-summary)
- [Compatibility](#compatibility)
- [Scope](#scope)
- [Limitations](#limitations)
- [Roadmap](#roadmap)
- [Documentation](#documentation)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgments](#acknowledgments)

## Install

```sh
go get github.com/snykk/qr-generator/qrgen
```

For the CLI:

```sh
go install github.com/snykk/qr-generator/cmd/qrgen@latest
```

## Library usage

The fastest path — encode a string and write the PNG to disk:

```go
package main

import (
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	if err := qrgen.EncodeToFile("HELLO WORLD", "qr.png"); err != nil {
		log.Fatal(err)
	}
}
```

Or get the PNG bytes back and route the output yourself:

```go
data, err := qrgen.Encode("https://example.com",
	qrgen.WithECLevel(qrgen.ECLevelQ), // higher recovery
	qrgen.WithModuleSize(10),          // bigger pixels per module
	qrgen.WithQuietZone(4),            // background margin around the symbol
)
```

For non-PNG rendering targets, `qrgen.Matrix` returns the underlying `[][]bool` module grid (true = dark) so you can render to SVG, terminal characters, or any other canvas:

```go
modules, _ := qrgen.Matrix("HELLO WORLD")
for _, row := range modules {
	for _, dark := range row {
		if dark {
			fmt.Print("██")
		} else {
			fmt.Print("  ")
		}
	}
	fmt.Println()
}
```

Runnable demos live in [examples/encode/basic](examples/encode/basic/main.go) and [examples/encode/styled](examples/encode/styled/main.go).

## Rendering to SVG

`qrgen.EncodeSVG` produces a scalable vector document instead of a PNG raster. It runs the exact same encoding pipeline as `Encode` and accepts all the same options (`WithModuleSize`, `WithQuietZone`, `WithColors`, …); only the final render step differs.

```go
data, err := qrgen.EncodeSVG("https://example.com", qrgen.WithModuleSize(10))
if err != nil { log.Fatal(err) }
os.WriteFile("qr.svg", data, 0o644)
// or in one call:
err = qrgen.EncodeSVGToFile("https://example.com", "qr.svg", qrgen.WithECLevel(qrgen.ECLevelQ))
```

The output uses a module-unit `viewBox` so it scales to any size with no blur, `shape-rendering="crispEdges"` so module boundaries stay decodable, and a single `<path>` for all dark modules. Pick SVG for resolution independence and HTML embedding — not for file size: a QR PNG is actually smaller on disk because its zlib compresses a monochrome bitmap very tightly, though a gzipped SVG lands close. SVG encoding is, however, several times faster than PNG because it skips rasterisation. See [docs/theory/16-svg-rendering.md](docs/theory/16-svg-rendering.md). A runnable demo lives in [examples/encode/svg](examples/encode/svg/main.go).

## Rendering to a terminal

`qrgen.EncodeTerminal` renders the symbol to a multi-line `string` of block characters you can print straight to a terminal and scan from the screen — no image file, no viewer. Like `EncodeSVG`, it runs the same encoding pipeline as `Encode` and differs only in the final render step.

```go
s, err := qrgen.EncodeTerminal("https://example.com")
if err != nil { log.Fatal(err) }
fmt.Print(s)
```

By default it packs two module rows per text row with Unicode half-block glyphs (`█ ▀ ▄`) so modules stay near-square against a terminal's roughly 2:1 cell. The output targets a light-background terminal; on a dark background pass `qrgen.WithTerminalInvert(true)` so the dark modules still read as dark to a scanner. `qrgen.WithTerminalASCII(true)` falls back to a portable double-width `##` form for terminals without block-element support, and `WithQuietZone` controls the light border (`WithModuleSize` and `WithColors` have no effect on text output). See [docs/theory/19-terminal-rendering.md](docs/theory/19-terminal-rendering.md) and the demo in [examples/encode/terminal](examples/encode/terminal/main.go).

## Payload helpers

The common real-world QR payloads have a recognised string format that scanner apps act on — joining a Wi-Fi network, adding a contact, composing an email. The payload builders format and escape these for you and return a `string`, so they compose with any output (`Encode`, `EncodeSVG`, or `Matrix`):

```go
// Wi-Fi join → PNG
data, err := qrgen.Encode(qrgen.WiFiPayload(qrgen.WiFi{
    SSID:     "Cafe Wi-Fi",
    Password: "latte123",
    Security: qrgen.WiFiWPA,
}))

// Contact card → SVG (same builder pattern, different output)
svg, err := qrgen.EncodeSVG(qrgen.VCardPayload(qrgen.VCard{
    Name:   "Ada Lovelace",
    Org:    "Analytical Engine, Ltd",
    Phones: []string{"+15551234567"},
    Emails: []string{"ada@example.com"},
}))
```

Builders exist for Wi-Fi, vCard, `mailto:`, `tel:`, SMS, and `geo:`. They escape special characters correctly (Wi-Fi `; , : \ "`, vCard text values, percent-encoded mail fields) but do not validate input. See [docs/theory/18-payload-formats.md](docs/theory/18-payload-formats.md) and the demo in [examples/encode/payloads](examples/encode/payloads/main.go).

## Decoding QR codes

`qrgen.DecodeBytes` reads PNG, JPEG, or GIF bytes back into the original text:

```go
data, err := os.ReadFile("qr.png")
if err != nil { log.Fatal(err) }
text, err := qrgen.DecodeBytes(data)
if err != nil { log.Fatal(err) }
fmt.Println(text)
```

If you already have an `image.Image` (e.g. decoded by another part of your program), call `qrgen.Decode` to skip the PNG-parsing step. For callers that have a clean top-down boolean matrix from a non-PNG source — say a custom rasteriser, a screenshot capture, or a deliberately constructed test fixture — `qrgen.DecodeMatrix` runs only the matrix stage (mask reversal, Reed–Solomon, segment parsing) and bypasses the image pipeline entirely.

Decoder failures are returned as typed sentinel errors so callers can branch with `errors.Is`: `ErrFinderNotFound` when the three finder patterns can't be located, `ErrInvalidVersion` when the finder spacing implies a version outside 1..40, `ErrFormatUnreadable` when the format-info strips are too corrupted, `ErrTooManyErrors` when any Reed–Solomon block exceeds its correction budget, and `ErrCorruptedPayload` when the recovered bit stream contains an unparseable segment header.

The image stage in v0.3 uses a global Otsu threshold by default and silently falls back to Sauvola adaptive thresholding when Otsu produces a binarisation that finder detection cannot resolve — useful for inputs whose quiet zone has been contaminated by uneven lighting or soft shadows. The fallback is internal: no public option, no behaviour change for clean inputs, no allocation cost on the Otsu fast path. See [docs/theory/14-adaptive-thresholding.md](docs/theory/14-adaptive-thresholding.md) for the algorithm and dispatch heuristic.

As of v0.4, the decoder also handles axis-aligned rotations (90 / 180 / 270 degrees) and soft tilts up to about 30 degrees off-axis without any caller intervention. Finder ordering uses a rotation-invariant cross-product handedness test; the homography stage then absorbs the rotation into its 3x3 projective transform exactly the same way it already absorbs translation and scale. See [docs/theory/15-rotation-handling.md](docs/theory/15-rotation-handling.md) for the geometry and the scope boundary.

Runnable demos live in [examples/decode/basic](examples/decode/basic/main.go) and [examples/decode/styled](examples/decode/styled/main.go).

## CLI usage

**Encoding:**

```sh
# Simplest case — writes qr.png in the current directory.
qrgen -text "HELLO WORLD"

# Styled output: higher EC, bigger modules, custom colours, fixed file path.
qrgen -text "https://example.com" -ec Q -size 12 -fg "#102E57" -bg "#FFF8E7" -out url.png

# Read the payload from stdin, pipe the PNG out to another tool.
echo -n "HELLO" | qrgen -out - | open -f -a Preview

# SVG output: inferred from the .svg extension, or forced with -format svg.
qrgen -text "https://example.com" -out url.svg
qrgen -text "HELLO" -format svg -out - > qr.svg

# Terminal output: print the symbol straight to the screen (defaults to stdout).
qrgen -text "https://example.com" -format terminal
qrgen -text "https://example.com" -format terminal -invert   # dark-background terminal
qrgen -text "https://example.com" -format ascii              # ASCII fallback
```

**Decoding:**

```sh
# Decode a PNG file, write text to stdout.
qrgen -decode -in qr.png

# Pipe PNG bytes in from another process.
cat qr.png | qrgen -decode

# Decode and save the recovered text to a file.
qrgen -decode -in qr.png -out text.txt
```

Run `qrgen -h` for the full flag list. The binary exits 1 with a clear `qrgen: …` message on invalid input, oversize payloads, or undecodable images.

## API summary

**Encoding:**

| Symbol | Purpose |
|---|---|
| `Encode(text, opts...) ([]byte, error)` | Text → PNG bytes. |
| `EncodeToFile(text, path, opts...) error` | Text → PNG file on disk. |
| `EncodeSVG(text, opts...) ([]byte, error)` | Text → SVG document bytes. Same options as `Encode`. |
| `EncodeSVGToFile(text, path, opts...) error` | Text → SVG file on disk. |
| `EncodeTerminal(text, opts...) (string, error)` | Text → multi-line block-character string for printing to a terminal. |
| `Matrix(text, opts...) ([][]bool, error)` | Text → raw boolean module grid for custom rendering. |
| `WithECLevel(ec)` | Error-correction level (`ECLevelL`, `ECLevelM`, `ECLevelQ`, `ECLevelH`). Default `M`. |
| `WithVersion(v)` | Force QR version `1..40`. Default `0` (auto-select smallest fitting). |
| `WithMask(k)` | Force mask pattern `0..7`. Default `-1` (auto, lowest-penalty). |
| `WithModuleSize(px)` | Pixels per module. Default `8`. |
| `WithQuietZone(modules)` | Module margin around the symbol. Default `4` (spec minimum). |
| `WithColors(fg, bg)` | Custom foreground/background `color.Color`. Default black-on-white. |
| `WithTerminalInvert(bool)` | Invert `EncodeTerminal` polarity for dark-background terminals. |
| `WithTerminalASCII(bool)` | Render `EncodeTerminal` with a double-width ASCII fallback instead of half-block glyphs. |

**Payload builders** (return a `string` to pass to `Encode`/`EncodeSVG`/`Matrix`):

| Symbol | Purpose |
|---|---|
| `WiFiPayload(WiFi) string` | Wi-Fi join string (`WIFI:`); `WiFi{SSID, Password, Security, Hidden}`, `WiFiWPA`/`WiFiWEP`/`WiFiNoPass`. |
| `VCardPayload(VCard) string` | vCard 3.0 contact; `VCard{Name, FamilyName, GivenName, Org, Title, Phones, Emails, URL, Address, Note}`. |
| `MailtoPayload(addr, subject, body) string` | `mailto:` with percent-encoded subject/body. |
| `TelPayload(number) string` | `tel:` phone. |
| `SMSPayload(number, message) string` | `SMSTO:` SMS. |
| `GeoPayload(lat, lon) string` | `geo:` location. |

**Decoding:**

| Symbol | Purpose |
|---|---|
| `Decode(img image.Image) (string, error)` | Image → text. Runs the full image pipeline. |
| `DecodeBytes(data []byte) (string, error)` | PNG / JPEG / GIF bytes → text. Convenience wrapper around `Decode`. |
| `DecodeMatrix(grid [][]bool) (string, error)` | Boolean module grid → text. Skips the image pipeline. |

**Sentinel errors** (use `errors.Is`):

| Error | Stage |
|---|---|
| `ErrCapacityExceeded` | Encoder: payload too large for any version at the chosen EC level. |
| `ErrFinderNotFound` | Decoder: image stage could not locate three valid finder patterns. |
| `ErrInvalidVersion` | Decoder: estimated version is outside 1..40. |
| `ErrFormatUnreadable` | Decoder: both format-info copies exceed the BCH correction budget. |
| `ErrTooManyErrors` | Decoder: a Reed–Solomon block exceeded its correction capacity. |
| `ErrCorruptedPayload` | Decoder: recovered bit stream has an unparseable segment header. |

## Compatibility

- **Go version:** 1.25 or newer (matches the `go` directive in `go.mod`).
- **Runtime dependencies:** none beyond the Go standard library.
- **Test-only dependencies:** `github.com/makiuchi-d/gozxing` is imported by one round-trip test for independent cross-validation; our own decoder closes a parallel loop without it. It never appears in `go list -deps` of `qrgen` or `cmd/qrgen`.
- **OS:** anywhere Go runs (Linux, macOS, Windows, BSD).

## Scope

In scope as of v0.2.0:

- Encoding modes: numeric, alphanumeric, byte (UTF-8 passthrough), with DP-optimal mixed-mode segmentation (as of v0.6) that splits a payload across modes to minimise symbol size.
- Versions: 1–40.
- Error-correction levels: L, M, Q, H.
- PNG output (grayscale or RGBA, depending on colour options), SVG output (scalable vector, as of v0.5), and terminal output (Unicode half-block or ASCII text, as of v0.8).
- Payload builders for Wi-Fi, vCard, mailto, tel, SMS, and geo (as of v0.7).
- Image decoding (PNG / JPEG / GIF / `image.Image`) with binarisation, finder detection, perspective transform, and alignment refinement.
- Matrix decoding from `[][]bool` for callers that already have a clean grid.

Still out of scope (kept open as roadmap items): Kanji mode, ECI segments, Micro QR, structured-append, logo embedding, JPEG/PDF renderers, arbitrary-angle decoding in the 30..90 degree band.

## Limitations

The library covers the encoder and the decoder end-to-end as of v0.2.0; the following are known not-yet-supported behaviours and intentional non-goals:

- **No ECI segment.** Byte-mode payloads are emitted as raw UTF-8 without an ECI character-set declaration, and the decoder treats byte segments as UTF-8 implicitly. Modern QR scanners assume UTF-8 anyway, but this is a known spec non-conformance on both sides.
- **No Kanji mode.** Japanese strings fall through to byte mode on encode and decode, and pay roughly 4× the bits per character compared to native Kanji encoding.
- **No Micro QR or rMQR.** Only the standard 40-version family is supported.
- **No structured-append.** Long payloads must fit in a single symbol (V40 caps at ~2,953 bytes in byte mode at EC-L).
- **No logo embedding.** Centred logos with automatic EC compensation are a roadmap item.
- **Limited arbitrary-angle decoding.** v0.4 added rotation handling for the axis-aligned cases (90 / 180 / 270 degrees) plus soft tilts up to about 30 degrees off-axis, but tilts in the 30..90 degree band defeat the 1:1:3:1:1 finder scanner's ±50% module-width tolerance and a wider scanner (contour-based or fan-of-orientations) is needed to close the remaining gap.
- **Adaptive thresholding only on the quiet zone.** v0.3 added a Sauvola fallback that recovers QR codes whose quiet zone has been darkened by uneven lighting or soft shadows, but mutations that compress the QR's own ink-paper contrast (very dim photos, heavy gradient across the symbol itself) still defeat both Otsu and Sauvola at default parameters.
- **Rule-4 mask penalty.** The dark-ratio bucket boundary uses the Thonky-style floor formula; other implementations use a ceiling-style formula and may pick a different mask for the same input. Output remains spec-compliant either way.

## Roadmap

Candidates for future minor releases (post-v0.2.0):

- **Additional renderers:** JPEG, PDF. (SVG shipped in v0.5; terminal/ASCII in v0.8.)
- **Encoding completeness:** ECI segments, Kanji mode. (DP-optimal mixed-mode segmentation shipped in v0.6.)
- **More payload builders:** calendar event (VEVENT), crypto/EPC payment, MeCard. (Wi-Fi, vCard, mailto, tel, SMS, and geo shipped in v0.7.)
- **Logo embedding:** centred logo with automatic EC-level bump for the occluded area.
- **Micro QR & rMQR:** smaller form factors for short payloads.
- **Structured-append:** split long text across multiple linked QR symbols.
- **Decoder robustness:** arbitrary-angle decoding for the 30..90 degree band via a contour-based or fan-of-orientations finder detector, tunable Sauvola parameters (k, window) plus morphological cleanup so heavier within-symbol contrast loss can also be recovered, multi-symbol detection.
- **Performance:** reduce allocations on the hot encode and decode paths.

Contributions for any of these are welcome — please open an issue first so we can sketch the API together.

## Documentation

- [Encoder plan](docs/plan.md) — encoder milestones M1..M11 ([Indonesian](docs/plan.id.md)).
- [Decoder plan](docs/plan-decoder.md) — decoder milestones D1..D14 ([Indonesian](docs/plan-decoder.id.md)).
- [Theory & references](docs/theory/README.md) — bilingual literature review covering every encoder and decoder stage (data encoding, GF(2⁸), Reed–Solomon both encoding and decoding, matrix construction, masking, BCH, rendering, image processing, decoder pipeline, plus data tables and a worked end-to-end example for `"HELLO WORLD"`).
- [CHANGELOG](CHANGELOG.md) — release notes.

## Development

```sh
go build ./...
go test -race ./...
go vet ./...
go test -bench=. -benchmem ./qrgen
```

CI runs the first three on every push and pull request.

## Contributing

PRs are welcome. A few guidelines:

- **Open an issue first** for new features so we can agree on scope and API before code is written.
- **Tests are required** for new behaviour. The encoder is largely covered by golden fixtures and third-party round-trip decoding; please add a matching test for any new mode, option, or renderer.
- **Keep zero runtime dependencies.** New runtime imports outside the Go standard library require strong justification.
- **Commit style:** Conventional Commits prefix (`feat`, `fix`, `docs`, `chore`, `test`, …) plus a one-paragraph description focusing on the *why*.
- **Run before pushing:** `go vet ./... && go test -race ./... && go test -bench=. -benchmem ./qrgen`.

## License

Licensed under the **Apache License, Version 2.0**. See [LICENSE](LICENSE) for the full text. Contributions submitted to this repository are licensed under the same terms.

## Acknowledgments

The implementation is grounded in three open references:

- **ISO/IEC 18004:2015** — the normative QR Code specification.
- **[Thonky's QR Code Tutorial](https://www.thonky.com/qr-code-tutorial/)** — readable walkthrough of every encoding stage.
- **[Project Nayuki](https://www.nayuki.io/page/qr-code-generator-library)** — published lookup tables (generator polynomials, format/version-info codewords, EC block structure) used as cross-check oracle.

Tooling: [`github.com/makiuchi-d/gozxing`](https://github.com/makiuchi-d/gozxing) is used in tests only to validate output against an independent reference decoder. It is never imported by the public package at runtime.
