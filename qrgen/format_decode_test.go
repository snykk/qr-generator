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
	"testing"
)

// copy1BitPositions maps format-info bit index (0 = LSB ... 14 = MSB) to the
// (row, col) of that bit in the first redundant strip (around the top-left
// finder), mirroring writeFormatInfo in matrix.go. Used by the corruption
// tests to flip specific bits in a controlled way.
var copy1BitPositions = [15]struct{ row, col int }{
	{8, 0}, {8, 1}, {8, 2}, {8, 3}, {8, 4}, {8, 5}, // bits 0..5
	{8, 7}, {8, 8}, {7, 8}, // bits 6..8 (col 6 and row 6 are timing, skipped)
	{5, 8}, {4, 8}, {3, 8}, {2, 8}, {1, 8}, {0, 8}, // bits 9..14
}

// copy2BitPositionsV1 mirrors copy1BitPositions for the second redundant
// strip when the matrix is V1 (n = 21).
var copy2BitPositionsV1 = [15]struct{ row, col int }{
	// bits 0..7 along the right side of row 8: cols n-1 down to n-8.
	{8, 20}, {8, 19}, {8, 18}, {8, 17}, {8, 16}, {8, 15}, {8, 14}, {8, 13},
	// bits 8..14 down column 8 of the bottom-left finder: rows n-7 to n-1.
	{14, 8}, {15, 8}, {16, 8}, {17, 8}, {18, 8}, {19, 8}, {20, 8},
}

// TestReadFormatInfoRoundTripAllPairs exercises every (EC level, mask) pair
// against the encoder-decoder pair. With no corruption, the recovered values
// must always match the input.
func TestReadFormatInfoRoundTripAllPairs(t *testing.T) {
	for ec := ECLevelL; ec <= ECLevelH; ec++ {
		for mask := 0; mask < numMasks; mask++ {
			m := newMatrix(1)
			m.placeFunctionalPatterns()
			m.writeFormatInfo(ec, mask)
			gotEC, gotMask, err := readFormatInfo(m)
			if err != nil {
				t.Errorf("ec=%s mask=%d: %v", ec, mask, err)
				continue
			}
			if gotEC != ec || gotMask != mask {
				t.Errorf("got (ec=%s, mask=%d), want (ec=%s, mask=%d)",
					gotEC, gotMask, ec, mask)
			}
		}
	}
}

func TestReadFormatInfoWithinCapacity(t *testing.T) {
	cases := []struct {
		name    string
		copy1   []int // bit indices to flip in copy 1
		copy2   []int // bit indices to flip in copy 2
	}{
		{"clean", nil, nil},
		{"1 flip copy1", []int{0}, nil},
		{"2 flips copy1", []int{0, 7}, nil},
		{"3 flips copy1 (per-copy limit)", []int{0, 7, 14}, nil},
		{"3 flips copy2", nil, []int{1, 8, 13}},
		{"3 flips each (combined limit)", []int{0, 5, 10}, []int{2, 7, 12}},
		{"6 flips on copy1 only (still inside combined budget)", []int{0, 2, 4, 6, 8, 10}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newMatrix(1)
			m.placeFunctionalPatterns()
			m.writeFormatInfo(ECLevelM, 3)
			for _, idx := range c.copy1 {
				p := copy1BitPositions[idx]
				m.modules[p.row][p.col] = !m.modules[p.row][p.col]
			}
			for _, idx := range c.copy2 {
				p := copy2BitPositionsV1[idx]
				m.modules[p.row][p.col] = !m.modules[p.row][p.col]
			}
			gotEC, gotMask, err := readFormatInfo(m)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if gotEC != ECLevelM || gotMask != 3 {
				t.Errorf("got (ec=%s, mask=%d), want (M, 3)", gotEC, gotMask)
			}
		})
	}
}

func TestReadFormatInfoExceedsCapacity(t *testing.T) {
	// Flip 8 bits in copy 1 and 8 bits in copy 2 = 16 total flips, well past
	// the combined budget of 6. The decoder must return ErrFormatUnreadable.
	m := newMatrix(1)
	m.placeFunctionalPatterns()
	m.writeFormatInfo(ECLevelM, 3)
	for idx := 0; idx < 8; idx++ {
		p1 := copy1BitPositions[idx]
		p2 := copy2BitPositionsV1[idx]
		m.modules[p1.row][p1.col] = !m.modules[p1.row][p1.col]
		m.modules[p2.row][p2.col] = !m.modules[p2.row][p2.col]
	}
	_, _, err := readFormatInfo(m)
	if !errors.Is(err, ErrFormatUnreadable) {
		t.Errorf("expected ErrFormatUnreadable, got %v", err)
	}
}

// TestReadFormatInfoPrefersClosestMatch checks that when one copy is heavily
// corrupted but the other is clean, the decoder still recovers the original.
func TestReadFormatInfoPrefersClosestMatch(t *testing.T) {
	m := newMatrix(1)
	m.placeFunctionalPatterns()
	m.writeFormatInfo(ECLevelQ, 5)
	// Trash 5 bits in copy 1; keep copy 2 clean. Combined Hamming budget is
	// 5 ≤ 6 so the decoder should still recover.
	for _, idx := range []int{0, 3, 6, 9, 12} {
		p := copy1BitPositions[idx]
		m.modules[p.row][p.col] = !m.modules[p.row][p.col]
	}
	gotEC, gotMask, err := readFormatInfo(m)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotEC != ECLevelQ || gotMask != 5 {
		t.Errorf("got (ec=%s, mask=%d), want (Q, 5)", gotEC, gotMask)
	}
}
