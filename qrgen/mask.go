// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// numMasks is the count of QR mask patterns defined by ISO/IEC 18004.
const numMasks = 8

// maskCondition returns true when mask k requires the module at (row, col) to
// be inverted. The eight functions come straight from docs/theory/06-masking.md.
func maskCondition(k, row, col int) bool {
	switch k {
	case 0:
		return (row+col)%2 == 0
	case 1:
		return row%2 == 0
	case 2:
		return col%3 == 0
	case 3:
		return (row+col)%3 == 0
	case 4:
		return (row/2+col/3)%2 == 0
	case 5:
		return (row*col)%2+(row*col)%3 == 0
	case 6:
		return ((row*col)%2+(row*col)%3)%2 == 0
	case 7:
		return ((row+col)%2+(row*col)%3)%2 == 0
	}
	panic(fmt.Sprintf("qrgen: invalid mask index %d", k))
}

// applyMask toggles every data module where maskCondition(k, ...) returns
// true. Reserved modules (functional patterns, format-info, version-info) are
// left untouched. Calling applyMask twice with the same k restores the matrix
// because XOR is its own inverse.
func (m *matrix) applyMask(k int) {
	for r := 0; r < m.size; r++ {
		for c := 0; c < m.size; c++ {
			if m.reserved[r][c] {
				continue
			}
			if maskCondition(k, r, c) {
				m.modules[r][c] = !m.modules[r][c]
			}
		}
	}
}

// penalty returns the sum of the four mask-penalty rules per ISO/IEC 18004:2015
// §8.8.2. The score is computed over every module, including format-info and
// version-info bits, because those participate in the visual patterns the
// rules try to suppress.
func (m *matrix) penalty() int {
	return m.penaltyRule1() + m.penaltyRule2() + m.penaltyRule3() + m.penaltyRule4()
}

// penaltyRule1 charges (L-2) for each run of L >= 5 same-colour modules in
// every row and every column.
func (m *matrix) penaltyRule1() int {
	score := 0
	n := m.size
	for r := 0; r < n; r++ {
		score += runScore(m.modules[r])
	}
	col := make([]bool, n)
	for c := 0; c < n; c++ {
		for r := 0; r < n; r++ {
			col[r] = m.modules[r][c]
		}
		score += runScore(col)
	}
	return score
}

func runScore(line []bool) int {
	score := 0
	runColor := line[0]
	runLen := 1
	for i := 1; i < len(line); i++ {
		if line[i] == runColor {
			runLen++
		} else {
			if runLen >= 5 {
				score += runLen - 2
			}
			runColor = line[i]
			runLen = 1
		}
	}
	if runLen >= 5 {
		score += runLen - 2
	}
	return score
}

// penaltyRule2 charges 3 for each 2x2 same-colour block. Overlapping blocks
// contribute independently — a 3x3 all-dark region holds four 2x2 sub-blocks,
// so 12 points.
func (m *matrix) penaltyRule2() int {
	score := 0
	n := m.size
	for r := 0; r < n-1; r++ {
		for c := 0; c < n-1; c++ {
			v := m.modules[r][c]
			if v == m.modules[r][c+1] && v == m.modules[r+1][c] && v == m.modules[r+1][c+1] {
				score += 3
			}
		}
	}
	return score
}

// rule3PatternA is the 11-module finder-like sequence 10111010000.
// rule3PatternB is its reverse 00001011101. Each occurrence in any row or
// column scores 40 penalty points.
var (
	rule3PatternA = []bool{true, false, true, true, true, false, true, false, false, false, false}
	rule3PatternB = []bool{false, false, false, false, true, false, true, true, true, false, true}
)

func (m *matrix) penaltyRule3() int {
	score := 0
	n := m.size
	for r := 0; r < n; r++ {
		score += linePatternScore(m.modules[r])
	}
	col := make([]bool, n)
	for c := 0; c < n; c++ {
		for r := 0; r < n; r++ {
			col[r] = m.modules[r][c]
		}
		score += linePatternScore(col)
	}
	return score
}

func linePatternScore(line []bool) int {
	score := 0
	for i := 0; i <= len(line)-11; i++ {
		if matchSlice(line, i, rule3PatternA) || matchSlice(line, i, rule3PatternB) {
			score += 40
		}
	}
	return score
}

func matchSlice(line []bool, start int, pattern []bool) bool {
	for i, v := range pattern {
		if line[start+i] != v {
			return false
		}
	}
	return true
}

// penaltyRule4 charges 10 * floor(|darkPercent - 50| / 5) where darkPercent is
// the fraction of dark modules across the entire symbol.
func (m *matrix) penaltyRule4() int {
	n := m.size
	dark := 0
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if m.modules[r][c] {
				dark++
			}
		}
	}
	total := n * n
	// Integer arithmetic: compute |dark*100 - 50*total| / (5*total)
	deviation := dark*100 - 50*total
	if deviation < 0 {
		deviation = -deviation
	}
	return (deviation / (5 * total)) * 10
}

// selectAndApplyMask tries every mask in turn (cloning m for each trial so the
// best-mask search is non-destructive), picks the one with the lowest penalty
// score, then applies that mask and writes the matching format-info bits to
// the original matrix. Returns the picked mask index.
func (m *matrix) selectAndApplyMask(ec ECLevel) int {
	bestMask := 0
	bestPenalty := -1
	for k := 0; k < numMasks; k++ {
		trial := m.clone()
		trial.applyMask(k)
		trial.writeFormatInfo(ec, k)
		p := trial.penalty()
		if bestPenalty == -1 || p < bestPenalty {
			bestPenalty = p
			bestMask = k
		}
	}
	m.applyMask(bestMask)
	m.writeFormatInfo(ec, bestMask)
	return bestMask
}
