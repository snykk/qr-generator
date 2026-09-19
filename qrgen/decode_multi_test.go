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
	"image"
	"image/png"
	"testing"
)

func mkFinder(x, y float64) finderCandidate {
	return finderCandidate{x: x, y: y, moduleSize: 10}
}

// greedyClaim mirrors the disjoint claim in decodeMultipleFromBitmap, but on
// geometry alone (accept every valid triple in cost order), so a pure grouping
// test needs no image.
func greedyClaim(triples []orderedTriple, n int) [][3]int {
	claimed := make([]bool, n)
	var groups [][3]int
	for _, tr := range triples {
		if claimed[tr.tl] || claimed[tr.tr] || claimed[tr.bl] {
			continue
		}
		claimed[tr.tl], claimed[tr.tr], claimed[tr.bl] = true, true, true
		groups = append(groups, [3]int{tr.tl, tr.tr, tr.bl})
	}
	return groups
}

func TestGroupFinderTriplesSingle(t *testing.T) {
	// One symbol: right angle at index 0, TR at index 1, BL at index 2.
	c := []finderCandidate{mkFinder(100, 100), mkFinder(240, 100), mkFinder(100, 240)}
	triples := groupFinderTriples(c)
	if len(triples) != 1 {
		t.Fatalf("got %d triples, want 1", len(triples))
	}
	got := triples[0]
	if got.tl != 0 || got.tr != 1 || got.bl != 2 {
		t.Errorf("ordering = (tl=%d tr=%d bl=%d), want (0,1,2)", got.tl, got.tr, got.bl)
	}
	if got.cost > 0.01 {
		t.Errorf("cost = %.4f, want ~0 for a clean square", got.cost)
	}
}

func TestGroupFinderTriplesTwoSymbols(t *testing.T) {
	// Two well-separated symbols of the same module size.
	c := []finderCandidate{
		mkFinder(100, 100), mkFinder(240, 100), mkFinder(100, 240), // A
		mkFinder(500, 100), mkFinder(640, 100), mkFinder(500, 240), // B
	}
	triples := groupFinderTriples(c)
	if len(triples) < 2 {
		t.Fatalf("got %d triples, want >=2", len(triples))
	}
	groups := greedyClaim(triples, len(c))
	if len(groups) != 2 {
		t.Fatalf("greedy claim got %d disjoint groups, want 2", len(groups))
	}
	// Every finder is claimed exactly once across the two groups.
	seen := make([]bool, len(c))
	for _, g := range groups {
		for _, idx := range g {
			if seen[idx] {
				t.Errorf("finder %d claimed twice", idx)
			}
			seen[idx] = true
		}
	}
	for i, s := range seen {
		if !s {
			t.Errorf("finder %d not claimed by either symbol", i)
		}
	}
}

func TestGroupFinderTriplesRejectsCollinear(t *testing.T) {
	c := []finderCandidate{mkFinder(100, 100), mkFinder(200, 100), mkFinder(300, 100)}
	if triples := groupFinderTriples(c); len(triples) != 0 {
		t.Errorf("collinear centres produced %d triples, want 0", len(triples))
	}
}

func TestGroupFinderTriplesTooFew(t *testing.T) {
	c := []finderCandidate{mkFinder(100, 100), mkFinder(240, 100)}
	if triples := groupFinderTriples(c); len(triples) != 0 {
		t.Errorf("two candidates produced %d triples, want 0", len(triples))
	}
}

// TestDecodeMultipleSingleSymbol is the engine smoke test: one rendered upright
// symbol must decode to exactly one decodedSymbol carrying the right text. Full
// multi-symbol composites are covered in MS5.
func TestDecodeMultipleSingleSymbol(t *testing.T) {
	const payload = "HELLO WORLD"
	pngBytes, err := Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	px, w, h := imageToGrayscale(img)
	th, _ := otsuThreshold(px)
	bm := otsuBinariseFromGray(px, w, h, th)

	syms := decodeMultipleFromBitmap(bm)
	if len(syms) != 1 {
		t.Fatalf("got %d symbols, want 1", len(syms))
	}
	if syms[0].text != payload {
		t.Errorf("text = %q, want %q", syms[0].text, payload)
	}
}

func TestDecodeAllSingleSymbol(t *testing.T) {
	const payload = "HELLO WORLD"
	pngBytes, err := Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	results, err := DecodeAll(img)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Text != payload {
		t.Errorf("Text = %q, want %q", r.Text, payload)
	}
	if r.Version < MinVersion || r.Version > MaxVersion {
		t.Errorf("Version = %d, want 1..40", r.Version)
	}
	// Every corner sits inside the source image.
	b := img.Bounds()
	for i, c := range r.Corners {
		if c.X < b.Min.X || c.X > b.Max.X || c.Y < b.Min.Y || c.Y > b.Max.Y {
			t.Errorf("corner %d = %v outside image %v", i, c, b)
		}
	}
	if bb := r.Bounds(); bb.Dx() <= 0 || bb.Dy() <= 0 {
		t.Errorf("Bounds() = %v, want positive area", bb)
	}
}

func TestDecodeAllBytes(t *testing.T) {
	pngBytes, err := Encode("HELLO WORLD")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	results, err := DecodeAllBytes(pngBytes)
	if err != nil {
		t.Fatalf("DecodeAllBytes: %v", err)
	}
	if len(results) != 1 || results[0].Text != "HELLO WORLD" {
		t.Fatalf("results = %+v, want one HELLO WORLD", results)
	}
}

func TestDecodeAllEmptyImage(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 120, 120))
	for i := range img.Pix {
		img.Pix[i] = 255 // blank white: no finders
	}
	_, err := DecodeAll(img)
	if !errors.Is(err, ErrFinderNotFound) {
		t.Errorf("err = %v, want ErrFinderNotFound", err)
	}
}
