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
	"testing"
)

// paintGrid renders a module grid into a bitmap at scale pixels per module,
// with a light quiet zone of quiet modules on every side.
func paintGrid(grid [][]bool, scale, quiet int) *bitmap {
	n := len(grid)
	side := (n + 2*quiet) * scale
	bm := newBlankBitmap(side, side)
	for r := range n {
		for c := range n {
			if !grid[r][c] {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					x, y := (quiet+c)*scale+dx, (quiet+r)*scale+dy
					bm.pixels[y*side+x] = true
				}
			}
		}
	}
	return bm
}

// rotateGrid90 returns grid turned a quarter turn clockwise.
func rotateGrid90(g [][]bool) [][]bool {
	n := len(g)
	out := make([][]bool, n)
	for r := range n {
		out[r] = make([]bool, n)
		for c := range n {
			out[r][c] = g[n-1-c][r]
		}
	}
	return out
}

// rotateGrid turns grid clockwise by quarter quarter turns.
func rotateGrid(g [][]bool, quarter int) [][]bool {
	for range quarter % 4 {
		g = rotateGrid90(g)
	}
	return g
}

// microTestGrid encodes text as a Micro symbol of the given type through the
// public Matrix API.
func microTestGrid(t *testing.T, text string, opts ...Option) [][]bool {
	t.Helper()
	grid, err := Matrix(text, append([]Option{WithMicro()}, opts...)...)
	if err != nil {
		t.Fatalf("Matrix(%q): %v", text, err)
	}
	return grid
}

// gatedAssignments counts the quarter-turn assignments of q whose two innermost
// timing modules read dark: the orientation test of theory doc 27, section 3.
func gatedAssignments(bm *bitmap, q finderQuad) int {
	corners := windCorners(outerRingCorners(q))
	n := 0
	for k := range 4 {
		hg, err := microFinderHomography(corners, k)
		if err == nil && moduleDark(bm, hg, 8, 0) && moduleDark(bm, hg, 0, 8) {
			n++
		}
	}
	return n
}

// writeFarFormatWord overwrites a Micro grid's format modules with a word more
// than three bits from every Micro format word, so the format information is
// unreadable while the timing patterns stay intact.
func writeFarFormatWord(grid [][]bool) {
	var far uint32
	for w := uint32(0); w < 1<<15; w++ {
		if _, _, d := nearestMicroFormat(w, 0); d > microFormatBudget {
			far = w
			break
		}
	}
	for i := range 15 {
		r, c := microFormatModule(i)
		grid[r][c] = bitOf(far, i)
	}
}

// gridImage renders a module grid as a grayscale image: scale pixels per
// module, black modules on white, with quiet modules of white margin.
func gridImage(grid [][]bool, scale, quiet int) *image.Gray {
	n := len(grid)
	side := (n + 2*quiet) * scale
	img := image.NewGray(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for r := range n {
		for c := range n {
			if !grid[r][c] {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					img.SetGray((quiet+c)*scale+dx, (quiet+r)*scale+dy, color.Gray{})
				}
			}
		}
	}
	return img
}

// finderOnlyGrid is a bare 7 by 7 finder pattern.
func finderOnlyGrid() [][]bool {
	g := make([][]bool, 7)
	for r := range 7 {
		g[r] = make([]bool, 7)
		for c := range 7 {
			g[r][c] = finderModules[r][c] == 1
		}
	}
	return g
}

// TestDecodeMicroZintImages: Decode reads every zint symbol rendered as an
// image, at each quarter turn and at two scales, through the Micro path.
func TestDecodeMicroZintImages(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		for quarter := range 4 {
			for _, scale := range []int{3, 8} {
				img := gridImage(rotateGrid(g.grid, quarter), scale, 2)
				text, path, err := decodeImageWhich(img)
				if err != nil || text != g.text || path != "micro" {
					t.Errorf("%s %q turn %d scale %d: got (%q, %q, %v)", g.typ, g.text, quarter, scale, text, path, err)
				}
			}
		}
	}
}

// TestDecodeMicroOwnPNGs: every Micro symbol type from our encoder decodes
// through DecodeBytes from its default PNG rendering.
func TestDecodeMicroOwnPNGs(t *testing.T) {
	cases := []struct {
		text string
		opts []Option
	}{
		{"12345", []Option{WithMicroVersion(M1)}},
		{"HELLO1", []Option{WithMicroVersion(M2), WithECLevel(ECLevelL)}},
		{"HELLO", []Option{WithMicroVersion(M2), WithECLevel(ECLevelM)}},
		{"Micro 3L", []Option{WithMicroVersion(M3), WithECLevel(ECLevelL)}},
		{"micro3M", []Option{WithMicroVersion(M3), WithECLevel(ECLevelM)}},
		{"Micro QR M4-L!", []Option{WithMicroVersion(M4), WithECLevel(ECLevelL)}},
		{"Micro QR M4M", []Option{WithMicroVersion(M4), WithECLevel(ECLevelM)}},
		{"micro M4Q", []Option{WithMicroVersion(M4), WithECLevel(ECLevelQ)}},
	}
	for _, tc := range cases {
		pngBytes, err := Encode(tc.text, append([]Option{WithMicro()}, tc.opts...)...)
		if err != nil {
			t.Fatalf("Encode(%q): %v", tc.text, err)
		}
		if got, err := DecodeBytes(pngBytes); err != nil || got != tc.text {
			t.Errorf("DecodeBytes(%q) = %q, %v", tc.text, got, err)
		}
	}
}

// TestDecodeMicroErrorContract pins the error contract of theory doc 27,
// section 8: a convincing Micro symbol reports its own error, while a bare
// finder pattern keeps "finder patterns not found".
func TestDecodeMicroErrorContract(t *testing.T) {
	t.Run("damaged data", func(t *testing.T) {
		grid := microTestGrid(t, "DAMAGED 1", WithMicroVersion(M3), WithECLevel(ECLevelL))
		for r := 9; r < 14; r++ {
			for c := 9; c < 14; c++ {
				grid[r][c] = !grid[r][c]
			}
		}
		if _, err := Decode(gridImage(grid, 6, 2)); !errors.Is(err, ErrTooManyErrors) {
			t.Errorf("err = %v, want ErrTooManyErrors", err)
		}
	})
	t.Run("unreadable format", func(t *testing.T) {
		grid := microTestGrid(t, "HELLO", WithMicroVersion(M3))
		writeFarFormatWord(grid)
		if _, err := Decode(gridImage(grid, 6, 2)); !errors.Is(err, ErrFormatUnreadable) {
			t.Errorf("err = %v, want ErrFormatUnreadable", err)
		}
	})
	t.Run("bare finder", func(t *testing.T) {
		if _, err := Decode(gridImage(finderOnlyGrid(), 6, 6)); !errors.Is(err, ErrFinderNotFound) {
			t.Errorf("err = %v, want ErrFinderNotFound", err)
		}
	})
}

// TestDecodeQRWithMissingFinderNoMicro wipes one finder of each of a corpus of
// QR symbols, so QR detection fails and the Micro search runs on the two that
// remain, and requires that Decode never returns text. It logs the errors that
// come back: the error contract keeps "finder patterns not found" unless a
// remaining finder looks convincingly like a Micro symbol.
func TestDecodeQRWithMissingFinderNoMicro(t *testing.T) {
	counts := map[string]int{}
	for v := Version(1); v <= 4; v++ {
		for mask := range 8 {
			grid, err := Matrix(fmt.Sprintf("WIPE %d %d", v, mask), WithVersion(v), WithMask(mask))
			if err != nil {
				t.Fatalf("Matrix: %v", err)
			}
			n := len(grid)
			for which, origin := range [3][2]int{{0, 0}, {0, n - 7}, {n - 7, 0}} {
				g := make([][]bool, n)
				for r := range n {
					g[r] = append([]bool(nil), grid[r]...)
				}
				for r := range 7 {
					for c := range 7 {
						g[origin[0]+r][origin[1]+c] = false
					}
				}
				text, err := Decode(gridImage(g, 4, 4))
				switch {
				case err == nil:
					t.Errorf("v%d mask %d, finder %d wiped: decoded %q", v, mask, which, text)
				case errors.Is(err, ErrFinderNotFound):
					counts["finder not found"]++
				default:
					counts[err.Error()]++
				}
			}
		}
	}
	t.Logf("errors for QR symbols with one finder wiped: %v", counts)
}

func TestMicroFormatModuleLayout(t *testing.T) {
	// Theory doc 26, section 7: bits 0..7 down column 8 from row 1, bits 8..14
	// left along row 8, bit 14 at (8, 1).
	want := map[int][2]int{0: {1, 8}, 7: {8, 8}, 8: {8, 7}, 14: {8, 1}}
	for i, rc := range want {
		if r, c := microFormatModule(i); r != rc[0] || c != rc[1] {
			t.Errorf("bit %d at (%d, %d), want (%d, %d)", i, r, c, rc[0], rc[1])
		}
	}
	seen := map[[2]int]bool{}
	for i := range 15 {
		r, c := microFormatModule(i)
		if seen[[2]int{r, c}] {
			t.Errorf("bit %d reuses module (%d, %d)", i, r, c)
		}
		seen[[2]int{r, c}] = true
	}
}

func TestWindCorners(t *testing.T) {
	tl, tr, br, bl := [2]float64{10, 10}, [2]float64{40, 12}, [2]float64{38, 41}, [2]float64{9, 39}
	want := [4][2]float64{tl, tr, br, bl}
	for _, in := range [][4][2]float64{
		{tl, tr, br, bl},
		{br, tl, bl, tr},
		{bl, br, tr, tl}, // reversed winding comes back in the positive one
	} {
		if got := windCorners(in); got != want {
			t.Errorf("windCorners(%v) = %v, want %v", in, got, want)
		}
	}
}

// TestMicroOrientationQuarterTurns pins theory doc 27, section 3: on a clean
// image exactly one of the four assignments passes the timing test, at every
// quarter turn and for every version, and it is the one that decodes.
func TestMicroOrientationQuarterTurns(t *testing.T) {
	for v := M1; v <= M4; v++ {
		grid := microTestGrid(t, "12345", WithMicroVersion(v))
		for quarter := range 4 {
			bm := paintGrid(rotateGrid(grid, quarter), 6, 2)
			quads := allFinderQuads(bm)
			if len(quads) != 1 {
				t.Fatalf("%v turn %d: %d finder quads, want 1", v, quarter, len(quads))
			}
			if n := gatedAssignments(bm, quads[0]); n != 1 {
				t.Errorf("%v turn %d: %d assignments pass the timing test, want 1", v, quarter, n)
			}
			text, ver, _, stage, err := decodeMicroQuad(bm, quads[0])
			if err != nil || text != "12345" || ver != v || stage != microStageLocated {
				t.Errorf("%v turn %d: got (%q, %v, stage %d, %v), want (\"12345\", %v, located, nil)", v, quarter, text, ver, stage, err, v)
			}
		}
	}
}

// TestMicroDetectZintGoldens decodes every zint symbol painted into a bitmap at
// each quarter turn. zint shares no code with this library, so the detector
// cannot pass by agreeing with a mistake in our own encoder.
func TestMicroDetectZintGoldens(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		for quarter := range 4 {
			bm := paintGrid(rotateGrid(g.grid, quarter), 5, 2)
			text, stage, err := decodeMicroFromQuads(bm, allFinderQuads(bm))
			if err != nil || text != g.text {
				t.Errorf("%s %q turn %d: got (%q, stage %d, %v)", g.typ, g.text, quarter, text, stage, err)
			}
		}
	}
}

// TestMicroTimingMismatchesWrongSize pins theory doc 27, section 5: a symbol
// read at its own size has no disagreement, and a claimed size one version off
// in either direction disagrees in exactly one module per edge.
func TestMicroTimingMismatchesWrongSize(t *testing.T) {
	for v := M1; v <= M4; v++ {
		grid := microTestGrid(t, "1", WithMicroVersion(v))
		bm := paintGrid(grid, 6, 2)
		quads := allFinderQuads(bm)
		if len(quads) != 1 {
			t.Fatalf("%v: %d finder quads, want 1", v, len(quads))
		}
		cands, _ := locateMicro(bm, quads[0])
		if len(cands) == 0 {
			t.Fatalf("%v: not located", v)
		}
		hg, n := cands[0].hg, v.Size()
		if got := microTimingMismatches(bm, hg, n); got != 0 {
			t.Errorf("%v at its own size: %d disagreements, want 0", v, got)
		}
		for _, other := range []int{n - 2, n + 2} {
			if other < M1.Size() || other > M4.Size() {
				continue
			}
			if got := microTimingMismatches(bm, hg, other); got != 2 {
				t.Errorf("%v read as size %d: %d disagreements, want 2", v, other, got)
			}
		}
	}
}

// TestMicroRejectsLoneFinder: a finder pattern with nothing beside it has no
// timing pattern, so no assignment passes and the error says no finder.
func TestMicroRejectsLoneFinder(t *testing.T) {
	bm := newBlankBitmap(120, 120)
	paintModules(bm, 40, 40, 5, finderModules)
	quads := allFinderQuads(bm)
	if len(quads) != 1 {
		t.Fatalf("%d finder quads, want 1", len(quads))
	}
	_, stage, err := decodeMicroFromQuads(bm, quads)
	if stage != microStageNone || !errors.Is(err, ErrFinderNotFound) {
		t.Errorf("got stage %d, err %v; want no stage and ErrFinderNotFound", stage, err)
	}
	if _, _, err := decodeMicroFromQuads(bm, nil); !errors.Is(err, ErrFinderNotFound) {
		t.Errorf("no quads: err %v, want ErrFinderNotFound", err)
	}
}

// TestMicroFormatDamageStage: with the timing intact but a format word more
// than three bits from every Micro word, the timing pattern still matches a
// Micro size, so the finder counts as timed and the error is the format error.
// (Inverting all fifteen bits would not do: the all-ones word belongs to the
// BCH code, so the complement of a format word is another valid one.)
func TestMicroFormatDamageStage(t *testing.T) {
	grid := microTestGrid(t, "HELLO", WithMicroVersion(M3))
	writeFarFormatWord(grid)
	bm := paintGrid(grid, 6, 2)
	_, stage, err := decodeMicroFromQuads(bm, allFinderQuads(bm))
	if stage != microStageTimed || !errors.Is(err, ErrFormatUnreadable) {
		t.Errorf("got stage %d, err %v; want timed and ErrFormatUnreadable", stage, err)
	}
}

// TestMicroIgnoresQRFinders runs the Micro search over every finder of a corpus
// of QR symbols (versions 1 to 7, every EC level and mask) and requires that
// none decodes as a Micro symbol. It also logs how far the QR finders got and
// how many timing disagreements the assignments that reached the cross-check
// had: the measurement behind microTimingTolerance and behind the error
// contract, which trusts only the timed and located stages.
func TestMicroIgnoresQRFinders(t *testing.T) {
	stages := map[microStage]int{}
	hist := map[int]int{}
	for v := Version(1); v <= 7; v++ {
		for _, ec := range []ECLevel{ECLevelL, ECLevelM, ECLevelQ, ECLevelH} {
			for mask := range 8 {
				payload := fmt.Sprintf("QR %d %v %d", v, ec, mask)
				grid, err := Matrix(payload, WithVersion(v), WithECLevel(ec), WithMask(mask))
				if err != nil {
					t.Fatalf("Matrix: %v", err)
				}
				bm := paintGrid(grid, 3, 4)
				for _, q := range allFinderQuads(bm) {
					text, _, _, stage, err := decodeMicroQuad(bm, q)
					if err == nil {
						t.Errorf("v%d %v mask %d: a QR finder decoded as Micro text %q", v, ec, mask, text)
					}
					stages[stage]++
					corners := windCorners(outerRingCorners(q))
					for k := range 4 {
						hg, err := microFinderHomography(corners, k)
						if err != nil || !moduleDark(bm, hg, 8, 0) || !moduleDark(bm, hg, 0, 8) {
							continue
						}
						if sym, _, dist := nearestMicroFormat(readMicroFormatWord(bm, hg), 0); dist <= microFormatBudget {
							hist[microTimingMismatches(bm, hg, sym.version.Size())]++
						}
					}
				}
			}
		}
	}
	t.Logf("QR finders by furthest Micro stage: none %d, oriented %d, format %d, timed %d, located %d",
		stages[microStageNone], stages[microStageOriented], stages[microStageFormat], stages[microStageTimed], stages[microStageLocated])
	t.Logf("timing cross-check disagreements of QR finder assignments that reached it: %v", hist)
}
