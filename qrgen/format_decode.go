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
	"math/bits"
)

// ErrFormatUnreadable is returned when both redundant copies of the format-
// information codeword are too corrupted for the BCH decoder to identify a
// valid (EC level, mask) pair.
var ErrFormatUnreadable = errors.New("qrgen: format-information codeword unreadable")

// formatCombinedBudget is the maximum Hamming distance, summed across the two
// redundant format-info copies, that we accept before declaring the format
// strip unreadable. The BCH(15, 5) code corrects up to 3 errors per copy, so
// six is the joint capacity when both copies independently stay within budget.
const formatCombinedBudget = 6

// readFormatInfo extracts the 15-bit format-information codeword from both
// redundant strips in the matrix, BCH-decodes them by brute force over the
// 32 precomputed valid codewords from M2, and returns the recovered (EC
// level, mask). Bit 0 (LSB) is read first, matching the encoder's layout in
// writeFormatInfo.
//
// See docs/theory/07-format-version-info.md and docs/theory/13-decoder-pipeline.md.
func readFormatInfo(m *matrix) (ECLevel, int, error) {
	n := m.size

	// First copy around the top-left finder.
	var code1 uint32
	for i := 0; i < 6; i++ {
		if m.modules[8][i] {
			code1 |= 1 << i
		}
	}
	if m.modules[8][7] {
		code1 |= 1 << 6
	}
	if m.modules[8][8] {
		code1 |= 1 << 7
	}
	if m.modules[7][8] {
		code1 |= 1 << 8
	}
	for i := 9; i < 15; i++ {
		if m.modules[14-i][8] {
			code1 |= 1 << i
		}
	}

	// Second copy: bits 0..7 along the right side of row 8, bits 8..14 down
	// column 8 of the bottom-left finder.
	var code2 uint32
	for i := 0; i < 8; i++ {
		if m.modules[8][n-1-i] {
			code2 |= 1 << i
		}
	}
	for i := 8; i < 15; i++ {
		if m.modules[n-15+i][8] {
			code2 |= 1 << i
		}
	}

	// Brute-force the 32 valid codewords for the minimum combined Hamming
	// distance. Ties break by EC level then mask, which matches the natural
	// table-iteration order.
	const fifteenBits uint32 = (1 << 15) - 1
	bestSum := -1
	bestEC := ECLevelL
	bestMask := 0
	for ec := ECLevelL; ec <= ECLevelH; ec++ {
		for mask := 0; mask < numMasks; mask++ {
			valid := uint32(formatInfo(ec, mask)) & fifteenBits
			d1 := bits.OnesCount32((code1 ^ valid) & fifteenBits)
			d2 := bits.OnesCount32((code2 ^ valid) & fifteenBits)
			sum := d1 + d2
			if bestSum < 0 || sum < bestSum {
				bestSum = sum
				bestEC = ec
				bestMask = mask
			}
		}
	}

	if bestSum > formatCombinedBudget {
		return 0, 0, ErrFormatUnreadable
	}
	return bestEC, bestMask, nil
}
