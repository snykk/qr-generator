// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// Default rendering knobs. See docs/theory/08-rendering.md.
const (
	defaultModuleSize = 8 // pixels per QR module
	defaultQuietZone  = 4 // modules of background on all sides
)

// renderOptions controls how a finished matrix is rasterised to PNG. Zero
// values fall back to the spec-recommended defaults via withDefaults.
type renderOptions struct {
	moduleSize int         // pixels per QR module
	quietZone  int         // modules of background around the symbol
	foreground color.Color // dark module colour (nil = black)
	background color.Color // light module colour (nil = white)
}

// defaultRenderOptions returns the spec-conformant defaults: 8 px modules, a
// 4-module quiet zone, black-on-white.
func defaultRenderOptions() renderOptions {
	return renderOptions{
		moduleSize: defaultModuleSize,
		quietZone:  defaultQuietZone,
		foreground: color.Black,
		background: color.White,
	}
}

// withDefaults fills any zero fields with the defaults so callers can supply
// partial overrides.
func (o renderOptions) withDefaults() renderOptions {
	if o.moduleSize <= 0 {
		o.moduleSize = defaultModuleSize
	}
	if o.quietZone < 0 {
		o.quietZone = defaultQuietZone
	}
	if o.foreground == nil {
		o.foreground = color.Black
	}
	if o.background == nil {
		o.background = color.White
	}
	return o
}

// isMonochromeDefault reports whether the colour pair is plain black-on-white,
// which lets the renderer emit a smaller 8-bit grayscale PNG instead of RGBA.
func (o renderOptions) isMonochromeDefault() bool {
	return colorEqual(o.foreground, color.Black) && colorEqual(o.background, color.White)
}

func colorEqual(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// renderPNG rasterises m into PNG bytes per opts. The output side length is
// opts.moduleSize * (m.size + 2*opts.quietZone) pixels.
func renderPNG(m *matrix, opts renderOptions) ([]byte, error) {
	opts = opts.withDefaults()
	if m == nil {
		return nil, fmt.Errorf("qrgen: renderPNG: nil matrix")
	}

	n := m.size
	side := opts.moduleSize * (n + 2*opts.quietZone)
	if side <= 0 {
		return nil, fmt.Errorf("qrgen: renderPNG: invalid side length %d", side)
	}

	var img image.Image
	if opts.isMonochromeDefault() {
		img = renderGray(m, opts, side)
	} else {
		img = renderRGBA(m, opts, side)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("qrgen: renderPNG: png encode: %w", err)
	}
	return buf.Bytes(), nil
}

// renderGray fills an 8-bit grayscale image with the matrix: dark modules at
// 0x00, light modules and quiet zone at 0xFF.
func renderGray(m *matrix, opts renderOptions, side int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, side, side))
	// Light background.
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	n := m.size
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m.modules[r][c] {
				continue
			}
			fillSquareGray(img, (c+opts.quietZone)*opts.moduleSize, (r+opts.quietZone)*opts.moduleSize, opts.moduleSize, 0x00)
		}
	}
	return img
}

func fillSquareGray(img *image.Gray, x0, y0, size int, v uint8) {
	for y := y0; y < y0+size; y++ {
		row := img.Pix[y*img.Stride+x0 : y*img.Stride+x0+size]
		for i := range row {
			row[i] = v
		}
	}
}

// renderRGBA fills an RGBA image using opts.foreground/opts.background.
func renderRGBA(m *matrix, opts renderOptions, side int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	bg := color.RGBAModel.Convert(opts.background).(color.RGBA)
	fg := color.RGBAModel.Convert(opts.foreground).(color.RGBA)

	// Light background.
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	n := m.size
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m.modules[r][c] {
				continue
			}
			fillSquareRGBA(img, (c+opts.quietZone)*opts.moduleSize, (r+opts.quietZone)*opts.moduleSize, opts.moduleSize, fg)
		}
	}
	return img
}

func fillSquareRGBA(img *image.RGBA, x0, y0, size int, c color.RGBA) {
	for y := y0; y < y0+size; y++ {
		for x := x0; x < x0+size; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
