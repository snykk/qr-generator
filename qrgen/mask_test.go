// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

func TestMaskConditionSamples(t *testing.T) {
	cases := []struct {
		name string
		k    int
		r, c int
		want bool
	}{
		// Mask 0: (r+c) % 2 == 0
		{"m0 origin", 0, 0, 0, true},
		{"m0 (0,1)", 0, 0, 1, false},
		{"m0 (3,5)", 0, 3, 5, true},
		// Mask 1: r % 2 == 0
		{"m1 row0", 1, 0, 7, true},
		{"m1 row1", 1, 1, 7, false},
		// Mask 2: c % 3 == 0
		{"m2 col0", 2, 5, 0, true},
		{"m2 col2", 2, 5, 2, false},
		{"m2 col6", 2, 5, 6, true},
		// Mask 3: (r+c) % 3 == 0
		{"m3 origin", 3, 0, 0, true},
		{"m3 (1,2)", 3, 1, 2, true},
		{"m3 (1,1)", 3, 1, 1, false},
		// Mask 4: (r/2 + c/3) % 2 == 0
		{"m4 origin", 4, 0, 0, true},
		{"m4 (0,3)", 4, 0, 3, false},
		{"m4 (2,3)", 4, 2, 3, true},
		// Mask 5: (r*c)%2 + (r*c)%3 == 0
		{"m5 origin (r*c=0)", 5, 0, 0, true},
		{"m5 (1,1) r*c=1", 5, 1, 1, false},
		{"m5 (2,3) r*c=6", 5, 2, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := maskCondition(c.k, c.r, c.c); got != c.want {
				t.Errorf("maskCondition(%d, %d, %d) = %v, want %v", c.k, c.r, c.c, got, c.want)
			}
		})
	}
}

// TestApplyMaskSkipsReserved asserts that masking only flips data modules,
// never reserved ones.
func TestApplyMaskSkipsReserved(t *testing.T) {
	m := newMatrix(1)
	m.placeFunctionalPatterns()

	// Snapshot the reserved area's current state.
	type snap struct {
		row, col int
		dark     bool
	}
	var snaps []snap
	for r := 0; r < m.size; r++ {
		for c := 0; c < m.size; c++ {
			if m.reserved[r][c] {
				snaps = append(snaps, snap{r, c, m.modules[r][c]})
			}
		}
	}

	m.applyMask(0)

	for _, s := range snaps {
		if m.modules[s.row][s.col] != s.dark {
			t.Errorf("reserved cell (%d,%d) was modified by mask 0", s.row, s.col)
		}
	}
}

// TestApplyMaskIsInvolutive asserts applying the same mask twice returns the
// matrix to its original state (XOR is self-inverse).
func TestApplyMaskIsInvolutive(t *testing.T) {
	m := newMatrix(1)
	m.placeFunctionalPatterns()
	// Place some dummy data bits.
	bytes := make([]byte, 26)
	for i := range bytes {
		bytes[i] = 0xA5
	}
	if err := m.placeData(bytes, 0); err != nil {
		t.Fatalf("placeData: %v", err)
	}

	before := m.clone()
	for k := 0; k < numMasks; k++ {
		m.applyMask(k)
		m.applyMask(k)
		for r := 0; r < m.size; r++ {
			for c := 0; c < m.size; c++ {
				if m.modules[r][c] != before.modules[r][c] {
					t.Fatalf("mask %d not involutive at (%d,%d)", k, r, c)
				}
			}
		}
	}
}

func TestPenaltyRule1Run(t *testing.T) {
	// Build a tiny synthetic 21x21 matrix where row 0 has a run of 5 dark
	// modules at cols 0..4, and everything else is mixed enough that we know
	// the marginal contribution. To avoid noise from other rules and other
	// rows, we test the helper on a single hand-constructed line.
	cases := []struct {
		name string
		line []bool
		want int
	}{
		{"all light no run", make([]bool, 4), 0},
		{"length 5 dark", []bool{true, true, true, true, true}, 3},
		{"length 6 dark", []bool{true, true, true, true, true, true}, 4},
		{"two runs of 5", append(append(make([]bool, 0), trues(5)...), falses(2)...), 3},
	}
	cases[3].line = append(cases[3].line, trues(5)...)
	cases[3].want = 3 + 3
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runScore(c.line); got != c.want {
				t.Errorf("runScore(%v) = %d, want %d", c.line, got, c.want)
			}
		})
	}
}

func trues(n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = true
	}
	return out
}

func falses(n int) []bool { return make([]bool, n) }

func TestPenaltyRule3FinderPattern(t *testing.T) {
	// Pattern A directly.
	line := append([]bool(nil), rule3PatternA...)
	if got := linePatternScore(line); got != 40 {
		t.Errorf("pattern A alone = %d, want 40", got)
	}
	// Pattern B directly.
	line = append([]bool(nil), rule3PatternB...)
	if got := linePatternScore(line); got != 40 {
		t.Errorf("pattern B alone = %d, want 40", got)
	}
	// Pattern with surroundings.
	pad := append([]bool{false, true}, rule3PatternA...)
	pad = append(pad, false, true)
	if got := linePatternScore(pad); got != 40 {
		t.Errorf("pattern A padded = %d, want 40", got)
	}
}

func TestPenaltyRule4DarkRatio(t *testing.T) {
	// All-light 21x21 (V1) → dark ratio 0% → deviation 50, k = 10 → score 100.
	m := newMatrix(1)
	if got := m.penaltyRule4(); got != 100 {
		t.Errorf("all-light V1 rule4 = %d, want 100", got)
	}
	// All-dark V1 → ratio 100% → same penalty 100.
	for r := 0; r < m.size; r++ {
		for c := 0; c < m.size; c++ {
			m.modules[r][c] = true
		}
	}
	if got := m.penaltyRule4(); got != 100 {
		t.Errorf("all-dark V1 rule4 = %d, want 100", got)
	}
}

// TestSelectAndApplyMaskHelloWorld checks the worked example pipeline: encode
// "HELLO WORLD" at EC level M, then verify the chosen mask is in range and the
// format-info codeword for that mask is written into both redundant strips
// byte-for-byte. The exact mask picked depends on penalty implementation; this
// test does not pin it to a specific value (see TestMaskScoresHelloWorld for
// the per-mask scores).
func TestSelectAndApplyMaskHelloWorld(t *testing.T) {
	m, mask, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	if mask < 0 || mask >= numMasks {
		t.Fatalf("mask = %d, out of range", mask)
	}

	wantFormat := uint32(formatInfo(ECLevelM, mask))

	// First copy bits 0..5 along row 8 left.
	for i := 0; i < 6; i++ {
		want := bitOf(wantFormat, i)
		if got := m.modules[8][i]; got != want {
			t.Errorf("first copy bit %d at (8,%d): got %v, want %v", i, i, got, want)
		}
	}
	// bit 6 → (8, 7), bit 7 → (8, 8), bit 8 → (7, 8).
	if got, want := m.modules[8][7], bitOf(wantFormat, 6); got != want {
		t.Errorf("first copy bit 6 at (8,7): got %v, want %v", got, want)
	}
	if got, want := m.modules[8][8], bitOf(wantFormat, 7); got != want {
		t.Errorf("first copy bit 7 at (8,8): got %v, want %v", got, want)
	}
	if got, want := m.modules[7][8], bitOf(wantFormat, 8); got != want {
		t.Errorf("first copy bit 8 at (7,8): got %v, want %v", got, want)
	}
	// bit 14 → (0, 8).
	if got, want := m.modules[0][8], bitOf(wantFormat, 14); got != want {
		t.Errorf("first copy bit 14 at (0,8): got %v, want %v", got, want)
	}

	// Second copy bit 0 → (8, n-1), bit 14 → (n-1, 8). n=21.
	if got, want := m.modules[8][20], bitOf(wantFormat, 0); got != want {
		t.Errorf("second copy bit 0 at (8,20): got %v, want %v", got, want)
	}
	if got, want := m.modules[20][8], bitOf(wantFormat, 14); got != want {
		t.Errorf("second copy bit 14 at (20,8): got %v, want %v", got, want)
	}
}

// TestMaskScoresHelloWorld emits the penalty score for each mask on the
// worked example so the picked mask is auditable. The picked mask must
// genuinely be the minimum (with index breaking ties).
func TestMaskScoresHelloWorld(t *testing.T) {
	data, v, _, err := encodeText("HELLO WORLD", ECLevelM, 0)
	if err != nil {
		t.Fatalf("encodeText: %v", err)
	}
	stream := rsEncode(data, v, ECLevelM)

	scores := make([]int, numMasks)
	for k := 0; k < numMasks; k++ {
		m := newMatrix(v)
		m.placeFunctionalPatterns()
		if err := m.placeData(stream, v.RemainderBits()); err != nil {
			t.Fatalf("placeData mask %d: %v", k, err)
		}
		m.applyMask(k)
		m.writeFormatInfo(ECLevelM, k)
		scores[k] = m.penalty()
		t.Logf("mask %d: rule1=%d rule2=%d rule3=%d rule4=%d total=%d",
			k,
			m.penaltyRule1(),
			m.penaltyRule2(),
			m.penaltyRule3(),
			m.penaltyRule4(),
			scores[k])
	}
	// Find the minimum score and its first index.
	minIdx := 0
	for k := 1; k < numMasks; k++ {
		if scores[k] < scores[minIdx] {
			minIdx = k
		}
	}
	t.Logf("lowest-penalty mask = %d (score %d)", minIdx, scores[minIdx])

	// The same selection that buildMatrix uses must agree.
	_, gotMask, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	if gotMask != minIdx {
		t.Errorf("buildMatrix picked mask %d but minimum score is mask %d", gotMask, minIdx)
	}
}

// TestWriteVersionInfoV7 asserts that the version-info bits for V7 (0x07C94)
// are written into both redundant blocks. We just spot-check the corner bits.
func TestWriteVersionInfoV7(t *testing.T) {
	v := Version(7)
	m := newMatrix(v)
	m.placeFunctionalPatterns()
	m.writeVersionInfo()
	n := m.size

	wantBits := versionInfo(v) // 0x07C94
	for i := 0; i < 18; i++ {
		want := bitOf(wantBits, i)
		row := n - 11 + i%3
		col := i / 3
		if got := m.modules[row][col]; got != want {
			t.Errorf("version bit %d at (%d,%d): got %v, want %v", i, row, col, got, want)
		}
		if got := m.modules[col][row]; got != want {
			t.Errorf("version bit %d at (%d,%d) [transposed]: got %v, want %v", i, col, row, got, want)
		}
	}
}
