// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
)

// ErrFinderNotFound is returned when the decoder cannot identify three valid
// finder patterns in the image. Callers can use errors.Is to branch on this.
var ErrFinderNotFound = errors.New("qrgen: finder patterns not found")

// ErrInvalidVersion is returned when the version estimated from the finder
// geometry falls outside the spec's 1..40 range.
var ErrInvalidVersion = errors.New("qrgen: estimated version out of range")

// bitmap is a binary 2D grid of pixels produced by binarising an image.
// Coordinates use (x, y) with (0, 0) at the top-left; the row-major pixels
// slice is true for dark pixels (foreground) and false for light pixels
// (background or quiet zone).
type bitmap struct {
	width, height int
	pixels        []bool
}

// get returns whether the pixel at (x, y) is dark.
func (b *bitmap) get(x, y int) bool { return b.pixels[y*b.width+x] }

// imageToGrayscale flattens img to an 8-bit luminance buffer using the
// ITU-R BT.601 weights via image/color.GrayModel. See
// docs/theory/12-image-processing.md §1.
func imageToGrayscale(img image.Image) (pixels []uint8, width, height int) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	pixels = make([]uint8, w*h)
	for y := range h {
		for x := range w {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			gray := color.GrayModel.Convert(c).(color.Gray)
			pixels[y*w+x] = gray.Y
		}
	}
	return pixels, w, h
}

// otsuThreshold picks the binarisation threshold that maximises between-class
// variance for the given luminance pixels and returns it together with the
// separability ratio η = σ²_B / σ²_T ∈ [0, 1] that Otsu's original paper uses
// as a "goodness of threshold" measure. Standard Otsu's method per
// docs/theory/12-image-processing.md §2; η is the Stage 1 gate documented in
// docs/theory/14-adaptive-thresholding.md §6. Returns 128 and 0 when the
// histogram is degenerate (all pixels share one value); η = 0 also kicks in
// for empty inputs and for histograms with zero total variance, both of
// which signal an unimodal distribution that the dispatch in decodeImage
// will route to Sauvola.
func otsuThreshold(pixels []uint8) (uint8, float64) {
	var hist [256]int
	for _, p := range pixels {
		hist[p]++
	}
	total := len(pixels)
	if total == 0 {
		return 128, 0
	}

	var sumAll, sumAllSq float64
	for i, c := range hist {
		fi := float64(i)
		fc := float64(c)
		sumAll += fi * fc
		sumAllSq += fi * fi * fc
	}
	// N² · σ²_T = N · Σ(i² · hist[i]) − (Σ(i · hist[i]))²
	n := float64(total)
	denom := n*sumAllSq - sumAll*sumAll

	var sumBg float64
	wBg := 0
	var bestVariance float64
	bestT := uint8(128)
	for t := range 256 {
		wBg += hist[t]
		if wBg == 0 {
			continue
		}
		wFg := total - wBg
		if wFg == 0 {
			break
		}
		sumBg += float64(t) * float64(hist[t])
		meanBg := sumBg / float64(wBg)
		meanFg := (sumAll - sumBg) / float64(wFg)
		meanDiff := meanBg - meanFg
		variance := float64(wBg) * float64(wFg) * meanDiff * meanDiff
		if variance > bestVariance {
			bestVariance = variance
			bestT = uint8(t)
		}
	}
	// bestVariance is N² · σ²_B, so dividing by N² · σ²_T cancels the N²
	// without ever having to materialise σ²_B or σ²_T separately.
	var eta float64
	if denom > 0 {
		eta = bestVariance / denom
	}
	return bestT, eta
}

// otsuBinariseFromGray applies a precomputed Otsu threshold to a flat
// grayscale buffer and returns a bitmap. Split out from binarise so the
// dispatch in decodeImage can run Otsu and Sauvola off the same grayscale
// pass without re-walking the image.
func otsuBinariseFromGray(pixels []uint8, w, h int, t uint8) *bitmap {
	bm := &bitmap{width: w, height: h, pixels: make([]bool, w*h)}
	for i, p := range pixels {
		bm.pixels[i] = p <= t
	}
	return bm
}

// binarise converts an image to a bitmap by computing the Otsu threshold and
// classifying each pixel as dark (luminance ≤ threshold) or light. The "≤"
// matches the convention inside otsuThreshold where the dark class C0 at
// threshold t is exactly the set {0, …, t}; using "<" would mark a fully
// black image as having no dark pixels (because Otsu would pick t = 0 for
// strictly-bimodal inputs at the extremes).
//
// See docs/theory/12-image-processing.md §1–2.
func binarise(img image.Image) *bitmap {
	pixels, w, h := imageToGrayscale(img)
	t, _ := otsuThreshold(pixels)
	return otsuBinariseFromGray(pixels, w, h, t)
}

// foregroundRatio returns the fraction of dark pixels in the bitmap. Used by
// the dispatch in decodeImage as the post-check that catches degenerate
// single-class output from Otsu before declaring ErrFinderNotFound.
func foregroundRatio(bm *bitmap) float64 {
	if len(bm.pixels) == 0 {
		return 0
	}
	var dark int
	for _, p := range bm.pixels {
		if p {
			dark++
		}
	}
	return float64(dark) / float64(len(bm.pixels))
}

// finderCandidate is the (x, y) centre and estimated module pitch of a
// detected 1:1:3:1:1 finder-pattern signature.
type finderCandidate struct {
	x, y       float64
	moduleSize float64
}

// finderTriple groups the three finder centres of a QR symbol in the natural
// reading order assuming the symbol is approximately right-side-up.
type finderTriple struct {
	topLeft, topRight, bottomLeft finderCandidate
}

// fitsFinderRatio reports whether five consecutive run lengths match the
// 1:1:3:1:1 finder-pattern ratio within ±50% tolerance per module.
// See docs/theory/12-image-processing.md §3.
func fitsFinderRatio(runs [5]int) bool {
	total := runs[0] + runs[1] + runs[2] + runs[3] + runs[4]
	if total < 7 {
		return false
	}
	moduleSize := float64(total) / 7.0
	tol := moduleSize / 2.0
	near := func(v, target, t float64) bool {
		d := v - target
		if d < 0 {
			d = -d
		}
		return d <= t
	}
	if !near(float64(runs[0]), moduleSize, tol) ||
		!near(float64(runs[1]), moduleSize, tol) ||
		!near(float64(runs[3]), moduleSize, tol) ||
		!near(float64(runs[4]), moduleSize, tol) {
		return false
	}
	return near(float64(runs[2]), moduleSize*3, tol*3)
}

// scanRowForFinders walks one row of the bitmap and records every horizontal
// 1:1:3:1:1 dark/light/dark/light/dark signature whose middle is dark.
func scanRowForFinders(bm *bitmap, y int) []finderCandidate {
	var out []finderCandidate
	if y < 0 || y >= bm.height {
		return out
	}
	runs := [5]int{}
	color := bm.get(0, y)
	runLen := 1
	check := func(endX int, endedColor bool) {
		// The just-completed run is in runs[4]; if it was dark, the 5-run
		// window is dark/light/dark/light/dark with the middle run dark.
		if !endedColor {
			return
		}
		if !fitsFinderRatio(runs) {
			return
		}
		centerX := float64(endX) - float64(runs[4]) - float64(runs[3]) - float64(runs[2])/2.0
		total := runs[0] + runs[1] + runs[2] + runs[3] + runs[4]
		out = append(out, finderCandidate{
			x:          centerX,
			y:          float64(y),
			moduleSize: float64(total) / 7.0,
		})
	}
	for x := 1; x < bm.width; x++ {
		if bm.get(x, y) == color {
			runLen++
			continue
		}
		runs[0], runs[1], runs[2], runs[3], runs[4] = runs[1], runs[2], runs[3], runs[4], runLen
		check(x, color)
		color = bm.get(x, y)
		runLen = 1
	}
	// Final run flush.
	runs[0], runs[1], runs[2], runs[3], runs[4] = runs[1], runs[2], runs[3], runs[4], runLen
	check(bm.width, color)
	return out
}

// crossCheckVertical verifies that the column at cx exhibits the same
// 1:1:3:1:1 dark/light/dark/light/dark structure centred near cy, and returns
// the refined centre Y plus the vertical module-size estimate. It returns
// ok = false if the structure is not present.
func crossCheckVertical(bm *bitmap, cx, cy int) (float64, float64, bool) {
	if cx < 0 || cx >= bm.width || cy < 0 || cy >= bm.height {
		return 0, 0, false
	}
	if !bm.get(cx, cy) {
		return 0, 0, false
	}
	h := bm.height
	// Walk up: extend the central dark run.
	up := 0
	for y := cy - 1; y >= 0 && bm.get(cx, y); y-- {
		up++
	}
	// Light run above.
	lightUp := 0
	for y := cy - 1 - up; y >= 0 && !bm.get(cx, y); y-- {
		lightUp++
	}
	if lightUp == 0 {
		return 0, 0, false
	}
	// Dark run above that.
	darkUp := 0
	for y := cy - 1 - up - lightUp; y >= 0 && bm.get(cx, y); y-- {
		darkUp++
	}
	if darkUp == 0 {
		return 0, 0, false
	}
	// Walk down.
	down := 0
	for y := cy + 1; y < h && bm.get(cx, y); y++ {
		down++
	}
	lightDown := 0
	for y := cy + 1 + down; y < h && !bm.get(cx, y); y++ {
		lightDown++
	}
	if lightDown == 0 {
		return 0, 0, false
	}
	darkDown := 0
	for y := cy + 1 + down + lightDown; y < h && bm.get(cx, y); y++ {
		darkDown++
	}
	if darkDown == 0 {
		return 0, 0, false
	}
	runs := [5]int{darkUp, lightUp, up + 1 + down, lightDown, darkDown}
	if !fitsFinderRatio(runs) {
		return 0, 0, false
	}
	// Refined centre Y: middle of the central dark run.
	centerY := float64(cy) + float64(down-up)/2.0
	total := runs[0] + runs[1] + runs[2] + runs[3] + runs[4]
	return centerY, float64(total) / 7.0, true
}

// clusterFinderCandidates groups nearby (x, y) candidates into clusters whose
// member centres lie within mergeRadius pixels of the cluster's mean. Each
// returned candidate is the mean of its cluster, weighted equally; the count
// field via the parallel weights slice tells the caller how strong each
// cluster's evidence is.
func clusterFinderCandidates(cands []finderCandidate) ([]finderCandidate, []int) {
	type cluster struct {
		sumX, sumY, sumSize float64
		count               int
	}
	var clusters []cluster
	for _, c := range cands {
		// Use the candidate's own module size as the merge radius — clusters
		// from the same finder produce candidates within ~one module of each
		// other in both x and y.
		radius := c.moduleSize
		matched := -1
		for i, k := range clusters {
			mx := k.sumX / float64(k.count)
			my := k.sumY / float64(k.count)
			if math.Abs(mx-c.x) <= radius && math.Abs(my-c.y) <= radius {
				matched = i
				break
			}
		}
		if matched < 0 {
			clusters = append(clusters, cluster{sumX: c.x, sumY: c.y, sumSize: c.moduleSize, count: 1})
		} else {
			clusters[matched].sumX += c.x
			clusters[matched].sumY += c.y
			clusters[matched].sumSize += c.moduleSize
			clusters[matched].count++
		}
	}
	out := make([]finderCandidate, len(clusters))
	weights := make([]int, len(clusters))
	for i, k := range clusters {
		out[i] = finderCandidate{
			x:          k.sumX / float64(k.count),
			y:          k.sumY / float64(k.count),
			moduleSize: k.sumSize / float64(k.count),
		}
		weights[i] = k.count
	}
	return out, weights
}

// orderFinderTriple identifies which of three centres is the right-angle
// vertex (top-left) and orders the other two as top-right and bottom-left
// assuming the symbol is approximately right-side-up.
func orderFinderTriple(a, b, c finderCandidate) (finderTriple, error) {
	dist := func(p, q finderCandidate) float64 {
		dx := p.x - q.x
		dy := p.y - q.y
		return math.Hypot(dx, dy)
	}
	dAB := dist(a, b)
	dBC := dist(b, c)
	dCA := dist(c, a)

	// The longest side connects top-right and bottom-left; the opposite
	// vertex is top-left.
	var tl, tr, bl finderCandidate
	switch {
	case dAB >= dBC && dAB >= dCA:
		tl = c
		tr, bl = a, b
	case dBC >= dAB && dBC >= dCA:
		tl = a
		tr, bl = b, c
	default:
		tl = b
		tr, bl = c, a
	}

	// Among tr and bl, "top-right" has smaller y in a right-side-up image; if
	// the y values are very close, the larger x is the top-right instead.
	if tr.y > bl.y || (math.Abs(tr.y-bl.y) < 1 && tr.x < bl.x) {
		tr, bl = bl, tr
	}

	// Sanity: the two short legs (TL→TR and TL→BL) should have comparable
	// length and the angle at TL should be ~90°.
	legA := dist(tl, tr)
	legB := dist(tl, bl)
	if legA <= 0 || legB <= 0 {
		return finderTriple{}, ErrFinderNotFound
	}
	ratio := legA / legB
	if ratio < 1 {
		ratio = 1 / ratio
	}
	if ratio > 1.5 {
		return finderTriple{}, ErrFinderNotFound
	}
	hypot := dist(tr, bl)
	expectedHypot := math.Hypot(legA, legB)
	if math.Abs(hypot-expectedHypot)/expectedHypot > 0.15 {
		return finderTriple{}, ErrFinderNotFound
	}
	return finderTriple{topLeft: tl, topRight: tr, bottomLeft: bl}, nil
}

// homography is a 3×3 matrix in row-major order that maps homogeneous module
// coordinates (col, row, 1) to homogeneous source-pixel coordinates
// (u, v, w); dividing u and v by w yields the actual (x, y) pixel position.
// See docs/theory/12-image-processing.md §5.
type homography [9]float64

// apply projects (col, row) module coordinates to source-pixel coordinates.
func (h homography) apply(col, row float64) (px, py float64) {
	u := h[0]*col + h[1]*row + h[2]
	v := h[3]*col + h[4]*row + h[5]
	w := h[6]*col + h[7]*row + h[8]
	return u / w, v / w
}

// computeHomography solves the 8×8 linear system for the homography that
// sends four source module coordinates (srcMod) to four destination pixel
// coordinates (dstPx). With four point correspondences and the (h22 = 1)
// normalisation, the system has a unique solution unless the source quad is
// degenerate (three points collinear), in which case it returns an error.
//
// Each correspondence (X, Y) ↔ (x, y) contributes two equations:
//
//	[ X Y 1 0 0 0 −xX −xY ] · [h00..h21]^T = x
//	[ 0 0 0 X Y 1 −yX −yY ] · [h00..h21]^T = y
func computeHomography(srcMod, dstPx [4][2]float64) (homography, error) {
	var A [8][8]float64
	var b [8]float64
	for i := range 4 {
		X, Y := srcMod[i][0], srcMod[i][1]
		x, y := dstPx[i][0], dstPx[i][1]

		A[2*i] = [8]float64{X, Y, 1, 0, 0, 0, -x * X, -x * Y}
		b[2*i] = x

		A[2*i+1] = [8]float64{0, 0, 0, X, Y, 1, -y * X, -y * Y}
		b[2*i+1] = y
	}

	h, err := solveLinear8(A, b)
	if err != nil {
		return homography{}, err
	}
	return homography{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}, nil
}

// solveLinear8 solves A · x = b for an 8×8 system using Gaussian elimination
// with partial pivoting. Returns an error if the system is too close to
// singular (smallest pivot magnitude below 1e-10).
func solveLinear8(A [8][8]float64, b [8]float64) ([8]float64, error) {
	const n = 8
	// Forward elimination.
	for col := range n {
		// Partial pivot: find the row with the largest |A[row][col]|.
		pivotRow := col
		pivotAbs := math.Abs(A[col][col])
		for row := col + 1; row < n; row++ {
			if v := math.Abs(A[row][col]); v > pivotAbs {
				pivotAbs = v
				pivotRow = row
			}
		}
		if pivotAbs < 1e-10 {
			return [8]float64{}, fmt.Errorf("qrgen: solveLinear8: singular matrix (pivot |%g| at col %d)", pivotAbs, col)
		}
		if pivotRow != col {
			A[col], A[pivotRow] = A[pivotRow], A[col]
			b[col], b[pivotRow] = b[pivotRow], b[col]
		}
		// Eliminate below the pivot.
		for row := col + 1; row < n; row++ {
			factor := A[row][col] / A[col][col]
			for c := col; c < n; c++ {
				A[row][c] -= factor * A[col][c]
			}
			b[row] -= factor * b[col]
		}
	}
	// Back-substitute.
	var x [8]float64
	for row := n - 1; row >= 0; row-- {
		sum := b[row]
		for c := row + 1; c < n; c++ {
			sum -= A[row][c] * x[c]
		}
		x[row] = sum / A[row][row]
	}
	return x, nil
}

// estimateBottomRight returns the source-pixel coordinates of the bottom-right
// matrix corner by completing the parallelogram defined by the three finder
// centres. This is a coarse estimate that assumes the symbol is a rectangle
// (no severe perspective distortion); D11 will refine it via the alignment
// pattern for higher versions.
func estimateBottomRight(tri finderTriple) (float64, float64) {
	brX := tri.topRight.x + tri.bottomLeft.x - tri.topLeft.x
	brY := tri.topRight.y + tri.bottomLeft.y - tri.topLeft.y
	return brX, brY
}

// estimateVersion infers the QR version from the spacing between the top-left
// and top-right finder centres and the per-finder module-size estimate.
// Returns ErrInvalidVersion if the result is not in [1, 40].
//
// The finder centres are 4 modules from the edge each, so the distance
// between TL and TR equals (n - 7) modules where n = 21 + 4·(v - 1).
func estimateVersion(tri finderTriple) (Version, error) {
	dx := tri.topRight.x - tri.topLeft.x
	dy := tri.topRight.y - tri.topLeft.y
	distance := math.Hypot(dx, dy)
	avgModule := (tri.topLeft.moduleSize + tri.topRight.moduleSize + tri.bottomLeft.moduleSize) / 3.0
	if avgModule <= 0 {
		return 0, ErrInvalidVersion
	}
	modulesBetween := distance / avgModule
	// modulesBetween ≈ n - 7 = 14 + 4·(v - 1) for v in [1, 40], so
	// v = (modulesBetween − 14) / 4 + 1.
	v := int(math.Round((modulesBetween-14)/4 + 1))
	if v < int(MinVersion) || v > int(MaxVersion) {
		return 0, ErrInvalidVersion
	}
	return Version(v), nil
}

// homographyFromFinders builds the homography for a QR symbol of the given
// version using the three finder centres and the parallelogram-completed
// bottom-right anchor. Module coordinates use (col, row) so (X, Y) in the
// homography maps to (col, row); the four anchors are at module positions
// (3, 3), (n-4, 3), (3, n-4) and (n-4, n-4).
func homographyFromFinders(tri finderTriple, v Version) (homography, error) {
	if !v.IsValid() {
		return homography{}, ErrInvalidVersion
	}
	n := v.Size()
	farMod := float64(n - 4)
	brX, brY := estimateBottomRight(tri)

	srcMod := [4][2]float64{
		{3, 3},           // TL
		{farMod, 3},      // TR
		{3, farMod},      // BL
		{farMod, farMod}, // BR (estimated)
	}
	dstPx := [4][2]float64{
		{tri.topLeft.x, tri.topLeft.y},
		{tri.topRight.x, tri.topRight.y},
		{tri.bottomLeft.x, tri.bottomLeft.y},
		{brX, brY},
	}
	return computeHomography(srcMod, dstPx)
}

// checkAlignmentAt reports whether the bitmap at (cx, cy) plausibly sits on
// the centre of a 5×5 alignment pattern given the local module pitch. The
// check samples nine pixels: the centre, the light ring one module away, and
// the dark ring two modules away.
//
// See docs/theory/12-image-processing.md §7.
func checkAlignmentAt(bm *bitmap, cx, cy int, moduleSize float64) bool {
	if cx < 0 || cy < 0 || cx >= bm.width || cy >= bm.height {
		return false
	}
	if !bm.get(cx, cy) {
		return false
	}
	m1 := int(math.Round(moduleSize))
	m2 := int(math.Round(moduleSize * 2))
	if m1 < 1 {
		return false
	}
	sampleAt := func(dx, dy int, wantDark bool) bool {
		x, y := cx+dx, cy+dy
		if x < 0 || y < 0 || x >= bm.width || y >= bm.height {
			return false
		}
		return bm.get(x, y) == wantDark
	}
	// Inner light ring at ±1 module along each cardinal direction.
	if !sampleAt(-m1, 0, false) || !sampleAt(m1, 0, false) || !sampleAt(0, -m1, false) || !sampleAt(0, m1, false) {
		return false
	}
	// Outer dark ring at ±2 modules along each cardinal direction.
	if !sampleAt(-m2, 0, true) || !sampleAt(m2, 0, true) || !sampleAt(0, -m2, true) || !sampleAt(0, m2, true) {
		return false
	}
	return true
}

// searchAlignmentPattern walks a square window of ±radius (~one module)
// around (predX, predY) and averages every candidate that passes the
// 5×5 alignment-pattern shape check, returning the mean centre. Falls back
// to ok = false if no candidate matched.
func searchAlignmentPattern(bm *bitmap, predX, predY, moduleSize float64) (float64, float64, bool) {
	cx0 := int(math.Round(predX))
	cy0 := int(math.Round(predY))
	radius := int(math.Ceil(moduleSize))
	if radius < 1 {
		radius = 1
	}
	var sumX, sumY float64
	count := 0
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			x, y := cx0+dx, cy0+dy
			if checkAlignmentAt(bm, x, y, moduleSize) {
				sumX += float64(x)
				sumY += float64(y)
				count++
			}
		}
	}
	if count == 0 {
		return 0, 0, false
	}
	return sumX / float64(count), sumY / float64(count), true
}

// refineHomography sharpens the initial perspective transform by locating
// the bottom-right alignment pattern (V2+) and using its actual centre as
// the fourth homography anchor in place of the parallelogram-completed
// matrix corner. For V1 (no alignment patterns) the original homography is
// returned unchanged, as it is when the alignment pattern is not found or
// the recomputed homography is numerically degenerate.
//
// See docs/theory/12-image-processing.md §7.
func refineHomography(bm *bitmap, h0 homography, tri finderTriple, v Version) homography {
	centers := v.AlignmentCenters()
	if len(centers) == 0 {
		return h0
	}
	// The bottom-right alignment centre is the last entry on the diagonal.
	alignMod := centers[len(centers)-1]
	predX, predY := h0.apply(float64(alignMod), float64(alignMod))
	avgModule := (tri.topLeft.moduleSize + tri.topRight.moduleSize + tri.bottomLeft.moduleSize) / 3.0
	foundX, foundY, ok := searchAlignmentPattern(bm, predX, predY, avgModule)
	if !ok {
		return h0
	}
	n := v.Size()
	farMod := float64(n - 4)
	srcMod := [4][2]float64{
		{3, 3},
		{farMod, 3},
		{3, farMod},
		{float64(alignMod), float64(alignMod)},
	}
	dstPx := [4][2]float64{
		{tri.topLeft.x, tri.topLeft.y},
		{tri.topRight.x, tri.topRight.y},
		{tri.bottomLeft.x, tri.bottomLeft.y},
		{foundX, foundY},
	}
	refined, err := computeHomography(srcMod, dstPx)
	if err != nil {
		return h0
	}
	return refined
}

// sampleMatrix uses the homography to sample one pixel per module centre out
// of the binarised image and returns the resulting [][]bool grid of side n.
// Out-of-bounds samples fall back to false (light), which is correct for
// pixels in the quiet zone or just beyond the image edge.
//
// See docs/theory/12-image-processing.md §8.
func sampleMatrix(bm *bitmap, h homography, n int) [][]bool {
	grid := make([][]bool, n)
	for r := range n {
		grid[r] = make([]bool, n)
		for c := range n {
			px, py := h.apply(float64(c), float64(r))
			ix := int(math.Round(px))
			iy := int(math.Round(py))
			if ix < 0 || iy < 0 || ix >= bm.width || iy >= bm.height {
				continue
			}
			grid[r][c] = bm.get(ix, iy)
		}
	}
	return grid
}

// decodeImage runs the full image-side pipeline (D8..D12) and dispatches the
// binarisation stage between Otsu (fast path) and Sauvola (adaptive
// fallback) per docs/theory/14-adaptive-thresholding.md §6. The production
// entry point discards which branch ran; decodeImageDebug exposes it for
// package-internal tests.
func decodeImage(img image.Image) (string, error) {
	text, _, err := decodeImageDebug(img)
	return text, err
}

// decodeImageDebug is the package-internal sibling of decodeImage that also
// returns the binariser branch that finder detection eventually succeeded
// on. Tests in the qrgen package use this to assert dispatch behaviour; the
// state is never exposed to external callers.
func decodeImageDebug(img image.Image) (string, binariserUsedState, error) {
	pixels, w, h := imageToGrayscale(img)
	t, eta := otsuThreshold(pixels)

	// Stage 1: the proactive bimodality gate. A unimodal histogram (η below
	// the cutoff) cannot give a meaningful global threshold, so we skip the
	// Otsu binarisation entirely and let Sauvola handle the image. This
	// saves one finder-detection pass over the obviously bad Otsu output.
	var bm *bitmap
	state := binariserOtsu
	if eta < etaMin {
		bm = sauvolaBinariseFromGray(pixels, w, h)
		state = binariserSauvolaProactive
	} else {
		bm = otsuBinariseFromGray(pixels, w, h, t)
	}

	tri, err := findFinders(bm)
	fgRatio := foregroundRatio(bm)
	unhealthy := err != nil || fgRatio < foregroundLo || fgRatio > foregroundHi

	// Stage 2: reactive fallback. If Otsu's binarisation gave a degenerate
	// foreground ratio or finder detection failed despite a healthy ratio,
	// rebinarise with Sauvola and retry. Proactive Sauvola has no further
	// fallback by design: if Sauvola already failed on a unimodal histogram,
	// Otsu cannot do better.
	if unhealthy && state == binariserOtsu {
		bm = sauvolaBinariseFromGray(pixels, w, h)
		state = binariserSauvolaReactive
		tri, err = findFinders(bm)
	}
	if err != nil {
		return "", state, err
	}

	v, err := estimateVersion(tri)
	if err != nil {
		return "", state, err
	}
	hg, err := homographyFromFinders(tri, v)
	if err != nil {
		return "", state, err
	}
	hg = refineHomography(bm, hg, tri, v)
	grid := sampleMatrix(bm, hg, v.Size())
	text, err := DecodeMatrix(grid)
	return text, state, err
}

// findFinders locates the three finder patterns in a binarised image and
// orders them as (top-left, top-right, bottom-left) assuming the symbol is
// approximately right-side-up. Returns ErrFinderNotFound if fewer than three
// valid finder triples can be identified or if their geometry is implausible.
//
// See docs/theory/12-image-processing.md §3–4.
func findFinders(bm *bitmap) (finderTriple, error) {
	var raw []finderCandidate
	for y := range bm.height {
		for _, c := range scanRowForFinders(bm, y) {
			cx, cy := int(math.Round(c.x)), int(math.Round(c.y))
			refinedY, vSize, ok := crossCheckVertical(bm, cx, cy)
			if !ok {
				continue
			}
			// Combine horizontal and vertical module-size estimates.
			c.y = refinedY
			c.moduleSize = (c.moduleSize + vSize) / 2.0
			raw = append(raw, c)
		}
	}
	if len(raw) < 3 {
		return finderTriple{}, ErrFinderNotFound
	}

	clusters, weights := clusterFinderCandidates(raw)
	if len(clusters) < 3 {
		return finderTriple{}, ErrFinderNotFound
	}

	// Sort clusters by descending weight so the three best-supported
	// candidates come first.
	indices := make([]int, len(clusters))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return weights[indices[i]] > weights[indices[j]]
	})
	top3 := []finderCandidate{
		clusters[indices[0]],
		clusters[indices[1]],
		clusters[indices[2]],
	}
	return orderFinderTriple(top3[0], top3[1], top3[2])
}
