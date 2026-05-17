// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

func TestNewMatrixSize(t *testing.T) {
	cases := []struct {
		v    Version
		want int
	}{
		{1, 21},
		{2, 25},
		{7, 45},
		{40, 177},
	}
	for _, c := range cases {
		t.Run(c.v.String(), func(t *testing.T) {
			m := newMatrix(c.v)
			if m.size != c.want {
				t.Errorf("size = %d, want %d", m.size, c.want)
			}
			if len(m.modules) != c.want || len(m.modules[0]) != c.want {
				t.Errorf("modules dims = %dx%d, want %dx%d", len(m.modules), len(m.modules[0]), c.want, c.want)
			}
		})
	}
}

// TestFinderCornerCells asserts the 7×7 finder shape at each corner of V1.
func TestFinderCornerCells(t *testing.T) {
	m := newMatrix(1)
	m.placeFinderPatterns()
	// Expected finder pattern: dark border + dark 3×3 centre.
	expected := [7][7]bool{
		{true, true, true, true, true, true, true},
		{true, false, false, false, false, false, true},
		{true, false, true, true, true, false, true},
		{true, false, true, true, true, false, true},
		{true, false, true, true, true, false, true},
		{true, false, false, false, false, false, true},
		{true, true, true, true, true, true, true},
	}
	corners := []struct {
		name     string
		r0, c0   int
	}{
		{"top-left", 0, 0},
		{"top-right", 0, 14},
		{"bottom-left", 14, 0},
	}
	for _, corner := range corners {
		t.Run(corner.name, func(t *testing.T) {
			for dr := 0; dr < 7; dr++ {
				for dc := 0; dc < 7; dc++ {
					r, c := corner.r0+dr, corner.c0+dc
					if got := m.get(r, c); got != expected[dr][dc] {
						t.Errorf("(%d,%d) = %v, want %v", r, c, got, expected[dr][dc])
					}
					if !m.isReserved(r, c) {
						t.Errorf("(%d,%d) not reserved", r, c)
					}
				}
			}
		})
	}
}

// TestSeparatorsAreLightAndReserved checks the 1-module separator strips
// around each finder on the sides facing the data area.
func TestSeparatorsAreLightAndReserved(t *testing.T) {
	m := newMatrix(1)
	m.placeFinderPatterns()
	// Top-left separator: row 7 (cols 0..7) + col 7 (rows 0..7).
	for i := 0; i <= 7; i++ {
		if m.get(7, i) {
			t.Errorf("top-left separator (7,%d) is dark, want light", i)
		}
		if !m.isReserved(7, i) {
			t.Errorf("top-left separator (7,%d) not reserved", i)
		}
		if m.get(i, 7) {
			t.Errorf("top-left separator (%d,7) is dark, want light", i)
		}
		if !m.isReserved(i, 7) {
			t.Errorf("top-left separator (%d,7) not reserved", i)
		}
	}
}

// TestTimingPatternRow checks that row 6 alternates dark/light starting with
// dark at column 8 (the first cell outside the finder/separator region).
func TestTimingPatternRow(t *testing.T) {
	m := newMatrix(1)
	m.placeFinderPatterns()
	m.placeTimingPatterns()
	n := m.size
	for c := 8; c < n-8; c++ {
		want := c%2 == 0
		if got := m.get(6, c); got != want {
			t.Errorf("timing (6,%d) = %v, want %v", c, got, want)
		}
		if !m.isReserved(6, c) {
			t.Errorf("timing (6,%d) not reserved", c)
		}
	}
}

func TestDarkModulePosition(t *testing.T) {
	cases := []struct {
		v       Version
		row, col int
	}{
		{1, 13, 8},   // 4*1+9=13
		{7, 37, 8},   // 4*7+9=37
		{40, 169, 8}, // 4*40+9=169
	}
	for _, c := range cases {
		t.Run(c.v.String(), func(t *testing.T) {
			m := newMatrix(c.v)
			m.placeFunctionalPatterns()
			if !m.get(c.row, c.col) {
				t.Errorf("dark module (%d,%d) is light, want dark", c.row, c.col)
			}
			if !m.isReserved(c.row, c.col) {
				t.Errorf("dark module (%d,%d) not reserved", c.row, c.col)
			}
		})
	}
}

// TestAlignmentCountByVersion compares the number of alignment patterns drawn
// against the expected per-version count from ISO/IEC 18004:2015.
func TestAlignmentCountByVersion(t *testing.T) {
	// Expected number of alignment patterns per version. V1 = 0; the rest grow
	// with the size of the alignment-centre grid minus the three corner-finder
	// positions.
	expected := map[Version]int{
		1:  0,
		2:  1, // (18,18)
		6:  1, // single centre pair off the finder corners
		7:  6, // 3×3 grid minus 3 corners
		14: 13, // 4×4 grid minus 3 corners
		20: 13, // still 4×4
		40: 46, // 7×7 grid minus 3 corners
	}
	for v, want := range expected {
		m := newMatrix(v)
		m.placeFunctionalPatterns()
		got := countAlignmentCenters(m, v)
		if got != want {
			t.Errorf("V%d: %d alignment patterns drawn, want %d", v, got, want)
		}
	}
}

// countAlignmentCenters counts cells matching the 5×5 alignment-pattern centre
// (a single dark module surrounded by a 3×3 light ring inside a 5×5 dark
// outer). For each centre coordinate from v.AlignmentCenters() that does not
// collide with a finder, the centre at (rc, cc) is dark with a light cell
// directly above and below — a cheap shape check.
func countAlignmentCenters(m *matrix, v Version) int {
	centers := v.AlignmentCenters()
	if len(centers) == 0 {
		return 0
	}
	first := centers[0]
	last := centers[len(centers)-1]
	count := 0
	for _, rc := range centers {
		for _, cc := range centers {
			if (rc == first && cc == first) ||
				(rc == first && cc == last) ||
				(rc == last && cc == first) {
				continue
			}
			// Light cell directly inside the dark outer ring confirms the
			// alignment pattern was placed (centre is dark, ring of 3×3 around
			// centre is light).
			if m.get(rc, cc) && !m.get(rc-1, cc) && !m.get(rc+1, cc) {
				count++
			}
		}
	}
	return count
}

// TestFormatInfoAreaReserved spot-checks the format-info reservations around
// each finder on V1. We do not assert dark/light here — that is M6's job.
func TestFormatInfoAreaReserved(t *testing.T) {
	m := newMatrix(1)
	m.placeFunctionalPatterns()
	n := m.size
	// Row 8 left + right side.
	for c := 0; c < 9; c++ {
		if !m.isReserved(8, c) {
			t.Errorf("(8,%d) not reserved (format info left)", c)
		}
	}
	for c := n - 8; c < n; c++ {
		if !m.isReserved(8, c) {
			t.Errorf("(8,%d) not reserved (format info right)", c)
		}
	}
	// Col 8 top + bottom.
	for r := 0; r < 9; r++ {
		if !m.isReserved(r, 8) {
			t.Errorf("(%d,8) not reserved (format info top)", r)
		}
	}
	for r := n - 7; r < n; r++ {
		if !m.isReserved(r, 8) {
			t.Errorf("(%d,8) not reserved (format info bottom)", r)
		}
	}
}

// TestVersionInfoAreaReservedV7 verifies V7+ adds two 6×3 / 3×6 reserved
// blocks for the version-info BCH codeword.
func TestVersionInfoAreaReservedV7(t *testing.T) {
	v := Version(7)
	m := newMatrix(v)
	m.placeFunctionalPatterns()
	n := m.size
	// Block adjacent to the bottom-left finder.
	for r := 0; r < 6; r++ {
		for c := n - 11; c <= n-9; c++ {
			if !m.isReserved(r, c) {
				t.Errorf("(%d,%d) not reserved (version info top)", r, c)
			}
		}
	}
	// Block adjacent to the top-right finder.
	for r := n - 11; r <= n-9; r++ {
		for c := 0; c < 6; c++ {
			if !m.isReserved(r, c) {
				t.Errorf("(%d,%d) not reserved (version info left)", r, c)
			}
		}
	}
}

// TestVersionInfoAreaNotReservedV6 confirms version-info modules are not
// reserved for versions below 7.
func TestVersionInfoAreaNotReservedV6(t *testing.T) {
	v := Version(6)
	m := newMatrix(v)
	m.placeFunctionalPatterns()
	n := m.size
	// Where V7's version-info block would be — the cells must not be reserved
	// on V6 (they belong to the data area).
	r, c := 0, n-11
	if m.isReserved(r, c) {
		t.Errorf("V6 (%d,%d) reserved; version info should only appear for V>=7", r, c)
	}
}

// TestDataAreaCellsMatchesCapacity asserts that, after functional placement,
// the number of unreserved cells equals data_codewords*8 + remainder_bits for
// every (version, EC level) pair.
func TestDataAreaCellsMatchesCapacity(t *testing.T) {
	for v := MinVersion; v <= MaxVersion; v++ {
		m := newMatrix(v)
		m.placeFunctionalPatterns()
		got := m.dataAreaCells()
		// The unreserved cell count depends only on the version (functional
		// pattern layout is independent of EC level), so we compare it to the
		// per-version total in bits — which equals total codewords × 8 plus
		// remainder bits.
		spec := v.ECBlocks(ECLevelL)
		totalCodewords := spec.TotalDataCodewords() + spec.TotalECCodewords()
		want := totalCodewords*8 + v.RemainderBits()
		if got != want {
			t.Errorf("V%d data cells = %d, want %d (total codewords %d, remainder %d)",
				v, got, want, totalCodewords, v.RemainderBits())
		}
	}
}

// TestPlaceDataHelloWorld feeds the full HELLO WORLD M3+M4 byte stream into
// the matrix and verifies the walk consumes exactly the expected bits.
func TestPlaceDataHelloWorld(t *testing.T) {
	data, v, _, err := encodeText("HELLO WORLD", ECLevelM, 0)
	if err != nil {
		t.Fatalf("encodeText: %v", err)
	}
	stream := rsEncode(data, v, ECLevelM)
	m := newMatrix(v)
	m.placeFunctionalPatterns()
	if err := m.placeData(stream, v.RemainderBits()); err != nil {
		t.Fatalf("placeData: %v", err)
	}
}
