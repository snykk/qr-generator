// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

func TestModeIndicator(t *testing.T) {
	cases := []struct {
		m    Mode
		want uint8
	}{
		{ModeNumeric, 0b0001},
		{ModeAlphanumeric, 0b0010},
		{ModeByte, 0b0100},
	}
	for _, c := range cases {
		if got := c.m.Indicator(); got != c.want {
			t.Errorf("Mode(%s).Indicator() = %04b, want %04b", c.m, got, c.want)
		}
	}
}

func TestCharCountBits(t *testing.T) {
	cases := []struct {
		m    Mode
		v    Version
		want int
	}{
		// V1-9 range
		{ModeNumeric, 1, 10},
		{ModeAlphanumeric, 1, 9},
		{ModeByte, 1, 8},
		{ModeNumeric, 9, 10},
		// V10-26 range
		{ModeNumeric, 10, 12},
		{ModeAlphanumeric, 10, 11},
		{ModeByte, 10, 16},
		{ModeNumeric, 26, 12},
		// V27-40 range
		{ModeNumeric, 27, 14},
		{ModeAlphanumeric, 27, 13},
		{ModeByte, 27, 16},
		{ModeNumeric, 40, 14},
	}
	for _, c := range cases {
		if got := c.m.CharCountBits(c.v); got != c.want {
			t.Errorf("Mode(%s).CharCountBits(V%d) = %d, want %d", c.m, c.v, got, c.want)
		}
	}
}

func TestCharCountBitsInvalidVersion(t *testing.T) {
	if got := ModeNumeric.CharCountBits(0); got != 0 {
		t.Errorf("CharCountBits(0) = %d, want 0 (invalid version)", got)
	}
	if got := ModeNumeric.CharCountBits(41); got != 0 {
		t.Errorf("CharCountBits(41) = %d, want 0 (invalid version)", got)
	}
}

func TestModeString(t *testing.T) {
	cases := []struct {
		m    Mode
		want string
	}{
		{ModeNumeric, "Numeric"},
		{ModeAlphanumeric, "Alphanumeric"},
		{ModeByte, "Byte"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("Mode(%d).String() = %q, want %q", c.m, got, c.want)
		}
	}
}
