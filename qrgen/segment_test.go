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

// greedyBitLength is the single-mode encoded length the v0.5 encoder would
// produce: one segment in the most restrictive covering mode.
func greedyBitLength(text string, v Version) int {
	m := analyzeMode(text)
	return 4 + m.CharCountBits(v) + payloadBitLength(m, text)
}

// joinSegments concatenates the segment texts; it must equal the original input.
func joinSegments(segs []segment) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.text)
	}
	return b.String()
}

func TestSegmentTextHomogeneousSingleSegment(t *testing.T) {
	cases := []struct {
		name string
		text string
		want Mode
	}{
		{"numeric", "0123456789", ModeNumeric},
		{"alphanumeric", "HELLO WORLD", ModeAlphanumeric},
		{"alphanumeric with symbols", "HTTP://A.COM/$-./:", ModeAlphanumeric},
		{"byte lowercase", "hello world", ModeByte},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			segs := segmentText(c.text, 1)
			if len(segs) != 1 {
				t.Fatalf("got %d segments, want 1: %+v", len(segs), segs)
			}
			if segs[0].mode != c.want {
				t.Errorf("mode = %v, want %v", segs[0].mode, c.want)
			}
			if segs[0].text != c.text {
				t.Errorf("text = %q, want %q", segs[0].text, c.text)
			}
			// Identity: a homogeneous input must cost exactly the greedy length.
			if got, want := segmentsBitLength(segs, 1), greedyBitLength(c.text, 1); got != want {
				t.Errorf("segmented bits = %d, want greedy %d", got, want)
			}
		})
	}
}

func TestSegmentTextMixedSplitsAndWins(t *testing.T) {
	// "Order #1234567890": lowercase forces byte greedily, but the 10-digit
	// run is long enough to split into a numeric segment. See doc 17 §4.
	const text = "Order #1234567890"
	segs := segmentText(text, 1)
	if joinSegments(segs) != text {
		t.Fatalf("segments do not reconstruct input: %q", joinSegments(segs))
	}
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2: %+v", len(segs), segs)
	}
	if segs[0].mode != ModeByte || segs[0].text != "Order #" {
		t.Errorf("segment 0 = {%v, %q}, want {byte, \"Order #\"}", segs[0].mode, segs[0].text)
	}
	if segs[1].mode != ModeNumeric || segs[1].text != "1234567890" {
		t.Errorf("segment 1 = {%v, %q}, want {numeric, \"1234567890\"}", segs[1].mode, segs[1].text)
	}
	// The whole point: segmentation must be strictly smaller than greedy here.
	got, greedy := segmentsBitLength(segs, 1), greedyBitLength(text, 1)
	if got >= greedy {
		t.Errorf("segmented %d bits not smaller than greedy %d", got, greedy)
	}
	if got != 116 || greedy != 148 {
		t.Errorf("bit counts = segmented %d / greedy %d, want 116 / 148 (doc 17 §4)", got, greedy)
	}
}

func TestSegmentTextShortDigitRunDoesNotSplit(t *testing.T) {
	// "PHONE: 12345" is fully alphanumeric; a 5-digit run is too short to
	// repay a numeric segment header, so the DP keeps one alphanumeric
	// segment. This is the counter-example from doc 17 §4.
	const text = "PHONE: 12345"
	segs := segmentText(text, 1)
	if len(segs) != 1 || segs[0].mode != ModeAlphanumeric {
		t.Fatalf("got %+v, want a single alphanumeric segment", segs)
	}
	if got, greedy := segmentsBitLength(segs, 1), greedyBitLength(text, 1); got != greedy || got != 79 {
		t.Errorf("segmented %d / greedy %d, want both 79", got, greedy)
	}
}

func TestSegmentTextNeverWorseThanGreedy(t *testing.T) {
	payloads := []string{
		"",
		"1",
		"A",
		"a",
		"12345",
		"HELLO WORLD",
		"hello world",
		"Order #1234567890",
		"PHONE: 12345",
		"https://example.com/cart?id=000111222333444555",
		"Invoice INV-2026-000123456789 total $42.00",
		"SKU12345678901234567890 qty 9",
		strings.Repeat("AB12", 50),
		strings.Repeat("9", 200),
		"ABC" + strings.Repeat("0", 100) + "xyz",
	}
	for _, v := range []Version{1, 9, 10, 26, 27, 40} {
		for _, p := range payloads {
			segs := segmentText(p, v)
			if joinSegments(segs) != p {
				t.Fatalf("v%d %q: segments do not reconstruct input (%q)", v, p, joinSegments(segs))
			}
			got := segmentsBitLength(segs, v)
			greedy := greedyBitLength(p, v)
			if got > greedy {
				t.Errorf("v%d %q: segmented %d bits > greedy %d (segmentation must never be worse)", v, p, got, greedy)
			}
		}
	}
}

func TestSegmentTextVersionGroupRecompute(t *testing.T) {
	// The same payload across the three version groups must produce a valid,
	// reconstructing segmentation that is never worse than greedy at each.
	const text = "ABC0000000000000xyz"
	for _, v := range []Version{9, 10, 27} {
		segs := segmentText(text, v)
		if joinSegments(segs) != text {
			t.Fatalf("v%d: reconstruction mismatch %q", v, joinSegments(segs))
		}
		if got, greedy := segmentsBitLength(segs, v), greedyBitLength(text, v); got > greedy {
			t.Errorf("v%d: segmented %d > greedy %d", v, got, greedy)
		}
	}
}

func TestSegmentTextUTF8KeepsRunesWhole(t *testing.T) {
	// Multi-byte runes can only live in byte segments and must never be split.
	// A trailing digit run may or may not split, but the concatenation must be
	// exact and any segment containing a multi-byte rune must be byte mode.
	const text = "café☕ 1234567890"
	segs := segmentText(text, 1)
	if joinSegments(segs) != text {
		t.Fatalf("reconstruction mismatch: %q != %q", joinSegments(segs), text)
	}
	for _, s := range segs {
		if s.mode == ModeByte {
			continue
		}
		// Non-byte segments must be pure ASCII numeric/alphanumeric.
		for _, r := range s.text {
			if r > 127 {
				t.Errorf("non-byte segment %+v contains multi-byte rune %q", s, r)
			}
		}
	}
}

func TestSegmentsBitLengthMatchesEncoding(t *testing.T) {
	// segmentsBitLength of a single segment equals the greedy header+payload.
	segs := []segment{{mode: ModeNumeric, text: "12345"}}
	// V1 numeric: 4 + 10 + (floor(5/3)*10 + 7) = 14 + 17 = 31.
	if got := segmentsBitLength(segs, 1); got != 31 {
		t.Errorf("segmentsBitLength = %d, want 31", got)
	}
}

func TestSegmentTextEmpty(t *testing.T) {
	segs := segmentText("", 1)
	if len(segs) != 1 || segs[0].mode != ModeNumeric || segs[0].text != "" {
		t.Fatalf("empty input = %+v, want a single empty numeric segment", segs)
	}
}
