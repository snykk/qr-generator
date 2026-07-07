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

// The sequence indicator packs position (high nibble) and count-minus-one (low
// nibble), and round-trips.
func TestSASequenceIndicator(t *testing.T) {
	cases := []struct {
		position, total int
		want            uint8
	}{
		{0, 2, 0x01},   // symbol 1 of 2
		{1, 4, 0x13},   // symbol 2 of 4
		{0, 4, 0x03},   // symbol 1 of 4
		{3, 4, 0x33},   // symbol 4 of 4
		{15, 16, 0xFF}, // symbol 16 of 16
	}
	for _, c := range cases {
		got := saSequenceIndicator(c.position, c.total)
		if got != c.want {
			t.Errorf("saSequenceIndicator(%d, %d) = 0x%02X, want 0x%02X", c.position, c.total, got, c.want)
		}
		pos, total := saParseSequence(got)
		if pos != c.position || total != c.total {
			t.Errorf("saParseSequence(0x%02X) = (%d, %d), want (%d, %d)", got, pos, total, c.position, c.total)
		}
	}
}

func TestSAParity(t *testing.T) {
	if got := saParity("AB"); got != 0x03 { // 0x41 ^ 0x42
		t.Errorf("saParity(\"AB\") = 0x%02X, want 0x03", got)
	}
	if got := saParity(""); got != 0 {
		t.Errorf("saParity(\"\") = 0x%02X, want 0", got)
	}
	// Parity over the whole message equals XOR of the concatenated chunks.
	const msg = "Héllo, structured world 123"
	var chunkXOR byte
	for _, chunk := range splitRunes(msg, 3) {
		chunkXOR ^= saParity(chunk)
	}
	if chunkXOR != saParity(msg) {
		t.Errorf("parity of parts (0x%02X) != parity of whole (0x%02X)", chunkXOR, saParity(msg))
	}
}

func TestSplitRunes(t *testing.T) {
	cases := []struct {
		text  string
		parts int
		want  []string
	}{
		{"abcdef", 3, []string{"ab", "cd", "ef"}},
		{"abcde", 3, []string{"ab", "cd", "e"}}, // remainder goes to earlier chunks
		{"héllo", 2, []string{"hél", "lo"}},     // multi-byte runes are not split
	}
	for _, c := range cases {
		got := splitRunes(c.text, c.parts)
		if strings.Join(got, "") != c.text {
			t.Errorf("splitRunes(%q, %d) chunks do not reconstruct the input: %q", c.text, c.parts, got)
		}
		if len(got) != len(c.want) {
			t.Fatalf("splitRunes(%q, %d) = %d chunks, want %d", c.text, c.parts, len(got), len(c.want))
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitRunes(%q, %d)[%d] = %q, want %q", c.text, c.parts, i, got[i], c.want[i])
			}
		}
	}
}

// encodeSASymbol prepends a readable structured-append header (mode 0011 +
// sequence indicator + parity) before the chunk data.
func TestEncodeSASymbolHeader(t *testing.T) {
	data, err := encodeSASymbol("HELLO", ECLevelM, Version(1), 1, 4, 0x03)
	if err != nil {
		t.Fatalf("encodeSASymbol: %v", err)
	}
	br := newBitReader(data)
	mode, err := br.readBits(4)
	if err != nil || mode != 0b0011 {
		t.Fatalf("first mode indicator = 0b%04b (err %v), want 0011 (structured append)", mode, err)
	}
	seq, _ := br.readBits(8)
	if uint8(seq) != saSequenceIndicator(1, 4) {
		t.Errorf("sequence indicator = 0x%02X, want 0x%02X", seq, saSequenceIndicator(1, 4))
	}
	parity, _ := br.readBits(8)
	if byte(parity) != 0x03 {
		t.Errorf("parity = 0x%02X, want 0x03", parity)
	}
}

func TestEncodeStructuredAppendValidation(t *testing.T) {
	for _, parts := range []int{1, 0, 17, -1} {
		if _, err := EncodeStructuredAppend("hello", parts); err == nil {
			t.Errorf("EncodeStructuredAppend with parts=%d: want error, got nil", parts)
		}
	}
	// Structured append does not combine with ECI in this version.
	if _, err := EncodeStructuredAppend("hello", 2, WithECI(ECIUTF8)); err == nil {
		t.Error("EncodeStructuredAppend with WithECI: want error, got nil")
	}
	// A forced version too small for the chunks is rejected.
	if _, err := EncodeStructuredAppend(strings.Repeat("A", 400), 2, WithVersion(1)); err == nil {
		t.Error("EncodeStructuredAppend forced V1 with oversized chunks: want error, got nil")
	}
}

// EncodeStructuredAppend returns one valid PNG per requested symbol.
func TestEncodeStructuredAppendProducesPNGs(t *testing.T) {
	const parts = 3
	pages, err := EncodeStructuredAppend("The quick brown fox jumps over the lazy dog, twice over.", parts)
	if err != nil {
		t.Fatalf("EncodeStructuredAppend: %v", err)
	}
	if len(pages) != parts {
		t.Fatalf("got %d symbols, want %d", len(pages), parts)
	}
	for i, pngBytes := range pages {
		if _, err := png.Decode(bytes.NewReader(pngBytes)); err != nil {
			t.Errorf("symbol %d is not a valid PNG: %v", i+1, err)
		}
	}
}
