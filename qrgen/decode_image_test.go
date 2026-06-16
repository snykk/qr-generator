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
	got, eta := otsuThreshold(pixels)
	// A threshold of 255 would classify the value-255 pixels as dark too (the
	// `p <= t` convention), collapsing both classes into one. Any t in [0, 254]
	// separates them; only t == 255 fails. (uint8 cannot exceed 255, so this is
	// the complete failure condition — staticcheck SA4003 flags `>= 255` here.)
	if got == 255 {
		t.Errorf("threshold = %d does not separate value-255 pixels from value-0 pixels", got)
	}
	// Perfectly bimodal 50/50 input gives the maximum separability η = 1.
	if eta < 0.99 {
		t.Errorf("eta = %g for perfectly bimodal input, want ≈ 1", eta)
	}
}

func TestOtsuThresholdMonochromeDefaults(t *testing.T) {
	// All-zero histogram (empty image) returns the safe default 128 with η = 0
	// so the dispatch in decodeImage routes it to Sauvola.
	if got, eta := otsuThreshold(nil); got != 128 || eta != 0 {
		t.Errorf("empty otsu = (%d, %g), want (128, 0)", got, eta)
	}
	// All same value gives a degenerate histogram; the function still returns
	// a sensible value rather than panicking, and η is 0 (unimodal).
	allZero := make([]uint8, 16)
	if got, eta := otsuThreshold(allZero); got != 128 || eta != 0 {
		t.Errorf("all-zero otsu = (%d, %g), want (128, 0)", got, eta)
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

// TestComputeHomographyIdentity feeds four source points to themselves and
// asserts the resulting homography is (numerically) the identity.
func TestComputeHomographyIdentity(t *testing.T) {
	pts := [4][2]float64{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	h, err := computeHomography(pts, pts)
	if err != nil {
		t.Fatalf("computeHomography: %v", err)
	}
	for _, p := range []struct{ x, y float64 }{{0, 0}, {5, 5}, {7, 3}, {10, 10}} {
		gotX, gotY := h.apply(p.x, p.y)
		if math.Abs(gotX-p.x) > 1e-9 || math.Abs(gotY-p.y) > 1e-9 {
			t.Errorf("identity h.apply(%v) = (%v, %v), want (%v, %v)", p, gotX, gotY, p.x, p.y)
		}
	}
}

// TestComputeHomographyTranslateAndScale checks a non-trivial mapping where
// module coords (0..10) map to a translated and scaled pixel rectangle.
func TestComputeHomographyTranslateAndScale(t *testing.T) {
	src := [4][2]float64{{0, 0}, {10, 0}, {0, 10}, {10, 10}}
	dst := [4][2]float64{{100, 50}, {180, 50}, {100, 130}, {180, 130}}
	h, err := computeHomography(src, dst)
	if err != nil {
		t.Fatalf("computeHomography: %v", err)
	}
	// Every source corner must map exactly to its destination.
	for i := range 4 {
		gotX, gotY := h.apply(src[i][0], src[i][1])
		if math.Abs(gotX-dst[i][0]) > 1e-6 || math.Abs(gotY-dst[i][1]) > 1e-6 {
			t.Errorf("corner %d: got (%v, %v), want (%v, %v)", i, gotX, gotY, dst[i][0], dst[i][1])
		}
	}
	// A linear interior point: (5, 5) should map to the centre (140, 90).
	gx, gy := h.apply(5, 5)
	if math.Abs(gx-140) > 1e-6 || math.Abs(gy-90) > 1e-6 {
		t.Errorf("interior (5,5) → (%v, %v), want (140, 90)", gx, gy)
	}
}

func TestComputeHomographyDegenerateReturnsError(t *testing.T) {
	// Three of four source points collinear — the system is rank-deficient.
	src := [4][2]float64{{0, 0}, {1, 0}, {2, 0}, {3, 1}}
	dst := [4][2]float64{{0, 0}, {10, 0}, {20, 0}, {30, 10}}
	if _, err := computeHomography(src, dst); err == nil {
		t.Error("expected error for collinear sources, got nil")
	}
}

func TestEstimateBottomRightCompletesParallelogram(t *testing.T) {
	tri := finderTriple{
		topLeft:    finderCandidate{x: 10, y: 20},
		topRight:   finderCandidate{x: 110, y: 20},
		bottomLeft: finderCandidate{x: 10, y: 120},
	}
	brX, brY := estimateBottomRight(tri)
	if brX != 110 || brY != 120 {
		t.Errorf("BR = (%v, %v), want (110, 120)", brX, brY)
	}
}

func TestEstimateVersionFromFinderSpacing(t *testing.T) {
	cases := []struct {
		name    string
		dx      float64
		modSize float64
		want    Version
	}{
		// V1: (21 - 7) = 14 modules between TL and TR centres.
		{"V1 at module 8", 14 * 8, 8, 1},
		// V5: n-7 = 30 modules.
		{"V5 at module 8", 30 * 8, 8, 5},
		// V10: n-7 = 50 modules.
		{"V10 at module 6", 50 * 6, 6, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tri := finderTriple{
				topLeft:    finderCandidate{x: 0, y: 0, moduleSize: c.modSize},
				topRight:   finderCandidate{x: c.dx, y: 0, moduleSize: c.modSize},
				bottomLeft: finderCandidate{x: 0, y: c.dx, moduleSize: c.modSize},
			}
			got, err := estimateVersion(tri)
			if err != nil {
				t.Fatalf("estimateVersion: %v", err)
			}
			if got != c.want {
				t.Errorf("version = %d, want %d", got, c.want)
			}
		})
	}
}

// TestHomographyFromFindersV1SamplesMatrix is the D8+D9+D10 integration test:
// binarise → findFinders → homographyFromFinders → sample every module via
// the homography, then compare each sampled bit to the original matrix.
// V1 has no alignment pattern so the parallelogram BR estimate is exact; for
// V2+ the same chain works as long as the image has no real distortion, which
// our PNG output never does.
func TestHomographyFromFindersV1SamplesMatrix(t *testing.T) {
	const text = "HELLO WORLD"
	grid, err := Matrix(text)
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	data, err := Encode(text)
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
	v, err := estimateVersion(tri)
	if err != nil {
		t.Fatalf("estimateVersion: %v", err)
	}
	if v != 1 {
		t.Fatalf("estimated version = %d, want 1", v)
	}
	h, err := homographyFromFinders(tri, v)
	if err != nil {
		t.Fatalf("homographyFromFinders: %v", err)
	}
	n := len(grid)
	for r := range n {
		for c := range n {
			px, py := h.apply(float64(c), float64(r))
			ix := int(math.Round(px))
			iy := int(math.Round(py))
			if ix < 0 || ix >= bm.width || iy < 0 || iy >= bm.height {
				t.Errorf("module (%d,%d) → pixel (%d,%d) out of bounds", r, c, ix, iy)
				continue
			}
			got := bm.get(ix, iy)
			want := grid[r][c]
			if got != want {
				t.Errorf("module (r=%d, c=%d): sampled %v at pixel (%d,%d), want %v",
					r, c, got, ix, iy, want)
			}
		}
	}
}

// TestRefineHomographyV1IsNoOp asserts that refineHomography returns the
// original transform unchanged for V1, which has no alignment patterns.
func TestRefineHomographyV1IsNoOp(t *testing.T) {
	data, _ := Encode("HELLO WORLD")
	img, _ := png.Decode(bytes.NewReader(data))
	bm := binarise(img)
	tri, err := findFinders(bm)
	if err != nil {
		t.Fatalf("findFinders: %v", err)
	}
	h0, err := homographyFromFinders(tri, 1)
	if err != nil {
		t.Fatalf("homographyFromFinders: %v", err)
	}
	h1 := refineHomography(bm, h0, tri, 1)
	for i, v := range h0 {
		if h1[i] != v {
			t.Errorf("V1 refinement changed h[%d] from %v to %v (expected no-op)", i, v, h1[i])
		}
	}
}

// TestRefineHomographyV2FindsAlignment confirms that for a V2+ symbol the
// refiner locates the bottom-right alignment pattern and that the refined
// transform still samples every module of the original matrix correctly.
func TestRefineHomographyV2FindsAlignment(t *testing.T) {
	const text = "HELLO WORLD"
	grid, err := Matrix(text, WithVersion(2))
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	data, err := Encode(text, WithVersion(2))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, _ := png.Decode(bytes.NewReader(data))
	bm := binarise(img)
	tri, err := findFinders(bm)
	if err != nil {
		t.Fatalf("findFinders: %v", err)
	}
	h0, err := homographyFromFinders(tri, 2)
	if err != nil {
		t.Fatalf("homographyFromFinders: %v", err)
	}
	h1 := refineHomography(bm, h0, tri, 2)

	// Sanity: the refined transform must still map every module to a dark/light
	// value matching the original matrix.
	n := len(grid)
	for r := range n {
		for c := range n {
			px, py := h1.apply(float64(c), float64(r))
			ix := int(math.Round(px))
			iy := int(math.Round(py))
			if ix < 0 || ix >= bm.width || iy < 0 || iy >= bm.height {
				t.Errorf("module (%d,%d) → pixel (%d,%d) out of bounds", r, c, ix, iy)
				continue
			}
			if got := bm.get(ix, iy); got != grid[r][c] {
				t.Errorf("module (r=%d, c=%d) sampled %v, want %v", r, c, got, grid[r][c])
			}
		}
	}
}

// TestRefineHomographyV7AlignmentRefinement runs the same per-module check on
// a higher-version symbol so the alignment pattern is well separated from the
// finder corners and the refiner has a meaningful effect.
func TestRefineHomographyV7AlignmentRefinement(t *testing.T) {
	const text = "HELLO WORLD"
	grid, err := Matrix(text, WithVersion(7))
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	data, err := Encode(text, WithVersion(7))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, _ := png.Decode(bytes.NewReader(data))
	bm := binarise(img)
	tri, err := findFinders(bm)
	if err != nil {
		t.Fatalf("findFinders: %v", err)
	}
	h0, err := homographyFromFinders(tri, 7)
	if err != nil {
		t.Fatalf("homographyFromFinders: %v", err)
	}
	h1 := refineHomography(bm, h0, tri, 7)

	n := len(grid)
	for r := range n {
		for c := range n {
			px, py := h1.apply(float64(c), float64(r))
			ix := int(math.Round(px))
			iy := int(math.Round(py))
			if ix < 0 || ix >= bm.width || iy < 0 || iy >= bm.height {
				t.Errorf("V7 module (%d,%d) → pixel (%d,%d) out of bounds", r, c, ix, iy)
				continue
			}
			if got := bm.get(ix, iy); got != grid[r][c] {
				t.Errorf("V7 module (r=%d, c=%d) sampled %v, want %v", r, c, got, grid[r][c])
			}
		}
	}
}

// TestRefineHomographyMissingAlignmentFallsBack covers the path where the
// search window contains no valid alignment pattern (e.g. the area has been
// wiped). The refiner must return the input homography unchanged so the rest
// of the pipeline still gets something to sample with.
func TestRefineHomographyMissingAlignmentFallsBack(t *testing.T) {
	const text = "HELLO WORLD"
	data, _ := Encode(text, WithVersion(2))
	img, _ := png.Decode(bytes.NewReader(data))
	bm := binarise(img)
	tri, err := findFinders(bm)
	if err != nil {
		t.Fatalf("findFinders: %v", err)
	}
	h0, err := homographyFromFinders(tri, 2)
	if err != nil {
		t.Fatalf("homographyFromFinders: %v", err)
	}
	// Wipe the area where the alignment pattern lives so the shape check fails.
	// V2 alignment is at module (18, 18). The 5×5 region is modules 16..20.
	// In pixel space that's around (quietZone + 16)*8 .. (quietZone + 21)*8 =
	// 160..168 for module 16, etc. Just zero out a 40×40 px region centred on
	// the predicted location.
	predX, predY := h0.apply(18, 18)
	cx, cy := int(math.Round(predX)), int(math.Round(predY))
	for dy := -20; dy <= 20; dy++ {
		for dx := -20; dx <= 20; dx++ {
			x, y := cx+dx, cy+dy
			if x >= 0 && x < bm.width && y >= 0 && y < bm.height {
				bm.pixels[y*bm.width+x] = false
			}
		}
	}
	h1 := refineHomography(bm, h0, tri, 2)
	for i := range h0 {
		if h0[i] != h1[i] {
			t.Errorf("missing alignment should fall back to h0; differs at index %d (%v vs %v)", i, h0[i], h1[i])
		}
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

// TestOrderFinderTripleRotationInvariance feeds synthetic finder positions
// representing the same physical QR symbol rotated to 0, 90, 180, 270, and
// 30 degrees. The expected output identities (tl, tr, bl) are taken from the
// rotation: regardless of which corner of the image they land in, the
// labeling must be consistent — top-left always at the right-angle vertex,
// top-right always at the head of v_tr (which becomes a 90-degree rotation
// of v_tr at each axis-aligned step), bottom-left always at the head of
// v_bl. The pre-v0.4 implementation passed only the upright case; everything
// else came back with tr and bl swapped because the discriminator relied on
// raw y-coordinates. See docs/theory/15-rotation-handling.md.
func TestOrderFinderTripleRotationInvariance(t *testing.T) {
	// Side length of the synthetic symbol in pixels.
	const a = 100.0
	// Helper: build a finderCandidate at (x, y) with a fixed module size.
	mk := func(x, y float64) finderCandidate {
		return finderCandidate{x: x, y: y, moduleSize: 1.0}
	}

	cases := []struct {
		name           string
		tl, tr, bl     finderCandidate
		permuteInputAs []int // permutation of [tl, tr, bl] indices fed to orderFinderTriple, to confirm input order does not matter
	}{
		{
			name: "upright",
			tl:   mk(10, 10),
			tr:   mk(10+a, 10),
			bl:   mk(10, 10+a),
		},
		{
			name: "rot90cw",
			// Original TL is now at the image's top-right corner.
			tl: mk(10+a, 10),
			tr: mk(10+a, 10+a),
			bl: mk(10, 10),
		},
		{
			name: "rot180",
			tl:   mk(10+a, 10+a),
			tr:   mk(10, 10+a),
			bl:   mk(10+a, 10),
		},
		{
			name: "rot270cw",
			tl:   mk(10, 10+a),
			tr:   mk(10, 10),
			bl:   mk(10+a, 10+a),
		},
		{
			name: "softTilt30deg",
			// Apply a 30-degree clockwise rotation to the upright fixture
			// around its TL anchor. Numbers come from the rotation matrix
			// [[cos, -sin], [sin, cos]] applied to v_tr=(a,0) and v_bl=(0,a):
			//   v_tr -> (a*cos(30), a*sin(30)) = (86.602, 50)
			//   v_bl -> (-a*sin(30), a*cos(30)) = (-50, 86.602)
			// Then translate by TL=(80, 30) so all points are positive.
			tl: mk(80, 30),
			tr: mk(80+86.602, 30+50),
			bl: mk(80-50, 30+86.602),
		},
	}

	// For every case, also try every permutation of the input arguments so
	// orderFinderTriple's first step (longest-side-opposite) is exercised in
	// every branch of its switch statement.
	perms := [][3]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}

	approxEqual := func(want, got finderCandidate, tag string) {
		t.Helper()
		if math.Abs(want.x-got.x) > 1e-6 || math.Abs(want.y-got.y) > 1e-6 {
			t.Errorf("%s: got %+v, want %+v", tag, got, want)
		}
	}

	for _, c := range cases {
		for _, p := range perms {
			in := [3]finderCandidate{c.tl, c.tr, c.bl}
			got, err := orderFinderTriple(in[p[0]], in[p[1]], in[p[2]])
			if err != nil {
				t.Fatalf("%s perm=%v: unexpected error %v", c.name, p, err)
			}
			approxEqual(c.tl, got.topLeft, c.name+" tl")
			approxEqual(c.tr, got.topRight, c.name+" tr")
			approxEqual(c.bl, got.bottomLeft, c.name+" bl")
		}
	}
}

// TestOrderFinderTripleRejectsBadGeometry confirms the leg-ratio and
// hypotenuse sanity checks still reject implausible triples after the
// cross-product change. These two cases used to fire under the upright
// implementation and must continue to fire under the rotation-invariant
// one — the cross-product replacement only touches the tr-vs-bl
// discriminator, not the geometric validation that follows.
func TestOrderFinderTripleRejectsBadGeometry(t *testing.T) {
	mk := func(x, y float64) finderCandidate {
		return finderCandidate{x: x, y: y, moduleSize: 1.0}
	}
	// Three collinear points — degenerate "triangle".
	if _, err := orderFinderTriple(mk(0, 0), mk(50, 0), mk(100, 0)); err == nil {
		t.Error("collinear triple unexpectedly accepted")
	}
	// Two legs with grossly different lengths (ratio > 1.5).
	if _, err := orderFinderTriple(mk(0, 0), mk(10, 0), mk(0, 100)); err == nil {
		t.Error("triple with leg ratio 10:1 unexpectedly accepted")
	}
}
