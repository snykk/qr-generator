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
	"testing"
)

// genPolyExpectedAlphaExponents lists, for each EC block length used by QR,
// the α-exponent of every coefficient of g(x) from highest degree to lowest.
// Source: docs/theory/09-data-tables.md §10, derived from Project Nayuki.
var genPolyExpectedAlphaExponents = map[int][]uint8{
	7:  {0, 87, 229, 146, 149, 238, 102, 21},
	10: {0, 251, 67, 46, 61, 118, 70, 64, 94, 32, 45},
	13: {0, 74, 152, 176, 100, 86, 100, 106, 104, 130, 218, 206, 140, 78},
	15: {0, 8, 183, 61, 91, 202, 37, 51, 58, 58, 237, 140, 124, 5, 99, 105},
	16: {0, 120, 104, 107, 109, 102, 161, 76, 3, 91, 191, 147, 169, 182, 194, 225, 120},
	17: {0, 43, 139, 206, 78, 43, 239, 123, 206, 214, 147, 24, 99, 150, 39, 243, 163, 136},
	18: {0, 215, 234, 158, 94, 184, 97, 118, 170, 79, 187, 152, 148, 252, 179, 5, 98, 96, 153},
	20: {0, 17, 60, 79, 50, 61, 163, 26, 187, 202, 180, 221, 225, 83, 239, 156, 164, 212, 212, 188, 190},
	22: {0, 210, 171, 247, 242, 93, 230, 14, 109, 221, 53, 200, 74, 8, 172, 98, 80, 219, 134, 160, 105, 165, 231},
	24: {0, 229, 121, 135, 48, 211, 117, 251, 126, 159, 180, 169, 152, 192, 226, 228, 218, 111, 0, 117, 232, 87, 96, 227, 21},
	26: {0, 173, 125, 158, 2, 103, 182, 118, 17, 145, 201, 111, 28, 165, 53, 161, 21, 245, 142, 13, 102, 48, 227, 153, 145, 218, 70},
	28: {0, 168, 223, 200, 104, 224, 234, 108, 180, 110, 190, 195, 147, 205, 27, 232, 201, 21, 43, 245, 87, 42, 195, 212, 119, 242, 37, 9, 123},
	30: {0, 41, 173, 145, 152, 216, 31, 179, 182, 50, 48, 110, 86, 239, 96, 222, 125, 42, 173, 226, 193, 224, 130, 156, 37, 251, 216, 238, 40, 192, 180},
}

// TestGenPolyAlphaExponents verifies the computed generator polynomial for
// every EC block length used by QR matches the published α-exponent sequence.
func TestGenPolyAlphaExponents(t *testing.T) {
	for n, wantExps := range genPolyExpectedAlphaExponents {
		g := genPoly(n)
		if len(g) != n+1 {
			t.Errorf("genPoly(%d): len = %d, want %d", n, len(g), n+1)
			continue
		}
		for i, c := range g {
			// In genPoly each coefficient is non-zero (the polynomial is monic
			// and built from non-zero roots), so logTable lookups are valid.
			if c == 0 {
				t.Errorf("genPoly(%d)[%d] = 0; expected non-zero", n, i)
				continue
			}
			gotExp := logTable[c]
			if gotExp != wantExps[i] {
				t.Errorf("genPoly(%d)[%d] = α^%d, want α^%d", n, i, gotExp, wantExps[i])
			}
		}
	}
}

// TestEncodeBlockHelloWorld is the canonical RS fixture from
// docs/theory/10-worked-example.md: 16 data codewords from V1-M HELLO WORLD
// must produce exactly these 10 EC codewords.
func TestEncodeBlockHelloWorld(t *testing.T) {
	data := []byte{
		0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D,
		0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11,
	}
	want := []byte{
		0xC4, 0x23, 0x27, 0x77, 0xEB, 0xD7, 0xE7, 0xE2, 0x5D, 0x17,
	}
	got := encodeBlock(data, 10)
	if !bytes.Equal(got, want) {
		t.Errorf("ec codewords = % X\nwant         % X", got, want)
	}
}

func TestInterleaveBlocksSingleBlock(t *testing.T) {
	// Single data block + single EC block → just concatenation.
	data := [][]byte{{1, 2, 3, 4}}
	ec := [][]byte{{0xA, 0xB}}
	got := interleaveBlocks(data, ec)
	want := []byte{1, 2, 3, 4, 0xA, 0xB}
	if !bytes.Equal(got, want) {
		t.Errorf("got % X, want % X", got, want)
	}
}

func TestInterleaveBlocksMultiBlockSameSize(t *testing.T) {
	// 2 data blocks of 3, 2 EC blocks of 2.
	data := [][]byte{
		{1, 2, 3},
		{4, 5, 6},
	}
	ec := [][]byte{
		{0xA, 0xB},
		{0xC, 0xD},
	}
	// Data column-major: 1,4, 2,5, 3,6 → EC column-major: A,C, B,D.
	want := []byte{1, 4, 2, 5, 3, 6, 0xA, 0xC, 0xB, 0xD}
	got := interleaveBlocks(data, ec)
	if !bytes.Equal(got, want) {
		t.Errorf("got % X, want % X", got, want)
	}
}

func TestInterleaveBlocksMultiBlockDifferentSize(t *testing.T) {
	// 2 data blocks of 2 + 2 data blocks of 3 (V5-Q-ish shape, miniaturised).
	// EC blocks all length 2.
	data := [][]byte{
		{1, 2},          // block A (group 1)
		{3, 4},          // block B (group 1)
		{5, 6, 99},      // block C (group 2)
		{7, 8, 100},     // block D (group 2)
	}
	ec := [][]byte{
		{0xA1, 0xA2},
		{0xB1, 0xB2},
		{0xC1, 0xC2},
		{0xD1, 0xD2},
	}
	// Data column-major (skip short blocks past their length):
	//   col 0: A0=1, B0=3, C0=5, D0=7
	//   col 1: A1=2, B1=4, C1=6, D1=8
	//   col 2:                C2=99, D2=100   (A and B have no col 2)
	// EC column-major: col 0 of all four, col 1 of all four.
	want := []byte{
		1, 3, 5, 7,
		2, 4, 6, 8,
		99, 100,
		0xA1, 0xB1, 0xC1, 0xD1,
		0xA2, 0xB2, 0xC2, 0xD2,
	}
	got := interleaveBlocks(data, ec)
	if !bytes.Equal(got, want) {
		t.Errorf("got % X\nwant % X", got, want)
	}
}

// TestRSEncodeHelloWorld puts the whole RS stage together: feed the 16 data
// codewords from M3 through V1-M and verify the full 26-codeword interleaved
// stream matches data || EC byte-for-byte.
func TestRSEncodeHelloWorld(t *testing.T) {
	data := []byte{
		0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D,
		0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11,
	}
	want := []byte{
		// 16 data codewords
		0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D,
		0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11,
		// 10 EC codewords
		0xC4, 0x23, 0x27, 0x77, 0xEB, 0xD7, 0xE7, 0xE2, 0x5D, 0x17,
	}
	got := rsEncode(data, Version(1), ECLevelM)
	if !bytes.Equal(got, want) {
		t.Errorf("rsEncode = % X\nwant      % X", got, want)
	}
}

// TestRSEncodePipelineFromEncodeText feeds encodeText → rsEncode and checks
// the final length equals the version's total codeword count for several
// (text, ec) cases.
func TestRSEncodePipelineFromEncodeText(t *testing.T) {
	cases := []struct {
		name string
		text string
		ec   ECLevel
	}{
		{"hello world m", "HELLO WORLD", ECLevelM},
		{"numeric short", "12345", ECLevelL},
		{"byte longer", "The quick brown fox jumps over the lazy dog.", ECLevelM},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, v, err := encodeText(c.text, c.ec, 0)
			if err != nil {
				t.Fatalf("encodeText: %v", err)
			}
			interleaved := rsEncode(data, v, c.ec)
			spec := v.ECBlocks(c.ec)
			wantLen := spec.TotalDataCodewords() + spec.TotalECCodewords()
			if len(interleaved) != wantLen {
				t.Errorf("rsEncode len = %d, want %d", len(interleaved), wantLen)
			}
		})
	}
}
