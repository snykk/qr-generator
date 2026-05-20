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
