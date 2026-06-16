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
	"errors"
	"strings"
	"testing"
)

func TestAnalyzeMode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Mode
	}{
		{"digits", "12345", ModeNumeric},
		{"empty", "", ModeNumeric},
		{"all alphanumeric uppercase", "HELLO WORLD", ModeAlphanumeric},
		{"alphanumeric with symbols", "ABC-123:OK", ModeAlphanumeric},
		{"lowercase forces byte", "Hello", ModeByte},
		{"mixed with punctuation forces byte", "Hello, World!", ModeByte},
		{"utf8 non-ascii forces byte", "café", ModeByte},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := analyzeMode(c.in); got != c.want {
				t.Errorf("analyzeMode(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

func TestAlphanumericValue(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{
		{'0', 0}, {'9', 9},
		{'A', 10}, {'Z', 35},
		{' ', 36}, {'$', 37}, {'%', 38}, {'*', 39}, {'+', 40},
		{'-', 41}, {'.', 42}, {'/', 43}, {':', 44},
		{'a', -1}, {'#', -1}, {'!', -1},
	}
	for _, c := range cases {
		t.Run(string(c.r), func(t *testing.T) {
			if got := alphanumericValue(c.r); got != c.want {
				t.Errorf("alphanumericValue(%q) = %d, want %d", c.r, got, c.want)
			}
		})
	}
}

func TestPayloadBitLength(t *testing.T) {
	cases := []struct {
		name string
		mode Mode
		in   string
		want int
	}{
		// Numeric: groups of 3 digits cost 10 bits; tail 1=4 bits, 2=7 bits.
		{"numeric 1 digit", ModeNumeric, "1", 4},
		{"numeric 2 digits", ModeNumeric, "12", 7},
		{"numeric 3 digits", ModeNumeric, "123", 10},
		{"numeric 8 digits", ModeNumeric, "12345678", 27}, // 2*10 + 7
		// Alphanumeric: pairs cost 11 bits; single tail costs 6.
		{"alphanumeric 1 char", ModeAlphanumeric, "A", 6},
		{"alphanumeric 2 chars", ModeAlphanumeric, "AB", 11},
		{"alphanumeric HELLO WORLD", ModeAlphanumeric, "HELLO WORLD", 61}, // 5*11 + 6
		// Byte: 8 bits per byte.
		{"byte 1 char", ModeByte, "A", 8},
		{"byte 4 chars", ModeByte, "Test", 32},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := payloadBitLength(c.mode, c.in); got != c.want {
				t.Errorf("payloadBitLength(%s, %q) = %d, want %d", c.mode, c.in, got, c.want)
			}
		})
	}
}

func TestWriteNumeric(t *testing.T) {
	// "01234567" → 012 | 345 | 67 → 0000001100 0101011001 1000011 → 27 bits.
	bb := &bitBuffer{}
	writeNumeric(bb, "01234567")
	if bb.bits() != 27 {
		t.Fatalf("bits() = %d, want 27", bb.bits())
	}
	// 27 bits packed MSB-first into 4 bytes:
	// 00000011 00010101 10011000 011????? = 0x03 0x15 0x98 0x60
	want := []byte{0x03, 0x15, 0x98, 0x60}
	if !bytes.Equal(bb.bytes(), want) {
		t.Errorf("bytes = % X, want % X", bb.bytes(), want)
	}
}

func TestWriteAlphanumericHEPair(t *testing.T) {
	// "HE" → 17*45+14 = 779 → 01100001011 (11 bits).
	bb := &bitBuffer{}
	writeAlphanumeric(bb, "HE")
	if bb.bits() != 11 {
		t.Fatalf("bits() = %d, want 11", bb.bits())
	}
	// 11 bits packed: 01100001 011????? = 0x61 0x60.
	want := []byte{0x61, 0x60}
	if !bytes.Equal(bb.bytes(), want) {
		t.Errorf("bytes = % X, want % X", bb.bytes(), want)
	}
}

func TestSelectVersion(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		ec     ECLevel
		want   Version
		errExp error
	}{
		// HELLO WORLD payload = 4 + 9 + 61 = 74 bits. V1-M holds 128. So V1.
		// All inputs here are homogeneous, so the optimal segmentation is a
		// single segment and version selection matches the single-mode math.
		{"hello world m", "HELLO WORLD", ECLevelM, 1, nil},
		// Empty: header = 4 + 9 + 0 = 13 bits. V1 anywhere.
		{"empty l", "", ECLevelL, 1, nil},
		// 17 alphanumeric chars at H. Payload bits = 17/2*11 + 6 = 88 + 6 = 94.
		// Header at V1: 4 + 9 = 13. Total 107. V1-H = 9 codewords = 72 bits. Too tight.
		// V2-H = 16 codewords = 128 bits. Header at V2 still 13 (V2 is in v1-9 range).
		// Total 107 fits in 128, so V2.
		{"17 chars h", "AAAAAAAAAAAAAAAAA", ECLevelH, 2, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := selectVersion(c.input, c.ec)
			if c.errExp != nil {
				if !errors.Is(err, c.errExp) {
					t.Fatalf("err = %v, want %v", err, c.errExp)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if v != c.want {
				t.Errorf("version = %d, want %d", v, c.want)
			}
		})
	}
}

func TestSelectVersionCapacityExceeded(t *testing.T) {
	// 7090 numeric digits exceeds V40-L max of 7089.
	huge := make([]byte, 7090)
	for i := range huge {
		huge[i] = '0'
	}
	_, err := selectVersion(string(huge), ECLevelL)
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("err = %v, want ErrCapacityExceeded", err)
	}
}

// TestEncodeTextHelloWorld is the canonical end-to-end fixture from
// docs/theory/10-worked-example.md: encoding "HELLO WORLD" at EC level M
// must produce exactly the 16 data codewords listed there.
func TestEncodeTextHelloWorld(t *testing.T) {
	data, v, err := encodeText("HELLO WORLD", ECLevelM, 0)
	if err != nil {
		t.Fatalf("encodeText: %v", err)
	}
	if v != 1 {
		t.Errorf("version = %d, want 1", v)
	}
	// "HELLO WORLD" is homogeneous alphanumeric, so segmentation yields a
	// single alphanumeric segment and the golden bytes are unchanged.
	want := []byte{
		0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D,
		0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11,
	}
	if !bytes.Equal(data, want) {
		t.Errorf("data = % X\nwant   % X", data, want)
	}
	if len(data) != Version(1).DataCodewords(ECLevelM) {
		t.Errorf("len(data) = %d, want %d", len(data), Version(1).DataCodewords(ECLevelM))
	}
}

// TestEncodeTextLengthAlwaysMatchesCapacity asserts that the final byte stream
// length equals v.DataCodewords(ec) regardless of input, for a sample of
// (text, ec) pairs.
func TestEncodeTextLengthAlwaysMatchesCapacity(t *testing.T) {
	cases := []struct {
		name string
		text string
		ec   ECLevel
	}{
		{"empty L", "", ECLevelL},
		{"empty H", "", ECLevelH},
		{"numeric short", "12345", ECLevelM},
		{"alphanumeric short", "HELLO WORLD", ECLevelQ},
		{"byte short", "Hello, World!", ECLevelM},
		{"byte longer", "The quick brown fox jumps over the lazy dog.", ECLevelL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, v, err := encodeText(c.text, c.ec, 0)
			if err != nil {
				t.Fatalf("encodeText: %v", err)
			}
			want := v.DataCodewords(c.ec)
			if len(data) != want {
				t.Errorf("len(data) = %d, want %d (V%d-%s)", len(data), want, v, c.ec)
			}
		})
	}
}

// TestEncodeMixedPayloadRoundTrip exercises the v0.6 segmentation end to end:
// payloads that mix character classes must round-trip exactly through both the
// matrix path (Matrix -> DecodeMatrix) and the byte path (Encode -> DecodeBytes),
// proving the multi-segment bit stream the encoder now emits is decodable.
func TestEncodeMixedPayloadRoundTrip(t *testing.T) {
	payloads := []string{
		"Order #1234567890",
		"Invoice INV-2026 000123456789 total",
		"café☕ 1234567890",
		"https://example.com/cart?id=000111222333444555",
		"ABC" + strings.Repeat("0", 80) + "xyz",
	}
	for _, p := range payloads {
		t.Run(p, func(t *testing.T) {
			grid, err := Matrix(p)
			if err != nil {
				t.Fatalf("Matrix: %v", err)
			}
			if got, err := DecodeMatrix(grid); err != nil {
				t.Fatalf("DecodeMatrix: %v", err)
			} else if got != p {
				t.Errorf("matrix round-trip = %q, want %q", got, p)
			}

			png, err := Encode(p)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if got, err := DecodeBytes(png); err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			} else if got != p {
				t.Errorf("bytes round-trip = %q, want %q", got, p)
			}
		})
	}
}

// TestEncodeMixedPayloadHonoursVersionAndMask confirms WithVersion and WithMask
// still apply when the payload is segmented into multiple modes.
func TestEncodeMixedPayloadHonoursVersionAndMask(t *testing.T) {
	const p = "Order #1234567890"
	grid, err := Matrix(p, WithVersion(5), WithMask(3))
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	if len(grid) != Version(5).Size() {
		t.Errorf("grid side = %d, want %d (V5)", len(grid), Version(5).Size())
	}
	if got, err := DecodeMatrix(grid); err != nil {
		t.Fatalf("DecodeMatrix: %v", err)
	} else if got != p {
		t.Errorf("round-trip = %q, want %q", got, p)
	}
}

// TestEncodeMixedPayloadFitsSmallerVersion is an end-to-end optimality check:
// a byte payload with a long embedded digit run encodes into a strictly
// smaller bit count under segmentation than the greedy single-mode choice
// would, and still round-trips.
func TestEncodeMixedPayloadFitsSmallerVersion(t *testing.T) {
	const p = "Order #1234567890"
	segmented := segmentsBitLength(segmentText(p, 1), 1)
	greedy := greedyBitLength(p, 1)
	if segmented >= greedy {
		t.Errorf("segmented %d bits not smaller than greedy %d", segmented, greedy)
	}
	png, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got, err := DecodeBytes(png); err != nil || got != p {
		t.Fatalf("round-trip got %q, err %v; want %q", got, err, p)
	}
}
