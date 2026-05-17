// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

// totalCodewordsPerVersion is the total (data + EC) codewords per version,
// derived from matrix geometry. Cross-check against docs/theory/09-data-tables.md §5.
// Values are independent of EC level — every (v, ec) pair must satisfy
// data(v, ec) + EC(v, ec) = total(v).
var totalCodewordsPerVersion = [40]int{
	26, 44, 70, 100, 134, 172, 196, 242, 292, 346,
	404, 466, 532, 581, 655, 733, 815, 901, 991, 1085,
	1156, 1258, 1364, 1474, 1588, 1706, 1828, 1921, 2051, 2185,
	2323, 2465, 2611, 2761, 2876, 3034, 3196, 3362, 3532, 3706,
}

func TestVersionSize(t *testing.T) {
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
		if got := c.v.Size(); got != c.want {
			t.Errorf("Version(%d).Size() = %d, want %d", c.v, got, c.want)
		}
	}
}

func TestVersionIsValid(t *testing.T) {
	cases := []struct {
		v    Version
		want bool
	}{
		{0, false},
		{1, true},
		{40, true},
		{41, false},
		{-1, false},
	}
	for _, c := range cases {
		if got := c.v.IsValid(); got != c.want {
			t.Errorf("Version(%d).IsValid() = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestDataCodewordsBoundaries(t *testing.T) {
	cases := []struct {
		v    Version
		ec   ECLevel
		want int
	}{
		{1, ECLevelL, 19},
		{1, ECLevelM, 16},
		{1, ECLevelQ, 13},
		{1, ECLevelH, 9},
		{40, ECLevelL, 2956},
		{40, ECLevelM, 2334},
		{40, ECLevelQ, 1666},
		{40, ECLevelH, 1276},
		{5, ECLevelQ, 62},
	}
	for _, c := range cases {
		if got := c.v.DataCodewords(c.ec); got != c.want {
			t.Errorf("V%d-%s DataCodewords = %d, want %d", c.v, c.ec, got, c.want)
		}
	}
}

// TestECBlockConsistency asserts that for every (version, EC level) the block
// structure adds up to the data-codeword count, and that (data + EC) equals
// the version's total codeword count.
func TestECBlockConsistency(t *testing.T) {
	for v := MinVersion; v <= MaxVersion; v++ {
		total := totalCodewordsPerVersion[v-1]
		for _, ec := range []ECLevel{ECLevelL, ECLevelM, ECLevelQ, ECLevelH} {
			spec := v.ECBlocks(ec)
			gotData := spec.TotalDataCodewords()
			wantData := v.DataCodewords(ec)
			if gotData != wantData {
				t.Errorf("V%d-%s: blocks sum to %d data codewords, want %d", v, ec, gotData, wantData)
			}
			gotEC := spec.TotalECCodewords()
			if gotData+gotEC != total {
				t.Errorf("V%d-%s: data %d + EC %d = %d, want total %d", v, ec, gotData, gotEC, gotData+gotEC, total)
			}
		}
	}
}

func TestRemainderBitsRanges(t *testing.T) {
	cases := []struct {
		v    Version
		want int
	}{
		{1, 0},
		{2, 7}, {6, 7},
		{7, 0}, {13, 0},
		{14, 3}, {20, 3},
		{21, 4}, {27, 4},
		{28, 3}, {34, 3},
		{35, 0}, {40, 0},
	}
	for _, c := range cases {
		if got := c.v.RemainderBits(); got != c.want {
			t.Errorf("V%d.RemainderBits() = %d, want %d", c.v, got, c.want)
		}
	}
}

func TestAlignmentCentersBoundaries(t *testing.T) {
	cases := []struct {
		v    Version
		want []int
	}{
		{1, nil},
		{2, []int{6, 18}},
		{7, []int{6, 22, 38}},
		{40, []int{6, 30, 58, 86, 114, 142, 170}},
	}
	for _, c := range cases {
		got := c.v.AlignmentCenters()
		if !equalIntSlice(got, c.want) {
			t.Errorf("V%d.AlignmentCenters() = %v, want %v", c.v, got, c.want)
		}
	}
}

// TestAlignmentCentersInBounds asserts that every coordinate is within the
// matrix bounds for its version.
func TestAlignmentCentersInBounds(t *testing.T) {
	for v := MinVersion; v <= MaxVersion; v++ {
		size := v.Size()
		for _, c := range v.AlignmentCenters() {
			if c < 0 || c >= size {
				t.Errorf("V%d: alignment centre %d out of bounds [0, %d)", v, c, size)
			}
		}
	}
}

func TestECLevelString(t *testing.T) {
	cases := []struct {
		ec   ECLevel
		want string
	}{
		{ECLevelL, "L"},
		{ECLevelM, "M"},
		{ECLevelQ, "Q"},
		{ECLevelH, "H"},
	}
	for _, c := range cases {
		if got := c.ec.String(); got != c.want {
			t.Errorf("ECLevel(%d).String() = %q, want %q", c.ec, got, c.want)
		}
	}
}

func TestECLevelFormatBits(t *testing.T) {
	cases := []struct {
		ec   ECLevel
		want uint8
	}{
		{ECLevelL, 0b01},
		{ECLevelM, 0b00},
		{ECLevelQ, 0b11},
		{ECLevelH, 0b10},
	}
	for _, c := range cases {
		if got := c.ec.formatBits(); got != c.want {
			t.Errorf("ECLevel(%s).formatBits() = %b, want %b", c.ec, got, c.want)
		}
	}
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
