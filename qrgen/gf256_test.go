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

// TestGF256InverseRoundTrip asserts a * inverse(a) == 1 for every non-zero
// element of GF(256) — an exhaustive 255-case sweep.
func TestGF256InverseRoundTrip(t *testing.T) {
	for a := 1; a < 256; a++ {
		inv := gf256Inverse(uint8(a))
		if got := gf256Mul(uint8(a), inv); got != 1 {
			t.Errorf("a=0x%02X * inverse(a)=0x%02X = 0x%02X, want 1", a, inv, got)
		}
	}
}

// TestGF256InverseZeroPanics confirms gf256Inverse(0) panics, since the zero
// element has no multiplicative inverse.
func TestGF256InverseZeroPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("gf256Inverse(0) did not panic")
		}
	}()
	gf256Inverse(0)
}

func TestPolyEval(t *testing.T) {
	cases := []struct {
		name string
		p    []uint8
		x    uint8
		want uint8
	}{
		// Empty polynomial evaluates to 0.
		{"empty", nil, 5, 0},
		// Constant: p(x) = 7 for all x.
		{"constant", []uint8{7}, 99, 7},
		// p(x) = x → p(5) = 5.
		{"identity", []uint8{1, 0}, 5, 5},
		// p(x) = x + 1 → p(5) = 5 ^ 1 = 4 (XOR over GF(2^8)).
		{"x plus 1 at 5", []uint8{1, 1}, 5, 4},
		// p(x) = (x + α)(x + α^2) at x = α^2 → (α^2 + α)(α^2 + α^2) = something * 0 = 0.
		{"factored at root", []uint8{1, 2 ^ 4, gf256Mul(2, 4)}, 4, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := polyEval(c.p, c.x); got != c.want {
				t.Errorf("polyEval(%v, %d) = %d, want %d", c.p, c.x, got, c.want)
			}
		})
	}
}

func TestPolyDeriv(t *testing.T) {
	cases := []struct {
		name string
		p    []uint8
		want []uint8
	}{
		// Constant: derivative is empty.
		{"constant", []uint8{7}, nil},
		// p(x) = x: derivative is 1.
		{"x", []uint8{1, 0}, []uint8{1}},
		// p(x) = x^2: derivative is 2x = 0 in GF(2^8).
		{"x squared", []uint8{1, 0, 0}, []uint8{0, 0}},
		// p(x) = x^3: derivative is 3x^2 = x^2 (odd integer multiplier).
		{"x cubed", []uint8{1, 0, 0, 0}, []uint8{1, 0, 0}},
		// p(x) = x^3 + x: derivative is 3x^2 + 1 = x^2 + 1.
		{"x cubed plus x", []uint8{1, 0, 1, 0}, []uint8{1, 0, 1}},
		// p(x) = x^2 + 1: derivative is 2x = 0.
		{"x squared plus 1", []uint8{1, 0, 1}, []uint8{0, 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := polyDeriv(c.p)
			if !bytes.Equal(got, c.want) {
				t.Errorf("polyDeriv(%v) = %v, want %v", c.p, got, c.want)
			}
		})
	}
}

func TestPolyDivQR(t *testing.T) {
	cases := []struct {
		name      string
		dividend  []uint8
		divisor   []uint8
		wantQ     []uint8
		wantR     []uint8
	}{
		// (x^2 + 1) / (x + 1) = (x + 1) remainder 0 over GF(2) since (x+1)^2 = x^2 + 1.
		{"perfect division", []uint8{1, 0, 1}, []uint8{1, 1}, []uint8{1, 1}, []uint8{0}},
		// x^3 / (x + 1) = x^2 + x + 1 remainder 1.
		{"x3 by x+1", []uint8{1, 0, 0, 0}, []uint8{1, 1}, []uint8{1, 1, 1}, []uint8{1}},
		// Dividend shorter than divisor → quotient 0, remainder = dividend padded.
		{"short dividend", []uint8{1}, []uint8{1, 0, 1}, []uint8{0}, []uint8{0, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotQ, gotR := polyDivQR(c.dividend, c.divisor)
			if !bytes.Equal(gotQ, c.wantQ) {
				t.Errorf("quotient = %v, want %v", gotQ, c.wantQ)
			}
			if !bytes.Equal(gotR, c.wantR) {
				t.Errorf("remainder = %v, want %v", gotR, c.wantR)
			}
		})
	}
}

// TestPolyDivQRReconstructsDividend is a property test: for every (a, b) pair
// from a small but non-trivial set, dividend = quotient * divisor + remainder
// must hold byte-for-byte over GF(256). Catches sign / shift / off-by-one bugs.
func TestPolyDivQRReconstructsDividend(t *testing.T) {
	dividends := [][]uint8{
		{1, 2, 3, 4, 5},
		{0x1D, 0x20, 0x08, 0x40},
		{1, 0, 0, 0, 0, 0, 0, 0},
		{0xC4, 0x23, 0x27, 0x77, 0xEB, 0xD7, 0xE7, 0xE2, 0x5D, 0x17},
	}
	divisors := [][]uint8{
		{1, 1},
		{1, 2, 4},
		{1, 0, 0, 0, 1}, // x^4 + 1
	}
	for di, dividend := range dividends {
		for dj, divisor := range divisors {
			if len(dividend) < len(divisor) {
				continue
			}
			t.Run("", func(t *testing.T) {
				q, r := polyDivQR(dividend, divisor)
				// reconstructed = q * divisor + r (padded to dividend length).
				prod := polyMul(q, divisor)
				// Align r to the low end of prod.
				if len(r) > 0 {
					for i := 0; i < len(r); i++ {
						prod[len(prod)-len(r)+i] ^= r[i]
					}
				}
				if !bytes.Equal(prod, dividend) {
					t.Errorf("d=%d j=%d: reconstruction = %v, want %v", di, dj, prod, dividend)
				}
			})
		}
	}
}
