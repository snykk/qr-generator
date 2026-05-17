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
