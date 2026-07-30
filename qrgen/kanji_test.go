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

// The worked example from docs/theory/22-kanji-mode.md: 亜 (U+4E9C), Shift-JIS
// 0x889F, encodes to the 13-bit value 0x59F.
func TestKanjiWorkedExample(t *testing.T) {
	v, ok := kanjiValue('亜')
	if !ok || v != 0x59F {
		t.Errorf("kanjiValue('亜') = 0x%X, %v; want 0x59F, true", v, ok)
	}
	r, ok := kanjiRune(0x59F)
	if !ok || r != '亜' {
		t.Errorf("kanjiRune(0x59F) = %q, %v; want '亜', true", r, ok)
	}
}

func TestKanjiEligibility(t *testing.T) {
	// Kanji plus hiragana and katakana are all in JIS X 0208.
	for _, r := range []rune{'亜', '日', '本', '語', 'あ', 'ア'} {
		if !kanjiEligible(r) {
			t.Errorf("kanjiEligible(%q) = false, want true", r)
		}
	}
	// Half-width ASCII and astral runes are not.
	for _, r := range []rune{'A', 'z', '1', '@', '😀'} {
		if kanjiEligible(r) {
			t.Errorf("kanjiEligible(%q) = true, want false", r)
		}
	}
}

// Every mapped code point round-trips through the codec, and every value fits
// the 13-bit field.
func TestKanjiTableRoundTrip(t *testing.T) {
	if len(kanjiByValue) == 0 {
		t.Fatal("kanjiByValue is empty")
	}
	for _, e := range kanjiByValue {
		if e.v > 0x1FFF {
			t.Fatalf("value 0x%X exceeds 13 bits", e.v)
		}
		if r, ok := kanjiRune(e.v); !ok || r != e.r {
			t.Fatalf("kanjiRune(0x%X) = %q, %v; want %q", e.v, r, ok, e.r)
		}
		// The rune encodes to a value that decodes back to the same rune
		// (canonical, since a few runes have more than one Shift-JIS code).
		v, ok := kanjiValue(e.r)
		if !ok {
			t.Fatalf("kanjiValue(%q) reports ineligible but the rune is in the table", e.r)
		}
		if r, ok := kanjiRune(v); !ok || r != e.r {
			t.Fatalf("canonical round-trip for %q failed: value 0x%X -> %q", e.r, v, r)
		}
	}
}

// A pure-Kanji string is a single Kanji segment at the expected bit length.
func TestKanjiSegmentation(t *testing.T) {
	segs := segmentText("日本語", 1)
	if len(segs) != 1 || segs[0].mode != ModeKanji || segs[0].text != "日本語" {
		t.Fatalf("segmentText(日本語) = %+v, want one ModeKanji segment", segs)
	}
	// 4 (mode) + 8 (V1 count) + 3*13 payload.
	if got, want := segmentsBitLength(segs, 1), 4+8+3*13; got != want {
		t.Errorf("segmentsBitLength = %d, want %d", got, want)
	}
}

// A run of kanji embedded in ASCII becomes its own Kanji segment.
func TestKanjiMixedSegmentation(t *testing.T) {
	segs := segmentText("AB日本CD", 1)
	var kanji string
	found := false
	for _, s := range segs {
		if s.mode == ModeKanji {
			kanji, found = s.text, true
		}
	}
	if !found || kanji != "日本" {
		t.Errorf("kanji segment = %q (found %v), want 日本 in %+v", kanji, found, segs)
	}
}

// The emitted stream begins with the Kanji header (mode 1000 + count) followed
// by the 13-bit values.
func TestKanjiEncodesInKanjiMode(t *testing.T) {
	const text = "日本語"
	data, v, err := encodeTextECI(text, ECLevelM, 0, ECINone)
	if err != nil {
		t.Fatalf("encodeTextECI: %v", err)
	}
	br := newBitReader(data)
	mode, _ := br.readBits(4)
	if mode != 0b1000 {
		t.Fatalf("first mode indicator = 0b%04b, want 1000 (Kanji)", mode)
	}
	count, _ := br.readBits(ModeKanji.CharCountBits(v))
	if count != 3 {
		t.Fatalf("kanji count = %d, want 3", count)
	}
	for _, r := range text {
		want, _ := kanjiValue(r)
		got, _ := br.readBits(13)
		if uint16(got) != want {
			t.Errorf("value for %q = 0x%X, want 0x%X", r, got, want)
		}
	}
}

// Adding Kanji to the segmenter must not pull non-Japanese input into Kanji
// mode, so its encoding is unchanged.
func TestKanjiDoesNotAffectNonKanji(t *testing.T) {
	for _, text := range []string{"HELLO WORLD", "12345", "https://example.com", "Order #1234567890"} {
		for _, s := range segmentText(text, 1) {
			if s.mode == ModeKanji {
				t.Errorf("%q produced a Kanji segment: %+v", text, s)
			}
		}
	}
}

// Full round-trip through the image pipeline for pure-Kanji and mixed payloads.
func TestKanjiRoundTrip(t *testing.T) {
	cases := []string{
		"日本語",
		"こんにちは世界",
		"QRコード",
		"テスト 2026 世界",
		strings.Repeat("漢字", 30),
	}
	for _, text := range cases {
		data, err := Encode(text)
		if err != nil {
			t.Fatalf("Encode(%q): %v", text, err)
		}
		got, err := DecodeBytes(data)
		if err != nil {
			t.Fatalf("DecodeBytes(%q): %v", text, err)
		}
		if got != text {
			t.Errorf("round-trip: got %q, want %q", got, text)
		}
	}
}
