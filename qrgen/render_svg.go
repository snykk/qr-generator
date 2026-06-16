// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// renderSVG rasterises m into a scalable SVG document per opts. It is the
// sibling of renderPNG: identical signature, same renderOptions struct, no
// shared interface (see docs/theory/16-svg-rendering.md section 7). The
// symbol is drawn as a single background rect covering the full canvas plus a
// single foreground path whose subpaths are one unit square per dark module.
//
// The viewBox is in module units (0 0 D D where D = size + 2*quietZone) so the
// symbol scales cleanly to any size, while width/height default to
// moduleSize*D pixels so an SVG and a PNG built with the same options describe
// the same nominal dimensions. shape-rendering="crispEdges" disables
// anti-aliasing so module boundaries stay decodable.
func renderSVG(m *matrix, opts renderOptions) ([]byte, error) {
	opts = opts.withDefaults()
	if m == nil {
		return nil, fmt.Errorf("qrgen: renderSVG: nil matrix")
	}

	n := m.size
	dim := n + 2*opts.quietZone // canvas side in module units
	if dim <= 0 {
		return nil, fmt.Errorf("qrgen: renderSVG: invalid dimension %d", dim)
	}
	side := opts.moduleSize * dim // canvas side in pixels
	if side <= 0 {
		return nil, fmt.Errorf("qrgen: renderSVG: invalid side length %d", side)
	}

	fgHex, fgOpacity := colorToHex(opts.foreground)
	bgHex, bgOpacity := colorToHex(opts.background)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges">`+"\n",
		side, side, dim, dim)

	// Background rect over the whole canvas, including the quiet zone.
	b.WriteString("  ")
	b.WriteString(`<rect width="`)
	b.WriteString(strconv.Itoa(dim))
	b.WriteString(`" height="`)
	b.WriteString(strconv.Itoa(dim))
	b.WriteString(`" fill="`)
	b.WriteString(bgHex)
	b.WriteString(`"`)
	writeOpacity(&b, "fill-opacity", bgOpacity)
	b.WriteString("/>\n")

	// Foreground path: one closed unit square per dark module, offset by the
	// quiet zone. Absolute moveto then relative h/v/h, z to close.
	var d strings.Builder
	q := opts.quietZone
	for r := 0; r < n; r++ {
		row := m.modules[r]
		for c := 0; c < n; c++ {
			if !row[c] {
				continue
			}
			fmt.Fprintf(&d, "M%d %dh1v1h-1z", c+q, r+q)
		}
	}
	if d.Len() > 0 {
		b.WriteString("  ")
		b.WriteString(`<path d="`)
		b.WriteString(d.String())
		b.WriteString(`" fill="`)
		b.WriteString(fgHex)
		b.WriteString(`"`)
		writeOpacity(&b, "fill-opacity", fgOpacity)
		b.WriteString("/>\n")
	}

	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}

// writeOpacity emits ` name="value"` only when opacity is below full (1.0), so
// the common fully-opaque case keeps the document minimal. The value is
// formatted with the shortest decimal that round-trips.
func writeOpacity(b *strings.Builder, name string, opacity float64) {
	if opacity >= 1 {
		return
	}
	b.WriteString(` `)
	b.WriteString(name)
	b.WriteString(`="`)
	b.WriteString(strconv.FormatFloat(opacity, 'g', 4, 64))
	b.WriteString(`"`)
}

// colorToHex converts a color.Color to a CSS-style #RRGGBB string plus a
// fractional opacity in [0, 1]. color.Color.RGBA returns alpha-premultiplied
// 16-bit channels; dividing each by the alpha recovers the straight colour and
// scales it to 8 bits in one step (r*0xff/a == (R16*a/0xffff)*0xff/a ==
// R16/0x101 == R8). A fully transparent colour has no meaningful hue, so it
// reports black at opacity 0. See docs/theory/16-svg-rendering.md section 6.
func colorToHex(c color.Color) (string, float64) {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return "#000000", 0
	}
	r8 := uint8(r * 0xff / a)
	g8 := uint8(g * 0xff / a)
	b8 := uint8(b * 0xff / a)
	return fmt.Sprintf("#%02X%02X%02X", r8, g8, b8), float64(a) / 0xffff
}
