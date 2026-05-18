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
	"image/color"
)

// bitmap is a binary 2D grid of pixels produced by binarising an image.
// Coordinates use (x, y) with (0, 0) at the top-left; the row-major pixels
// slice is true for dark pixels (foreground) and false for light pixels
// (background or quiet zone).
type bitmap struct {
	width, height int
	pixels        []bool
}

// get returns whether the pixel at (x, y) is dark.
func (b *bitmap) get(x, y int) bool { return b.pixels[y*b.width+x] }

// imageToGrayscale flattens img to an 8-bit luminance buffer using the
// ITU-R BT.601 weights via image/color.GrayModel. See
// docs/theory/12-image-processing.md §1.
func imageToGrayscale(img image.Image) (pixels []uint8, width, height int) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	pixels = make([]uint8, w*h)
	for y := range h {
		for x := range w {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			gray := color.GrayModel.Convert(c).(color.Gray)
			pixels[y*w+x] = gray.Y
		}
	}
	return pixels, w, h
}

// otsuThreshold picks the binarisation threshold that maximises between-class
// variance for the given luminance pixels. Standard Otsu's method per
// docs/theory/12-image-processing.md §2; returns 128 as a safe default when
// the histogram is degenerate (all pixels share one value).
func otsuThreshold(pixels []uint8) uint8 {
	var hist [256]int
	for _, p := range pixels {
		hist[p]++
	}
	total := len(pixels)
	if total == 0 {
		return 128
	}

	var sumAll float64
	for i, c := range hist {
		sumAll += float64(i) * float64(c)
	}

	var sumBg float64
	wBg := 0
	var bestVariance float64
	bestT := uint8(128)
	for t := range 256 {
		wBg += hist[t]
		if wBg == 0 {
			continue
		}
		wFg := total - wBg
		if wFg == 0 {
			break
		}
		sumBg += float64(t) * float64(hist[t])
		meanBg := sumBg / float64(wBg)
		meanFg := (sumAll - sumBg) / float64(wFg)
		meanDiff := meanBg - meanFg
		variance := float64(wBg) * float64(wFg) * meanDiff * meanDiff
		if variance > bestVariance {
			bestVariance = variance
			bestT = uint8(t)
		}
	}
	return bestT
}

// binarise converts an image to a bitmap by computing the Otsu threshold and
// classifying each pixel as dark (luminance ≤ threshold) or light. The "≤"
// matches the convention inside otsuThreshold where the dark class C0 at
// threshold t is exactly the set {0, …, t}; using "<" would mark a fully
// black image as having no dark pixels (because Otsu would pick t = 0 for
// strictly-bimodal inputs at the extremes).
//
// See docs/theory/12-image-processing.md §1–2.
func binarise(img image.Image) *bitmap {
	pixels, w, h := imageToGrayscale(img)
	t := otsuThreshold(pixels)
	bm := &bitmap{width: w, height: h, pixels: make([]bool, w*h)}
	for i, p := range pixels {
		bm.pixels[i] = p <= t
	}
	return bm
}
