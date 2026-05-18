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
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestImageToGrayscaleDimsAndValues(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.White)
		}
	}
	pixels, w, h := imageToGrayscale(img)
	if w != 4 || h != 4 || len(pixels) != 16 {
		t.Fatalf("dims = %dx%d (len=%d), want 4x4 (16)", w, h, len(pixels))
	}
	for i, p := range pixels {
		if p != 0xFF {
			t.Errorf("pixel %d = %d, want 0xFF", i, p)
		}
	}
}

// TestImageToGrayscaleHandlesNonZeroBounds covers the case where an image's
// bounds do not start at (0, 0) — sub-images are a common source of this.
func TestImageToGrayscaleHandlesNonZeroBounds(t *testing.T) {
	parent := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			if (x+y)%2 == 0 {
				parent.Set(x, y, color.Black)
			} else {
				parent.Set(x, y, color.White)
			}
		}
	}
	sub := parent.SubImage(image.Rect(2, 2, 6, 6))
	pixels, w, h := imageToGrayscale(sub)
	if w != 4 || h != 4 {
		t.Fatalf("dims = %dx%d, want 4x4", w, h)
	}
	// The checkerboard from (2,2) starts on a dark cell (2+2 even).
	wantDark := []bool{
		true, false, true, false,
		false, true, false, true,
		true, false, true, false,
		false, true, false, true,
	}
	for i, p := range pixels {
		isDark := p < 0x80
		if isDark != wantDark[i] {
			t.Errorf("pixel %d (x=%d, y=%d): gray=%d (dark=%v), want dark=%v", i, i%4, i/4, p, isDark, wantDark[i])
		}
	}
}

func TestOtsuThresholdBimodal(t *testing.T) {
	// 50 pixels at 0, 50 pixels at 255. Otsu's variance is tied across every
	// t in [0, 254] for a strictly-bimodal histogram at the extremes, so the
	// returned threshold can land anywhere in that range. The contract we care
	// about is that the threshold cleanly separates the two classes: every
	// value-0 pixel must satisfy `p <= t` and every value-255 pixel must not.
	pixels := make([]uint8, 100)
	for i := 50; i < 100; i++ {
		pixels[i] = 255
	}
	got := otsuThreshold(pixels)
	if got >= 255 {
		t.Errorf("threshold = %d does not separate value-255 pixels from value-0 pixels", got)
	}
}

func TestOtsuThresholdMonochromeDefaults(t *testing.T) {
	// All-zero histogram (empty image) returns the safe default 128.
	if got := otsuThreshold(nil); got != 128 {
		t.Errorf("empty otsu = %d, want 128", got)
	}
	// All same value gives a degenerate histogram; the function still returns a
	// sensible value rather than panicking.
	allZero := make([]uint8, 16)
	if got := otsuThreshold(allZero); got != 128 {
		t.Errorf("all-zero otsu = %d, want 128 (fallback)", got)
	}
}

func TestBinariseAllWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.White)
		}
	}
	bm := binarise(img)
	if bm.width != 8 || bm.height != 8 {
		t.Fatalf("dims = %dx%d, want 8x8", bm.width, bm.height)
	}
	for _, p := range bm.pixels {
		if p {
			t.Error("all-white image binarised any dark pixel")
			break
		}
	}
}

func TestBinariseAllBlack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.Black)
		}
	}
	bm := binarise(img)
	for _, p := range bm.pixels {
		if !p {
			t.Error("all-black image binarised any light pixel")
			break
		}
	}
}

// TestBinariseRoundTripsEncoderPNG is the integration check: feed a real
// encoder PNG back into the binariser and sample each module's centre. The
// dark/light verdict at every sample must match the original [][]bool.
func TestBinariseRoundTripsEncoderPNG(t *testing.T) {
	const moduleSize = 8
	const quietZone = 4
	const text = "HELLO WORLD"

	grid, err := Matrix(text)
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	pngBytes, err := Encode(text)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}

	bm := binarise(img)
	wantSide := moduleSize * (len(grid) + 2*quietZone)
	if bm.width != wantSide || bm.height != wantSide {
		t.Fatalf("bitmap = %dx%d, want %dx%d", bm.width, bm.height, wantSide, wantSide)
	}

	for r := range grid {
		for c := range grid[r] {
			cx := (c+quietZone)*moduleSize + moduleSize/2
			cy := (r+quietZone)*moduleSize + moduleSize/2
			want := grid[r][c]
			got := bm.get(cx, cy)
			if got != want {
				t.Errorf("module (%d,%d) at pixel (%d,%d): got dark=%v, want %v", r, c, cx, cy, got, want)
			}
		}
	}
}
