// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

func TestFormatInfoKnownValues(t *testing.T) {
	cases := []struct {
		ec   ECLevel
		mask int
		want uint16
	}{
		// (M, mask 3) is the value used in docs/theory/10-worked-example.md.
		{ECLevelM, 3, 0x5B4B},
		// Corners of the table.
		{ECLevelL, 0, 0x77C4},
		{ECLevelL, 7, 0x6976},
		{ECLevelH, 0, 0x1689},
		{ECLevelH, 7, 0x083B},
	}
	for _, c := range cases {
		got := formatInfo(c.ec, c.mask)
		if got != c.want {
			t.Errorf("formatInfo(%s, mask=%d) = 0x%04X, want 0x%04X", c.ec, c.mask, got, c.want)
		}
	}
}

// TestFormatInfoAllFitIn15Bits asserts every codeword stays within 15 bits.
func TestFormatInfoAllFitIn15Bits(t *testing.T) {
	for ec := ECLevelL; ec <= ECLevelH; ec++ {
		for mask := 0; mask <= 7; mask++ {
			got := formatInfo(ec, mask)
			if got > 0x7FFF {
				t.Errorf("formatInfo(%s, %d) = 0x%X exceeds 15 bits", ec, mask, got)
			}
		}
	}
}

func TestFormatInfoOutOfRange(t *testing.T) {
	if got := formatInfo(ECLevelL, -1); got != 0 {
		t.Errorf("formatInfo(L, -1) = 0x%X, want 0", got)
	}
	if got := formatInfo(ECLevelL, 8); got != 0 {
		t.Errorf("formatInfo(L, 8) = 0x%X, want 0", got)
	}
}

func TestVersionInfoKnownValues(t *testing.T) {
	cases := []struct {
		v    Version
		want uint32
	}{
		{7, 0x07C94},
		{8, 0x085BC},
		{9, 0x09A99},
		{20, 0x149A6},
		{40, 0x28C69},
	}
	for _, c := range cases {
		got := versionInfo(c.v)
		if got != c.want {
			t.Errorf("versionInfo(V%d) = 0x%05X, want 0x%05X", c.v, got, c.want)
		}
	}
}

func TestVersionInfoBelowSeven(t *testing.T) {
	for v := MinVersion; v < 7; v++ {
		if got := versionInfo(v); got != 0 {
			t.Errorf("versionInfo(V%d) = 0x%X, want 0 (no version info for V<7)", v, got)
		}
	}
}

// TestVersionInfoAllFitIn18Bits asserts every codeword stays within 18 bits.
func TestVersionInfoAllFitIn18Bits(t *testing.T) {
	for v := Version(7); v <= MaxVersion; v++ {
		got := versionInfo(v)
		if got > 0x3FFFF {
			t.Errorf("versionInfo(V%d) = 0x%X exceeds 18 bits", v, got)
		}
	}
}
