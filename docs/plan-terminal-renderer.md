# QR Encoder — Terminal / ASCII Renderer Plan

This document describes the implementation plan for a **terminal / ASCII renderer** targeting the `v0.8.0` minor release. It continues the output-breadth phase (SVG in v0.5) by drawing the QR symbol with Unicode block elements (or plain ASCII) so a code can be printed straight to a terminal, piped into a text file, or embedded in any text context, and scanned with a phone camera off the screen.

> Status: **draft / living document.** Milestones TR1..TR5 land incrementally on the `terminal-renderer` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M, D, T, R, S, MM, and P milestones.

> Indonesian version: [docs/plan-terminal-renderer.id.md](plan-terminal-renderer.id.md).

---

## 1. Vision & Goals

- Render the encoded module matrix to a multi-line `string` of block characters that a phone camera can scan directly from a screen, with no image file in the loop.
- Make it **compose with the same pipeline.** `EncodeTerminal(text, opts...)` runs the exact version selection, mask choice, and option handling as `Encode` and `EncodeSVG`, and differs only in the final render step, exactly the way `EncodeSVG` already differs from `Encode`.
- Be **compact by default** via half-block vertical packing: each text row encodes a vertical pair of module rows using the block glyphs, so the rendering is half as tall as a naive one-row-per-module grid and each module stays close to square given the roughly 2:1 height-to-width aspect ratio of a terminal cell.
- Provide an **ASCII fallback** for terminals or fonts without Unicode block-element support, and an **invert** control for dark-background terminals.
- Keep the change **purely additive and engine-free.** The renderer reads the finished matrix and emits text; it touches neither the encoder pipeline nor the decoder, and no existing API changes.
- Same philosophy as every prior milestone: pure Go, zero runtime dependencies (`strings.Builder` from the standard library), spec/reference-first with a bilingual doc, and golden-string tests plus a parse-back round-trip through `DecodeMatrix`.

## 2. Design Principles

1. **`EncodeTerminal` returns a `string`.** Terminal output is text meant to be printed, so a `string` composes naturally with `fmt.Print` and is trivially unit-testable. This is a deliberate departure from `Encode`/`EncodeSVG`, which return `[]byte` because their payloads are a binary raster and a document; here the payload is human-facing text.
2. **Half-block packing by default.** Each output cell encodes a vertical pair of modules with the Unicode Block Elements `█` (both dark), `▀` (top dark), `▄` (bottom dark), and a space (both light). This halves the row count versus one terminal row per module and keeps modules near-square, because a terminal cell is about twice as tall as it is wide and the top/bottom split gives each module a roughly 1:1 footprint.
3. **ASCII fallback for portability.** `WithTerminalASCII` renders each dark module as two characters (`##`) and each light module as two spaces, doubling width so modules stay square without relying on block-element glyphs. It is one terminal row per module row, so it is taller but maximally compatible.
4. **Polarity is explicit, not guessed.** The default assumes a light-background terminal, where a block glyph reads as dark; `WithTerminalInvert` swaps the polarity for dark-background terminals so the rendered dark modules still read as dark to a camera. v0.8 emits plain text with no ANSI colour; a theme-independent ANSI mode that honours `WithColors` is noted as a future enhancement, not part of this release.
5. **Quiet zone reused.** `WithQuietZone` applies unchanged (default 4 light modules on all sides), which the scanner needs. `WithModuleSize` and `WithColors` have no effect on the plain-text output and are documented as no-ops here, exactly as they are for `Matrix`, reserved for the future ANSI mode.
6. **Tests first, including a parse-back round-trip.** Beyond golden strings that pin the exact visual output, a round-trip test renders the matrix, parses the block characters back into a `[][]bool` grid, runs it through `DecodeMatrix`, and asserts the decoded text equals the input. This proves the rendering is loss-free and correctly oriented without needing an image decoder.

## 3. Scope

### In scope for v0.8.0

- `EncodeTerminal(text string, opts ...Option) (string, error)` — half-block Unicode rendering of the encoded symbol, including the quiet zone.
- `WithTerminalInvert(bool) Option` — swap dark/light polarity for dark-background terminals.
- `WithTerminalASCII(bool) Option` — ASCII double-width fallback (`##` / spaces) for terminals without block-element support.
- `WithQuietZone` honoured on terminal output; `WithModuleSize` and `WithColors` documented as no-ops for terminal output (like `Matrix`), reserved for a future ANSI mode.
- CLI: `-format terminal` (Unicode half-block) and `-format ascii` (ASCII fallback), plus an `-invert` flag, writing to stdout or `-out`.
- Reference doc `docs/theory/19-terminal-rendering.md` (EN + ID) covering the cell aspect ratio, half-block packing, the block glyphs, ASCII fallback, polarity/inversion, quiet zone, and scannability from a screen, with citations.
- A runnable example `examples/encode/terminal/main.go` and a README usage section.

### Out of scope (still)

- **ANSI colour output.** A theme-independent rendering that emits foreground/background escape codes honouring `WithColors`. Deferred to a possible follow-up; v0.8 is plain text plus the `invert` switch.
- **`WithModuleSize` zoom on terminal.** Repeating modules horizontally/vertically to enlarge the terminal symbol. Possible follow-up; v0.8 fixes one half-block cell per module.
- **Auto-detecting the terminal background or TTY** to pick polarity automatically. The caller chooses with `WithTerminalInvert`; auto-detection belongs in the CLI layer at most, not the library.
- **Inline-image terminal protocols** (iTerm2, Kitty, sixel). Those are image output, not text rendering, and would re-introduce a raster path.
- **An `EncodeTerminalToFile` wrapper.** A `string` is trivially written by the caller or the CLI `-out`, so a dedicated file helper would add surface with no value.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after TR3) gives the renderer with correct, golden-pinned output. **Checkpoint B** (TR5) is the `v0.8.0` release.

### TR1 — Plan Doc `(S)`

- [x] `docs/plan-terminal-renderer.md` and its Indonesian counterpart covering vision, principles, scope, milestones TR1..TR5, file-layout delta, risks, references, and open questions.

### TR2 — Terminal-Rendering Reference Doc `(S)`

Goal: document the rendering technique and its trade-offs before any code lands.

- [ ] `docs/theory/19-terminal-rendering.md` — the terminal cell aspect ratio and why half-block packing keeps modules square; the exact module-pair to glyph mapping (`█ ▀ ▄` and space) with the Unicode Block Elements code points; the ASCII double-width fallback; the polarity problem (a glyph reads dark on a light terminal and light on a dark one) and how `invert` resolves it; the quiet-zone requirement; how an odd total row count leaves a final unpaired module row; and a worked example for a small symbol. Closes with the compose-not-encode rationale and implementation pointers.
- [ ] Indonesian counterpart `docs/theory/19-terminal-rendering.id.md`.
- [ ] Updated `docs/theory/README.md` and `.id.md`: entry 19 plus a code-mapping row pointing at `qrgen/render_terminal.go`.

### TR3 — Renderer + Options + Golden Tests `(M)`

Goal: the renderer itself, with output pinned by golden strings.

- [ ] `qrgen/render_terminal.go` with `renderTerminal(m *matrix, opts terminalOptions) string` (sibling of `renderPNG`/`renderSVG`), implementing half-block packing, the ASCII fallback, the invert switch, and the quiet zone. Handles the trailing unpaired row (top half only) cleanly.
- [ ] `EncodeTerminal` in `qrgen/api.go` and `WithTerminalInvert` / `WithTerminalASCII` in `qrgen/options.go`, threaded through the resolved options.
- [ ] Golden-string tests in `qrgen/render_terminal_test.go`: a pinned-mask symbol rendered in half-block, ASCII, and inverted modes compared against golden multi-line strings; quiet-zone width; the trailing-odd-row case.

### Checkpoint A — the renderer produces correct, stable, scannable terminal output.

### TR4 — Parse-Back Round-Trip + CLI + Example `(S)`

Goal: prove the rendering is loss-free and wire it into the CLI.

- [ ] `TestTerminalRoundTrip` renders a range of payloads, parses the block characters back into `[][]bool`, runs `DecodeMatrix`, and asserts the decoded text equals the input — for half-block, ASCII, and inverted modes.
- [ ] CLI: `-format terminal` and `-format ascii` route through `EncodeTerminal`; an `-invert` flag maps to `WithTerminalInvert`; output goes to stdout by default for these formats. Usage text and examples updated.
- [ ] Runnable example `examples/encode/terminal/main.go` printing a URL QR to stdout in half-block and again inverted; verified with `go run`.

### TR5 — Polish & Release `(S)`

Goal: cut `v0.8.0`.

- [ ] README: a "Terminal output" usage section; a terminal row in the API summary listing `EncodeTerminal`, `WithTerminalInvert`, `WithTerminalASCII`; Scope gains a terminal-output line; the Roadmap "Additional renderers" bullet notes terminal/ASCII shipped in v0.8 (JPEG/PDF still open).
- [ ] `CHANGELOG.md` `v0.8.0` entry plus compare/tag anchors written; left unstaged in the working tree for the maintainer to commit with the release (mirroring v0.6 and v0.7).
- [ ] `go test -race ./...` clean, gofmt-clean.
- [ ] Tag `v0.8.0` (left for the maintainer per the established git/release workflow; annotation recommended in the release conversation).

---

## 5. Proposed File Layout Delta

```
qrgen/
├── render_terminal.go        # new — renderTerminal(m, opts) string
├── render_terminal_test.go   # new — golden + parse-back round-trip
├── api.go                    # +EncodeTerminal
├── options.go                # +WithTerminalInvert, +WithTerminalASCII (+fields)
cmd/qrgen/
└── main.go                   # +terminal/ascii formats, +-invert flag
docs/
├── plan-terminal-renderer.md     # this file
├── plan-terminal-renderer.id.md  # Indonesian counterpart
└── theory/
    ├── 19-terminal-rendering.md     # new
    └── 19-terminal-rendering.id.md  # new
examples/encode/terminal/
└── main.go                   # new — half-block + inverted demo to stdout
```

## 6. Risks & Technical Notes

- **Polarity is the whole game for scannability.** A block glyph reads dark on a light-background terminal but light on a dark-background one; if the polarity is wrong, the rendered dark modules look light to a camera and the symbol will not scan. The default targets a light background; `WithTerminalInvert` covers dark backgrounds. This is the terminal analogue of the contrast guidance on `WithColors`, and it is documented prominently.
- **Half-block font support.** `█ ▀ ▄` are Unicode Block Elements (U+2588, U+2580, U+2584), supported by every modern monospace font and terminal. The ASCII fallback (`WithTerminalASCII`) covers the rare environment that lacks them.
- **Odd total row count.** A QR symbol side is always odd (21, 25, ... 177), and adding a symmetric quiet zone keeps the total odd. Half-block packs rows in pairs, so the last row is unpaired and renders as a top-half glyph (`▀`) over an implicit light bottom. The renderer handles this trailing case explicitly so the bottom edge is correct.
- **Aspect ratio.** A terminal cell is roughly twice as tall as it is wide. Half-block splits each cell into a top and bottom half, giving each module a near-square footprint; the ASCII fallback doubles width per module to achieve the same. Both keep the symbol visually square so it scans cleanly.
- **No anti-aliasing or scaling concerns.** Unlike the PNG path, text rendering is exact: a module is either a glyph or a space, so there is no binarisation ambiguity for a camera beyond the polarity choice.
- **Round-trip without an image.** Parsing the rendered glyphs back into `[][]bool` and calling `DecodeMatrix` validates correctness directly, sidestepping the image decoder; the parse-back must exactly invert the char-to-module mapping for each mode (half-block, ASCII, inverted), which is itself a useful test of the mapping's clarity.

## 7. References

- Unicode Block Elements, U+2580–U+259F — the half and full block glyphs (`▀` U+2580, `▄` U+2584, `█` U+2588).
- De-facto terminal-QR conventions — `qrencode -t UTF8` / `UTF8i` and the `qrterminal` Go library, which establish half-block packing and the invert switch for dark terminals.
- ISO/IEC 18004:2015 — the QR symbology itself; terminal rendering is an output target layered above it, like PNG and SVG.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **Default character set.** Unicode half-block (chosen: compact, near-square, universally supported in modern terminals) versus full-block double-width (maximum compatibility but twice as tall). The ASCII fallback already covers non-Unicode environments, so half-block is the default. Confirm in TR2.
- **Return type.** `string` (chosen, see principle 1) versus `[]byte` for symmetry with `Encode`/`EncodeSVG`. Text output favours `string`.
- **Polarity default.** Assume a light-background terminal plus an `invert` switch (chosen) versus emitting ANSI colour to be theme-independent (deferred to a follow-up). Confirm in TR2.
- **CLI tokens.** `-format terminal` (Unicode) plus `-format ascii` (ASCII) plus an `-invert` flag (proposed) versus a single `-format terminal` with a separate `-ascii` flag. Two format tokens read more clearly on the command line.
- **Trailing newline.** Whether `EncodeTerminal` terminates the last row with a newline. Proposed: yes, every row including the last is newline-terminated so the output prints cleanly and concatenates predictably.
