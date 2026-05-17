# qr-generator

[![Go Reference](https://pkg.go.dev/badge/github.com/snykk/qr-generator/qrgen.svg)](https://pkg.go.dev/github.com/snykk/qr-generator/qrgen)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)
[![CI](https://github.com/snykk/qr-generator/actions/workflows/ci.yml/badge.svg)](https://github.com/snykk/qr-generator/actions/workflows/ci.yml)

A pure-Go QR code generator implemented from scratch, following **ISO/IEC 18004:2015**, with no runtime dependencies beyond the Go standard library.

The encoder produces scannable PNG output for every QR version (1–40), every error-correction level (L/M/Q/H), and the three text-mode payloads (numeric, alphanumeric, byte). It is shipped as both an importable library (`qrgen`) and a thin CLI (`cmd/qrgen`).

## Contents

- [Install](#install)
- [Library usage](#library-usage)
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

Runnable demos live in [examples/basic](examples/basic/main.go) and [examples/styled](examples/styled/main.go).

## CLI usage

```sh
# Simplest case — writes qr.png in the current directory.
qrgen -text "HELLO WORLD"

# Styled output: higher EC, bigger modules, custom colours, fixed file path.
qrgen -text "https://example.com" -ec Q -size 12 -fg "#102E57" -bg "#FFF8E7" -out url.png

# Read the payload from stdin, pipe the PNG out to another tool.
echo -n "HELLO" | qrgen -out - | open -f -a Preview
```

Run `qrgen -h` for the full flag list. The binary exits 1 with a clear `qrgen: …` message on invalid input or oversize payloads.

## API summary

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

Validation errors (invalid EC level, out-of-range version, oversize payload, etc.) are returned as plain `error` values so callers can inspect with `errors.Is` against `qrgen.ErrCapacityExceeded` or the wrapped messages.

## Compatibility

- **Go version:** 1.25 or newer (matches the `go` directive in `go.mod`).
- **Runtime dependencies:** none beyond the Go standard library.
- **Test-only dependencies:** `github.com/makiuchi-d/gozxing` is imported by the round-trip test to validate output against an independent decoder. It never appears in `go list -deps` of `qrgen` or `cmd/qrgen`.
- **OS:** anywhere Go runs (Linux, macOS, Windows, BSD).

## Scope

In scope for v0.1:

- Encoding modes: numeric, alphanumeric, byte (UTF-8 passthrough).
- Versions: 1–40.
- Error-correction levels: L, M, Q, H.
- PNG output (grayscale or RGBA, depending on colour options).

Out of scope for v0.1 (kept open as roadmap items): Kanji mode, ECI segments, Micro QR, structured-append, logo embedding, SVG/terminal renderers, decoder.

## Limitations

The library is intentionally narrow for v0.1.0; the following are known not-yet-supported behaviours and intentional non-goals:

- **Encoder only.** This package generates QR images; it does not decode them. Use any standard QR scanner (phone camera, ZXing-based tools, `gozxing` in tests) to read the output back.
- **No ECI segment.** Byte-mode payloads are emitted as raw UTF-8 without an ECI character-set declaration. Modern decoders guess UTF-8 correctly in practice, but this is a known spec non-conformance.
- **No Kanji mode.** Japanese strings fall through to byte mode and pay roughly 4× the bits per character compared to native Kanji encoding.
- **No Micro QR or rMQR.** Only the standard 40-version family is supported.
- **No structured-append.** Long payloads must fit in a single symbol (V40 caps at ~2,953 bytes in byte mode at EC-L).
- **No logo embedding.** Centred logos with automatic EC compensation are a roadmap item.
- **Greedy mode analyzer.** A single mode is chosen for the whole input; mixed-mode segmentation (DP-optimal) is deferred. A string like `"PHONE: 12345"` is encoded entirely in alphanumeric instead of splitting into alphanumeric + numeric.
- **Rule-4 mask penalty.** The dark-ratio bucket boundary uses the Thonky-style floor formula; other implementations use a ceiling-style formula and may pick a different mask for the same input. Output remains spec-compliant either way.

## Roadmap

Candidates for future minor releases (post-v0.1.0):

- **Additional renderers:** SVG, terminal/ASCII, JPEG, PDF.
- **Encoding completeness:** ECI segments, Kanji mode, mixed-mode segmentation for tighter packing.
- **Convenience helpers:** `EncodeURL`, `EncodeWiFi`, `EncodeVCard`, `EncodeEmail` for well-known payload shapes that ship with the right formatting.
- **Logo embedding:** centred logo with automatic EC-level bump for the occluded area.
- **Micro QR & rMQR:** smaller form factors for short payloads.
- **Structured-append:** split long text across multiple linked QR symbols.
- **Performance:** reduce allocations on the hot encode path (current baseline: ~450 allocs/op for `HELLO WORLD`).

Contributions for any of these are welcome — please open an issue first so we can sketch the API together.

## Documentation

- [Implementation plan](docs/plan.md) — milestones and progress. ([Indonesian](docs/plan.id.md))
- [Theory & references](docs/theory/README.md) — bilingual literature review covering every encoder stage (data encoding, GF(2⁸), Reed–Solomon, matrix construction, masking, BCH, rendering, plus data tables and a worked end-to-end example for `"HELLO WORLD"`).
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
