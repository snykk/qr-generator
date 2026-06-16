// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"strings"
	"testing"
)

// BenchmarkEncodeSmall covers the most common payload size: a short
// alphanumeric string at default EC. This is V1, single Reed–Solomon block,
// 232x232 grayscale PNG.
func BenchmarkEncodeSmall(b *testing.B) {
	const text = "HELLO WORLD"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeURL is representative of the most common real-world payload:
// a URL of ~40 bytes. Forces byte mode and lands around V2-V3.
func BenchmarkEncodeURL(b *testing.B) {
	const text = "https://github.com/snykk/qr-generator"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeMultiBlock exercises Reed–Solomon block splitting and
// column-major interleaving by encoding a payload that lands at V5+ with
// EC level Q (the worked example for multi-block in the theory doc).
func BenchmarkEncodeMultiBlock(b *testing.B) {
	text := strings.Repeat("ABC123", 10) // 60 alphanumeric chars → V5-Q
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(text, WithECLevel(ECLevelQ)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeLarge stresses higher versions and all four mask trials.
func BenchmarkEncodeLarge(b *testing.B) {
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeMixed measures a payload that the v0.6 segmenter splits into
// byte + numeric segments. The DP runs (cached per version group) sit on the
// encode hot path here, so this is the figure to watch for segmentation cost.
func BenchmarkEncodeMixed(b *testing.B) {
	const text = "Order #1234567890 shipped to bay 42"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Encode(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMatrixOnly skips PNG rendering to isolate the encoder pipeline
// (M3..M6) from the rasterisation path.
func BenchmarkMatrixOnly(b *testing.B) {
	const text = "HELLO WORLD"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Matrix(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeSVGSmall mirrors BenchmarkEncodeSmall on the SVG path so the
// two renderers can be compared directly. The bytes/op figure also publishes
// the SVG document size for the common short payload.
func BenchmarkEncodeSVGSmall(b *testing.B) {
	const text = "HELLO WORLD"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := EncodeSVG(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeSVGURL mirrors BenchmarkEncodeURL on the SVG path: the
// representative ~40-byte URL payload that lands around V2-V3.
func BenchmarkEncodeSVGURL(b *testing.B) {
	const text = "https://github.com/snykk/qr-generator"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := EncodeSVG(text); err != nil {
			b.Fatal(err)
		}
	}
}
