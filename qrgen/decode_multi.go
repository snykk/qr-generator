// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "sort"

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
