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
	"strings"
	"testing"
)

// BenchmarkDecodeMatrixSmall isolates the matrix-stage decoder (D4..D7) on a
// V1-M payload — the cheapest possible decoder workload.
func BenchmarkDecodeMatrixSmall(b *testing.B) {
	grid, err := Matrix("HELLO WORLD")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeMatrix(grid); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeMatrixMultiBlock exercises the deinterleave + RS path on a
// V5-Q payload so each block decode goes through Berlekamp-Massey + Forney.
func BenchmarkDecodeMatrixMultiBlock(b *testing.B) {
	grid, err := Matrix(strings.Repeat("ABC123", 10), WithECLevel(ECLevelQ))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeMatrix(grid); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeImageSmall covers the full image pipeline (D8..D12) on a V1
// PNG: binarise, finder detection, homography, sampling, then the matrix
// decoder. Useful as a regression baseline for the CV stages.
func BenchmarkDecodeImageSmall(b *testing.B) {
	pngBytes, err := Encode("HELLO WORLD")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBytes(pngBytes); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeImageMultiBlock measures a V5 PNG decode where alignment
// refinement contributes to runtime in addition to finder detection.
func BenchmarkDecodeImageMultiBlock(b *testing.B) {
	pngBytes, err := Encode(strings.Repeat("ABC123", 10), WithECLevel(ECLevelQ))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBytes(pngBytes); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeImageURL is the realistic-payload case: a URL of ~40 bytes,
// which lands at V2-V3 in byte mode — what most callers will actually decode.
func BenchmarkDecodeImageURL(b *testing.B) {
	pngBytes, err := Encode("https://github.com/snykk/qr-generator")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBytes(pngBytes); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeImageFromPNGDecode is the lowest-overhead variant: it
// pre-decodes the PNG once and only benchmarks Decode(image.Image). Useful
// for separating CV cost from PNG parsing cost in profiling.
func BenchmarkDecodeImageFromPNGDecode(b *testing.B) {
	pngBytes, err := Encode("HELLO WORLD")
	if err != nil {
		b.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(img); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeImageSauvolaFallback measures the cost of the reactive
// Sauvola fallback path. The fixture mirrors TestT4ConstantQuietZoneDarkening:
// the QR's internal modules stay intact but the quiet zone is uniformly
// darkened to 70, which defeats Otsu's global threshold and forces the
// dispatch into Sauvola. The number captured here lets us watch the
// fallback-path cost evolve over future releases (rotation work, possible
// integral-image reuse, etc.) without ever hiding it inside an aggregate
// over mixed cases.
func BenchmarkDecodeImageSauvolaFallback(b *testing.B) {
	pngBytes, err := Encode("HELLO")
	if err != nil {
		b.Fatal(err)
	}
	clean, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		b.Fatal(err)
	}
	// Mirror TestT4ConstantQuietZoneDarkening: QR area pristine, quiet
	// zone uniformly grey. Coordinates from cleanV1 (V1 + module 8 +
	// quiet zone 4 → QR rect (32, 32)..(200, 200)).
	bounds := clean.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.GrayModel.Convert(clean.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.Gray)
			if x >= 32 && x < 200 && y >= 32 && y < 200 {
				img.Pix[y*img.Stride+x] = c.Y
			} else {
				img.Pix[y*img.Stride+x] = 70
			}
		}
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Decode(img); err != nil {
			b.Fatal(err)
		}
	}
}
