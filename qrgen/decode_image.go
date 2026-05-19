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
	"image"
	"image/color"
	"math"
	"sort"
)

// ErrFinderNotFound is returned when the decoder cannot identify three valid
// finder patterns in the image. Callers can use errors.Is to branch on this.
var ErrFinderNotFound = errors.New("qrgen: finder patterns not found")

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
// variance for the given luminance pixels. Standard Otsu's method per
// docs/theory/12-image-processing.md §2; returns 128 as a safe default when
// the histogram is degenerate (all pixels share one value).
func otsuThreshold(pixels []uint8) uint8 {
	var hist [256]int
	for _, p := range pixels {
		hist[p]++
	}
	total := len(pixels)
	if total == 0 {
		return 128
	}

	var sumAll float64
	for i, c := range hist {
		sumAll += float64(i) * float64(c)
	}

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
	return bestT
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
	t := otsuThreshold(pixels)
	bm := &bitmap{width: w, height: h, pixels: make([]bool, w*h)}
	for i, p := range pixels {
		bm.pixels[i] = p <= t
	}
	return bm
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
