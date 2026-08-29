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
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// logoScaleFit returns the logo's width, height, and background-pad margin, all
// in the same unit as box (the logo bounding-box side = reference width times
// the scale), fitting the logo inside a box-by-box square while preserving its
// aspect ratio. Shared by the PNG and SVG paths so both lay the logo out the
// same way.
func logoScaleFit(lg *Logo, box float64) (lw, lh, pad float64) {
	sb := lg.Image.Bounds()
	sw, sh := float64(sb.Dx()), float64(sb.Dy())
	lw, lh = box, box
	switch {
	case sw > sh:
		lh = box * sh / sw
	case sh > sw:
		lw = box * sw / sh
	}
	padFrac := lg.Padding
	if padFrac == 0 {
		padFrac = defaultLogoPadding
	}
	if padFrac < 0 {
		padFrac = 0
	}
	return lw, lh, box * padFrac
}

// logoDataURI encodes the logo as a base64 PNG data URI for embedding in an SVG
// <image>, keeping the document a single self-contained file.
func logoDataURI(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Logo configures a centred logo overlaid on the rendered symbol via [WithLogo].
// Only Image is required; the zero value of each other field selects a safe
// default. See docs/theory/23-logo-embedding.md.
type Logo struct {
	Image      image.Image // the logo to overlay (required)
	Scale      float64     // fraction of the symbol width; 0 selects the default 0.2
	Padding    float64     // background-pad margin as a fraction of the logo box; 0 selects 0.12
	Background color.Color // pad colour; nil selects white
}

// Logo presentation defaults. A 20%-width logo occludes about 4% of the modules
// by area, well inside the EC-H recovery budget the logo forces; the pad adds a
// small local quiet zone. See docs/theory/23-logo-embedding.md.
const (
	defaultLogoScale   = 0.2
	defaultLogoPadding = 0.12
)

// drawLogo composites lg onto the side-by-side RGBA image dst: a background pad
// filled first, then the aspect-preserving scaled logo drawn over it, both
// centred. It is a no-op for a nil or empty logo.
func drawLogo(dst *image.RGBA, lg *Logo, side int) {
	if lg == nil || lg.Image == nil {
		return
	}
	scale := lg.Scale
	if scale <= 0 {
		scale = defaultLogoScale
	}
	box := float64(side) * scale
	if box < 1 {
		return
	}
	if b := lg.Image.Bounds(); b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}

	lwF, lhF, padF := logoScaleFit(lg, box)
	lw, lh, pad := int(lwF), int(lhF), int(padF)
	if lw < 1 {
		lw = 1
	}
	if lh < 1 {
		lh = 1
	}

	bg := lg.Background
	if bg == nil {
		bg = color.White
	}

	cx, cy := side/2, side/2
	lx, ly := cx-lw/2, cy-lh/2

	// Background pad, hugging the logo plus its margin.
	padRect := image.Rect(lx-pad, ly-pad, lx+lw+pad, ly+lh+pad)
	draw.Draw(dst, padRect.Intersect(dst.Bounds()), image.NewUniform(bg), image.Point{}, draw.Src)

	// The scaled logo, composited over the pad with its own alpha.
	scaled := resample(lg.Image, lw, lh)
	logoRect := image.Rect(lx, ly, lx+lw, ly+lh)
	draw.Draw(dst, logoRect.Intersect(dst.Bounds()), scaled, image.Point{}, draw.Over)
}

// resample scales src to dstW-by-dstH pixels with a box filter: each
// destination pixel averages the source pixels that fall within its footprint.
// When the destination is larger than the source the footprint collapses to one
// pixel, so upscaling degrades gracefully to nearest-neighbour. It works in
// alpha-premultiplied space (the form image.Image.RGBA and color.RGBA both use),
// so compositing stays correct without an un-premultiply step. There is no
// external image dependency; the standard library has no scaler.
func resample(src image.Image, dstW, dstH int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw <= 0 || sh <= 0 || dstW <= 0 || dstH <= 0 {
		return dst
	}
	for dy := 0; dy < dstH; dy++ {
		sy0 := dy * sh / dstH
		sy1 := (dy + 1) * sh / dstH
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for dx := 0; dx < dstW; dx++ {
			sx0 := dx * sw / dstW
			sx1 := (dx + 1) * sw / dstW
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, b, a, n uint64
			for yy := sy0; yy < sy1; yy++ {
				for xx := sx0; xx < sx1; xx++ {
					pr, pg, pb, pa := src.At(sb.Min.X+xx, sb.Min.Y+yy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					b += uint64(pb)
					a += uint64(pa)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8((r / n) >> 8),
				G: uint8((g / n) >> 8),
				B: uint8((b / n) >> 8),
				A: uint8((a / n) >> 8),
			})
		}
	}
	return dst
}
