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
	"math"
	"testing"
)

// rmqrForcedGoldens returns the forced zint records, the ones with a grid.
func rmqrForcedGoldens(t *testing.T) []rmqrGolden {
	t.Helper()
	var out []rmqrGolden
	for _, g := range loadRMQRGoldens(t) {
		if !g.auto {
			out = append(out, g)
		}
	}
	return out
}

// rmqrGoldenText is what a golden decodes to, which DecodeMatrix establishes
// independently of the image search.
func rmqrGoldenText(t *testing.T, g rmqrGolden) string {
	t.Helper()
	text, err := DecodeMatrix(g.grid)
	if err != nil {
		t.Fatalf("%v: the golden itself does not decode: %v", g.version, err)
	}
	return text
}

// rmqrHeightGoldens picks one zint symbol per height, the widest of each, for
// the slower sweeps.
func rmqrHeightGoldens(t *testing.T) []rmqrGolden {
	t.Helper()
	best := map[int]rmqrGolden{}
	for _, g := range rmqrForcedGoldens(t) {
		h := g.version.Height()
		if b, ok := best[h]; !ok || g.version.Width() > b.version.Width() {
			best[h] = g
		}
	}
	var out []rmqrGolden
	for _, h := range rmqrHeights {
		out = append(out, best[h])
	}
	return out
}

// decodeRMQRBitmap runs the rMQR search alone over every finder of bm.
func decodeRMQRBitmap(bm *bitmap) (string, rmqrStage, error) {
	scan := scanComponents(bm)
	return decodeRMQRFromQuads(bm, scan, scan.quads)
}

// rmqrImageBitmap binarises an image as the connected-component attempt does
// with Otsu's threshold.
func rmqrImageBitmap(img image.Image) *bitmap {
	pixels, w, h := imageToGrayscale(img)
	th, _ := otsuThreshold(pixels)
	return otsuBinariseFromGray(pixels, w, h, th)
}

// rmqrSceneOutcome reports "ok" when the rMQR search reads want from img, and
// otherwise the stage it reached.
func rmqrSceneOutcome(img image.Image, want string) string {
	text, stage, err := decodeRMQRBitmap(rmqrImageBitmap(img))
	switch {
	case err == nil && text == want:
		return "ok"
	case err == nil:
		return fmt.Sprintf("WRONG %q", text)
	}
	return fmt.Sprintf("stage %d (%v)", stage, err)
}

// TestRMQRDetectZintGoldens: every zint symbol, painted upright at 3 and 5
// pixels per module and turned through every quarter turn, is located and
// decoded by the rMQR search from its finder and sub-pattern.
func TestRMQRDetectZintGoldens(t *testing.T) {
	for _, g := range rmqrForcedGoldens(t) {
		want := rmqrGoldenText(t, g)
		for _, scale := range []int{3, 5} {
			for quarter := range 4 {
				bm := paintGrid(rotateGrid(g.grid, quarter), scale, 2)
				text, stage, err := decodeRMQRBitmap(bm)
				if err != nil || text != want {
					t.Errorf("%v-%v %q at %d px/module, %d quarter turns: (%q, stage %d, %v)", g.version, g.ec, g.note, scale, quarter, text, stage, err)
				}
			}
		}
	}
}

// TestRMQRRotationBand guards the measured rotation band. Measured every 5
// degrees over all 70 zint symbols, the search reads 1255 of 1260 scenes at 4
// pixels per module, 1231 at 3, and 991 at 2; the misses at 4 are the longest
// symbols near 45 degrees. Asserted here at 6 pixels per module every 15
// degrees and at 4 at three angles, with no failure allowed.
func TestRMQRRotationBand(t *testing.T) {
	for _, g := range rmqrForcedGoldens(t) {
		want := rmqrGoldenText(t, g)
		for _, c := range []struct {
			scale  int
			angles []float64
		}{
			{6, []float64{15, 30, 45, 60, 75}},
			{4, []float64{10, 30, 70}},
		} {
			src := gridImage(g.grid, c.scale, 2)
			for _, a := range c.angles {
				if o := rmqrSceneOutcome(rotateImage(src, a), want); o != "ok" {
					t.Errorf("%v-%v %q at %d px/module rotated %.0f deg: %s", g.version, g.ec, g.note, c.scale, a, o)
				}
			}
		}
	}
}

// TestRMQRTiltBand guards the measured tilt band. Over all 70 symbols under
// four in-plane rotations, the search reads every scene to 55 degrees of tilt
// at 10 pixels per module (278 of 280 at 60) and every scene at 30 degrees at
// 5 (262 of 280 at 45). Asserted here for the widest symbol of each height.
func TestRMQRTiltBand(t *testing.T) {
	cases := []struct {
		scale int
		tilt  float64
	}{{10, 45}, {10, 55}, {5, 30}}
	for _, g := range rmqrHeightGoldens(t) {
		want := rmqrGoldenText(t, g)
		for _, c := range cases {
			src := gridImage(g.grid, c.scale, 4)
			for _, rot := range []float64{0, 60} {
				if o := rmqrSceneOutcome(warpTilt(t, src, c.tilt, rot), want); o != "ok" {
					t.Errorf("%v %q at %d px/module, tilt %.0f, rotation %.0f: %s", g.version, g.text, c.scale, c.tilt, rot, o)
				}
			}
		}
	}
}

// TestRMQRQuietZones: the search needs no quiet zone, since every gate reads
// modules of the symbol itself.
func TestRMQRQuietZones(t *testing.T) {
	for _, g := range rmqrHeightGoldens(t) {
		want := rmqrGoldenText(t, g)
		for _, quiet := range []int{0, 1, 4} {
			if o := rmqrSceneOutcome(rotateImage(gridImage(g.grid, 4, quiet), 20), want); o != "ok" {
				t.Errorf("%v with a %d-module quiet zone: %s", g.version, quiet, o)
			}
		}
	}
}

// overwriteRMQRFormat writes word, masked as the copy's own mask requires,
// over one format copy of an rMQR grid: the copy beside the finder, or, with
// sub, the copy beside the sub-pattern.
func overwriteRMQRFormat(grid [][]bool, word uint32, sub bool) {
	rows, cols := len(grid), len(grid[0])
	for i := range 18 {
		r, c := rmqrFormatModuleFinder(i)
		if sub {
			r, c = rmqrFormatModuleSub(i, rows, cols)
		}
		grid[r][c] = bitOf(word, i)
	}
}

// rmqrFarWord returns a word more than 3 bits from every valid rMQR format
// word under both masks, so a copy holding it reads as unreadable.
func rmqrFarWord(t *testing.T) uint32 {
	t.Helper()
	for w := uint32(0); w < 1<<18; w++ {
		near := false
		for v := R7x43; v <= R17x139 && !near; v++ {
			for lvl := range 2 {
				word := bch18Word(rmqrFormatData(v, lvl))
				for _, mask := range [2]uint32{rmqrFormatMaskFinder, rmqrFormatMaskSub} {
					if popcount(w^word^mask) <= rmqrFormatBudget {
						near = true
					}
				}
			}
		}
		if !near {
			return w
		}
	}
	t.Fatal("no word is far from every format word")
	return 0
}

func popcount(x uint32) int {
	n := 0
	for ; x != 0; x &= x - 1 {
		n++
	}
	return n
}

// TestRMQRFormatCopyFallback: with the format copy beside the finder
// destroyed, the search still locates and reads every symbol through the copy
// beside the sub-pattern; with both destroyed nothing names its version, so
// it is not located and the error is ErrFinderNotFound.
func TestRMQRFormatCopyFallback(t *testing.T) {
	far := rmqrFarWord(t)
	for _, g := range rmqrHeightGoldens(t) {
		want := rmqrGoldenText(t, g)
		grid := cloneGrid(g.grid)
		overwriteRMQRFormat(grid, far, false)
		for _, a := range []float64{0, 25} {
			if o := rmqrSceneOutcome(rotateImage(gridImage(grid, 4, 2), a), want); o != "ok" {
				t.Errorf("%v with the finder-side copy destroyed, rotated %.0f: %s", g.version, a, o)
			}
		}
		overwriteRMQRFormat(grid, far, true)
		_, stage, err := decodeRMQRBitmap(paintGrid(grid, 4, 2))
		if err == nil || stage >= rmqrStageLocated || !errors.Is(err, ErrFinderNotFound) {
			t.Errorf("%v with both copies destroyed: stage %d, error %v; want an unlocated finder", g.version, stage, err)
		}
	}
}

func cloneGrid(g [][]bool) [][]bool {
	out := make([][]bool, len(g))
	for i := range g {
		out[i] = append([]bool(nil), g[i]...)
	}
	return out
}

// TestRMQRRefinementValue measures what the edge fit adds over the
// eight-point homography. Over the widest symbol of each height rendered at 2
// and 3 pixels per module and rotated in 10 degree steps, it counts the
// scenes each decodes with, and requires the edge fit to read at least as
// many as the eight-point homography alone, and more overall (theory doc 30,
// section 5).
func TestRMQRRefinementValue(t *testing.T) {
	var eight, fitted, total int
	for _, g := range rmqrHeightGoldens(t) {
		want := rmqrGoldenText(t, g)
		for _, scale := range []int{2, 3} {
			src := gridImage(g.grid, scale, 2)
			for a := 5; a < 90; a += 10 {
				total++
				bm := rmqrImageBitmap(rotateImage(src, float64(a)))
				scan := scanComponents(bm)
				var gotEight, gotFitted bool
				for _, q := range scan.quads {
					cands, _ := locateRMQR(bm, scan, q)
					for _, c := range cands {
						v := c.sym.version
						read := func(h homography) bool {
							text, err := decodeRMQRMatrix(sampleGrid(bm, h, v.Height(), v.Width()))
							return err == nil && text == want
						}
						h8 := c.hgs[len(c.hgs)-1] // the variants end with the eight-point homography
						gotEight = gotEight || read(h8)
						for _, h := range c.hgs[:len(c.hgs)-1] {
							gotFitted = gotFitted || read(h)
						}
					}
				}
				if gotEight {
					eight++
				}
				if gotFitted || gotEight {
					fitted++
				}
			}
		}
	}
	t.Logf("of %d scenes: eight-point homography %d, with the edge fit %d", total, eight, fitted)
	if fitted <= eight {
		t.Errorf("the edge fit reads no scene the eight-point homography misses (%d vs %d)", fitted, eight)
	}
}

// TestSubPatternCandidates: a lone 5 by 5 sub-pattern is found with its centre
// and module size, at any rotation of a few pixels per module, and a finder's
// own nesting, a 5-module light ring around a 3-module centre, is not mistaken
// for one.
func TestSubPatternCandidates(t *testing.T) {
	sub := make([][]bool, 5)
	for r := range 5 {
		sub[r] = make([]bool, 5)
		for c := range 5 {
			sub[r][c] = r == 0 || r == 4 || c == 0 || c == 4 || (r == 2 && c == 2)
		}
	}
	for _, scale := range []int{3, 6} {
		bm := paintGrid(sub, scale, 3)
		subs := scanComponents(bm).subPatterns()
		if len(subs) != 1 {
			t.Fatalf("%d px/module: %d candidates, want 1", scale, len(subs))
		}
		centre := (3 + 2.5) * float64(scale)
		sp := subs[0]
		if math.Abs(sp.x+0.5-centre) > 0.6 || math.Abs(sp.y+0.5-centre) > 0.6 || math.Abs(sp.moduleSize-float64(scale)) > 0.5 {
			t.Errorf("%d px/module: candidate at (%.1f, %.1f), module %.2f; want (%.1f, %.1f), %d", scale, sp.x+0.5, sp.y+0.5, sp.moduleSize, centre, centre, scale)
		}
		for _, a := range []float64{20, 45} {
			if n := len(scanComponents(rmqrImageBitmap(rotateImage(gridImage(sub, scale, 3), a))).subPatterns()); n != 1 {
				t.Errorf("%d px/module rotated %.0f: %d candidates, want 1", scale, a, n)
			}
		}
	}
	if n := len(scanComponents(paintGrid(finderOnlyGrid(), 4, 3)).subPatterns()); n != 0 {
		t.Errorf("a finder alone gave %d sub-pattern candidates", n)
	}
}

// TestFinderAffine: for a finder seen under a similarity (rotation, scale,
// shift), the affine fit maps every module, near or far, exactly where the
// similarity does.
func TestFinderAffine(t *testing.T) {
	theta, s, tx, ty := 0.4, 5.0, 100.0, 40.0
	sim := func(u, v float64) [2]float64 {
		return [2]float64{tx + s*(u*math.Cos(theta)-v*math.Sin(theta)), ty + s*(u*math.Sin(theta)+v*math.Cos(theta))}
	}
	var finder [4][2]float64
	for i, c := range finderRingCorners {
		finder[i] = sim(c[0], c[1])
	}
	a := finderAffine(finder)
	for _, m := range [][2]float64{{0, 0}, {6, 6}, {136, 14}, {-3, 50}} {
		x, y := a.apply(m[0], m[1])
		if want := sim(m[0], m[1]); math.Hypot(x-want[0], y-want[1]) > 1e-9 {
			t.Errorf("module %v: (%.6f, %.6f), want %v", m, x, y, want)
		}
	}
	if inv, ok := invertAffine(a); !ok {
		t.Error("the affine map does not invert")
	} else if u, v := inv.apply(a.apply(37, 9)); math.Hypot(u-37, v-9) > 1e-9 {
		t.Errorf("inverse maps back to (%.6f, %.6f), want (37, 9)", u, v)
	}
}

// TestRMQREdgeModules: every module the edge fit marches from is dark in
// every zint symbol of its version, and each edge has enough of them.
func TestRMQREdgeModules(t *testing.T) {
	for _, g := range rmqrForcedGoldens(t) {
		edges := rmqrEdgeModules(g.version)
		for i, edge := range edges {
			if len(edge) < 2 {
				t.Errorf("%v edge %d: %d modules", g.version, i, len(edge))
			}
			for _, m := range edge {
				if !g.grid[m[1]][m[0]] {
					t.Errorf("%v edge %d: module (col %d, row %d) is light in zint's symbol", g.version, i, m[0], m[1])
				}
			}
		}
	}
}
