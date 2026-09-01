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

// rotateImage renders src into a new image.Gray rotated by angleDeg degrees
// clockwise around its centre, using inverse-mapped bilinear sampling. The
// destination rectangle is the axis-aligned bounding box of the rotated
// source so no content is clipped. Background fill is solid white (255),
// the QR paper / quiet-zone colour, so the rotated symbol still sits in a
// quiet zone of unchanged contrast.
//
// Used by the R3 fixtures to build the v0.4 rotation regression suite in
// memory, without committing any binary fixtures under testdata/. See
// docs/theory/15-rotation-handling.md.
func rotateImage(src image.Image, angleDeg float64) *image.Gray {
	angleRad := angleDeg * math.Pi / 180.0
	cos, sin := math.Cos(angleRad), math.Sin(angleRad)

	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()

	absCos, absSin := math.Abs(cos), math.Abs(sin)
	dstW := int(math.Ceil(float64(srcW)*absCos + float64(srcH)*absSin))
	dstH := int(math.Ceil(float64(srcW)*absSin + float64(srcH)*absCos))
	dst := image.NewGray(image.Rect(0, 0, dstW, dstH))
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}

	// Precompute the source grayscale buffer so the inner loop avoids the
	// per-pixel image.At + color.Model.Convert overhead.
	srcGray := make([]uint8, srcW*srcH)
	for y := range srcH {
		for x := range srcW {
			c := color.GrayModel.Convert(src.At(srcBounds.Min.X+x, srcBounds.Min.Y+y)).(color.Gray)
			srcGray[y*srcW+x] = c.Y
		}
	}

	srcCx := float64(srcW-1) / 2.0
	srcCy := float64(srcH-1) / 2.0
	dstCx := float64(dstW-1) / 2.0
	dstCy := float64(dstH-1) / 2.0
	maxX := float64(srcW - 1)
	maxY := float64(srcH - 1)

	for dy := range dstH {
		for dx := range dstW {
			// Inverse rotation: dst (centred at dstCx, dstCy) -> src.
			ddx := float64(dx) - dstCx
			ddy := float64(dy) - dstCy
			sx := ddx*cos + ddy*sin + srcCx
			sy := -ddx*sin + ddy*cos + srcCy
			if sx < 0 || sx > maxX || sy < 0 || sy > maxY {
				continue
			}
			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1
			if x1 >= srcW {
				x1 = x0
			}
			if y1 >= srcH {
				y1 = y0
			}
			fx := sx - float64(x0)
			fy := sy - float64(y0)
			p00 := float64(srcGray[y0*srcW+x0])
			p10 := float64(srcGray[y0*srcW+x1])
			p01 := float64(srcGray[y1*srcW+x0])
			p11 := float64(srcGray[y1*srcW+x1])
			v := p00*(1-fx)*(1-fy) + p10*fx*(1-fy) + p01*(1-fx)*fy + p11*fx*fy
			dst.Pix[dy*dst.Stride+dx] = uint8(v + 0.5)
		}
	}
	return dst
}

// cleanRotationSource returns the default-options Encode output for "HELLO"
// decoded into an image.Image. Centralised so every rotation fixture shares
// the same untouched source.
func cleanRotationSource(t *testing.T) image.Image {
	t.Helper()
	pngBytes, err := Encode("HELLO")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	return img
}

// assertRotationRecovery is the shared helper for the R3 round-trip tests.
// It runs the rotated image through the public DecodeBytes pipeline,
// asserts payload equality, and checks the dispatch landed on the Otsu
// branch (rotation should not perturb the binariser choice on a clean
// rotated PNG; the Sauvola fallback is reserved for quiet-zone
// contamination, not orientation).
func assertRotationRecovery(t *testing.T, rotated *image.Gray, payload string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, rotated); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	text, err := DecodeBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	if text != payload {
		t.Errorf("payload = %q, want %q", text, payload)
	}
	_, state, err := decodeImageDebug(rotated)
	if err != nil {
		t.Errorf("decodeImageDebug error: %v", err)
	} else if state != binariserOtsu {
		t.Errorf("state = %v, want binariserOtsu (rotation should not perturb the binariser dispatch)", state)
	}
}

func TestRotation90(t *testing.T) {
	rotated := rotateImage(cleanRotationSource(t), 90)
	assertRotationRecovery(t, rotated, "HELLO")
}

func TestRotation180(t *testing.T) {
	rotated := rotateImage(cleanRotationSource(t), 180)
	assertRotationRecovery(t, rotated, "HELLO")
}

func TestRotation270(t *testing.T) {
	rotated := rotateImage(cleanRotationSource(t), 270)
	assertRotationRecovery(t, rotated, "HELLO")
}

func TestRotationSoftTilt15(t *testing.T) {
	rotated := rotateImage(cleanRotationSource(t), 15)
	assertRotationRecovery(t, rotated, "HELLO")
}

func TestRotationSoftTilt30(t *testing.T) {
	rotated := rotateImage(cleanRotationSource(t), 30)
	assertRotationRecovery(t, rotated, "HELLO")
}

// TestRotationSoftTilt45 was the v0.4 scope boundary: at 45 degrees the
// 1:1:3:1:1 line scanner's ±50% tolerance stopped keeping the affine pipeline
// reliable, and decoding failed. v0.13 widened detection with the
// connected-component finder detector and a projective homography, so a 45
// degree rotation now decodes -- through the projective fallback, since the
// affine line-scan path still cannot resolve the finders at this angle.
//
// See docs/theory/24-arbitrary-angle-detection.md.
func TestRotationSoftTilt45(t *testing.T) {
	rotated := rotateImage(cleanRotationSource(t), 45)
	var buf bytes.Buffer
	if err := png.Encode(&buf, rotated); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	text, err := DecodeBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("DecodeBytes at 45 degrees: %v", err)
	}
	if text != "HELLO" {
		t.Errorf("text = %q, want %q", text, "HELLO")
	}
	if _, path, werr := decodeImageWhich(rotated); werr != nil {
		t.Errorf("decodeImageWhich: %v", werr)
	} else if path != "projective" {
		t.Errorf("45 degree rotation decoded via %s, want the projective fallback", path)
	}
}
