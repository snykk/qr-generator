// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

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
