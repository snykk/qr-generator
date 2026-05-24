// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"image"
	"math"
)

// Sauvola defaults are the textbook choices from Sauvola & Pietikainen (2000)
// and the integral-image acceleration paper by Shafait et al. (2008). They
// stay unexported in v0.3; promote to functional options only when real-world
// inputs ask for tuning. See docs/theory/14-adaptive-thresholding.md sections
// 3 and 4.
const (
	sauvolaWindow = 25    // square window side length
	sauvolaK      = 0.2   // standard "paper" tuning constant
	sauvolaR      = 128.0 // dynamic-range normaliser for 8-bit images
)

// Dispatch tuning for the Otsu-or-Sauvola gate used by decodeImage. The
// proactive bimodality gate (etaMin) is documented in
// docs/theory/14-adaptive-thresholding.md §6.
const etaMin = 0.5

// binariserUsedState records which branch of the Otsu-or-Sauvola dispatch in
// decodeImage actually produced the bitmap that finder detection ran on. The
// production decodeImage entry point throws this away; decodeImageDebug
// returns it so package-internal tests can assert the dispatch behaviour
// without exposing any of this on the public API.
type binariserUsedState int

const (
	binariserOtsu binariserUsedState = iota
	binariserSauvolaProactive
	binariserSauvolaReactive
)

func (s binariserUsedState) String() string {
	switch s {
	case binariserOtsu:
		return "otsu"
	case binariserSauvolaProactive:
		return "sauvola-proactive"
	case binariserSauvolaReactive:
		return "sauvola-reactive"
	default:
		return "unknown"
	}
}

// sauvolaBinarise applies the Sauvola local adaptive thresholding algorithm
// to img and returns a bitmap with the same p <= t foreground convention used
// by binarise. The implementation uses two integral images (sum and sum of
// squares) so the per-pixel cost stays constant regardless of the window
// size. See docs/theory/14-adaptive-thresholding.md.
func sauvolaBinarise(img image.Image) *bitmap {
	pixels, w, h := imageToGrayscale(img)
	return sauvolaBinariseFromGray(pixels, w, h)
}

// sauvolaBinariseFromGray runs Sauvola on a flat grayscale buffer. Split out
// so the dispatch in decodeImage (planned for T3) can reuse the grayscale
// pixels already computed for the Otsu pass without re-walking the image.
func sauvolaBinariseFromGray(pixels []uint8, w, h int) *bitmap {
	bm := &bitmap{width: w, height: h, pixels: make([]bool, w*h)}
	if w == 0 || h == 0 {
		return bm
	}
	sum, sum2 := buildIntegralImages(pixels, w, h)
	half := sauvolaWindow / 2
	for y := range h {
		for x := range w {
			mean, std := windowMeanStd(sum, sum2, w, h, x, y, half)
			threshold := mean * (1.0 + sauvolaK*(std/sauvolaR-1.0))
			bm.pixels[y*w+x] = float64(pixels[y*w+x]) <= threshold
		}
	}
	return bm
}

// buildIntegralImages returns the integral images of pixel values and
// squared pixel values for the given grayscale buffer. Both are sized
// (w+1) x (h+1) with a zero-filled first row and column so windowMeanStd
// can subtract neighbour cells without bounds checks.
//
// uint64 is required: a 4096x4096 grayscale image sums to about 4.3e9 for
// the linear integral and 1.1e12 for the squared integral, both of which
// blow uint32 but fit comfortably in uint64. See doc 14 section 4.
func buildIntegralImages(pixels []uint8, w, h int) (sum, sum2 []uint64) {
	stride := w + 1
	sum = make([]uint64, stride*(h+1))
	sum2 = make([]uint64, stride*(h+1))
	for y := 1; y <= h; y++ {
		var rowSum, rowSum2 uint64
		base := (y - 1) * w
		for x := 1; x <= w; x++ {
			v := uint64(pixels[base+(x-1)])
			rowSum += v
			rowSum2 += v * v
			sum[y*stride+x] = sum[(y-1)*stride+x] + rowSum
			sum2[y*stride+x] = sum2[(y-1)*stride+x] + rowSum2
		}
	}
	return sum, sum2
}

// windowMeanStd returns the mean and standard deviation of pixels inside a
// square window of half-extent `half` centred at (x, y), clipped at the
// image bounds. The integral images are (w+1) x (h+1); the closed rectangle
// in pixel coordinates [(x0, y0), (x1-1, y1-1)] corresponds to integral
// corners S[y1][x1], S[y0][x1], S[y1][x0], S[y0][x0].
func windowMeanStd(sum, sum2 []uint64, w, h, x, y, half int) (mean, std float64) {
	x0 := x - half
	if x0 < 0 {
		x0 = 0
	}
	y0 := y - half
	if y0 < 0 {
		y0 = 0
	}
	x1 := x + half + 1
	if x1 > w {
		x1 = w
	}
	y1 := y + half + 1
	if y1 > h {
		y1 = h
	}
	stride := w + 1
	s := sum[y1*stride+x1] - sum[y0*stride+x1] - sum[y1*stride+x0] + sum[y0*stride+x0]
	s2 := sum2[y1*stride+x1] - sum2[y0*stride+x1] - sum2[y1*stride+x0] + sum2[y0*stride+x0]
	area := float64((x1 - x0) * (y1 - y0))
	mean = float64(s) / area
	variance := float64(s2)/area - mean*mean
	if variance < 0 {
		variance = 0
	}
	std = math.Sqrt(variance)
	return mean, std
}
