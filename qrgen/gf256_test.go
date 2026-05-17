// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"bytes"
	"testing"
)

// TestExpLogTableConsistency asserts that for every non-zero element v in
// GF(256), exp[log[v]] == v, and conversely log[exp[i]] == i for i in 0..254.
func TestExpLogTableConsistency(t *testing.T) {
	for i := 0; i < 255; i++ {
		v := expTable[i]
		if v == 0 {
			t.Errorf("expTable[%d] = 0; α never reaches zero", i)
			continue
		}
		if int(logTable[v]) != i {
			t.Errorf("logTable[expTable[%d]] = logTable[%d] = %d, want %d", i, v, logTable[v], i)
		}
	}
}

// TestExpTableCyclic asserts the duplicated upper half: exp[i + 255] = exp[i].
func TestExpTableCyclic(t *testing.T) {
	for i := 0; i < 255; i++ {
		if expTable[i] != expTable[i+255] {
			t.Errorf("expTable[%d] = %d, expTable[%d] = %d; should be equal", i, expTable[i], i+255, expTable[i+255])
		}
	}
}

func TestGF256MulIdentity(t *testing.T) {
	cases := []struct {
		name    string
		a, b, w uint8
	}{
		{"zero left", 0, 17, 0},
		{"zero right", 17, 0, 0},
		{"both zero", 0, 0, 0},
		{"one left", 1, 17, 17},
		{"one right", 17, 1, 17},
		// α^3 · α^5 = α^8 = 0x1D (worked example in docs/theory/03-galois-field.md).
		{"alpha3 times alpha5", 0x08, 0x20, 0x1D},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := gf256Mul(c.a, c.b); got != c.w {
				t.Errorf("gf256Mul(0x%02X, 0x%02X) = 0x%02X, want 0x%02X", c.a, c.b, got, c.w)
			}
		})
	}
}

// TestGF256MulCommutative asserts a * b = b * a across the field. Cheap because
// 256x256 = 65536 pairs.
func TestGF256MulCommutative(t *testing.T) {
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			if gf256Mul(uint8(a), uint8(b)) != gf256Mul(uint8(b), uint8(a)) {
				t.Fatalf("non-commutative at (%d, %d)", a, b)
			}
		}
	}
}

func TestPolyMul(t *testing.T) {
	cases := []struct {
		name string
		a, b []uint8
		want []uint8
	}{
		// (x + 1)^2 = x^2 + 1 over GF(2) (middle term XORs to zero).
		{"x+1 squared", []uint8{1, 1}, []uint8{1, 1}, []uint8{1, 0, 1}},
		// (x + α)(x + α^2) over GF(256).
		// = x^2 + (α + α^2)x + α^3
		// α = 2, α^2 = 4, α + α^2 = 6, α^3 = 8.
		{"x+a times x+a2", []uint8{1, 2}, []uint8{1, 4}, []uint8{1, 6, 8}},
		// Multiply by 1 is identity.
		{"identity", []uint8{1}, []uint8{1, 2, 3}, []uint8{1, 2, 3}},
		// Empty inputs return nil.
		{"empty left", nil, []uint8{1}, nil},
		{"empty right", []uint8{1}, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := polyMul(c.a, c.b)
			if !bytes.Equal(got, c.want) {
				t.Errorf("polyMul = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPolyMod(t *testing.T) {
	cases := []struct {
		name     string
		dividend []uint8
		divisor  []uint8
		want     []uint8
	}{
		// x^3 mod (x + 1) over GF(2) = 1 (substituting x = 1 gives 1).
		{"x3 mod x+1", []uint8{1, 0, 0, 0}, []uint8{1, 1}, []uint8{1}},
		// (x^2 + 1) mod (x + 1) over GF(2) = 0 (since (x+1)^2 = x^2 + 1).
		{"square root", []uint8{1, 0, 1}, []uint8{1, 1}, []uint8{0}},
		// Anything mod (x + 1) over GF(2) is the parity bit: sum of coefficients.
		{"parity", []uint8{1, 1, 1, 1}, []uint8{1, 1}, []uint8{0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := polyMod(c.dividend, c.divisor)
			if !bytes.Equal(got, c.want) {
				t.Errorf("polyMod = %v, want %v", got, c.want)
			}
		})
	}
}
