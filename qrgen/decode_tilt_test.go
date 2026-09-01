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

// warpTilt renders src through a synthetic perspective tilt: it rotates the
// source plane out of the image plane about its vertical centre axis by
// tiltDeg, applies an in-plane rotation of rotDeg, projects with a weak
// perspective, and inverse-maps each destination pixel with bilinear sampling
// onto a white (quiet-zone) background. The result is a synthetic "photo" of
// the symbol seen at an angle, whose finders foreshorten enough to defeat the
// 1:1:3:1:1 line scanner. This is test-only scaffolding, never part of the
// library; its own sampling quality only affects how hard the fixtures are.
// See docs/theory/24-arbitrary-angle-detection.md section 6.
func warpTilt(t *testing.T, src image.Image, tiltDeg, rotDeg float64) *image.Gray {
	t.Helper()
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	tilt := tiltDeg * math.Pi / 180
	rot := rotDeg * math.Pi / 180
	cosT, sinT := math.Cos(tilt), math.Sin(tilt)
	cosR, sinR := math.Cos(rot), math.Sin(rot)
	half := float64(w) / 2
	const focal = 3.0 // in half-widths; larger means a weaker perspective
	cx, cy := float64(w-1)/2, float64(h-1)/2

	// project maps a source pixel to the tilted, rotated, perspective-divided
	// plane, centred on the origin.
	project := func(u, v float64) (float64, float64) {
		x, y := u-cx, v-cy
		x, y = x*cosR-y*sinR, x*sinR+y*cosR
		z := x * sinT
		x *= cosT
		s := focal * half / (focal*half - z)
		return x * s, y * s
	}

	srcPix := [4][2]float64{{0, 0}, {float64(w - 1), 0}, {float64(w - 1), float64(h - 1)}, {0, float64(h - 1)}}
	var dstC, srcC [4][2]float64
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, c := range srcPix {
		px, py := project(c[0], c[1])
		dstC[i], srcC[i] = [2]float64{px, py}, c
		minX, minY = math.Min(minX, px), math.Min(minY, py)
		maxX, maxY = math.Max(maxX, px), math.Max(maxY, py)
	}
	const pad = 24
	for i := range dstC {
		dstC[i][0] += pad - minX
		dstC[i][1] += pad - minY
	}
	dstW := int(math.Ceil(maxX-minX)) + 2*pad
	dstH := int(math.Ceil(maxY-minY)) + 2*pad

	// Homography sending destination pixels back to source pixels.
	hinv, err := computeHomography(dstC, srcC)
	if err != nil {
		t.Fatalf("computeHomography: %v", err)
	}

	srcGray := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcGray[y*w+x] = color.GrayModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y
		}
	}
	dst := image.NewGray(image.Rect(0, 0, dstW, dstH))
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}
	maxU, maxV := float64(w-1), float64(h-1)
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			su, sv := hinv.apply(float64(dx), float64(dy))
			if su < 0 || su > maxU || sv < 0 || sv > maxV {
				continue
			}
			x0, y0 := int(math.Floor(su)), int(math.Floor(sv))
			x1, y1 := x0+1, y0+1
			if x1 > w-1 {
				x1 = x0
			}
			if y1 > h-1 {
				y1 = y0
			}
			fx, fy := su-float64(x0), sv-float64(y0)
			p00 := float64(srcGray[y0*w+x0])
			p10 := float64(srcGray[y0*w+x1])
			p01 := float64(srcGray[y1*w+x0])
			p11 := float64(srcGray[y1*w+x1])
			val := p00*(1-fx)*(1-fy) + p10*fx*(1-fy) + p01*(1-fx)*fy + p11*fx*fy
			dst.Pix[dy*dst.Stride+dx] = uint8(val + 0.5)
		}
	}
	return dst
}

// tiltSource renders payload to a clean high-resolution PNG and decodes it back
// to an image, the untilted source every tilt fixture warps.
func tiltSource(t *testing.T, payload string) image.Image {
	t.Helper()
	pngBytes, err := Encode(payload, WithModuleSize(10), WithQuietZone(4))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	return img
}

func toPNGBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// TestArbitraryAngleSweep is an informational sweep: it logs the decode outcome
// and which pipeline (affine or projective) carried each tilt, so the reliable
// band can be read empirically. It does not assert, keeping the record of the
// full band visible alongside the asserting tests below.
func TestArbitraryAngleSweep(t *testing.T) {
	const payload = "TILT-TEST-1234"
	src := tiltSource(t, payload)
	for _, tilt := range []float64{0, 10, 20, 30, 40, 45, 50, 55, 60} {
		warped := warpTilt(t, src, tilt, 12)
		text, path, err := decodeImageWhich(warped)
		switch {
		case err != nil:
			t.Logf("tilt %2.0f deg: FAIL (%v)", tilt, err)
		case text != payload:
			t.Logf("tilt %2.0f deg: WRONG payload %q via %s", tilt, text, path)
		default:
			t.Logf("tilt %2.0f deg: ok via %s", tilt, path)
		}
	}
}

// TestArbitraryAngleRoundTrip asserts the milestone's target band: symbols
// tilted across it decode correctly through the public DecodeBytes, and the
// steep end is carried by the projective pipeline (proving the fallback fires
// rather than the affine path coincidentally handling the tilt).
func TestArbitraryAngleRoundTrip(t *testing.T) {
	const payload = "TILT-TEST-1234"
	src := tiltSource(t, payload)

	for _, tilt := range []float64{20, 30, 40, 45, 50} {
		warped := warpTilt(t, src, tilt, 12)
		text, err := DecodeBytes(toPNGBytes(t, warped))
		if err != nil {
			t.Errorf("tilt %2.0f deg: DecodeBytes: %v", tilt, err)
			continue
		}
		if text != payload {
			t.Errorf("tilt %2.0f deg: payload = %q, want %q", tilt, text, payload)
		}
	}

	// At a steep tilt the affine path cannot decode, so a successful decode
	// must come from the projective fallback.
	steep := warpTilt(t, src, 45, 12)
	text, path, err := decodeImageWhich(steep)
	if err != nil {
		t.Fatalf("steep tilt: decodeImageWhich: %v", err)
	}
	if text != payload {
		t.Fatalf("steep tilt: payload = %q, want %q", text, payload)
	}
	if path != "projective" {
		t.Errorf("steep 45 deg tilt decoded via %s, want the projective fallback", path)
	}
}

// TestUprightUsesAffinePath guards against a regression: an untilted symbol must
// still decode through the affine fast path, never needing the projective
// fallback.
func TestUprightUsesAffinePath(t *testing.T) {
	const payload = "TILT-TEST-1234"
	src := tiltSource(t, payload)
	text, path, err := decodeImageWhich(src)
	if err != nil {
		t.Fatalf("decodeImageWhich: %v", err)
	}
	if text != payload {
		t.Fatalf("payload = %q, want %q", text, payload)
	}
	if path != "affine" {
		t.Errorf("upright symbol decoded via %s, want affine", path)
	}
}
