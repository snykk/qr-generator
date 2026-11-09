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
	"math/bits"
	"sort"
	"sync"
)

// This file locates rMQR symbols in a binarised image. The finder's four ring
// corners fix a homography up to a quarter turn, as for Micro QR; the top
// timing pattern picks the turn; the format copy beside the finder names the
// version and so both dimensions; and the finder sub-pattern, found where that
// version puts it, anchors the far end of a symbol up to 139 modules long.
// See docs/theory/30-rmqr-detection.md.

// Pairing a finder with a sub-pattern (theory doc 30, section 4). Each
// candidate's expected position is corrected for perspective by its module
// size (subPatternsNear), and the window around it is rmqrPairBase modules
// plus a fraction of the finder-to-sub-pattern distance: rmqrAlongFrac along
// the symbol's long axis and rmqrAcrossFrac across it. Measured over the zint
// symbols, in-plane rotation leaves the true centre within 4 percent of the
// distance of the affine prediction along the axis and 5 percent across it;
// perspective tilt moves it up to 79 percent along and 15 percent across, and
// the correction brings the along error under 28 percent, but no lower, since
// binarisation thickens dark modules and so measures the sub-pattern's light
// ring about a fifth small. Tilt also makes the far modules up to 2.8 times
// the finder's along the axis; rmqrModuleRatio bounds the measured ratio
// either way. Up to rmqrMaxPairs candidates are tried per version, nearest
// first; the second format copy picks among them.
const (
	rmqrPairBase    = 2.0
	rmqrAlongFrac   = 0.6
	rmqrAcrossFrac  = 0.18
	rmqrModuleRatio = 2.6
	rmqrMaxPairs    = 12
)

// rmqrStage records how far a finder got through the rMQR gates, ordered by
// how strongly it shows an rMQR symbol is there (theory doc 30, section 7).
// Only a located symbol, a finder with a sub-pattern where its version puts
// one, is convincing.
type rmqrStage int

const (
	rmqrStageNone     rmqrStage = iota // no quarter turn passed the orientation test
	rmqrStageOriented                  // the orientation test passed, and no format copy named a version
	rmqrStageFormat                    // a format copy named a version, but no symbol of that version lies there
	rmqrStageLocated                   // a finder and a sub-pattern whose top and bottom timing rows read correctly; the outcome is the matrix decode's
)

// rmqrOrientTolerance is how many of the five modules of the orientation test
// may disagree. Measured on the zint symbols under 45 to 55 degrees of tilt at
// 4 and 5 pixels per module, the right quarter turn of a found finder missed
// one module about twice as often as two, since row 0 borders the quiet zone
// and a small error pushes a sample off it; allowing one lets random modules
// pass 6 times in 32 instead of once, which the format and sub-pattern gates
// after it more than absorb (theory doc 30, section 2).
const rmqrOrientTolerance = 1

// rmqrOriented reports whether row 0, columns 7 to 11, reads through h as an
// rMQR symbol's does beside its finder, the light separator and then the top
// timing pattern, dark at even columns, with at most rmqrOrientTolerance
// modules disagreeing.
func rmqrOriented(bm *bitmap, h homography) bool {
	miss := 0
	for c := 7; c <= 11; c++ {
		if moduleDark(bm, h, c, 0) != (c%2 == 0) {
			miss++
		}
	}
	return miss <= rmqrOrientTolerance
}

// readRMQRFormatWord reads one format copy through h, still masked: the copy
// beside the finder, or, when sub is set, the copy beside the sub-pattern of
// an rows by cols symbol.
func readRMQRFormatWord(bm *bitmap, h homography, rows, cols int, sub bool) uint32 {
	var word uint32
	for i := range 18 {
		r, c := rmqrFormatModuleFinder(i)
		if sub {
			r, c = rmqrFormatModuleSub(i, rows, cols)
		}
		if moduleDark(bm, h, c, r) {
			word |= 1 << uint(i)
		}
	}
	return word
}

// nearestRMQRFormat matches a finder-side format word against all 64 words and
// returns the symbol type of the nearest and its Hamming distance.
func nearestRMQRFormat(word uint32) (rmqrSymbol, int) {
	best, bestDist := rmqrSymbol{}, 19
	for v := R7x43; v <= R17x139; v++ {
		for lvl := range 2 {
			if d := bits.OnesCount32(word ^ rmqrFormatMaskFinder ^ bch18Word(rmqrFormatData(v, lvl))); d < bestDist {
				best, bestDist = rmqrSymbol{version: v, lvl: lvl}, d
			}
		}
	}
	return best, bestDist
}

// rmqrSubFormat matches a sub-side format word against the two words of
// version v and returns the nearer level and its distance.
func rmqrSubFormat(word uint32, v RMQRVersion) (lvl, dist int) {
	dist = 19
	for l := range 2 {
		if d := bits.OnesCount32(word ^ rmqrFormatMaskSub ^ bch18Word(rmqrFormatData(v, l))); d < dist {
			lvl, dist = l, d
		}
	}
	return lvl, dist
}

// rmqrSubRing are the module coordinates of the outer corners of the
// sub-pattern's light ring in an rows by cols symbol, in the order top-left,
// top-right, bottom-right, bottom-left.
func rmqrSubRing(rows, cols int) [4][2]float64 {
	x0, x1 := float64(cols)-4.5, float64(cols)-1.5
	y0, y1 := float64(rows)-4.5, float64(rows)-1.5
	return [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

// finderAffine fits the affine map that best sends the finder's ring corners
// to their pixels, finder[i] being finderRingCorners[i]. Extrapolated far
// beyond the finder it is much steadier than the finder homography, whose
// perspective terms, fixed by four corners 7 modules apart, swing widely with
// a fraction of a pixel of corner error: over a long rMQR symbol the
// homography's prediction of the far end was measured up to 27 percent of the
// distance off, the affine map's far less (theory doc 30, section 4). The
// ring corners lie symmetrically about the finder centre (3, 3), so the
// least-squares fit has a closed form.
func finderAffine(finder [4][2]float64) homography {
	var a homography
	for axis := range 2 {
		var sx, sy, mean float64
		for i, p := range finder {
			dx, dy := finderRingCorners[i][0]-3, finderRingCorners[i][1]-3
			sx += dx * p[axis]
			sy += dy * p[axis]
			mean += p[axis] / 4
		}
		gx, gy := sx/49, sy/49 // the sum of squares of the offsets, 4 times 3.5 squared, on each axis
		a[3*axis], a[3*axis+1], a[3*axis+2] = gx, gy, mean-3*gx-3*gy
	}
	a[8] = 1
	return a
}

// subPatternsNear returns the sub-pattern candidates for version v, nearest
// first, around where the finder's affine map predict puts the sub-pattern's
// centre, measured in the finder's own module frame from the finder's centre.
//
// Perspective changes the module size along the symbol, and the affine map
// keeps it constant, so each candidate's expected position is corrected by
// its own module size: along a line, a projective map sends a point D modules
// away to D times the geometric mean of the module sizes at the two ends, so
// with r the candidate's module size over the finder's, the sub-pattern lies
// sqrt(r) times further out than the affine map predicts. What remains is the
// error of the finder's angle and of the measured module sizes, which the
// window absorbs: rmqrPairBase modules plus rmqrAlongFrac of the distance
// along the long axis and rmqrAcrossFrac across it. Several candidates are
// returned because a QR alignment pattern or a light ring in the data may lie
// nearer than the real sub-pattern; the second format copy picks among them
// (theory doc 30, sections 4 and 6).
func subPatternsNear(scan *ccScan, predict homography, v RMQRVersion, finderModule float64) []subPattern {
	rows, cols := v.Height(), v.Width()
	inv, ok := invertAffine(predict)
	if !ok {
		return nil
	}
	du0, dv0 := float64(cols-6), float64(rows-6) // from the finder centre (3, 3) to the sub-pattern centre
	dist := math.Hypot(du0, dv0)
	alongTol := rmqrPairBase + rmqrAlongFrac*dist
	acrossTol := rmqrPairBase + rmqrAcrossFrac*dist
	type near struct {
		sp subPattern
		d  float64
	}
	var found []near
	for _, sp := range scan.subPatterns() {
		r := sp.moduleSize / finderModule
		if r > rmqrModuleRatio || r < 1/rmqrModuleRatio {
			continue
		}
		u, w := inv.apply(sp.x, sp.y)
		g := math.Sqrt(r)
		du, dv := (u-3-g*du0)/alongTol, (w-3-g*dv0)/acrossTol
		if d := du*du + dv*dv; d <= 1 {
			found = append(found, near{sp, d})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].d < found[j].d })
	if len(found) > rmqrMaxPairs {
		found = found[:rmqrMaxPairs]
	}
	out := make([]subPattern, len(found))
	for i, n := range found {
		out[i] = n.sp
	}
	return out
}

// invertAffine inverts an affine homography (bottom row 0, 0, 1).
func invertAffine(a homography) (homography, bool) {
	det := a[0]*a[4] - a[1]*a[3]
	if math.Abs(det) < 1e-12 {
		return homography{}, false
	}
	i0, i1, i3, i4 := a[4]/det, -a[1]/det, -a[3]/det, a[0]/det
	return homography{i0, i1, -(i0*a[2] + i1*a[5]), i3, i4, -(i3*a[2] + i4*a[5]), 0, 0, 1}, true
}

// rmqrHomography fits the homography of an rows by cols symbol to the eight
// corners of its two patterns: the finder's outer ring corners, already wound
// and assigned (finder[i] is finderRingCorners[i]), and the outer corners of
// the sub-pattern's light ring, assigned to their module corners by the
// cyclic shift that lies nearest to h's prediction of them.
func rmqrHomography(scan *ccScan, h homography, finder [4][2]float64, sp subPattern, rows, cols int) (homography, error) {
	ring := rmqrSubRing(rows, cols)
	found := scan.ringCorners(sp)
	shift, bestErr := 0, math.Inf(1)
	for s := range 4 {
		e := 0.0
		for i, m := range ring {
			px, py := h.apply(m[0], m[1])
			p := found[(i+s)%4]
			e += math.Hypot(p[0]-px, p[1]-py)
		}
		if e < bestErr {
			shift, bestErr = s, e
		}
	}
	src := make([][2]float64, 0, 8)
	dst := make([][2]float64, 0, 8)
	for i := range 4 {
		src = append(src, finderRingCorners[i], ring[i])
		dst = append(dst, finder[i], found[(i+shift)%4])
	}
	return computeHomographyLS(src, dst)
}

// rmqrFixedModule is a function module at (col, row) whose value the layout
// fixes.
type rmqrFixedModule struct {
	col, row int
	dark     bool
}

// rmqrBorder is what detection needs of one version's layout, taken from the
// layout itself (placeRMQRFunctionPatterns) so that finder, timing, alignment,
// corner, and sub-pattern modules are all accounted for: the always-dark
// border modules of each edge, top, bottom, left, and right, from which the
// edge fit marches; and every function module of the top and bottom rows, dark
// or light, which the timing check compares.
type rmqrBorder struct {
	dark   [4][][2]int
	timing []rmqrFixedModule
}

// rmqrBorders holds the border layout of every version, computed once.
var rmqrBorders = sync.OnceValue(func() (out [32]rmqrBorder) {
	for v := R7x43; v <= R17x139; v++ {
		m := newRMQRMatrix(v)
		m.placeRMQRFunctionPatterns()
		rows, cols := v.Height(), v.Width()
		b := &out[v-1]
		dark := func(r, c int) bool { return m.isReserved(r, c) && m.get(r, c) }
		for c := range cols {
			for i, r := range [2]int{0, rows - 1} {
				if m.isReserved(r, c) {
					b.timing = append(b.timing, rmqrFixedModule{c, r, m.get(r, c)})
				}
				if dark(r, c) {
					b.dark[i] = append(b.dark[i], [2]int{c, r})
				}
			}
		}
		for r := range rows {
			for i, c := range [2]int{0, cols - 1} {
				if dark(r, c) {
					b.dark[2+i] = append(b.dark[2+i], [2]int{c, r})
				}
			}
		}
	}
	return out
})

// rmqrEdgeModules returns the always-dark border modules of version v, edge
// by edge: top, bottom, left, right.
func rmqrEdgeModules(v RMQRVersion) [4][][2]int { return rmqrBorders()[v-1].dark }

// rmqrTimingTolerance is the fraction of the top and bottom rows' function
// modules that may disagree for a symbol to count as located. A real symbol
// read through a good homography disagrees in almost none; modules read at
// random disagree in half, and with at least 54 of them, from the two edges of
// the narrowest version, the chance of passing by accident is negligible
// (theory doc 30, section 7).
const rmqrTimingTolerance = 0.1

// rmqrTimingAgrees reports whether the function modules of the top and bottom
// rows of version v read through h as the layout has them, with at most
// rmqrTimingTolerance of them disagreeing.
func rmqrTimingAgrees(bm *bitmap, h homography, v RMQRVersion) bool {
	b := rmqrBorders()[v-1]
	miss := 0
	for _, m := range b.timing {
		if moduleDark(bm, h, m.col, m.row) != m.dark {
			miss++
		}
	}
	return float64(miss) <= rmqrTimingTolerance*float64(len(b.timing))
}

// edgeFitRMQRHomography refits the homography h of an rMQR symbol of version v
// to the symbol's four outer edges (theory doc 30, section 5). From every
// always-dark border module it marches outward to the edge, as the Micro edge
// fit does, fits a line to each edge, and intersects the lines in the
// symbol's four corners; those, with the eight corners of the finder and the
// sub-pattern's light ring as h places them, give a least-squares homography
// that measures the long edges instead of interpolating between the two ends.
// It returns false when an edge has too few points or the fit is degenerate.
func edgeFitRMQRHomography(bm *bitmap, h homography, v RMQRVersion) (homography, bool) {
	rows, cols := v.Height(), v.Width()
	mods := rmqrEdgeModules(v)
	dirs := [4][2]float64{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	var p, d [4][2]float64
	for i := range 4 {
		var ok bool
		p[i], d[i], ok = microEdgeLine(bm, h, mods[i], dirs[i][0], dirs[i][1])
		if !ok {
			return h, false
		}
	}
	right, bottom := float64(cols)-0.5, float64(rows)-0.5
	var src, dst [][2]float64
	for _, c := range [4]struct {
		mod  [2]float64
		a, b int // the two edges meeting at the corner
	}{
		{[2]float64{-0.5, -0.5}, 0, 2},
		{[2]float64{right, -0.5}, 0, 3},
		{[2]float64{right, bottom}, 1, 3},
		{[2]float64{-0.5, bottom}, 1, 2},
	} {
		x, ok := intersectLines(p[c.a], d[c.a], p[c.b], d[c.b])
		if !ok {
			return h, false
		}
		src, dst = append(src, c.mod), append(dst, x)
	}
	ring := rmqrSubRing(rows, cols)
	for i := range 4 {
		for _, m := range [2][2]float64{finderRingCorners[i], ring[i]} {
			px, py := h.apply(m[0], m[1])
			src, dst = append(src, m), append(dst, [2]float64{px, py})
		}
	}
	refined, err := computeHomographyLS(src, dst)
	if err != nil {
		return h, false
	}
	return refined, true
}

// rmqrHomographyVariants returns the homographies to try for a symbol of
// version v located through the eight-point homography h8, most refined
// first: two rounds of the edge fit, then h8 itself. Reed-Solomon decides
// among them, so a refinement that goes wrong costs a decode attempt, not a
// symbol.
func rmqrHomographyVariants(bm *bitmap, h8 homography, v RMQRVersion) []homography {
	out := []homography{h8}
	h := h8
	for range 2 {
		fitted, ok := edgeFitRMQRHomography(bm, h, v)
		if !ok {
			break
		}
		out = append([]homography{fitted}, out...)
		h = fitted
	}
	return out
}

// rmqrCandidate is a located symbol ready to sample: its homographies, most
// refined first, its symbol type, and how far its format copy was from a valid
// word.
type rmqrCandidate struct {
	hgs  []homography
	sym  rmqrSymbol
	dist int
}

// locateRMQR tries the four quarter-turn assignments of finder quad q as the
// finder of an rMQR symbol (theory doc 30, sections 2 to 7). An assignment
// that passes the orientation test reads the finder-side format copy. If that
// names a version, each sub-pattern candidate near where the version puts one
// is paired with the finder, the eight-point homography is fitted and refined
// along the edges, and the pairing is a located symbol when the top and bottom
// timing rows read correctly through one of its homographies. If the
// finder-side copy names no version, every version is tried the same way, but
// a pairing goes on to the timing check only when the sub-side copy, read
// through its eight-point homography, names that version. It returns the
// located symbols, nearest format word first, and the furthest stage reached.
func locateRMQR(bm *bitmap, scan *ccScan, q finderQuad) ([]rmqrCandidate, rmqrStage) {
	corners := windCorners(outerRingCorners(q))
	module := q.center.moduleSize
	stage := rmqrStageNone
	var out []rmqrCandidate
	// try pairs the finder with each sub-pattern near where version v puts
	// one and returns the pairings whose timing rows agree; with subCopy set,
	// only pairings whose sub-side copy names v are checked.
	try := func(hg, predict homography, finder [4][2]float64, v RMQRVersion, subCopy bool) []rmqrCandidate {
		var found []rmqrCandidate
		rows, cols := v.Height(), v.Width()
		for _, sp := range subPatternsNear(scan, predict, v, module) {
			h8, err := rmqrHomography(scan, hg, finder, sp, rows, cols)
			if err != nil {
				continue
			}
			lvl, subDist := rmqrSubFormat(readRMQRFormatWord(bm, h8, rows, cols, true), v)
			if subCopy {
				if subDist > rmqrFormatBudget {
					continue
				}
				stage = max(stage, rmqrStageFormat)
			}
			hgs := rmqrHomographyVariants(bm, h8, v)
			for _, h := range hgs {
				if rmqrTimingAgrees(bm, h, v) {
					found = append(found, rmqrCandidate{hgs: hgs, sym: rmqrSymbol{version: v, lvl: lvl}, dist: subDist})
					break
				}
			}
		}
		return found
	}
	for k := range 4 {
		hg, err := finderHomography(corners, k)
		if err != nil {
			continue
		}
		var finder [4][2]float64
		for i := range 4 {
			finder[i] = corners[(i+k)%4]
		}
		predict := finderAffine(finder)
		// The timing and format modules lie up to 11 modules from the finder.
		// Near a small or foreshortened finder, the homography's perspective
		// terms can already push them off their modules, and the affine map
		// can miss a real perspective, so both are read and the better one
		// kept.
		sym, dist, oriented := rmqrSymbol{}, 19, false
		for _, near := range [2]homography{hg, predict} {
			if !rmqrOriented(bm, near) {
				continue
			}
			oriented = true
			if s, d := nearestRMQRFormat(readRMQRFormatWord(bm, near, 0, 0, false)); d < dist {
				sym, dist = s, d
			}
		}
		if !oriented {
			continue
		}
		stage = max(stage, rmqrStageOriented)
		if dist <= rmqrFormatBudget {
			stage = max(stage, rmqrStageFormat)
			for _, c := range try(hg, predict, finder, sym.version, false) {
				c.sym, c.dist = sym, min(dist, c.dist)
				out = append(out, c)
			}
			continue
		}
		// The finder-side copy is unreadable: let the sub-side copy name the
		// version instead.
		for v := R7x43; v <= R17x139; v++ {
			out = append(out, try(hg, predict, finder, v, true)...)
		}
	}
	if len(out) > 0 {
		stage = rmqrStageLocated
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].dist < out[j].dist })
	return out, stage
}

// rmqrStageError is the error reported for a finder that stopped before the
// matrix decode, at stage s.
func rmqrStageError(s rmqrStage) error {
	switch s {
	case rmqrStageNone:
		return fmt.Errorf("%w: no rMQR timing pattern beside the finder", ErrFinderNotFound)
	case rmqrStageOriented:
		return fmt.Errorf("%w: no rMQR format information beside the finder", ErrFinderNotFound)
	default:
		return fmt.Errorf("%w: no rMQR symbol where the format information puts one", ErrFinderNotFound)
	}
}

// decodeRMQRQuad decodes the rMQR symbol whose finder is q: it locates the
// symbol (locateRMQR), samples the grid at the size the format information
// names through each of a located symbol's homographies, and decodes it with
// decodeRMQRMatrix, best candidate first. It returns the text, the version,
// the homography from module coordinates to pixels, and the furthest stage
// reached; on failure, the error of that stage, which for a located symbol is
// the matrix decoder's.
func decodeRMQRQuad(bm *bitmap, scan *ccScan, q finderQuad) (string, RMQRVersion, homography, rmqrStage, error) {
	cands, stage := locateRMQR(bm, scan, q)
	if len(cands) == 0 {
		return "", 0, homography{}, stage, rmqrStageError(stage)
	}
	var firstErr error
	for _, c := range cands {
		v := c.sym.version
		for _, h := range c.hgs {
			text, err := decodeRMQRMatrix(sampleGrid(bm, h, v.Height(), v.Width()))
			if err == nil {
				return text, v, h, stage, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return "", 0, homography{}, stage, firstErr
}

// decodeRMQRFromQuads tries each finder quad of scan as the finder of an rMQR
// symbol, largest module first and at most maxCCCandidates of them, and
// returns the first that decodes. On failure it returns the error of the quad
// that got furthest through the gates (the larger one on a tie) together with
// that stage.
func decodeRMQRFromQuads(bm *bitmap, scan *ccScan, quads []finderQuad) (string, rmqrStage, error) {
	order := append([]finderQuad(nil), quads...)
	sort.SliceStable(order, func(i, j int) bool {
		return order[i].center.moduleSize > order[j].center.moduleSize
	})
	if len(order) > maxCCCandidates {
		order = order[:maxCCCandidates]
	}
	bestStage := rmqrStageNone
	var bestErr error = ErrFinderNotFound // no finder at all
	for i, q := range order {
		text, _, _, stage, err := decodeRMQRQuad(bm, scan, q)
		if err == nil {
			return text, stage, nil
		}
		if i == 0 || stage > bestStage {
			bestStage, bestErr = stage, err
		}
	}
	return "", bestStage, bestErr
}
