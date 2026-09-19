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
	"image/color"
	"image/draw"
	"image/png"
	"testing"
)

// renderQR encodes payload to a PNG and decodes it back to an image, one tile
// for compositing multi-symbol test scenes.
func renderQR(t *testing.T, payload string, opts ...Option) image.Image {
	t.Helper()
	pngBytes, err := Encode(payload, opts...)
	if err != nil {
		t.Fatalf("Encode(%q): %v", payload, err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	return img
}

type placedTile struct {
	img  image.Image
	x, y int
}

// compositeTiles draws each tile onto one white canvas at its offset, building
// a multi-symbol image in memory without any binary fixture.
func compositeTiles(w, h int, tiles ...placedTile) image.Image {
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for _, ti := range tiles {
		b := ti.img.Bounds()
		dst := image.Rect(ti.x, ti.y, ti.x+b.Dx(), ti.y+b.Dy())
		draw.Draw(canvas, dst, ti.img, b.Min, draw.Src)
	}
	return canvas
}

func resultTexts(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Text
	}
	return out
}

// assertTextSet checks the decoded payloads match want as a set (order aside).
func assertTextSet(t *testing.T, results []Result, want ...string) {
	t.Helper()
	got := map[string]int{}
	for _, r := range results {
		got[r.Text]++
	}
	if len(results) != len(want) {
		t.Errorf("got %d results %v, want %d %v", len(results), resultTexts(results), len(want), want)
	}
	for _, w := range want {
		if got[w] == 0 {
			t.Errorf("missing payload %q; got %v", w, resultTexts(results))
		}
	}
}

func TestDecodeAllTwoUpright(t *testing.T) {
	a := renderQR(t, "ALPHA")
	b := renderQR(t, "BRAVO")
	tw, th := a.Bounds().Dx(), a.Bounds().Dy()
	canvas := compositeTiles(tw*2+80, th+40,
		placedTile{a, 20, 20},
		placedTile{b, tw + 60, 20})

	results, err := DecodeAll(canvas)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	assertTextSet(t, results, "ALPHA", "BRAVO")
	// Same row, so reading order runs left to right: ALPHA before BRAVO.
	if len(results) == 2 && results[0].Text != "ALPHA" {
		t.Errorf("reading order = %v, want ALPHA first", resultTexts(results))
	}
}

func TestDecodeAllDifferentSizes(t *testing.T) {
	const bigPayload = "DATA-DATA-DATA-DATA-DATA-DATA-0123456789"
	small := renderQR(t, "HI")
	big := renderQR(t, bigPayload)
	sw, sh := small.Bounds().Dx(), small.Bounds().Dy()
	bw, bh := big.Bounds().Dx(), big.Bounds().Dy()
	canvas := compositeTiles(sw+bw+120, max(sh, bh)+40,
		placedTile{small, 20, 20},
		placedTile{big, sw + 80, 20})

	results, err := DecodeAll(canvas)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	assertTextSet(t, results, "HI", bigPayload)
	ver := map[string]Version{}
	for _, r := range results {
		ver[r.Text] = r.Version
	}
	if ver["HI"] == ver[bigPayload] {
		t.Errorf("versions did not differ: HI=%d big=%d", ver["HI"], ver[bigPayload])
	}
}

func TestDecodeAllTwoTilts(t *testing.T) {
	t1 := warpTilt(t, tiltSource(t, "TILT-A-111"), 30, 8)
	t2 := warpTilt(t, tiltSource(t, "TILT-B-222"), 40, -8)
	w := t1.Bounds().Dx() + t2.Bounds().Dx() + 120
	h := max(t1.Bounds().Dy(), t2.Bounds().Dy()) + 40
	canvas := compositeTiles(w, h,
		placedTile{t1, 20, 20},
		placedTile{t2, t1.Bounds().Dx() + 80, 20})

	results, err := DecodeAll(canvas)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	assertTextSet(t, results, "TILT-A-111", "TILT-B-222")
}

func TestDecodeAllMatchesSingleDecode(t *testing.T) {
	img := renderQR(t, "SINGLE-SYMBOL-42")
	single, err := Decode(img)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	all, err := DecodeAll(img)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("DecodeAll returned %d results, want 1", len(all))
	}
	if all[0].Text != single {
		t.Errorf("DecodeAll text = %q, want %q (Decode)", all[0].Text, single)
	}
}

func TestDecodeAllAdjacent(t *testing.T) {
	// Tiles touching edge to edge: their quiet zones abut but the symbol data
	// stays separated, so both must still decode.
	a := renderQR(t, "LEFT-CODE")
	b := renderQR(t, "RIGHT-CODE")
	tw, th := a.Bounds().Dx(), a.Bounds().Dy()
	canvas := compositeTiles(tw*2+40, th+40,
		placedTile{a, 20, 20},
		placedTile{b, tw + 20, 20})

	results, err := DecodeAll(canvas)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	assertTextSet(t, results, "LEFT-CODE", "RIGHT-CODE")
}

func TestDecodeAllGrid(t *testing.T) {
	// A 3x3 grid of same-size symbols: the case that stresses the attempt cap,
	// since same-pitch cross-symbol triples pass the geometry gate.
	payloads := []string{"G00", "G01", "G02", "G10", "G11", "G12", "G20", "G21", "G22"}
	tile0 := renderQR(t, payloads[0])
	tw, th := tile0.Bounds().Dx(), tile0.Bounds().Dy()
	const gap = 30
	canvas := image.NewRGBA(image.Rect(0, 0, 3*tw+4*gap, 3*th+4*gap))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			tile := renderQR(t, payloads[r*3+c])
			x, y := gap+c*(tw+gap), gap+r*(th+gap)
			draw.Draw(canvas, image.Rect(x, y, x+tw, y+th), tile, tile.Bounds().Min, draw.Src)
		}
	}

	results, err := DecodeAll(canvas)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	assertTextSet(t, results, payloads...)
}

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
