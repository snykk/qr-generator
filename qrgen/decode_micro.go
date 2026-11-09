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

// finderHomography maps module coordinates to pixels under assignment k:
// finder ring corner i (top-left, top-right, bottom-right, bottom-left) goes to
// wound image corner (i+k) mod 4, so k picks which image corner is the
// symbol's outer corner.
func finderHomography(corners [4][2]float64, k int) (homography, error) {
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

// microCandidate is one quarter-turn assignment of a finder, with one of its
// homographies, that passed every gate before decoding: the homography, the
// symbol type its format word names, and that word's Hamming distance.
type microCandidate struct {
	hg   homography
	sym  microSymbol
	dist int
}

// locateMicro tries the four quarter-turn assignments of finder quad q as the
// finder of a Micro symbol. An assignment survives if the two innermost timing
// modules, (row 0, column 8) and (row 8, column 0), read dark; its format word
// lies within the 3-bit budget of one of the 32 Micro words; and the timing
// pattern agrees with the size that word names, through one of the
// homographies microHomographyVariants offers. Survivors are returned nearest
// format word first and, within an assignment, most refined homography first,
// with the furthest stage any assignment reached. An assignment whose format
// word does not decode still counts as timed if its timing pattern matches
// some Micro size exactly: that is a Micro symbol with damaged format
// information.
func locateMicro(bm *bitmap, q finderQuad) ([]microCandidate, microStage) {
	corners := windCorners(outerRingCorners(q))
	stage := microStageNone
	var out []microCandidate
	for k := range 4 {
		hg, err := finderHomography(corners, k)
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
		n := sym.version.Size()
		for _, h := range microHomographyVariants(bm, hg, n) {
			if microTimingMismatches(bm, h, n) > microTimingTolerance {
				continue
			}
			stage = microStageLocated
			out = append(out, microCandidate{hg: h, sym: sym, dist: dist})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].dist < out[j].dist })
	return out, stage
}

// microHomographyVariants returns the homographies to try for a symbol of size
// n found through finder homography hg, most refined first: two rounds of the
// edge fit (edgeFitMicroHomography), the far-corner refinement they start from
// (refineMicroHomography), and hg itself. Each refinement needs the previous
// one close enough that its searches start in the right modules, which is why
// they run in this order; the cross-check and Reed-Solomon then decide among
// them, so a refinement that goes wrong costs a decode attempt, not a symbol.
// See docs/theory/27-micro-qr-detection.md, section 6.
func microHomographyVariants(bm *bitmap, hg homography, n int) []homography {
	refined, ok := refineMicroHomography(bm, hg, n)
	if !ok {
		return []homography{hg}
	}
	out := []homography{refined, hg}
	h := refined
	for range 2 {
		fitted, ok := edgeFitMicroHomography(bm, h, n)
		if !ok {
			break
		}
		out = append([]homography{fitted}, out...)
		h = fitted
	}
	return out
}

// microFarCorner finds again, in the image, the outer corner of a dark module at
// the far end of a timing pattern: module (col, row), whose outer corner lies
// half a module along the outward diagonal (su, sv) in module units, (1, -1)
// for the top-right module (n-1, 0) and (-1, 1) for the bottom-left module
// (0, n-1). Both modules sit at even timing indices, so they are always dark,
// and the quiet zone bounds them on their two outer sides, which makes the
// corner the dark pixel furthest along the outward diagonal near its predicted
// position. The search covers about 1.8 modules around the module's predicted
// centre, and the pixel found is moved out to its own boundary, as in
// outerRingCorners. ok is false when no dark pixel lies in reach.
func microFarCorner(bm *bitmap, h homography, col, row int, su, sv float64) (corner [2]float64, ok bool) {
	cx, cy := h.apply(float64(col), float64(row))
	ox, oy := h.apply(float64(col)+su/2, float64(row)+sv/2)
	dx, dy := ox-cx, oy-cy
	half := math.Hypot(dx, dy) // half a module diagonal, in pixels
	if !(half > 0) || math.IsInf(half, 0) || math.IsNaN(cx) || math.IsNaN(cy) {
		return corner, false
	}
	dx, dy = dx/half, dy/half
	r := 2.5 * half
	x0, x1 := max(int(math.Floor(cx-r)), 0), min(int(math.Ceil(cx+r)), bm.width-1)
	y0, y1 := max(int(math.Floor(cy-r)), 0), min(int(math.Ceil(cy+r)), bm.height-1)
	best := math.Inf(-1)
	var bx, by int
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			ex, ey := float64(x)-cx, float64(y)-cy
			if !bm.get(x, y) || ex*ex+ey*ey > r*r {
				continue
			}
			if p := ex*dx + ey*dy; p > best {
				best, bx, by, ok = p, x, y, true
			}
		}
	}
	if !ok {
		return corner, false
	}
	m := math.Max(math.Abs(dx), math.Abs(dy))
	return [2]float64{float64(bx) + 0.5*dx/m, float64(by) + 0.5*dy/m}, true
}

// refineMicroHomography sharpens the finder homography h of a symbol of size n,
// which is fixed by a 7-module square and extrapolated to n modules, by finding
// the symbol's top-right and bottom-left corners again (microFarCorner) and
// refitting by least squares over the four finder corners plus those two. It
// returns false when neither corner was found or the fit is degenerate.
// See docs/theory/27-micro-qr-detection.md, section 6.
func refineMicroHomography(bm *bitmap, h homography, n int) (homography, bool) {
	var src, dst [][2]float64
	for _, c := range finderRingCorners {
		px, py := h.apply(c[0], c[1])
		src = append(src, c)
		dst = append(dst, [2]float64{px, py})
	}
	far := float64(n) - 0.5
	found := false
	if p, ok := microFarCorner(bm, h, n-1, 0, 1, -1); ok {
		src, dst, found = append(src, [2]float64{far, -0.5}), append(dst, p), true
	}
	if p, ok := microFarCorner(bm, h, 0, n-1, -1, 1); ok {
		src, dst, found = append(src, [2]float64{-0.5, far}), append(dst, p), true
	}
	if !found {
		return h, false
	}
	refined, err := computeHomographyLS(src, dst)
	if err != nil {
		return h, false
	}
	return refined, true
}

// pixelDark reports whether the pixel containing point (x, y) is dark; points
// outside the image, or not finite, read light.
func pixelDark(bm *bitmap, x, y float64) bool {
	if !(x > -0.5 && y > -0.5 && x < float64(bm.width)-0.5 && y < float64(bm.height)-0.5) {
		return false
	}
	return bm.get(int(math.Round(x)), int(math.Round(y)))
}

// microEdgePoint marches from the predicted centre of border module (col, row)
// outward, one module being (du, dv) in module units, in quarter-pixel steps
// for up to 1.5 modules, and returns the midpoint between the last dark and the
// first light sample: a point on the symbol's outer edge, since only the quiet
// zone lies beyond a border module. ok is false when the march starts on a
// light pixel or never leaves the dark.
func microEdgePoint(bm *bitmap, h homography, col, row int, du, dv float64) ([2]float64, bool) {
	cx, cy := h.apply(float64(col), float64(row))
	ex, ey := h.apply(float64(col)+du, float64(row)+dv)
	dx, dy := ex-cx, ey-cy
	reach := 1.5 * math.Hypot(dx, dy)
	if !(reach > 0) || math.IsInf(reach, 0) || !pixelDark(bm, cx, cy) {
		return [2]float64{}, false
	}
	steps := int(math.Ceil(4 * reach))
	sx, sy := 1.5*dx/float64(steps), 1.5*dy/float64(steps)
	px, py := cx, cy
	for i := 1; i <= steps; i++ {
		qx, qy := cx+sx*float64(i), cy+sy*float64(i)
		if !pixelDark(bm, qx, qy) {
			return [2]float64{(px + qx) / 2, (py + qy) / 2}, true
		}
		px, py = qx, qy
	}
	return [2]float64{}, false
}

// fitLine returns the total-least-squares line through pts as a point on it
// (the centroid) and a unit direction; ok is false for fewer than two points.
func fitLine(pts [][2]float64) (p, d [2]float64, ok bool) {
	if len(pts) < 2 {
		return p, d, false
	}
	for _, q := range pts {
		p[0] += q[0] / float64(len(pts))
		p[1] += q[1] / float64(len(pts))
	}
	var sxx, sxy, syy float64
	for _, q := range pts {
		x, y := q[0]-p[0], q[1]-p[1]
		sxx, sxy, syy = sxx+x*x, sxy+x*y, syy+y*y
	}
	theta := 0.5 * math.Atan2(2*sxy, sxx-syy)
	return p, [2]float64{math.Cos(theta), math.Sin(theta)}, true
}

// intersectLines returns the intersection of the lines through p1 along d1 and
// through p2 along d2; ok is false when they are near parallel.
func intersectLines(p1, d1, p2, d2 [2]float64) ([2]float64, bool) {
	den := d1[0]*d2[1] - d1[1]*d2[0]
	if math.Abs(den) < 1e-6 {
		return [2]float64{}, false
	}
	t := ((p2[0]-p1[0])*d2[1] - (p2[1]-p1[1])*d2[0]) / den
	return [2]float64{p1[0] + t*d1[0], p1[1] + t*d1[1]}, true
}

// microEdgeLine fits the line of one outer edge of a symbol from the dark
// border modules listed as (col, row) pairs, marching outward along (du, dv);
// points more than half a module from a first fit are dropped and the line is
// fitted again, which removes marches that started in the wrong module.
func microEdgeLine(bm *bitmap, h homography, mods [][2]int, du, dv float64) (p, d [2]float64, ok bool) {
	var pts [][2]float64
	for _, m := range mods {
		if q, ok := microEdgePoint(bm, h, m[0], m[1], du, dv); ok {
			pts = append(pts, q)
		}
	}
	p, d, ok = fitLine(pts)
	if !ok {
		return p, d, false
	}
	ax, ay := h.apply(0, 0)
	bx, by := h.apply(1, 0)
	halfModule := 0.5 * math.Hypot(bx-ax, by-ay)
	var keep [][2]float64
	for _, q := range pts {
		if math.Abs((q[0]-p[0])*d[1]-(q[1]-p[1])*d[0]) <= halfModule {
			keep = append(keep, q)
		}
	}
	if len(keep) < 2 || len(keep) == len(pts) {
		return p, d, true
	}
	return fitLine(keep)
}

// edgeFitMicroHomography refits the homography of a symbol of size n to the
// four outer edges of the symbol itself. The top and left edges use the
// modules known to be dark, the finder's outer row and column and the even
// timing modules; the right and bottom edges use the border modules h samples
// as dark, which the Micro masking rule keeps plentiful. Each edge is a line
// fitted to many edge points, and the four corners where the lines meet, with
// the finder corners, give a least-squares homography that no longer
// extrapolates into the bottom-right corner. It returns false when an edge has
// too few points or the fit is degenerate.
func edgeFitMicroHomography(bm *bitmap, h homography, n int) (homography, bool) {
	var top, left, right, bottom [][2]int
	for i := 0; i < n; i++ {
		if i < 7 || (i >= 8 && i%2 == 0) {
			top = append(top, [2]int{i, 0})
			left = append(left, [2]int{0, i})
		}
		if moduleDark(bm, h, n-1, i) {
			right = append(right, [2]int{n - 1, i})
		}
		if moduleDark(bm, h, i, n-1) {
			bottom = append(bottom, [2]int{i, n - 1})
		}
	}
	tp, td, ok1 := microEdgeLine(bm, h, top, 0, -1)
	lp, ld, ok2 := microEdgeLine(bm, h, left, -1, 0)
	rp, rd, ok3 := microEdgeLine(bm, h, right, 1, 0)
	bp, bd, ok4 := microEdgeLine(bm, h, bottom, 0, 1)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return h, false
	}
	far := float64(n) - 0.5
	var src, dst [][2]float64
	for _, c := range [4]struct {
		mod    [2]float64
		p1, d1 [2]float64
		p2, d2 [2]float64
	}{
		{[2]float64{-0.5, -0.5}, tp, td, lp, ld},
		{[2]float64{far, -0.5}, tp, td, rp, rd},
		{[2]float64{far, far}, bp, bd, rp, rd},
		{[2]float64{-0.5, far}, bp, bd, lp, ld},
	} {
		x, ok := intersectLines(c.p1, c.d1, c.p2, c.d2)
		if !ok {
			return h, false
		}
		src, dst = append(src, c.mod), append(dst, x)
	}
	for _, c := range finderRingCorners {
		px, py := h.apply(c[0], c[1])
		src, dst = append(src, c), append(dst, [2]float64{px, py})
	}
	refined, err := computeHomographyLS(src, dst)
	if err != nil {
		return h, false
	}
	return refined, true
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
