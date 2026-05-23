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
	"math"
	"testing"
)

func TestBuildIntegralImages(t *testing.T) {
	// 3 x 2 image, easy to hand-check.
	//   1 2 3
	//   4 5 6
	pixels := []uint8{1, 2, 3, 4, 5, 6}
	w, h := 3, 2
	sum, sum2 := buildIntegralImages(pixels, w, h)
	stride := w + 1

	// Helper for table access.
	at := func(s []uint64, x, y int) uint64 { return s[y*stride+x] }

	cases := []struct {
		x, y           int
		wantSum, wantS2 uint64
	}{
		{0, 0, 0, 0},
		{1, 1, 1, 1},
		{3, 1, 1 + 2 + 3, 1 + 4 + 9},
		{2, 2, 1 + 2 + 4 + 5, 1 + 4 + 16 + 25},
		{3, 2, 1 + 2 + 3 + 4 + 5 + 6, 1 + 4 + 9 + 16 + 25 + 36},
	}
	for _, c := range cases {
		if got := at(sum, c.x, c.y); got != c.wantSum {
			t.Errorf("sum at (%d, %d) = %d, want %d", c.x, c.y, got, c.wantSum)
		}
		if got := at(sum2, c.x, c.y); got != c.wantS2 {
			t.Errorf("sum2 at (%d, %d) = %d, want %d", c.x, c.y, got, c.wantS2)
		}
	}
}

func TestWindowMeanStdMatchesNaive(t *testing.T) {
	// Random-ish 12 x 10 image; check the integral-image-based mean and std
	// against a direct O(w^2) computation over the same window.
	const w, h = 12, 10
	pixels := make([]uint8, w*h)
	for i := range pixels {
		pixels[i] = uint8((i*37 + 11) & 0xFF)
	}
	sum, sum2 := buildIntegralImages(pixels, w, h)

	naive := func(cx, cy, half int) (mean, std float64) {
		x0, y0 := cx-half, cy-half
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		x1, y1 := cx+half+1, cy+half+1
		if x1 > w {
			x1 = w
		}
		if y1 > h {
			y1 = h
		}
		var s, s2 float64
		var n int
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				p := float64(pixels[y*w+x])
				s += p
				s2 += p * p
				n++
			}
		}
		mean = s / float64(n)
		variance := s2/float64(n) - mean*mean
		if variance < 0 {
			variance = 0
		}
		return mean, math.Sqrt(variance)
	}

	for _, half := range []int{1, 3, 5, 12} {
		for cy := range h {
			for cx := range w {
				wantM, wantS := naive(cx, cy, half)
				gotM, gotS := windowMeanStd(sum, sum2, w, h, cx, cy, half)
				if math.Abs(wantM-gotM) > 1e-9 {
					t.Errorf("half=%d (%d,%d): mean got %g, want %g", half, cx, cy, gotM, wantM)
				}
				if math.Abs(wantS-gotS) > 1e-6 {
					t.Errorf("half=%d (%d,%d): std got %g, want %g", half, cx, cy, gotS, wantS)
				}
			}
		}
	}
}

func TestSauvolaUniformImageStaysLight(t *testing.T) {
	// Uniform mid-gray. In a region with no contrast, Sauvola's std → 0, so
	// threshold = mean * (1 + k * (0 - 1)) = mean * (1 - k) = 0.8 * mean.
	// Every pixel equals mean, so every pixel is strictly greater than the
	// threshold and gets classified as light (false). No spurious black
	// speckles allowed; that is precisely the noise problem Sauvola fixes
	// over Niblack and the property we lean on in the proactive gate at T3.
	const w, h = 40, 30
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 200
	}

	bm := sauvolaBinarise(img)
	if bm.width != w || bm.height != h {
		t.Fatalf("size = %dx%d, want %dx%d", bm.width, bm.height, w, h)
	}
	for i, dark := range bm.pixels {
		if dark {
			t.Fatalf("uniform image produced dark pixel at index %d", i)
		}
	}
}

func TestSauvolaResolvesTwoIlluminationRegions(t *testing.T) {
	// Two halves with very different illumination but the same logical
	// ink/paper layout. A single global Otsu threshold cannot separate ink
	// from paper in both halves at once. Sauvola's local window can.
	//
	//   left  half: paper = 80,  ink = 20
	//   right half: paper = 220, ink = 140
	//
	// The four 3x3 ink patches sit two per side at known positions; we then
	// assert that every ink-patch pixel is dark and every clean paper pixel
	// is light after sauvolaBinarise. The asserted points were picked to be
	// well clear of patch edges so the local window has enough ink+paper
	// samples to compute a meaningful std.
	const w, h = 60, 60
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			var v uint8 = 80
			if x >= 30 {
				v = 220
			}
			img.Pix[y*img.Stride+x] = v
		}
	}
	// Drop 3x3 ink patches at four known centres.
	type pt struct{ x, y int }
	inkLeft := []pt{{12, 15}, {12, 45}}
	inkRight := []pt{{45, 15}, {45, 45}}
	paintPatch := func(cx, cy int, v uint8) {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				img.Pix[(cy+dy)*img.Stride+(cx+dx)] = v
			}
		}
	}
	for _, p := range inkLeft {
		paintPatch(p.x, p.y, 20)
	}
	for _, p := range inkRight {
		paintPatch(p.x, p.y, 140)
	}

	bm := sauvolaBinarise(img)

	// Ink centres must come back dark.
	for _, p := range append(inkLeft, inkRight...) {
		if !bm.get(p.x, p.y) {
			t.Errorf("ink at (%d, %d) classified as light", p.x, p.y)
		}
	}
	// Paper sample points (well away from ink and edges) must come back light.
	paperPoints := []pt{{12, 30}, {17, 15}, {45, 30}, {50, 15}}
	for _, p := range paperPoints {
		if bm.get(p.x, p.y) {
			t.Errorf("paper at (%d, %d) classified as dark", p.x, p.y)
		}
	}

	// Document the regression value of the test: on this same image, the
	// global Otsu binariser must fail in at least one direction. Otherwise
	// the fixture is not exercising the Sauvola-vs-Otsu gap we care about
	// and any "Sauvola passes" result would be uninformative.
	otsu := binarise(img)
	otsuWrong := false
	for _, p := range append(inkLeft, inkRight...) {
		if !otsu.get(p.x, p.y) {
			otsuWrong = true
			break
		}
	}
	if !otsuWrong {
		for _, p := range paperPoints {
			if otsu.get(p.x, p.y) {
				otsuWrong = true
				break
			}
		}
	}
	if !otsuWrong {
		t.Fatal("test fixture failed to demonstrate Otsu's two-region failure mode; Sauvola correctness here is not a meaningful guard")
	}
}

func TestSauvolaSmallerThanWindow(t *testing.T) {
	// 5 x 5 is well under the 25-pixel default window. The windowMeanStd
	// clipping should kick in for every pixel without panicking.
	img := image.NewGray(image.Rect(0, 0, 5, 5))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 9)
	}
	bm := sauvolaBinarise(img)
	if bm.width != 5 || bm.height != 5 {
		t.Fatalf("size = %dx%d, want 5x5", bm.width, bm.height)
	}
	if len(bm.pixels) != 25 {
		t.Fatalf("pixels len = %d, want 25", len(bm.pixels))
	}
}

func TestSauvolaZeroSizedImage(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 0, 0))
	bm := sauvolaBinarise(img)
	if bm.width != 0 || bm.height != 0 {
		t.Fatalf("size = %dx%d, want 0x0", bm.width, bm.height)
	}
	if len(bm.pixels) != 0 {
		t.Fatalf("pixels len = %d, want 0", len(bm.pixels))
	}
}

func TestOtsuEtaIsZeroForUniform(t *testing.T) {
	// A flat-grey image has no between-class variance to maximise and zero
	// total variance to normalise by. The dispatch reads η = 0 here as the
	// signal to skip Otsu and let Sauvola take over.
	pixels := make([]uint8, 256)
	for i := range pixels {
		pixels[i] = 200
	}
	t_, eta := otsuThreshold(pixels)
	if eta != 0 {
		t.Errorf("eta for uniform = %g, want 0", eta)
	}
	if t_ != 128 {
		t.Errorf("threshold fallback = %d, want 128", t_)
	}
}

func TestForegroundRatio(t *testing.T) {
	bm := &bitmap{width: 10, height: 10, pixels: make([]bool, 100)}
	if got := foregroundRatio(bm); got != 0 {
		t.Errorf("all-light ratio = %g, want 0", got)
	}
	for i := range 25 {
		bm.pixels[i] = true
	}
	if got := foregroundRatio(bm); got != 0.25 {
		t.Errorf("25/100 ratio = %g, want 0.25", got)
	}
	empty := &bitmap{}
	if got := foregroundRatio(empty); got != 0 {
		t.Errorf("empty bitmap ratio = %g, want 0", got)
	}
}

// applyPixelTransform maps img to an image.Gray with each pixel rewritten by
// fn(x, y, originalGray). Used by the dispatch tests below to mutate a clean
// QR PNG into the failure modes Sauvola is supposed to rescue.
func applyPixelTransform(img image.Image, fn func(x, y int, v uint8) uint8) *image.Gray {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.GrayModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
			out.Pix[y*out.Stride+x] = fn(x, y, c.Y)
		}
	}
	return out
}

func TestDispatchUsesOtsuOnCleanPNG(t *testing.T) {
	const payload = "HELLO"
	pngBytes, err := Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	text, state, err := decodeImageDebug(img)
	if err != nil {
		t.Fatalf("decodeImageDebug: %v", err)
	}
	if text != payload {
		t.Errorf("payload = %q, want %q", text, payload)
	}
	if state != binariserOtsu {
		t.Errorf("state = %v, want binariserOtsu", state)
	}
}

func TestDispatchUsesSauvolaProactiveOnMonochrome(t *testing.T) {
	// The textbook lower bound for Otsu's separability η on a non-degenerate
	// distribution is around 0.64 (Gaussian) or 0.75 (uniform); η genuinely
	// drops below the etaMin gate only when the input collapses to a single
	// delta-like peak. A monochrome image is the canonical case — it
	// triggers the "total variance is zero" branch where we return η = 0 by
	// convention. This regression-tests that the dispatch then skips Otsu's
	// binarisation pass and lands on Sauvola.
	//
	// A QR is intentionally absent. The decode is expected to fail; what
	// matters is the binariser state on the failure path.
	const w, h = 80, 80
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	pixels, _, _ := imageToGrayscale(img)
	if _, eta := otsuThreshold(pixels); eta >= etaMin {
		t.Fatalf("fixture eta = %g, expected < %g so the proactive gate fires", eta, etaMin)
	}
	_, state, err := decodeImageDebug(img)
	if err == nil {
		t.Fatal("expected decode failure on a fixture with no QR")
	}
	if state != binariserSauvolaProactive {
		t.Errorf("state = %v, want binariserSauvolaProactive", state)
	}
}

func TestDispatchFallsBackToSauvolaReactive(t *testing.T) {
	// Mutate a clean QR with a brightness compression that shifts both ink
	// and paper upward across x, so the right-side ink ends up brighter
	// than the left-side paper. No single Otsu threshold can separate ink
	// from paper any more, but the histogram is still bimodal enough that
	// η stays above etaMin and the proactive gate does not fire. The
	// dispatch must therefore run Otsu first, notice that finder detection
	// fails on its output, and fall back to Sauvola.
	//
	// This test asserts the dispatch transitioned to binariserSauvolaReactive
	// and nothing more; the question of whether Sauvola itself can fully
	// recover this particular fixture is a T4 concern (T4 will add a
	// curated synthetic suite that the decoder can round-trip).
	pngBytes, err := Encode("HELLO")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	clean, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	w := clean.Bounds().Dx()
	mutated := applyPixelTransform(clean, func(x, _ int, v uint8) uint8 {
		const slope = 110.0 / 255.0
		offset := 30.0 + 120.0*float64(x)/float64(w-1)
		out := slope*float64(v) + offset
		if out < 0 {
			out = 0
		}
		if out > 255 {
			out = 255
		}
		return uint8(out)
	})

	// Confirm Otsu alone really fails on this fixture; otherwise the Sauvola
	// fallback assertion would be vacuous.
	if _, ferr := findFinders(binarise(mutated)); ferr == nil {
		t.Fatal("fixture too easy: Otsu alone located finders, so the reactive-state assertion below would not prove a fallback happened")
	}
	// Confirm the proactive gate stays open — i.e. η ≥ etaMin — so the
	// reactive path is the one that fires, not the proactive one.
	pixels, _, _ := imageToGrayscale(mutated)
	if _, eta := otsuThreshold(pixels); eta < etaMin {
		t.Fatalf("fixture eta = %g < %g; reactive branch would never run", eta, etaMin)
	}

	_, state, _ := decodeImageDebug(mutated)
	if state != binariserSauvolaReactive {
		t.Errorf("state = %v, want binariserSauvolaReactive", state)
	}
}
