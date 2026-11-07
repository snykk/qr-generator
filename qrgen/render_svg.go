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

	dimW := m.cols() + 2*opts.quietZone // canvas width in module units
	dimH := m.rows() + 2*opts.quietZone // canvas height in module units
	if dimW <= 0 || dimH <= 0 {
		return nil, fmt.Errorf("qrgen: renderSVG: invalid dimensions %dx%d", dimW, dimH)
	}
	width, height := opts.moduleSize*dimW, opts.moduleSize*dimH // in pixels
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("qrgen: renderSVG: invalid size %dx%d", width, height)
	}

	fgHex, fgOpacity := colorToHex(opts.foreground)
	bgHex, bgOpacity := colorToHex(opts.background)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges">`+"\n",
		width, height, dimW, dimH)

	// Background rect over the whole canvas, including the quiet zone.
	b.WriteString("  ")
	b.WriteString(`<rect width="`)
	b.WriteString(strconv.Itoa(dimW))
	b.WriteString(`" height="`)
	b.WriteString(strconv.Itoa(dimH))
	b.WriteString(`" fill="`)
	b.WriteString(bgHex)
	b.WriteString(`"`)
	writeOpacity(&b, "fill-opacity", bgOpacity)
	b.WriteString("/>\n")

	// Foreground path: one closed unit square per dark module, offset by the
	// quiet zone. Absolute moveto then relative h/v/h, z to close.
	var d strings.Builder
	q := opts.quietZone
	for r := 0; r < m.rows(); r++ {
		row := m.modules[r]
		for c := 0; c < m.cols(); c++ {
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

	if opts.logo != nil && opts.logo.Image != nil {
		if err := writeSVGLogo(&b, opts.logo, dimW); err != nil { // logos go only on square QR symbols
			return nil, fmt.Errorf("qrgen: renderSVG: logo: %w", err)
		}
	}

	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}

// writeSVGLogo appends the logo's background pad and a base64 data-URI <image>,
// centred in the dim-by-dim module-unit viewBox. The layout mirrors the PNG
// path (docs/theory/23-logo-embedding.md) via the shared logoScaleFit.
func writeSVGLogo(b *strings.Builder, lg *Logo, dim int) error {
	scale := lg.Scale
	if scale <= 0 {
		scale = defaultLogoScale
	}
	box := float64(dim) * scale
	if box <= 0 {
		return nil
	}
	if bnd := lg.Image.Bounds(); bnd.Dx() <= 0 || bnd.Dy() <= 0 {
		return nil
	}
	lw, lh, pad := logoScaleFit(lg, box)
	c := float64(dim) / 2
	lx, ly := c-lw/2, c-lh/2

	bg := lg.Background
	if bg == nil {
		bg = color.White
	}
	bgHex, bgOpacity := colorToHex(bg)

	uri, err := logoDataURI(lg.Image)
	if err != nil {
		return err
	}

	// Background pad, hugging the logo plus its margin.
	fmt.Fprintf(b, `  <rect x="%s" y="%s" width="%s" height="%s" fill="%s"`,
		svgNum(lx-pad), svgNum(ly-pad), svgNum(lw+2*pad), svgNum(lh+2*pad), bgHex)
	writeOpacity(b, "fill-opacity", bgOpacity)
	b.WriteString("/>\n")

	// The logo image, already sized to its aspect ratio.
	fmt.Fprintf(b, `  <image x="%s" y="%s" width="%s" height="%s" preserveAspectRatio="none" href="%s"/>`+"\n",
		svgNum(lx), svgNum(ly), svgNum(lw), svgNum(lh), uri)
	return nil
}

// svgNum formats a coordinate as the shortest decimal without an exponent.
func svgNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
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
