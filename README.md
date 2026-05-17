# qr-generator

A pure-Go QR code generator implemented from scratch, following **ISO/IEC 18004:2015**, with no runtime dependencies beyond the Go standard library.

> **Status:** in development (pre-v0.1.0). The encoder works end-to-end (text → scannable PNG) but the public API may still shift before v0.1.0 is tagged.

## Goals

- Provide an importable Go library (`qrgen`) for generating QR codes as PNG images.
- Implement the encoder from scratch — not a wrapper — so the codebase doubles as a learning resource for how QR really works.
- Ship a thin CLI (`cmd/qrgen`) as a usage example.

## Quick links

- [Implementation plan](docs/plan.md) — English
- [Rencana implementasi](docs/plan.id.md) — Indonesian
- [Theory & references](docs/theory/README.md) — literature review of the algorithms used (data encoding, Reed–Solomon, masking, BCH, rendering)
- [LICENSE](LICENSE) — Apache License 2.0

## Install

```sh
go get github.com/snykk/qr-generator/qrgen
```

## Usage

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

Or get the PNG bytes back and handle the output yourself:

```go
data, err := qrgen.Encode("https://example.com",
	qrgen.WithECLevel(qrgen.ECLevelQ), // higher recovery
	qrgen.WithModuleSize(10),          // bigger pixels per module
	qrgen.WithQuietZone(4),            // background margin around the symbol
)
```

For non-PNG rendering targets, `qrgen.Matrix` gives the underlying `[][]bool`
module grid (true = dark) you can render to SVG, terminal characters, or any
other canvas. Full option list: `WithECLevel`, `WithVersion`, `WithMask`,
`WithModuleSize`, `WithQuietZone`, `WithColors`. See [examples/basic](examples/basic/main.go)
and [examples/styled](examples/styled/main.go) for runnable demos.

## Scope (target v0.1.0)

- Modes: numeric, alphanumeric, byte (UTF-8).
- Versions: 1–40.
- Error correction levels: L, M, Q, H.
- Output: PNG.

Out of scope for v0.1: Kanji, ECI, Micro QR, structured append, logo embedding, SVG/terminal output, decoder. See the [plan](docs/plan.md) for details.

## Development

```sh
go build ./...
go test -race ./...
go vet ./...
```

CI runs the same three commands on every push and pull request.

## License

Licensed under the **Apache License, Version 2.0**. See [LICENSE](LICENSE) for the full text. Contributions submitted to this repository are licensed under the same terms.
