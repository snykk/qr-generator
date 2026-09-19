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
	"image"
	"math"
	"sort"
)

// decodedSymbol is one QR symbol recovered from a bitmap by the multi-symbol
// grouping loop: its text plus the geometry needed to locate it (the winning
// version and homography, and the ordered finder triple).
type decodedSymbol struct {
	text string
	v    Version
	hg   homography
	tq   finderTripleQuad
}

// decodeMultipleFromBitmap finds and decodes every QR symbol in a binarised
// bitmap. It enumerates all geometrically valid finder triples
// (groupFinderTriples), tries them best-first, and claims a triple's three
// finders only when it actually Reed-Solomon decodes -- so a plausible but bogus
// grouping (for example a cross-symbol triple in a same-size grid) fails to
// decode, claims nothing, and leaves its finders free for the real triples.
//
// The attempt cap scales with the finder count: a same-size grid can produce
// many geometry-passing cross-symbol triples, and a fixed cap could exhaust on
// them before the real symbols are reached. Triples that reuse an already
// claimed finder are skipped without spending an attempt.
//
// See docs/theory/25-multi-symbol-decoding.md.
func decodeMultipleFromBitmap(bm *bitmap) []decodedSymbol {
	quads, err := detectFinderQuads(bm)
	if err != nil {
		return nil
	}
	if len(quads) > maxCCCandidates {
		sort.Slice(quads, func(i, j int) bool {
			return quads[i].center.moduleSize > quads[j].center.moduleSize
		})
		quads = quads[:maxCCCandidates]
	}

	centres := make([]finderCandidate, len(quads))
	for i := range quads {
		centres[i] = quads[i].center
	}
	triples := groupFinderTriples(centres)

	claimed := make([]bool, len(quads))
	maxAttempts := 4*len(quads) + 16
	attempts := 0
	var out []decodedSymbol
	for _, tr := range triples {
		if attempts >= maxAttempts {
			break
		}
		if claimed[tr.tl] || claimed[tr.tr] || claimed[tr.bl] {
			continue
		}
		tq := finderTripleQuad{tl: quads[tr.tl], tr: quads[tr.tr], bl: quads[tr.bl]}
		text, v, hg, derr := decodeOneQuadTriple(bm, tq)
		attempts++
		if derr != nil {
			continue
		}
		claimed[tr.tl], claimed[tr.tr], claimed[tr.bl] = true, true, true
		out = append(out, decodedSymbol{text: text, v: v, hg: hg, tq: tq})
	}
	return out
}

// Result is one QR symbol decoded from an image by [DecodeAll] or
// [DecodeAllBytes].
type Result struct {
	// Text is the decoded payload of this symbol.
	Text string

	// Corners are the four outer corners of the symbol in source-image pixel
	// coordinates, in reading order relative to the symbol itself:
	// [0]=top-left, [1]=top-right, [2]=bottom-right, [3]=bottom-left. For a
	// rotated or perspective-tilted symbol the quad rotates and shears with it,
	// tracing the symbol's true outline rather than an axis-aligned box.
	Corners [4]image.Point

	// Finders are the centres of the three finder patterns in pixel
	// coordinates: [0]=top-left, [1]=top-right, [2]=bottom-left. With Corners
	// they pin down the symbol's orientation.
	Finders [3]image.Point

	// Version is the decoded QR version, 1..40.
	Version Version
}

// Bounds returns the axis-aligned bounding rectangle of the symbol in
// source-pixel coordinates, derived from Corners. Convenient for cropping or
// drawing a highlight; it is a loose box for a tilted symbol (use Corners for
// the exact outline).
func (r Result) Bounds() image.Rectangle {
	minX, minY := r.Corners[0].X, r.Corners[0].Y
	maxX, maxY := minX, minY
	for _, p := range r.Corners[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// DecodeAll finds and decodes every QR symbol present in img and returns one
// Result per symbol that decoded successfully, in reading order (top to bottom,
// then left to right by each symbol's top-left corner).
//
// Per-symbol failures are not fatal: a symbol whose finder pattern, format
// info, or Reed-Solomon blocks cannot be recovered is skipped, and the symbols
// that did decode are still returned. DecodeAll returns a non-nil error only
// when the image as a whole yields nothing decodable, in which case it is
// [ErrFinderNotFound]. It never returns a partial slice together with a
// non-nil error.
//
// DecodeAll uses the connected-component detector, so it reads symbols at
// arbitrary rotation and moderate perspective tilt (see
// docs/theory/25-multi-symbol-decoding.md). It does not reassemble a
// structured-append set; use [DecodeStructuredAppend] for that. The existing
// single-symbol [Decode] is unchanged.
func DecodeAll(img image.Image) ([]Result, error) {
	syms := decodeAllFromImage(img)
	if len(syms) == 0 {
		return nil, ErrFinderNotFound
	}
	results := make([]Result, len(syms))
	for i, s := range syms {
		results[i] = Result{
			Text:    s.text,
			Corners: symbolCorners(s.v, s.hg),
			Finders: [3]image.Point{
				roundPoint(s.tq.tl.center.x, s.tq.tl.center.y),
				roundPoint(s.tq.tr.center.x, s.tq.tr.center.y),
				roundPoint(s.tq.bl.center.x, s.tq.bl.center.y),
			},
			Version: s.v,
		}
	}
	sortReadingOrder(results)
	return results, nil
}

// DecodeAllBytes is the multi-symbol counterpart of [DecodeBytes]: it decodes a
// PNG, JPEG, or GIF byte slice via image.Decode and runs [DecodeAll] on it. A
// malformed image byte stream is reported as the error from image.Decode.
func DecodeAllBytes(data []byte) ([]Result, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return DecodeAll(img)
}

// decodeAllFromImage binarises img and decodes every symbol, reusing the same
// Otsu/Sauvola dispatch as the single-symbol projective path: Sauvola when the
// histogram is unimodal, otherwise Otsu with a reactive Sauvola retry when Otsu
// yields no symbols at all.
func decodeAllFromImage(img image.Image) []decodedSymbol {
	pixels, w, h := imageToGrayscale(img)
	t, eta := otsuThreshold(pixels)
	if eta < etaMin {
		return decodeMultipleFromBitmap(sauvolaBinariseFromGray(pixels, w, h))
	}
	if syms := decodeMultipleFromBitmap(otsuBinariseFromGray(pixels, w, h, t)); len(syms) > 0 {
		return syms
	}
	return decodeMultipleFromBitmap(sauvolaBinariseFromGray(pixels, w, h))
}

// symbolCorners returns the four outer corners of a symbol in pixel coordinates
// by applying its homography to the module-grid outer edges. A symbol of n
// modules spans module coordinates -0.5..n-0.5, so these are the true (possibly
// perspective-distorted) corners, not the finder-ring corners. Order is
// [top-left, top-right, bottom-right, bottom-left].
func symbolCorners(v Version, hg homography) [4]image.Point {
	n := float64(v.Size())
	corner := func(col, row float64) image.Point {
		px, py := hg.apply(col, row)
		return roundPoint(px, py)
	}
	return [4]image.Point{
		corner(-0.5, -0.5),
		corner(n-0.5, -0.5),
		corner(n-0.5, n-0.5),
		corner(-0.5, n-0.5),
	}
}

func roundPoint(x, y float64) image.Point {
	return image.Pt(int(math.Round(x)), int(math.Round(y)))
}

// sortReadingOrder puts results into a deterministic reading order: symbols are
// banded into rows by their top-left corner's vertical position (band width is
// half the median symbol height), rows run top to bottom and symbols left to
// right within a row, with an exact (Y, then input order) tie-break so the
// output is reproducible. Best-effort for grid and document layouts.
func sortReadingOrder(results []Result) {
	if len(results) < 2 {
		return
	}
	heights := make([]int, len(results))
	for i, r := range results {
		heights[i] = r.Bounds().Dy()
	}
	sort.Ints(heights)
	band := heights[len(heights)/2] / 2
	if band < 1 {
		band = 1
	}
	sort.SliceStable(results, func(a, b int) bool {
		ta, tb := results[a].Corners[0], results[b].Corners[0]
		ra, rb := ta.Y/band, tb.Y/band
		if ra != rb {
			return ra < rb
		}
		if ta.X != tb.X {
			return ta.X < tb.X
		}
		return ta.Y < tb.Y
	})
}
