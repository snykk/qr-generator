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

// TestFindFindersInEncoderPNG runs the row-scan + vertical-check + clustering
// + geometry pipeline against a real encoder PNG and asserts the three
// returned centres land within a pixel or two of where the encoder placed the
// finder patterns. Tolerance accounts for the half-module rounding in the
// scan-line centre estimate.
func TestFindFindersInEncoderPNG(t *testing.T) {
	cases := []struct {
		name              string
		text              string
		opts              []Option
		modules           int // matrix side
		tlX, tlY, trX, trY, blX, blY float64
	}{
		{
			name: "V1 default",
			text: "HELLO WORLD",
			// V1: 21 modules; module 8 px; quiet 4 → centres of finders at
			// modules (3,3), (3,17), (17,3) → pixels (60,60), (172,60), (60,172).
			modules: 21,
			tlX: 60, tlY: 60,
			trX: 172, trY: 60,
			blX: 60, blY: 172,
		},
		{
			name: "V5 forced",
			text: "ABC123ABC123",
			opts: []Option{WithECLevel(ECLevelQ), WithVersion(5)},
			// V5: 37 modules; module 8 px; quiet 4 → centres at modules
			// (3,3), (3,33), (33,3) → pixels (60,60), (300,60), (60,300).
			modules: 37,
			tlX: 60, tlY: 60,
			trX: 300, trY: 60,
			blX: 60, blY: 300,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := Encode(c.text, c.opts...)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("png.Decode: %v", err)
			}
			bm := binarise(img)
			tri, err := findFinders(bm)
			if err != nil {
				t.Fatalf("findFinders: %v", err)
			}
			const tol = 2.0
			checks := []struct {
				name           string
				gotX, gotY     float64
				wantX, wantY   float64
			}{
				{"top-left", tri.topLeft.x, tri.topLeft.y, c.tlX, c.tlY},
				{"top-right", tri.topRight.x, tri.topRight.y, c.trX, c.trY},
				{"bottom-left", tri.bottomLeft.x, tri.bottomLeft.y, c.blX, c.blY},
			}
			for _, ck := range checks {
				dx := ck.gotX - ck.wantX
				dy := ck.gotY - ck.wantY
				if dx < -tol || dx > tol || dy < -tol || dy > tol {
					t.Errorf("%s centre = (%.1f, %.1f), want (%.1f, %.1f) ±%g",
						ck.name, ck.gotX, ck.gotY, ck.wantX, ck.wantY, tol)
				}
			}
			// Module pitch should be ≈ 8 px for the encoder defaults.
			avgPitch := (tri.topLeft.moduleSize + tri.topRight.moduleSize + tri.bottomLeft.moduleSize) / 3.0
			if avgPitch < 7 || avgPitch > 9 {
				t.Errorf("avg module pitch = %.2f, want ~8", avgPitch)
			}
		})
	}
}

func TestFindFindersRejectsNoise(t *testing.T) {
	// All-white image: no finders.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			img.Set(x, y, color.White)
		}
	}
	bm := binarise(img)
	if _, err := findFinders(bm); err == nil {
		t.Error("expected ErrFinderNotFound on all-white image, got nil")
	}
}

func TestFindFindersRejectsTwoFinders(t *testing.T) {
	// Encode a real QR, then erase the bottom-left finder by overpainting
	// that corner of the PNG with white pixels.
	data, err := Encode("HELLO WORLD")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	bm := binarise(img)
	// Wipe the bottom-left 8×8 module finder + separator (16×16 px region
	// starting at the matrix-area's bottom-left corner).
	const moduleSize = 8
	const quietZone = 4
	startX := quietZone * moduleSize
	startY := (14 + quietZone) * moduleSize // V1 bottom-left finder at row 14
	for y := startY; y < startY+8*moduleSize; y++ {
		for x := startX; x < startX+8*moduleSize; x++ {
			bm.pixels[y*bm.width+x] = false
		}
	}
	if _, err := findFinders(bm); err == nil {
		t.Error("expected ErrFinderNotFound after wiping a corner, got nil")
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
