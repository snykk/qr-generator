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
| `Matrix(text, opts...) ([][]bool, error)` | Text → raw boolean module grid for custom rendering. |
| `WithECLevel(ec)` | Error-correction level (`ECLevelL`, `ECLevelM`, `ECLevelQ`, `ECLevelH`). Default `M`. |
| `WithVersion(v)` | Force QR version `1..40`. Default `0` (auto-select smallest fitting). |
| `WithMask(k)` | Force mask pattern `0..7`. Default `-1` (auto, lowest-penalty). |
| `WithModuleSize(px)` | Pixels per module. Default `8`. |
| `WithQuietZone(modules)` | Module margin around the symbol. Default `4` (spec minimum). |
| `WithColors(fg, bg)` | Custom foreground/background `color.Color`. Default black-on-white. |

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

- Encoding modes: numeric, alphanumeric, byte (UTF-8 passthrough).
- Versions: 1–40.
- Error-correction levels: L, M, Q, H.
- PNG output (grayscale or RGBA, depending on colour options).
- Image decoding (PNG / JPEG / GIF / `image.Image`) with binarisation, finder detection, perspective transform, and alignment refinement.
- Matrix decoding from `[][]bool` for callers that already have a clean grid.

Still out of scope (kept open as roadmap items): Kanji mode, ECI segments, Micro QR, structured-append, logo embedding, SVG/terminal renderers, rotated-image decoding.

## Limitations

The library covers the encoder and the decoder end-to-end as of v0.2.0; the following are known not-yet-supported behaviours and intentional non-goals:

- **No ECI segment.** Byte-mode payloads are emitted as raw UTF-8 without an ECI character-set declaration, and the decoder treats byte segments as UTF-8 implicitly. Modern QR scanners assume UTF-8 anyway, but this is a known spec non-conformance on both sides.
- **No Kanji mode.** Japanese strings fall through to byte mode on encode and decode, and pay roughly 4× the bits per character compared to native Kanji encoding.
- **No Micro QR or rMQR.** Only the standard 40-version family is supported.
- **No structured-append.** Long payloads must fit in a single symbol (V40 caps at ~2,953 bytes in byte mode at EC-L).
- **No logo embedding.** Centred logos with automatic EC compensation are a roadmap item.
- **No rotated-image decoding.** The decoder assumes the source image is approximately right-side-up; arbitrary rotations are roadmap.
- **Adaptive thresholding only on the quiet zone.** v0.3 added a Sauvola fallback that recovers QR codes whose quiet zone has been darkened by uneven lighting or soft shadows, but mutations that compress the QR's own ink-paper contrast (very dim photos, heavy gradient across the symbol itself) still defeat both Otsu and Sauvola at default parameters.
- **Greedy mode analyzer.** A single mode is chosen for the whole input; mixed-mode segmentation (DP-optimal) is deferred. A string like `"PHONE: 12345"` is encoded entirely in alphanumeric instead of splitting into alphanumeric + numeric.
- **Rule-4 mask penalty.** The dark-ratio bucket boundary uses the Thonky-style floor formula; other implementations use a ceiling-style formula and may pick a different mask for the same input. Output remains spec-compliant either way.

## Roadmap

Candidates for future minor releases (post-v0.2.0):

- **Additional renderers:** SVG, terminal/ASCII, JPEG, PDF.
- **Encoding completeness:** ECI segments, Kanji mode, mixed-mode segmentation for tighter packing.
- **Convenience helpers:** `EncodeURL`, `EncodeWiFi`, `EncodeVCard`, `EncodeEmail` for well-known payload shapes that ship with the right formatting.
- **Logo embedding:** centred logo with automatic EC-level bump for the occluded area.
- **Micro QR & rMQR:** smaller form factors for short payloads.
- **Structured-append:** split long text across multiple linked QR symbols.
- **Decoder robustness:** arbitrary rotations, tunable Sauvola parameters (k, window) plus morphological cleanup so heavier within-symbol contrast loss can also be recovered, multi-symbol detection.
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
