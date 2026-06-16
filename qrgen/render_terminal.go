// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "strings"

// Half-block glyphs from the Unicode Block Elements range. See
// docs/theory/19-terminal-rendering.md section 3.
const (
	blockFull  = '█' // U+2588 FULL BLOCK: both module halves dark
	blockUpper = '▀' // U+2580 UPPER HALF BLOCK: top dark, bottom light
	blockLower = '▄' // U+2584 LOWER HALF BLOCK: top light, bottom dark
	blockSpace = ' ' // both halves light

	asciiDark  = "##" // ASCII fallback: one dark module, double-width
	asciiLight = "  " // ASCII fallback: one light module, double-width
)

// terminalOptions controls how a finished matrix is rendered to a terminal
// string. quietZone is the light border in modules; invert swaps the
// dark/light polarity for dark-background terminals; ascii selects the
// double-width ASCII fallback instead of Unicode half-block glyphs. Its sole
// caller is EncodeTerminal. See docs/theory/19-terminal-rendering.md.
type terminalOptions struct {
	quietZone int
	invert    bool
	ascii     bool
}

// withDefaults fills a negative quiet zone with the spec default so callers can
// pass a zero-value struct. invert and ascii default to false.
func (o terminalOptions) withDefaults() terminalOptions {
	if o.quietZone < 0 {
		o.quietZone = defaultQuietZone
	}
	return o
}

// renderTerminal renders m to a multi-line string of block characters. It is
// the sibling of renderPNG and renderSVG (doc 16 section 7) but returns text
// rather than bytes, because terminal output is human-facing and meant to be
// printed.
//
// By default it packs two module rows per text row with half-block glyphs so
// modules stay near-square against a terminal cell's roughly 2:1 aspect ratio;
// opts.ascii switches to a one-row-per-module double-width form. opts.invert
// swaps polarity for dark-background terminals. The quiet zone is emitted
// around the symbol, and every row including the last ends in a newline. See
// docs/theory/19-terminal-rendering.md.
func renderTerminal(m *matrix, opts terminalOptions) string {
	opts = opts.withDefaults()
	if m == nil {
		return ""
	}
	dim := m.size + 2*opts.quietZone // padded side in modules
	if dim <= 0 {
		return ""
	}
	if opts.ascii {
		return renderTerminalASCII(m, opts, dim)
	}
	return renderTerminalHalfBlock(m, opts, dim)
}

// paintAt reports whether the padded cell (r, c) should be drawn as foreground
// ink. A cell is dark when it falls inside the symbol and the module is set;
// the quiet zone is light. invert flips every in-grid cell so a dark-background
// terminal reads the symbol with correct polarity. Cells outside the padded
// grid — the implicit bottom half below an odd-sized symbol — are never
// painted: they are terminal background, not part of the symbol or quiet zone.
func (o terminalOptions) paintAt(m *matrix, r, c, dim int) bool {
	if r < 0 || r >= dim || c < 0 || c >= dim {
		return false
	}
	dark := false
	mr, mc := r-o.quietZone, c-o.quietZone
	if mr >= 0 && mr < m.size && mc >= 0 && mc < m.size {
		dark = m.modules[mr][mc]
	}
	return dark != o.invert
}

// renderTerminalHalfBlock walks the padded grid two rows at a time, choosing a
// block glyph per column from the top/bottom paint pair. When dim is odd the
// final row pairs with an out-of-grid bottom (paintAt returns false), so the
// trailing row renders as a top-half glyph over a light bottom.
func renderTerminalHalfBlock(m *matrix, opts terminalOptions, dim int) string {
	var b strings.Builder
	for r := 0; r < dim; r += 2 {
		for c := 0; c < dim; c++ {
			top := opts.paintAt(m, r, c, dim)
			bottom := opts.paintAt(m, r+1, c, dim)
			switch {
			case top && bottom:
				b.WriteRune(blockFull)
			case top:
				b.WriteRune(blockUpper)
			case bottom:
				b.WriteRune(blockLower)
			default:
				b.WriteRune(blockSpace)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// renderTerminalASCII renders one text row per module row, doubling width so a
// module stays near-square without the half-block glyphs.
func renderTerminalASCII(m *matrix, opts terminalOptions, dim int) string {
	var b strings.Builder
	for r := 0; r < dim; r++ {
		for c := 0; c < dim; c++ {
			if opts.paintAt(m, r, c, dim) {
				b.WriteString(asciiDark)
			} else {
				b.WriteString(asciiLight)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
