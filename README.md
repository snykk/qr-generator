# qr-generator

[![Go Reference](https://pkg.go.dev/badge/github.com/snykk/qr-generator/qrgen.svg)](https://pkg.go.dev/github.com/snykk/qr-generator/qrgen)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)
[![CI](https://github.com/snykk/qr-generator/actions/workflows/ci.yml/badge.svg)](https://github.com/snykk/qr-generator/actions/workflows/ci.yml)

A pure-Go QR code generator **and decoder** implemented from scratch, following **ISO/IEC 18004:2015**, with no runtime dependencies beyond the Go standard library.

The encoder produces scannable PNG output for every QR version (1–40), every error-correction level (L/M/Q/H), and all four data modes (numeric, alphanumeric, byte, Kanji), as of v0.15 also Micro QR Code (M1–M4), and as of v0.17 rMQR Code (32 rectangular sizes, R7x43 to R17x139). The decoder closes the loop: given a PNG, a JPEG, or a raw `image.Image` holding a QR symbol or, as of v0.16, a Micro QR symbol, it runs binarisation, finder-pattern detection, perspective transform, alignment refinement, Reed–Solomon error correction, and segment parsing to return the original string. The library is shipped together with a thin CLI (`cmd/qrgen`) and, as of v0.18, a [web demo](#web-demo) that runs it in the browser through WebAssembly.

## Contents

- [Install](#install)
- [Library usage](#library-usage)
- [Decoding QR codes](#decoding-qr-codes)
- [CLI usage](#cli-usage)
- [Web demo](#web-demo)
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

## Character sets (ECI)

By default the encoder emits byte-mode data as UTF-8 with no character-set declaration, and the decoder reads it back as UTF-8 — which is what virtually every scanner assumes. `WithECI` adds an explicit Extended Channel Interpretation header so the symbol states its charset:

```go
// Explicit UTF-8 (ECI 26): standards-conformant rather than merely assumed.
data, err := qrgen.Encode("Héllo 世界", qrgen.WithECI(qrgen.ECIUTF8))

// ISO-8859-1 / Latin-1 (ECI 3): the byte payload is genuine Latin-1.
data, err = qrgen.Encode("Café résumé", qrgen.WithECI(qrgen.ECILatin1))
```

Only the two charsets transcodable with the standard library are supported — `ECIUTF8` (a passthrough, since Go strings are UTF-8) and `ECILatin1` (one byte per rune, erroring if any rune is above U+00FF) — which keeps the zero-dependency rule. Other code pages such as Shift-JIS would need an external charset library and are out of scope. The default (no ECI) output is byte-for-byte unchanged. See [docs/theory/20-eci-segments.md](docs/theory/20-eci-segments.md) and the demo in [examples/encode/eci](examples/encode/eci/main.go).

## Structured append

A message too large for one symbol can be split across a linked set of up to 16 symbols. `EncodeStructuredAppend` returns one PNG per symbol; `DecodeStructuredAppend` reads a full set back — in any scan order — into the original text:

```go
pages, err := qrgen.EncodeStructuredAppend(longText, 3, qrgen.WithECLevel(qrgen.ECLevelM))
if err != nil { log.Fatal(err) }
for i, png := range pages {
    os.WriteFile(fmt.Sprintf("part-%d.png", i+1), png, 0o644)
}

// ...later, from the scanned symbols (order does not matter):
text, err := qrgen.DecodeStructuredAppend(pages)
```

The text is divided on rune boundaries, every symbol carries a header with its position, the total count, and a shared parity byte, and all symbols use one common version. A single symbol still decodes on its own with `DecodeBytes`, returning just that symbol's chunk. See [docs/theory/21-structured-append.md](docs/theory/21-structured-append.md) and the demo in [examples/encode/structured-append](examples/encode/structured-append/main.go).

## Japanese text (Kanji mode)

Japanese characters are encoded automatically. When a payload contains JIS X 0208 characters, the segmenter uses **Kanji mode** — 13 bits per character rather than the 24 a BMP kanji costs in UTF-8 byte mode, about 1.8x denser — for the runs where it is cheaper, mixing freely with numeric, alphanumeric, and byte segments for the rest. There is no option to set; `Encode`, `EncodeSVG`, `EncodeTerminal`, and the decoder all handle it:

```go
data, err := qrgen.Encode("日本語のテスト 2026")
```

Only JIS X 0208 is covered (the character set QR Kanji mode targets); characters outside it, such as many emoji, stay in byte mode. The Unicode-to-Shift-JIS mapping is generated and committed, so the runtime keeps its zero dependencies. See [docs/theory/22-kanji-mode.md](docs/theory/22-kanji-mode.md) and the demo in [examples/encode/kanji](examples/encode/kanji/main.go).

## Logo

Overlay a centred logo with `WithLogo`, on both the PNG and SVG outputs. Requesting a logo raises the error-correction level to at least H, so the modules the logo covers are recovered by error correction and the code still scans:

```go
data, err := qrgen.Encode("https://example.com", qrgen.WithLogo(qrgen.Logo{Image: logo}))
svg, err := qrgen.EncodeSVG("https://example.com", qrgen.WithLogo(qrgen.Logo{Image: logo}))
```

The logo is centred, clear of the corner finder patterns, scaled to a fraction of the symbol width (`Logo.Scale`, default 0.2), and drawn on a background pad (`Logo.Background`, `Logo.Padding`). Keep it modest — much past a quarter of the width and even EC-H cannot recover it. The PNG bakes the logo into the raster (with an in-tree resampler, so still zero dependencies); the SVG embeds it as a self-contained base64 data URI. See [docs/theory/23-logo-embedding.md](docs/theory/23-logo-embedding.md) and the demo in [examples/encode/logo](examples/encode/logo/main.go).

## Micro QR Code

Micro QR Code is the small sibling of QR Code in the same standard: one finder pattern, a 2-module quiet zone, and symbols of 11 to 17 modules. Its smallest version, M1, holds five digits in about a quarter of the area of the smallest QR symbol. Ask for one with `WithMicro`, which picks the smallest symbol that fits, or force a version with `WithMicroVersion`; every encode function and `Matrix` accept them:

```go
data, err := qrgen.Encode("12345", qrgen.WithMicro())                       // M1, 11x11
svg, err := qrgen.EncodeSVG("HELLO", qrgen.WithMicroVersion(qrgen.M3))       // M3, 15x15
grid, err := qrgen.Matrix("HELLO", qrgen.WithMicro(), qrgen.WithECLevel(qrgen.ECLevelQ)) // M4-Q
```

Without `WithECLevel` the encoder takes the smallest symbol and prefers level M over L within a version, which can give the detection-only M1 for short numbers. An explicit level restricts the choice to the symbols that offer it (L and M from M2, Q only in M4; Micro QR has no H) and never yields M1, which corrects nothing. Micro QR has no ECI, structured append, or logo support, and its masks run 0..3; those options are rejected with a specific error. If the text does not fit M4 the encoder returns an `ErrCapacityExceeded` error rather than quietly producing a full QR symbol.

As of v0.16 the decoders read Micro symbols too: `Decode`, `DecodeBytes`, and the CLI's `-decode` find a Micro symbol in an image from its single finder pattern, `DecodeAll` reports Micro symbols next to QR ones, and `DecodeMatrix` recognises Micro grids by their size (see [Decoding QR codes](#decoding-qr-codes)). The encoder was validated module for module against zint and read by zxing-cpp's independent decoder, and the image detector reads the same zint symbols rendered into images; see [docs/theory/26-micro-qr.md](docs/theory/26-micro-qr.md), [docs/theory/27-micro-qr-detection.md](docs/theory/27-micro-qr-detection.md), and the demos in [examples/encode/micro](examples/encode/micro/main.go) and [examples/decode/micro](examples/decode/micro/main.go).

## rMQR Code

rMQR Code (Rectangular Micro QR Code, ISO/IEC 23941) is the rectangular member of the family: 32 sizes from R7x43 to R17x139, 7 to 17 modules tall and 27 to 139 wide, for long narrow spaces such as the edge of a label. R7x139 holds 44 data codewords in a strip 7 modules tall, almost three times the largest Micro QR symbol. Ask for one with `WithRMQR`, which picks the smallest-area symbol that fits, fix the height with `WithRMQRHeight` and let the width follow, or force a version with `WithRMQRVersion`; every encode function and `Matrix` accept them:

```go
data, err := qrgen.Encode("HELLO RMQR", qrgen.WithRMQR())                        // R13x27-M
strip, err := qrgen.Encode("https://example.com/rmqr", qrgen.WithRMQRHeight(7)) // R7x99-M, 7 rows
svg, err := qrgen.EncodeSVG("BATCH 2026-11", qrgen.WithRMQRVersion(qrgen.R13x43), qrgen.WithECLevel(qrgen.ECLevelH))
grid, err := qrgen.Matrix("RECTANGULAR", qrgen.WithRMQR())                      // 13 rows of 27 modules
```

rMQR has two error-correction levels, M and H. Without `WithECLevel` the encoder takes the smallest symbol that fits at M and switches to H when the text also fits at H in that same symbol, as zint does; an explicit M or H picks the smallest symbol at that level, and L or Q is rejected. The quiet zone defaults to 2 modules and `WithECI` works as for QR. Every rMQR symbol uses one fixed mask, so `WithMask` is rejected, as are `WithLogo`, `WithVersion`, and structured append, each with a specific error. If the text does not fit, the encoder returns an `ErrCapacityExceeded` error rather than quietly producing a QR symbol.

`DecodeMatrix` reads rMQR grids, recognised by their rectangular shape; finding rMQR symbols in images is not supported yet. The encoder reproduces 70 zint symbols module for module, covering every version at both levels, and zxing-cpp's independent reader decodes our rendered symbols; see [docs/theory/28-rmqr.md](docs/theory/28-rmqr.md) and the demo in [examples/encode/rmqr](examples/encode/rmqr/main.go).

## Inspecting a symbol

`Inspect` encodes exactly as `Matrix` does and takes the result apart, for study and visualisation: the segments the text was split into, the codewords and Reed-Solomon blocks, every candidate mask with its score, and, for each module, its role (finder, timing, format, data, and so on), its value before masking, and the codeword bit it carries:

```go
in, err := qrgen.Inspect("HELLO WORLD")
// in.Symbology, in.Version, in.ECLevel, in.Mask:  QR Code 1 M 0
// in.MaskScores: [311 406 442 463 367 528 395 445], so mask 0 has the lowest penalty
// in.Segments[0]: Alphanumeric "HELLO WORLD", 4 + 9 + 61 = 74 bits
// in.Roles[20][20] is qrgen.RoleData: bit in.ModuleBit[20][20] (7) of codeword in.ModuleCodeword[20][20] (0)
```

It accepts every option `Matrix` does, for QR Code, Micro QR, and rMQR alike, and its `Modules` equal `Matrix`'s output for the same arguments. See [docs/theory/29-symbol-inspection.md](docs/theory/29-symbol-inspection.md); the [web demo](#web-demo) draws all of it.

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

Decoder failures are returned as typed sentinel errors so callers can branch with `errors.Is`: `ErrFinderNotFound` when no symbol's finder patterns can be located, `ErrInvalidVersion` when the finder spacing implies a version outside 1..40, `ErrFormatUnreadable` when the format-info strips are too corrupted, `ErrTooManyErrors` when any Reed–Solomon block exceeds its correction budget, and `ErrCorruptedPayload` when the recovered bit stream contains an unparseable segment header.

The image stage in v0.3 uses a global Otsu threshold by default and silently falls back to Sauvola adaptive thresholding when Otsu produces a binarisation that finder detection cannot resolve — useful for inputs whose quiet zone has been contaminated by uneven lighting or soft shadows. The fallback is internal: no public option, no behaviour change for clean inputs, no allocation cost on the Otsu fast path. See [docs/theory/14-adaptive-thresholding.md](docs/theory/14-adaptive-thresholding.md) for the algorithm and dispatch heuristic.

As of v0.4, the decoder also handles axis-aligned rotations (90 / 180 / 270 degrees) and soft tilts up to about 30 degrees off-axis without any caller intervention. Finder ordering uses a rotation-invariant cross-product handedness test; the homography stage then absorbs the rotation into its 3x3 projective transform exactly the same way it already absorbs translation and scale. See [docs/theory/15-rotation-handling.md](docs/theory/15-rotation-handling.md) for the geometry and the scope boundary.

As of v0.13, the decoder reads symbols under real perspective tilt. When the affine pipeline cannot decode — the signature of a slanted photo, whose keystoning a three-centre affine homography cannot undo — it falls back to locating the finders by connected-component analysis, extracting the four corners of each finder pattern, and fitting a projective homography from those twelve correspondences (with a version trial gated by Reed–Solomon, since a foreshortened symbol reports its version unreliably). The fallback is internal and adds only decodes: upright and rotated inputs keep their exact previous behaviour and speed. It reliably reads tilts to roughly 35–40 degrees across orientations and up to about 55 degrees at favourable rotations; beyond that finder detection itself gives out. See [docs/theory/24-arbitrary-angle-detection.md](docs/theory/24-arbitrary-angle-detection.md).

As of v0.14, `qrgen.DecodeAll` reads every QR symbol in one image — a page of codes, a strip of tickets, a scene with several at once — returning one `Result` per symbol with its text, its four outer corners (the true perspective outline, with a `Bounds()` helper for an axis-aligned box), its finder centres, and its version, in reading order. Per-symbol failures are skipped rather than failing the batch; only an image with nothing decodable returns `ErrFinderNotFound`. `DecodeAll` uses the connected-component detector and grouping described in [docs/theory/25-multi-symbol-decoding.md](docs/theory/25-multi-symbol-decoding.md); the single-symbol `Decode` and `DecodeBytes` are unchanged.

```go
results, err := qrgen.DecodeAllBytes(data)
for _, r := range results {
    fmt.Printf("%q at %v (v%d)\n", r.Text, r.Bounds(), r.Version)
}
```

As of v0.16, the decoder also finds Micro QR symbols, which have a single finder pattern. When no QR symbol decodes, each finder in the image is tried as a Micro finder: its four corners give a homography up to a quarter turn, the timing patterns beside it settle the orientation, the format information names the version, and the outline is refined from the timing patterns and the symbol's own edges before sampling. QR symbols keep priority: the Micro search runs only after QR detection has failed on the same binarised image. A Micro symbol that is found but cannot be read reports `ErrFormatUnreadable` or `ErrTooManyErrors` rather than `ErrFinderNotFound`. `DecodeAll` reports Micro symbols with `Result.MicroVersion` set; their `Version` is zero and their single finder is in `Finders[0]`. Measured on zint's symbols, Micro symbols decode at any in-plane rotation from 3 pixels per module up and under perspective tilt to about 60 degrees at 10 pixels per module. See [docs/theory/27-micro-qr-detection.md](docs/theory/27-micro-qr-detection.md).

```go
results, err := qrgen.DecodeAll(img)
for _, r := range results {
    if r.MicroVersion != 0 {
        fmt.Printf("Micro %v: %q\n", r.MicroVersion, r.Text)
    }
}
```

Runnable demos live in [examples/decode/basic](examples/decode/basic/main.go), [examples/decode/styled](examples/decode/styled/main.go), [examples/decode/multi](examples/decode/multi/main.go), and [examples/decode/micro](examples/decode/micro/main.go).

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

# Micro QR: the smallest symbol that fits, or a forced version and level.
qrgen -text "12345" -micro -out micro.png
qrgen -text "HELLO" -micro-version M2 -ec L -format terminal

# rMQR: the smallest rectangle that fits, a fixed height, or a forced version.
qrgen -text "HELLO RMQR" -rmqr -out rmqr.png
qrgen -text "12345" -rmqr-height 7 -format terminal
qrgen -text "BATCH 2026-11" -rmqr-version R13x43 -ec H -out batch.png
```

**Decoding:**

```sh
# Decode a PNG file, write text to stdout.
qrgen -decode -in qr.png

# Pipe PNG bytes in from another process.
cat qr.png | qrgen -decode

# Decode and save the recovered text to a file.
qrgen -decode -in qr.png -out text.txt

# Micro QR symbols decode the same way.
qrgen -decode -in micro.png
```

Run `qrgen -h` for the full flag list. The binary exits 1 with a clear `qrgen: …` message on invalid input, oversize payloads, or undecodable images.

## Web demo

[web/](web/README.md) holds a static page that runs the library in the browser, compiled to WebAssembly and running in a Web Worker, so nothing leaves the machine: create QR Code, Micro QR, and rMQR symbols with every option and download them as PNG or SVG, look inside a symbol with `Inspect` (module roles, the symbol before masking, codewords and blocks, and every mask's score), and scan image files or the camera with `DecodeAll`. It is published to GitHub Pages, at https://snykk.github.io/qr-generator/, on every push to `master`. To run it locally:

```sh
sh web/build.sh
python3 -m http.server 8080 --directory web/site
```

## API summary

**Encoding:**

| Symbol | Purpose |
|---|---|
| `Encode(text, opts...) ([]byte, error)` | Text → PNG bytes. |
| `EncodeToFile(text, path, opts...) error` | Text → PNG file on disk. |
| `EncodeSVG(text, opts...) ([]byte, error)` | Text → SVG document bytes. Same options as `Encode`. |
| `EncodeSVGToFile(text, path, opts...) error` | Text → SVG file on disk. |
| `EncodeTerminal(text, opts...) (string, error)` | Text → multi-line block-character string for printing to a terminal. |
| `EncodeStructuredAppend(text, parts, opts...) ([][]byte, error)` | Text → a set of `parts` (2..16) linked PNG symbols. |
| `Matrix(text, opts...) ([][]bool, error)` | Text → raw boolean module grid for custom rendering. |
| `Inspect(text, opts...) (*Inspection, error)` | Text → the symbol taken apart: segments, codewords and blocks, mask scores, and each module's role, unmasked value, and codeword bit. |
| `WithECLevel(ec)` | Error-correction level (`ECLevelL`, `ECLevelM`, `ECLevelQ`, `ECLevelH`). Default `M`; for Micro QR, unset means the smallest symbol at M or L; for rMQR, only M and H exist, and unset means M, or H when it fits in the same symbol. |
| `WithVersion(v)` | Force QR version `1..40`. Default `0` (auto-select smallest fitting). |
| `WithMicro()` | Produce a Micro QR Code (M1..M4), choosing the smallest symbol that fits. |
| `WithMicroVersion(v)` | Produce a Micro QR Code at version `M1`..`M4`. |
| `WithRMQR()` | Produce an rMQR Code, choosing the smallest-area symbol that fits. |
| `WithRMQRVersion(v)` | Produce an rMQR Code at version `R7x43`..`R17x139`. |
| `WithRMQRHeight(rows)` | Produce an rMQR Code of a fixed height (7, 9, 11, 13, 15, or 17 modules), choosing the narrowest width that fits. |
| `WithMask(k)` | Force mask pattern `0..7`, or `0..3` for Micro QR. Default `-1` (auto: lowest penalty for QR, highest edge score for Micro). Not accepted for rMQR, which always uses one fixed mask. |
| `WithModuleSize(px)` | Pixels per module. Default `8`. |
| `WithQuietZone(modules)` | Module margin around the symbol. Default `4`, or `2` for Micro QR and rMQR (each the spec minimum). |
| `WithColors(fg, bg)` | Custom foreground/background `color.Color`. Default black-on-white. |
| `WithTerminalInvert(bool)` | Invert `EncodeTerminal` polarity for dark-background terminals. |
| `WithTerminalASCII(bool)` | Render `EncodeTerminal` with a double-width ASCII fallback instead of half-block glyphs. |
| `WithECI(eci)` | Declare a byte-mode character set via an ECI header. `ECIUTF8` (26), `ECILatin1` (3); default none (implicit UTF-8). |
| `WithLogo(logo)` | Overlay a centred logo on the PNG and SVG output. `Logo{Image, Scale, Padding, Background}`; forces EC ≥ H. |

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
| `Decode(img image.Image) (string, error)` | Image → text. Runs the full image pipeline; reads QR and Micro QR symbols. |
| `DecodeBytes(data []byte) (string, error)` | PNG / JPEG / GIF bytes → text. Convenience wrapper around `Decode`. |
| `DecodeMatrix(grid [][]bool) (string, error)` | Boolean module grid → text. Skips the image pipeline. Also reads Micro QR grids (11, 13, 15, 17 modules) and rMQR grids (any of the 32 rectangular sizes). |
| `DecodeAll(img image.Image) ([]Result, error)` | Image → every QR and Micro QR symbol in it, each a `Result` (text, corners, finder centres, version or Micro version), in reading order. |
| `DecodeAllBytes(data []byte) ([]Result, error)` | PNG / JPEG / GIF bytes → all symbols. Convenience wrapper around `DecodeAll`. |
| `DecodeStructuredAppend(symbols [][]byte) (string, error)` | A set of structured-append symbols (any order) → reassembled text. |

**Sentinel errors** (use `errors.Is`):

| Error | Stage |
|---|---|
| `ErrCapacityExceeded` | Encoder: payload too large for any version at the chosen EC level. |
| `ErrFinderNotFound` | Decoder: image stage could not locate a symbol's finder patterns (three for QR, one with its timing patterns for Micro QR). |
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

- Encoding modes: numeric, alphanumeric, byte (UTF-8 passthrough), and Kanji (JIS X 0208, as of v0.11), with DP-optimal mixed-mode segmentation (as of v0.6) that splits a payload across modes to minimise symbol size.
- Versions: 1–40.
- Error-correction levels: L, M, Q, H.
- PNG output (grayscale or RGBA, depending on colour options), SVG output (scalable vector, as of v0.5), and terminal output (Unicode half-block or ASCII text, as of v0.8).
- Payload builders for Wi-Fi, vCard, mailto, tel, SMS, and geo (as of v0.7).
- Optional ECI character-set declaration for byte mode (`ECIUTF8`, `ECILatin1`; as of v0.9).
- Structured append: split a long message across up to 16 linked symbols and reassemble them (as of v0.10).
- Centred logo overlay on PNG and SVG with an automatic EC-H bump (as of v0.12).
- Micro QR Code, M1 to M4 at every EC level they define, through every encoder and renderer, plus decoding from a matrix (as of v0.15) and from images (as of v0.16).
- rMQR Code, all 32 rectangular versions at levels M and H, through every encoder and renderer, plus decoding from a matrix (as of v0.17).
- Symbol inspection with `Inspect`, and a browser demo that runs the library through WebAssembly (as of v0.18).
- Image decoding (PNG / JPEG / GIF / `image.Image`) with binarisation, finder detection, perspective transform, and alignment refinement.
- Matrix decoding from `[][]bool` for callers that already have a clean grid.

Still out of scope (kept open as roadmap items): locating rMQR symbols in images, JPEG/PDF renderers, decoding QR symbols at near-grazing angles beyond about 55 degrees of tilt.

## Limitations

The library covers the encoder and the decoder end-to-end as of v0.2.0; the following are known not-yet-supported behaviours and intentional non-goals:

- **ECI limited to UTF-8 and Latin-1.** `WithECI` (v0.9) declares a byte-mode character set with an explicit ECI header, but only `ECIUTF8` and `ECILatin1` are supported, since they are the only charsets transcodable with the standard library; other code pages such as Shift-JIS would need an external dependency. Without `WithECI` the encoder emits, and the decoder reads, byte mode as UTF-8 — a deliberate deviation from the spec's ISO-8859-1 default that matches real-world scanners.
- **Kanji is JIS X 0208 only.** Kanji mode (v0.11) encodes the JIS X 0208 character set at 13 bits per character, automatically and mixed with the other modes; characters outside JIS X 0208 (JIS X 0212, most emoji) stay in byte mode. The density gain over UTF-8 byte mode is about 1.8x (13 versus 24 bits per BMP kanji), not the 4x an earlier version of this note claimed.
- **Micro QR detection has limits.** `Decode`, `DecodeBytes`, and `DecodeAll` (v0.16) find Micro symbols with the connected-component detector only, and only after QR detection has failed on the same image. Measured on zint's symbols, they read any in-plane rotation from 3 pixels per module up and most at 2, and perspective tilt to about 60 degrees at 10 pixels per module, 55 at 5, and 45 at 4; past about 65 degrees the finder itself is no longer found. A Micro symbol whose format information is unreadable is reported, not recovered, and mirrored symbols are not read (nor are mirrored QR symbols). With an explicit `WithECLevel(ECLevelL)` the Micro encoder never picks M1, where zint does; M1 only detects errors, so it cannot provide level L.
- **Structured-append is opt-in and PNG-only.** `EncodeStructuredAppend` (v0.10) splits a long message across 2..16 linked symbols and `DecodeStructuredAppend` reassembles them, but the encoder returns PNG bytes only (no SVG or terminal variant yet), takes an explicit part count rather than auto-sizing, and does not combine with `WithECI`. A single `Encode` still caps at one symbol (V40 ~2,953 bytes in byte mode at EC-L).
- **Logo size is the caller's responsibility.** `WithLogo` (v0.12) forces EC-H and defaults to a safe ~20%-width logo, but it does not stop an oversized logo from exceeding the recovery budget; keep the logo modest and clear of the corner finder patterns.
- **Arbitrary-angle decoding is bounded.** v0.4 handles the axis-aligned rotations and soft tilts to ~30 degrees; v0.13 adds a projective connected-component fallback that reads real perspective tilt reliably to ~35–40 degrees across orientations and up to ~55 degrees at favourable rotations. Beyond ~55–60 degrees the finder patterns themselves stop being detectable, so near-grazing angles, heavy blur, and glare that breaks a finder's nesting still fail.
- **rMQR decodes from a matrix only.** `WithRMQR` (v0.17) encodes all 32 versions and `DecodeMatrix` reads rMQR grids, but `Decode`, `DecodeBytes`, and `DecodeAll` do not look for rMQR symbols in images yet. rMQR's FNC1 modes are neither written nor read.
- **Multi-symbol decoding is best-effort.** `DecodeAll` (v0.14) uses the connected-component detector only, so a blurry or low-resolution upright symbol the single-symbol `Decode` reads via the line scanner may be missed. Each symbol's finder patterns must be fully within the frame — a symbol running off the edge is skipped, not partially recovered — and two symbols packed tightly enough to merge their quiet zones can lose an edge finder. It does not reassemble a structured-append set from one image; use `DecodeStructuredAppend` for that.
- **Adaptive thresholding only on the quiet zone.** v0.3 added a Sauvola fallback that recovers QR codes whose quiet zone has been darkened by uneven lighting or soft shadows, but mutations that compress the QR's own ink-paper contrast (very dim photos, heavy gradient across the symbol itself) still defeat both Otsu and Sauvola at default parameters.
- **Rule-4 mask penalty.** The dark-ratio bucket boundary uses the Thonky-style floor formula; other implementations use a ceiling-style formula and may pick a different mask for the same input. Output remains spec-compliant either way.

## Roadmap

Candidates for future minor releases (post-v0.2.0):

- **Additional renderers:** JPEG, PDF. (SVG shipped in v0.5; terminal/ASCII in v0.8.)
- **Encoding completeness:** complete — the numeric, alphanumeric, byte, and Kanji data modes plus ECI. (Mixed-mode segmentation shipped in v0.6, ECI in v0.9, Kanji in v0.11.)
- **More payload builders:** calendar event (VEVENT), crypto/EPC payment, MeCard. (Wi-Fi, vCard, mailto, tel, SMS, and geo shipped in v0.7.)
- **Logo embedding:** shipped in v0.12 — a centred logo with an automatic EC-H bump on PNG and SVG.
- **Micro QR & rMQR:** Micro QR encoding and matrix decoding shipped in v0.15 and finding Micro symbols in images in v0.16; rMQR encoding and matrix decoding shipped in v0.17, and finding rMQR symbols in images (a finder and a finder sub-pattern at opposite corners) is next.
- **Structured-append extensions:** auto part-count, SVG/terminal output, and ECI combination. (Core PNG structured-append shipped in v0.10.)
- **Decoder robustness:** pushing tilt past ~55 degrees (sub-pixel corner refinement, alignment-grid sampling), tunable Sauvola parameters (k, window) plus morphological cleanup so heavier within-symbol contrast loss can also be recovered.
- **Performance:** reduce allocations on the hot encode and decode paths, and speed up the finder search on frames of fine noise, where every speck is a candidate (about 0.7 s for a 640 by 480 frame, which the web demo works around by scanning a smaller square).

Contributions for any of these are welcome — please open an issue first so we can sketch the API together.

## Documentation

- [Encoder plan](docs/plan.md) — encoder milestones M1..M11 ([Indonesian](docs/plan.id.md)).
- [Decoder plan](docs/plan-decoder.md) — decoder milestones D1..D14 ([Indonesian](docs/plan-decoder.id.md)).
- [Theory & references](docs/theory/README.md) — bilingual literature review covering every encoder and decoder stage (data encoding, GF(2⁸), Reed–Solomon both encoding and decoding, matrix construction, masking, BCH, rendering, image processing, decoder pipeline, plus data tables and a worked end-to-end example for `"HELLO WORLD"`).
- [Web demo](web/README.md) — how the browser demo is built, run, and deployed ([plan](docs/plan-web-demo.md), [Indonesian](docs/plan-web-demo.id.md)).
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
