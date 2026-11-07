// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"fmt"
	"math"
	"sort"
)

// This file locates Micro QR symbols in a binarised image from a single finder
// pattern. The finder's four ring corners fix a homography up to a quarter
// turn; the two innermost timing modules pick the turn; the format
// information, read before the size is known, names the symbol type; and a
// timing-pattern cross-check confirms the size before the grid is sampled and
// handed to decodeMicroMatrix. See docs/theory/27-micro-qr-detection.md.

// microTimingTolerance is how many of the 2(n-5) modules read by the timing
// cross-check may disagree with a Micro symbol of size n. Clean symbols read
// with no disagreement. Measured over every finder of 224 QR symbols (versions
// 1 to 7, every EC level and mask), the assignments that reached the
// cross-check disagreed in 1 to 14 modules, 8 at the median; three fell within
// this tolerance and Reed-Solomon rejected all three (TestMicroIgnoresQRFinders;
// theory doc 27, sections 5 and 7).
const microTimingTolerance = 2

// finderRingCorners are the module coordinates (column, row) of the outer ring
// corners of a finder whose top-left module is (0, 0), in the order top-left,
// top-right, bottom-right, bottom-left: a positive winding with rows growing
// downward.
var finderRingCorners = [4][2]float64{{-0.5, -0.5}, {6.5, -0.5}, {6.5, 6.5}, {-0.5, 6.5}}

// microStage records how far a finder got through the Micro gates, ordered by
// how strongly it shows a Micro symbol is there, so that when every candidate
// fails, the error of the most convincing one is reported. Only the last two
// are strong evidence: a QR finder passes the timing test about one time in
// three and then names a valid format word about half the time, but its
// surroundings almost never read as a Micro timing pattern.
type microStage int

const (
	microStageNone     microStage = iota // no assignment passed the timing test
	microStageOriented                   // the timing test passed, and nothing after it
	microStageFormat                     // the format word decoded, but the timing pattern disagrees with its size
	microStageTimed                      // the timing pattern matches a Micro size, but the format word did not decode
	microStageLocated                    // every gate passed; the outcome is the matrix decode's
)

// outerRingCorners returns a finder quad's corners corrected for the half-pixel
// bias of extractQuadCorners. Each corner it reports is the centre of the
// ring's outermost pixel, which lies inside the ring's true outer corner by up
// to half a pixel on each axis. Moving each corner outward from the finder
// centre to the boundary of its own pixel removes the bias exactly for an
// upright ring, (0.5, 0.5), and for one turned 45 degrees, (0.5, 0), and
// approximately in between. The bias is a scale error, so it grows with
// distance from the finder: uncorrected, at 3 pixels per module it shifts the
// far timing modules of an M4 symbol by a whole module (theory doc 27,
// section 6). Only the Micro path applies it, so QR decoding is unchanged.
func outerRingCorners(q finderQuad) [4][2]float64 {
	out := q.corners
	for i, p := range out {
		dx, dy := p[0]-q.center.x, p[1]-q.center.y
		m := math.Max(math.Abs(dx), math.Abs(dy))
		if m == 0 {
			continue
		}
		out[i] = [2]float64{p[0] + 0.5*dx/m, p[1] + 0.5*dy/m}
	}
	return out
}

// windCorners orders a convex quad's corners by angle about their centroid,
// ascending in image coordinates (y down). For a symbol seen from the front
// this is the winding of finderRingCorners, whatever the rotation, so matching
// the two in one of their four cyclic shifts is a quarter-turn choice and never
// a mirror image.
func windCorners(c [4][2]float64) [4][2]float64 {
	var cx, cy float64
	for _, p := range c {
		cx += p[0] / 4
		cy += p[1] / 4
	}
	sort.Slice(c[:], func(i, j int) bool {
		return math.Atan2(c[i][1]-cy, c[i][0]-cx) < math.Atan2(c[j][1]-cy, c[j][0]-cx)
	})
	return c
}

// microFinderHomography maps module coordinates to pixels under assignment k:
// finder ring corner i (top-left, top-right, bottom-right, bottom-left) goes to
// wound image corner (i+k) mod 4, so k picks which image corner is the
// symbol's outer corner.
func microFinderHomography(corners [4][2]float64, k int) (homography, error) {
	var dst [4][2]float64
	for i := range 4 {
		dst[i] = corners[(i+k)%4]
	}
	return computeHomography(finderRingCorners, dst)
}

// moduleDark reports whether module (col, row) reads dark through h. A module
// that projects outside the image, or not to a finite point, reads light, like
// the quiet zone.
func moduleDark(bm *bitmap, h homography, col, row int) bool {
	px, py := h.apply(float64(col), float64(row))
	if !(px > -0.5 && py > -0.5 && px < float64(bm.width)-0.5 && py < float64(bm.height)-0.5) {
		return false
	}
	return bm.get(int(math.Round(px)), int(math.Round(py)))
}

// readMicroFormatWord reads the single format copy through h, at the positions
// given by microFormatModule.
func readMicroFormatWord(bm *bitmap, h homography) uint32 {
	var word uint32
	for i := range 15 {
		if r, c := microFormatModule(i); moduleDark(bm, h, c, r) {
			word |= 1 << uint(i)
		}
	}
	return word
}

// microTimingMismatches counts the modules of row 0 and of column 0, from the
// separator at index 7 through the quiet-zone modules at n and n+1, that
// disagree with a Micro symbol of size n: light at 7, dark at even and light at
// odd indices from 8 to n-1, light at n and n+1. Both quiet-zone modules are
// read because odd timing indices are light: index n alone would accept a
// claimed size two smaller than the real one, while index n+1 then lands on a
// dark timing module.
func microTimingMismatches(bm *bitmap, h homography, n int) int {
	miss := 0
	for i := 7; i <= n+1; i++ {
		want := i >= 8 && i < n && i%2 == 0
		if moduleDark(bm, h, i, 0) != want {
			miss++
		}
		if moduleDark(bm, h, 0, i) != want {
			miss++
		}
	}
	return miss
}

// microCandidate is one quarter-turn assignment of a finder that passed every
// gate before decoding: its homography, the symbol type its format word names,
// that word's Hamming distance, and the timing cross-check's disagreements.
type microCandidate struct {
	hg       homography
	sym      microSymbol
	dist     int
	mismatch int
}

// locateMicro tries the four quarter-turn assignments of finder quad q as the
// finder of a Micro symbol. An assignment survives if the two innermost timing
// modules, (row 0, column 8) and (row 8, column 0), read dark; its format word
// lies within the 3-bit budget of one of the 32 Micro words; and the timing
// pattern agrees with the size that word names. Survivors are returned best
// first (nearest format word, then fewest timing disagreements), with the
// furthest stage any assignment reached. An assignment whose format word does
// not decode still counts as timed if its timing pattern matches some Micro
// size: that is a Micro symbol with damaged format information.
func locateMicro(bm *bitmap, q finderQuad) ([]microCandidate, microStage) {
	corners := windCorners(outerRingCorners(q))
	stage := microStageNone
	var out []microCandidate
	for k := range 4 {
		hg, err := microFinderHomography(corners, k)
		if err != nil {
			continue
		}
		if !moduleDark(bm, hg, 8, 0) || !moduleDark(bm, hg, 0, 8) {
			continue
		}
		stage = max(stage, microStageOriented)
		sym, _, dist := nearestMicroFormat(readMicroFormatWord(bm, hg), 0)
		if dist > microFormatBudget {
			if microTimingMatchesSomeSize(bm, hg, 0) {
				stage = max(stage, microStageTimed)
			}
			continue
		}
		stage = max(stage, microStageFormat)
		miss := microTimingMismatches(bm, hg, sym.version.Size())
		if miss > microTimingTolerance {
			continue
		}
		stage = microStageLocated
		out = append(out, microCandidate{hg: hg, sym: sym, dist: dist, mismatch: miss})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].dist != out[j].dist {
			return out[i].dist < out[j].dist
		}
		return out[i].mismatch < out[j].mismatch
	})
	return out, stage
}

// microTimingMatchesSomeSize reports whether the timing pattern seen through h
// matches a Micro symbol of any of the four sizes with at most tol
// disagreements.
func microTimingMatchesSomeSize(bm *bitmap, h homography, tol int) bool {
	for v := M1; v <= M4; v++ {
		if microTimingMismatches(bm, h, v.Size()) <= tol {
			return true
		}
	}
	return false
}

// microStageError is the error reported for a finder that stopped before the
// matrix decode, at stage s.
func microStageError(s microStage) error {
	switch s {
	case microStageNone:
		return fmt.Errorf("%w: no Micro QR timing pattern beside the finder", ErrFinderNotFound)
	case microStageOriented:
		return fmt.Errorf("%w: no Micro QR symbol at the finder", ErrFinderNotFound)
	case microStageFormat:
		return fmt.Errorf("%w: Micro QR timing pattern disagrees with the size its format information names", ErrFinderNotFound)
	default:
		return fmt.Errorf("%w: Micro QR format information", ErrFormatUnreadable)
	}
}

// decodeMicroQuad decodes the Micro symbol whose finder is q: it locates the
// symbol (locateMicro), samples the grid at the size the format information
// names, and decodes it with decodeMicroMatrix, trying the surviving
// assignments best first. It returns the text, the version, the homography from
// module coordinates to pixels, and the furthest stage reached; on failure, the
// error of that stage, which for a located symbol is the best assignment's
// decode error.
func decodeMicroQuad(bm *bitmap, q finderQuad) (string, MicroVersion, homography, microStage, error) {
	cands, stage := locateMicro(bm, q)
	if len(cands) == 0 {
		return "", 0, homography{}, stage, microStageError(stage)
	}
	var firstErr error
	for _, c := range cands {
		text, err := decodeMicroMatrix(sampleMatrix(bm, c.hg, c.sym.version.Size()))
		if err == nil {
			return text, c.sym.version, c.hg, stage, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return "", 0, homography{}, stage, firstErr
}

// decodeMicroFromQuads tries each finder quad as the finder of a Micro symbol,
// largest module first and at most maxCCCandidates of them, and returns the
// first that decodes. On failure it returns the error of the quad that got
// furthest through the gates (the larger one on a tie) together with that
// stage, so a caller can tell a damaged Micro symbol from no Micro symbol.
func decodeMicroFromQuads(bm *bitmap, quads []finderQuad) (string, microStage, error) {
	order := make([]finderQuad, len(quads))
	copy(order, quads)
	sort.SliceStable(order, func(i, j int) bool {
		return order[i].center.moduleSize > order[j].center.moduleSize
	})
	if len(order) > maxCCCandidates {
		order = order[:maxCCCandidates]
	}
	bestStage := microStageNone
	var bestErr error = ErrFinderNotFound // no finder at all
	for i, q := range order {
		text, _, _, stage, err := decodeMicroQuad(bm, q)
		if err == nil {
			return text, stage, nil
		}
		if i == 0 || stage > bestStage {
			bestStage, bestErr = stage, err
		}
	}
	return "", bestStage, bestErr
}
