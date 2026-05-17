// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// matrix is the QR module grid plus a parallel "reserved" mask that marks
// functional patterns and the format/version-info areas so the zig-zag data
// walk skips them. Coordinates use (row, col) with (0, 0) at the top-left,
// matching docs/theory/README.md.
type matrix struct {
	size     int
	version  Version
	modules  [][]bool // dark/light state; true == dark module
	reserved [][]bool // true if the module is functional/reserved
}

// newMatrix returns an all-light, all-unreserved matrix sized for v.
func newMatrix(v Version) *matrix {
	n := v.Size()
	m := &matrix{
		size:     n,
		version:  v,
		modules:  make([][]bool, n),
		reserved: make([][]bool, n),
	}
	for i := 0; i < n; i++ {
		m.modules[i] = make([]bool, n)
		m.reserved[i] = make([]bool, n)
	}
	return m
}

func (m *matrix) get(row, col int) bool        { return m.modules[row][col] }
func (m *matrix) set(row, col int, dark bool)  { m.modules[row][col] = dark }
func (m *matrix) isReserved(row, col int) bool { return m.reserved[row][col] }
func (m *matrix) reserve(row, col int)         { m.reserved[row][col] = true }

// placeFunctionalPatterns writes finders + separators, timing strips, alignment
// patterns, the always-dark module, and marks the format/version-info areas as
// reserved. See docs/theory/05-matrix-construction.md.
func (m *matrix) placeFunctionalPatterns() {
	m.placeFinderPatterns()
	m.placeTimingPatterns()
	m.placeAlignmentPatterns()
	m.placeDarkModule()
	m.reserveFormatInfoArea()
	m.reserveVersionInfoArea()
}

// placeFinderPatterns places the three 7×7 finder patterns plus their
// one-module-wide light separators. The separator on the edge of the matrix
// is not stored (it sits in the quiet zone outside the symbol).
func (m *matrix) placeFinderPatterns() {
	n := m.size
	m.placeSingleFinder(0, 0)
	m.placeSingleFinder(0, n-7)
	m.placeSingleFinder(n-7, 0)
}

// placeSingleFinder draws a 7×7 finder anchored at (r0, c0) and reserves the
// 8×8 region including the inward separator strip. Cells outside the matrix
// are silently skipped.
func (m *matrix) placeSingleFinder(r0, c0 int) {
	for dr := -1; dr <= 7; dr++ {
		for dc := -1; dc <= 7; dc++ {
			r, c := r0+dr, c0+dc
			if r < 0 || r >= m.size || c < 0 || c >= m.size {
				continue
			}
			inFinder := dr >= 0 && dr <= 6 && dc >= 0 && dc <= 6
			if !inFinder {
				// Separator: light + reserved.
				m.set(r, c, false)
				m.reserve(r, c)
				continue
			}
			isBorder := dr == 0 || dr == 6 || dc == 0 || dc == 6
			isCenter := dr >= 2 && dr <= 4 && dc >= 2 && dc <= 4
			m.set(r, c, isBorder || isCenter)
			m.reserve(r, c)
		}
	}
}

// placeTimingPatterns writes alternating dark/light modules (starting dark) on
// row 6 and column 6, between the finder patterns. Cells already reserved by
// the finders are left alone.
func (m *matrix) placeTimingPatterns() {
	n := m.size
	for i := 8; i < n-8; i++ {
		dark := i%2 == 0
		// Row 6
		m.set(6, i, dark)
		m.reserve(6, i)
		// Column 6
		m.set(i, 6, dark)
		m.reserve(i, 6)
	}
}

// placeAlignmentPatterns draws every alignment pattern whose (rowCentre,
// colCentre) pair comes from v.AlignmentCenters() and does not collide with a
// finder pattern. Per the spec, three pairs of centres fall inside the finder
// regions (the corner triple) and are explicitly skipped.
func (m *matrix) placeAlignmentPatterns() {
	centers := m.version.AlignmentCenters()
	if len(centers) == 0 {
		return
	}
	first := centers[0]
	last := centers[len(centers)-1]
	for _, r := range centers {
		for _, c := range centers {
			// Skip the three centres that would land on a finder pattern.
			if (r == first && c == first) ||
				(r == first && c == last) ||
				(r == last && c == first) {
				continue
			}
			m.placeSingleAlignment(r, c)
		}
	}
}

// placeSingleAlignment draws a 5×5 concentric alignment pattern centred at
// (rc, cc) and reserves all 25 cells. Per the spec, alignment patterns may
// overwrite the timing patterns they happen to cross.
func (m *matrix) placeSingleAlignment(rc, cc int) {
	for dr := -2; dr <= 2; dr++ {
		for dc := -2; dc <= 2; dc++ {
			r, c := rc+dr, cc+dc
			isOuter := dr == -2 || dr == 2 || dc == -2 || dc == 2
			isCenter := dr == 0 && dc == 0
			m.set(r, c, isOuter || isCenter)
			m.reserve(r, c)
		}
	}
}

// placeDarkModule sets the single always-dark module at (4·v + 9, 8) for the
// matrix's version. It sits just above the bottom-left format-info column.
func (m *matrix) placeDarkModule() {
	r := 4*int(m.version) + 9
	c := 8
	m.set(r, c, true)
	m.reserve(r, c)
}

// reserveFormatInfoArea reserves the 15-bit format-info modules in both
// redundant locations so the data walk skips them. Their final values are
// written later, after masking, by the format-info stage.
func (m *matrix) reserveFormatInfoArea() {
	n := m.size
	// Around the top-left finder: row 8 cols 0..8 plus column 8 rows 0..8.
	for i := 0; i <= 8; i++ {
		m.reserve(8, i)
		m.reserve(i, 8)
	}
	// Right side: row 8, cols n-8..n-1.
	for i := n - 8; i < n; i++ {
		m.reserve(8, i)
	}
	// Bottom side: column 8, rows n-7..n-1.
	for i := n - 7; i < n; i++ {
		m.reserve(i, 8)
	}
}

// reserveVersionInfoArea reserves the two 6×3 / 3×6 version-info blocks for
// version 7 and above. No-op for smaller versions.
func (m *matrix) reserveVersionInfoArea() {
	if m.version < 7 {
		return
	}
	n := m.size
	// 6×3 block above the bottom-left finder.
	for r := 0; r < 6; r++ {
		for c := n - 11; c <= n-9; c++ {
			m.reserve(r, c)
		}
	}
	// 3×6 block to the left of the top-right finder.
	for r := n - 11; r <= n-9; r++ {
		for c := 0; c < 6; c++ {
			m.reserve(r, c)
		}
	}
}

// placeData runs the zig-zag walk described in docs/theory/05-matrix-construction.md.
// It walks two-column-wide bands starting at the right edge, alternating upward
// and downward, skipping reserved modules and the entire timing column 6. The
// first call after placeFunctionalPatterns must supply a stream of exactly
// dataAreaBits == v.DataCodewords(ec) * 8 + v.RemainderBits(); short streams
// return an error. The MSB of each byte is written first.
func (m *matrix) placeData(stream []byte, remainderBits int) error {
	n := m.size
	totalBits := len(stream)*8 + remainderBits
	bitIdx := 0
	upward := true

	for col := n - 1; col > 0; col -= 2 {
		if col == 6 {
			col--
		}
		for i := 0; i < n; i++ {
			row := i
			if upward {
				row = n - 1 - i
			}
			for c := 0; c < 2; c++ {
				cc := col - c
				if m.isReserved(row, cc) {
					continue
				}
				if bitIdx >= totalBits {
					return fmt.Errorf("qrgen: data walk found more unreserved cells than bits available (extra cell at row=%d col=%d)", row, cc)
				}
				m.set(row, cc, bitAt(stream, bitIdx))
				bitIdx++
			}
		}
		upward = !upward
	}

	if bitIdx != totalBits {
		return fmt.Errorf("qrgen: data walk wrote %d bits, expected %d", bitIdx, totalBits)
	}
	return nil
}

// bitAt returns bit idx from stream, MSB first within each byte. Indices past
// the stream's last byte return false (used for the trailing remainder bits
// that some versions need).
func bitAt(stream []byte, idx int) bool {
	if idx >= len(stream)*8 {
		return false
	}
	return (stream[idx>>3]>>uint(7-(idx&7)))&1 == 1
}

// dataAreaCells returns the number of unreserved cells in a freshly-prepared
// matrix (i.e. after placeFunctionalPatterns but before placeData). It equals
// v.DataCodewords(ec)*8 + v.RemainderBits() for every (v, ec); this function
// is mostly useful as a self-check in tests.
func (m *matrix) dataAreaCells() int {
	count := 0
	for r := 0; r < m.size; r++ {
		for c := 0; c < m.size; c++ {
			if !m.reserved[r][c] {
				count++
			}
		}
	}
	return count
}
