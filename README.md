# qr-generator

A pure-Go QR code generator implemented from scratch, following **ISO/IEC 18004:2015**, with no runtime dependencies beyond the Go standard library.

> **Status:** in development (pre-v0.1.0). The public API is unstable and the encoder is still being built. Not yet ready for production use.

## Goals

- Provide an importable Go library (`qrgen`) for generating QR codes as PNG images.
- Implement the encoder from scratch — not a wrapper — so the codebase doubles as a learning resource for how QR really works.
- Ship a thin CLI (`cmd/qrgen`) as a usage example.

## Quick links

- [Implementation plan](docs/plan.md) — English
- [Rencana implementasi](docs/plan.id.md) — Indonesian
- [Theory & references](docs/theory/README.md) — literature review of the algorithms used (data encoding, Reed–Solomon, masking, BCH, rendering)
- [LICENSE](LICENSE) — Apache License 2.0

## Planned usage (preview, not yet implemented)

```go
import "github.com/najibfikri/qr-generator/qrgen"

png, err := qrgen.Encode("https://example.com",
    qrgen.WithECLevel(qrgen.ECLevelM),
    qrgen.WithModuleSize(8),
    qrgen.WithQuietZone(4),
)
if err != nil {
    log.Fatal(err)
}
_ = os.WriteFile("qr.png", png, 0o644)
```

## Scope (target v0.1.0)

- Modes: numeric, alphanumeric, byte (UTF-8).
- Versions: 1–40.
- Error correction levels: L, M, Q, H.
- Output: PNG.

Out of scope for v0.1: Kanji, ECI, Micro QR, structured append, logo embedding, SVG/terminal output, decoder. See the [plan](docs/plan.md) for details.

## Development

Once milestones M2+ produce code, the standard Go workflow applies:

```sh
go build ./...
go test -race ./...
go vet ./...
```

CI runs the same three commands on every push and pull request.

## License

Licensed under the **Apache License, Version 2.0**. See [LICENSE](LICENSE) for the full text. Contributions submitted to this repository are licensed under the same terms.
