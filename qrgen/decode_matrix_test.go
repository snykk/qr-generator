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
	"strings"
	"testing"
)

func TestMatrixFromGridValidatesSize(t *testing.T) {
	cases := []struct {
		name    string
		side    int
		wantErr bool
	}{
		{"V1 (21)", 21, false},
		{"V2 (25)", 25, false},
		{"V40 (177)", 177, false},
		{"too small", 17, true},
		{"non-aligned", 22, true},
		{"too big", 181, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grid := make([][]bool, c.side)
			for i := range grid {
				grid[i] = make([]bool, c.side)
			}
			_, err := matrixFromGrid(grid)
			if (err != nil) != c.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestMatrixFromGridRejectsRaggedRows(t *testing.T) {
	grid := make([][]bool, 21)
	for i := range grid {
		grid[i] = make([]bool, 21)
	}
	grid[5] = make([]bool, 20)
	if _, err := matrixFromGrid(grid); err == nil {
		t.Error("expected error for ragged grid, got nil")
	}
}

// TestReadCodewordStreamRoundTripsEncoder asserts that for a freshly-encoded
// QR matrix, reading the codeword stream back gives exactly the same byte
// sequence that the encoder fed into placeData. This validates both the
// reverse walk and the mask reversal end-to-end.
func TestReadCodewordStreamRoundTripsEncoder(t *testing.T) {
	cases := []struct {
		name string
		text string
		ec   ECLevel
	}{
		{"V1-M HELLO WORLD", "HELLO WORLD", ECLevelM},
		{"V1-L numeric", "12345", ECLevelL},
		{"V1-H alphanumeric", "ABCD", ECLevelH},
		{"V2-M longer", "HELLO WORLD ABC123", ECLevelM},
		{"V5-Q multi-block", strings.Repeat("ABC123", 10), ECLevelQ},
		{"V7+ exercises version info", strings.Repeat("X", 80), ECLevelM},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Build a complete matrix via the public encoder pipeline.
			m, mask, err := buildMatrix(c.text, defaultsForEC(c.ec))
			if err != nil {
				t.Fatalf("buildMatrix: %v", err)
			}
			// Compute the expected interleaved codeword stream the encoder
			// would have produced for this (text, ec).
			data, v, _, err := encodeText(c.text, c.ec, 0)
			if err != nil {
				t.Fatalf("encodeText: %v", err)
			}
			want := rsEncode(data, v, c.ec)

			// Reconstruct a decoder-side matrix from the encoder's modules.
			grid := make([][]bool, m.size)
			for i := 0; i < m.size; i++ {
				grid[i] = append([]bool(nil), m.modules[i]...)
			}
			dec, err := matrixFromGrid(grid)
			if err != nil {
				t.Fatalf("matrixFromGrid: %v", err)
			}
			got := readCodewordStream(dec, mask)

			if !bytes.Equal(got, want) {
				t.Errorf("codeword stream mismatch:\n got  % X\n want % X", got, want)
			}
		})
	}
}

// TestReadCodewordStreamReversesMaskForAllPatterns confirms that read-after-
// write commutes for every mask pattern, by forcing each mask in turn and
// checking the round-trip stream still matches rsEncode's output.
func TestReadCodewordStreamReversesMaskForAllPatterns(t *testing.T) {
	for k := 0; k < numMasks; k++ {
		t.Run("mask"+itoaSmall(k), func(t *testing.T) {
			opts := defaultsForEC(ECLevelM)
			opts.mask = k
			m, mask, err := buildMatrix("HELLO WORLD", opts)
			if err != nil {
				t.Fatalf("buildMatrix: %v", err)
			}
			if mask != k {
				t.Fatalf("forced mask %d but got %d", k, mask)
			}
			data, v, _, _ := encodeText("HELLO WORLD", ECLevelM, 0)
			want := rsEncode(data, v, ECLevelM)
			grid := make([][]bool, m.size)
			for i := 0; i < m.size; i++ {
				grid[i] = append([]bool(nil), m.modules[i]...)
			}
			dec, err := matrixFromGrid(grid)
			if err != nil {
				t.Fatalf("matrixFromGrid: %v", err)
			}
			got := readCodewordStream(dec, mask)
			if !bytes.Equal(got, want) {
				t.Errorf("mask %d: stream mismatch:\n got  % X\n want % X", k, got, want)
			}
		})
	}
}

// itoaSmall is a tiny base-10 formatter for non-negative single digits, used
// to name subtests without pulling in fmt.Sprintf.
func itoaSmall(n int) string {
	if n < 0 || n > 9 {
		return "?"
	}
	return string([]byte{'0' + byte(n)})
}

// TestDeinterleaveBlocksReversesInterleave checks that deinterleaveBlocks is
// the exact left-inverse of the encoder's interleaveBlocks helper for both
// single-block and multi-block versions.
func TestDeinterleaveBlocksReversesInterleave(t *testing.T) {
	cases := []struct {
		name string
		v    Version
		ec   ECLevel
	}{
		{"V1-M single block", 1, ECLevelM},
		{"V1-H single block", 1, ECLevelH},
		{"V5-Q two groups", 5, ECLevelQ},
		{"V10-M two groups", 10, ECLevelM},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Build a deterministic block layout the encoder would produce.
			spec := c.v.ECBlocks(c.ec)
			dataBlocks := make([][]byte, 0, spec.TotalBlocks())
			val := byte(1)
			for range spec.Group1.Count {
				block := make([]byte, spec.Group1.DataPerBlock)
				for j := range block {
					block[j] = val
					val++
				}
				dataBlocks = append(dataBlocks, block)
			}
			for range spec.Group2.Count {
				block := make([]byte, spec.Group2.DataPerBlock)
				for j := range block {
					block[j] = val
					val++
				}
				dataBlocks = append(dataBlocks, block)
			}
			ecBlocks := make([][]byte, len(dataBlocks))
			for i := range ecBlocks {
				ecBlocks[i] = encodeBlock(dataBlocks[i], spec.ECPerBlock)
			}
			interleaved := interleaveBlocks(dataBlocks, ecBlocks)

			gotData, gotEC, err := deinterleaveBlocks(interleaved, c.v, c.ec)
			if err != nil {
				t.Fatalf("deinterleaveBlocks: %v", err)
			}
			if len(gotData) != len(dataBlocks) {
				t.Fatalf("data block count = %d, want %d", len(gotData), len(dataBlocks))
			}
			for i, want := range dataBlocks {
				if !bytes.Equal(gotData[i], want) {
					t.Errorf("data block %d: got % X, want % X", i, gotData[i], want)
				}
			}
			for i, want := range ecBlocks {
				if !bytes.Equal(gotEC[i], want) {
					t.Errorf("ec block %d: got % X, want % X", i, gotEC[i], want)
				}
			}
		})
	}
}

func TestDeinterleaveBlocksRejectsWrongLength(t *testing.T) {
	if _, _, err := deinterleaveBlocks(make([]byte, 99), 1, ECLevelM); err == nil {
		t.Error("expected error for short stream, got nil")
	}
}

// TestDeinterleaveAndCorrectRoundTrip glues the M3..M4 encoder to D5..D6 and
// asserts the corrected data byte stream matches the encoder's intermediate
// data codewords for a range of versions and EC levels.
func TestDeinterleaveAndCorrectRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		text string
		ec   ECLevel
	}{
		{"V1-M HELLO WORLD", "HELLO WORLD", ECLevelM},
		{"V1-H short", "ABC", ECLevelH},
		{"V5-Q multi-block", strings.Repeat("ABC123", 10), ECLevelQ},
		{"V10-M two groups", strings.Repeat("X", 80), ECLevelM},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, v, _, err := encodeText(c.text, c.ec, 0)
			if err != nil {
				t.Fatalf("encodeText: %v", err)
			}
			stream := rsEncode(data, v, c.ec)
			got, err := deinterleaveAndCorrect(stream, v, c.ec)
			if err != nil {
				t.Fatalf("deinterleaveAndCorrect: %v", err)
			}
			if !bytes.Equal(got, data) {
				t.Errorf("recovered % X\nwant      % X", got, data)
			}
		})
	}
}

// TestDeinterleaveAndCorrectTolerates corrupts a small number of bytes
// (within the spec's RS budget) and verifies recovery still succeeds.
func TestDeinterleaveAndCorrectTolerates(t *testing.T) {
	data, v, _, _ := encodeText("HELLO WORLD", ECLevelM, 0)
	stream := rsEncode(data, v, ECLevelM)
	// V1-M has 1 block, n=10 EC codewords → t=5 byte budget.
	corrupted := append([]byte(nil), stream...)
	for _, pos := range []int{0, 7, 14, 21, 25} { // 5 flips
		corrupted[pos] ^= 0x5A
	}
	got, err := deinterleaveAndCorrect(corrupted, v, ECLevelM)
	if err != nil {
		t.Fatalf("deinterleaveAndCorrect: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("got % X\nwant % X", got, data)
	}
}

func TestDeinterleaveAndCorrectFailsBeyondCapacity(t *testing.T) {
	data, v, _, _ := encodeText("HELLO WORLD", ECLevelM, 0)
	stream := rsEncode(data, v, ECLevelM)
	// 6 byte flips exceed the t=5 budget for V1-M's single block.
	corrupted := append([]byte(nil), stream...)
	for _, pos := range []int{0, 2, 4, 6, 8, 10} {
		corrupted[pos] ^= 0x5A
	}
	got, err := deinterleaveAndCorrect(corrupted, v, ECLevelM)
	if err == nil && bytes.Equal(got, data) {
		t.Error("decoder silently 'recovered' correct data past RS capacity")
	}
}
