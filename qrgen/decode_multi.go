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

// decodedSymbol is one symbol recovered from a bitmap by the multi-symbol
// loop: its text plus the geometry needed to locate it (the winning version and
// homography, and the ordered finder triple). For a Micro QR symbol, micro is
// its version, v is zero, and tq.tl holds its single finder; an rMQR symbol is
// the same with rmqr in place of micro.
type decodedSymbol struct {
	text  string
	v     Version
	micro MicroVersion
	rmqr  RMQRVersion
	hg    homography
	tq    finderTripleQuad
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
// After the QR triples, every finder no QR symbol claimed is tried as the
// single finder of a Micro QR symbol and claimed if it decodes, so a QR finder
// is never offered to the Micro search once its symbol has decoded; the
// finders still unclaimed are then tried as the finder of an rMQR symbol.
//
// See docs/theory/25-multi-symbol-decoding.md,
// docs/theory/27-micro-qr-detection.md, and docs/theory/30-rmqr-detection.md.
func decodeMultipleFromBitmap(bm *bitmap) []decodedSymbol {
	scan := scanComponents(bm)
	quads := scan.quads
	if len(quads) == 0 {
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
	for i, q := range quads {
		if claimed[i] {
			continue
		}
		if text, mv, hg, _, err := decodeMicroQuad(bm, q); err == nil {
			claimed[i] = true
			out = append(out, decodedSymbol{text: text, micro: mv, hg: hg, tq: finderTripleQuad{tl: q}})
		}
	}
	for i, q := range quads {
		if claimed[i] {
			continue
		}
		if text, rv, hg, _, err := decodeRMQRQuad(bm, scan, q); err == nil {
			claimed[i] = true
			out = append(out, decodedSymbol{text: text, rmqr: rv, hg: hg, tq: finderTripleQuad{tl: q}})
		}
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
	// they pin down the symbol's orientation. A Micro QR or rMQR symbol has a
	// single finder, in Finders[0]; the other two entries are zero.
	Finders [3]image.Point

	// Version is the decoded QR version, 1..40, and zero for a Micro QR or
	// rMQR symbol.
	Version Version

	// MicroVersion is the version of a Micro QR symbol, M1..M4, and zero
	// otherwise.
	MicroVersion MicroVersion

	// RMQRVersion is the version of an rMQR symbol, R7x43..R17x139, and zero
	// otherwise. With MicroVersion it tells the three kinds apart: a QR symbol
	// has both zero.
	RMQRVersion RMQRVersion
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

// DecodeAll finds and decodes every QR, Micro QR, and rMQR symbol present in
// img and returns one Result per symbol that decoded successfully, in reading
// order (top to bottom, then left to right by each symbol's top-left corner).
// A Micro QR symbol is reported with its MicroVersion set and Version zero, an
// rMQR symbol with its RMQRVersion set and Version zero.
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
// docs/theory/25-multi-symbol-decoding.md). Micro QR symbols are found from
// their single finder pattern among the finders no QR symbol claimed (see
// docs/theory/27-micro-qr-detection.md), and rMQR symbols from their finder and
// finder sub-pattern among the finders still unclaimed (see
// docs/theory/30-rmqr-detection.md). It does not reassemble a
// structured-append set; use [DecodeStructuredAppend] for that. The existing
// single-symbol [Decode] is unchanged.
func DecodeAll(img image.Image) ([]Result, error) {
	syms := decodeAllFromImage(img)
	if len(syms) == 0 {
		return nil, ErrFinderNotFound
	}
	results := make([]Result, len(syms))
	for i, s := range syms {
		tl := roundPoint(s.tq.tl.center.x, s.tq.tl.center.y)
		if s.micro != 0 {
			results[i] = Result{
				Text:         s.text,
				Corners:      symbolCorners(s.micro.Size(), s.hg),
				Finders:      [3]image.Point{tl},
				MicroVersion: s.micro,
			}
			continue
		}
		if s.rmqr != 0 {
			results[i] = Result{
				Text:        s.text,
				Corners:     gridCorners(s.rmqr.Height(), s.rmqr.Width(), s.hg),
				Finders:     [3]image.Point{tl},
				RMQRVersion: s.rmqr,
			}
			continue
		}
		results[i] = Result{
			Text:    s.text,
			Corners: symbolCorners(s.v.Size(), s.hg),
			Finders: [3]image.Point{
				tl,
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

// symbolCorners returns the four outer corners of a symbol of side n modules in
// pixel coordinates by applying its homography to the module-grid outer edges.
// The symbol spans module coordinates -0.5..n-0.5, so these are the true
// (possibly perspective-distorted) corners, not the finder-ring corners. Order
// is [top-left, top-right, bottom-right, bottom-left].
func symbolCorners(side int, hg homography) [4]image.Point {
	return gridCorners(side, side, hg)
}

// gridCorners is symbolCorners for a symbol of rows by cols modules, the
// shape of an rMQR symbol.
func gridCorners(rows, cols int, hg homography) [4]image.Point {
	w, h := float64(cols), float64(rows)
	corner := func(col, row float64) image.Point {
		px, py := hg.apply(col, row)
		return roundPoint(px, py)
	}
	return [4]image.Point{
		corner(-0.5, -0.5),
		corner(w-0.5, -0.5),
		corner(w-0.5, h-0.5),
		corner(-0.5, h-0.5),
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
