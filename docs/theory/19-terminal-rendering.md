# Terminal Rendering

The v0.8 renderer adds a **text** output alongside the PNG raster (doc 08) and the SVG vector document (doc 16). Where those write an image, `renderTerminal` writes a multi-line string of block characters that a terminal draws as a scannable symbol and a phone camera reads straight off the screen. This document records the cell aspect ratio that motivates half-block packing, the module-pair to glyph mapping, the ASCII fallback, the polarity problem that decides whether the symbol scans at all, the quiet-zone and odd-row handling, and why the renderer is a sibling function that returns a string.

> Indonesian version: [19-terminal-rendering.id.md](19-terminal-rendering.id.md).

## 1. Why a terminal renderer

- **No file, no viewer.** A QR symbol is often wanted in the same place the program already runs: a shell. Printing it to the terminal lets a setup script show a Wi-Fi join code, a TOTP enrolment, or a pairing URL with no image file written and no image viewer opened.
- **Pipeable text.** The output is a plain `string`, so it composes with redirection, logging, and any text context, exactly the way the payload builders (doc 18) produce plain strings.
- **Scannable from a screen.** A camera does not care whether the dark and light cells are pixels or characters; it only needs square-ish modules, sufficient contrast, and a quiet zone. A terminal can supply all three (sections 2, 5, 6).
- **Not about resolution.** Unlike SVG, the point is not scaling — a terminal symbol is as large as its character grid. The point is immediacy: the symbol appears in the same stream as the program's other output.

## 2. Cell aspect ratio and half-block packing

A monospace terminal cell is not square. It is typically about twice as tall as it is wide (a height-to-width ratio near 2:1). A QR module is square. So the naive rendering — one terminal cell per module — produces a symbol stretched to roughly twice its proper height, which wastes vertical space and can confuse a scanner expecting square modules.

The fix is **half-block packing**: each terminal cell carries a *vertical pair* of modules, a top module and a bottom module, using the Unicode Block Elements that fill the top or bottom half of a cell. Because the cell is about 2:1, each half-cell is about 1:1 — square — so each module lands in a near-square footprint, and the whole symbol is half as tall as the naive grid. This is the same technique `qrencode -t UTF8` and the `qrterminal` library use.

## 3. The module-pair to glyph mapping

For a vertical pair of modules — top `T`, bottom `B`, each either dark or light — the cell glyph is chosen so the drawn (foreground) ink covers exactly the dark halves:

| Top `T` | Bottom `B` | Glyph | Name (code point)            |
|---------|------------|-------|------------------------------|
| dark    | dark       | `█`   | Full Block (U+2588)          |
| dark    | light      | `▀`   | Upper Half Block (U+2580)    |
| light   | dark       | `▄`   | Lower Half Block (U+2584)    |
| light   | light      | (space) | space (U+0020)             |

On a light-background terminal the glyph is painted in the dark foreground colour and the unpainted half shows the light background, so the four cases reproduce the four dark/light module pairs faithfully. A whole symbol is emitted row-pair by row-pair, left to right, one glyph per column, each output line terminated by a newline.

A worked micro-example. Take this 3-by-3 pattern (`X` dark, `.` light), quiet zone omitted for clarity:

```text
X . X
. X .
X . X
```

Three rows is odd, so there is one full pair `(row0, row1)` and a trailing unpaired `row2` (section 6). Reading columns left to right:

- pair `(row0, row1)`: `X/.` -> `▀`, `./X` -> `▄`, `X/.` -> `▀`  gives `▀▄▀`
- tail `row2` with an implicit light bottom: `X` -> `▀`, `.` -> space, `X` -> `▀`  gives `▀ ▀`

```text
▀▄▀
▀ ▀
```

## 4. ASCII fallback

The block glyphs are universally available in modern terminals, but a pure-ASCII mode (`WithTerminalASCII`) covers the rare font or pipe that cannot show them. ASCII has no half-cell glyph, so it reverts to **one terminal row per module row** and restores square modules by doubling the width: each dark module becomes two characters (`##`) and each light module two spaces. A single character per module would render at the cell's native 1:2 width-to-height and look stretched the other way; two characters wide against one cell tall is about 2:2, i.e. square again. The fallback is therefore correct but roughly twice as tall as half-block, which is the cost of dropping the block glyphs.

## 5. Polarity and scannability

This is the one thing that decides whether the symbol scans. A block glyph is painted in the terminal's foreground colour. On a **light-background** terminal that foreground is dark, so a glyph reads as a dark module — correct. On a **dark-background** terminal the foreground is light, so the same glyph reads as a *light* module, inverting the symbol; a scanner then sees the QR with dark and light swapped, and standard decoders will not read it.

The renderer does not try to detect the terminal theme. It defaults to the light-background case and exposes `WithTerminalInvert` for dark backgrounds. Inversion is exactly the complement: every module's dark/light is flipped before the section 3 mapping runs, which swaps `█` with space and `▀` with `▄`, and turns the light quiet zone into painted glyphs. On a dark terminal those painted quiet-zone cells read as light and the swapped modules read with correct polarity, so the camera sees a normal symbol again. This is the terminal analogue of the contrast guidance on `WithColors` (doc 08): the library can place the modules correctly, but the human has to ensure dark actually looks darker than light on their display.

v0.8 emits plain text only. A theme-independent mode that writes ANSI foreground/background escape codes — which would honour `WithColors` and remove the polarity guesswork entirely — is a deliberate follow-up, not part of this release.

## 6. Quiet zone and the odd-row tail

The quiet zone is not optional. A scanner needs a band of light modules around the symbol to lock onto the finder patterns, so `WithQuietZone` (default 4) is honoured here exactly as in the raster renderers; the light border is emitted as spaces (or, under invert, as painted glyphs).

Half-block packing has one structural edge case. A QR symbol side `n` is always odd (21, 25, ... 177), and adding a symmetric quiet zone of `q` modules keeps the total `n + 2q` odd. Pairing rows two at a time therefore always leaves a single unpaired final row. The renderer emits that last row as a top-half glyph over an implicit light bottom: a dark module becomes `▀`, a light module becomes a space. Handling the tail explicitly keeps the bottom edge of the symbol — usually the last rows of the quiet zone — correct rather than dropped or doubled.

## 7. Sibling function returning a string

`renderTerminal` follows the sibling-function pattern established for the renderers (doc 16, section 7): there is no `Render` interface and no runtime format variable. `Encode` calls `renderPNG`, `EncodeSVG` calls `renderSVG`, and `EncodeTerminal` calls `renderTerminal`, each directly. Choosing the output by which function you call keeps every return contract unambiguous.

The one deliberate difference is the return type. `renderPNG` and `renderSVG` return `[]byte` because their payloads are a binary raster and a document meant to be written to a file or socket. `renderTerminal` returns a `string` because its payload is human-facing text meant to be printed; a `string` composes naturally with `fmt.Print` and needs no decoding to inspect in a test. The terminal-specific switches — invert and ASCII — travel in a small `terminalOptions` struct rather than the raster `renderOptions`, because the pixel-oriented fields (`moduleSize`, `foreground`, `background`) have no meaning for text output and are documented as no-ops, the same way they are for `Matrix`.

## 8. Implementation pointers

- `qrgen/render_terminal.go` hosts `renderTerminal(m *matrix, opts terminalOptions) string` plus the small `terminalOptions` struct (`quietZone`, `invert`, `ascii`). It builds the output with `strings.Builder`, walking the padded grid in row pairs and selecting each glyph from the section 3 mapping (or the ASCII forms), with the section 6 tail handled after the pair loop.
- `EncodeTerminal` in `qrgen/api.go` runs the same `resolveOptions -> validate -> buildMatrix` front-half as `Encode`, then calls `renderTerminal` instead of a raster renderer. `WithTerminalInvert` and `WithTerminalASCII` in `qrgen/options.go` thread the two switches through.
- Tests pin the exact output with golden multi-line strings for the half-block, ASCII, and inverted modes, and — because the rendering is loss-free — parse the glyphs back into a `[][]bool` grid and run it through `DecodeMatrix`, asserting the decoded text equals the input. The parse-back is the inverse of the section 3 mapping and doubles as a check that the mapping is unambiguous.

## References

- The Unicode Standard — *Block Elements* (U+2580–U+259F): the half and full block glyphs `▀` (U+2580), `▄` (U+2584), `█` (U+2588). <https://www.unicode.org/charts/PDF/U2580.pdf>
- `qrencode` — the `-t UTF8` and `-t UTF8i` (inverted) terminal output modes, which establish half-block packing and the invert switch for dark terminals. <https://fukuchi.org/works/qrencode/>
- `qrterminal` — a Go library rendering QR codes to the terminal with half-block and invert options, a direct point of comparison. <https://github.com/mdp/qrterminal>
- `docs/theory/08-rendering.md` and `docs/theory/16-svg-rendering.md` — the raster and vector renderers; the quiet-zone requirement and the sibling-function rationale carry over, and the contrast guidance there is the polarity guidance here.
- ISO/IEC 18004:2015 — the QR symbology itself; terminal rendering is an output target layered above it.
