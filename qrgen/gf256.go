// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// primitivePolynomial is the GF(2^8) primitive polynomial used by QR codes,
// x^8 + x^4 + x^3 + x^2 + 1, encoded as 0x11D. See docs/theory/03-galois-field.md.
const primitivePolynomial = 0x11D

// expTable[i] = α^i mod p(x), with α = 0x02. The table is duplicated so that
// indices up to 509 (= 254 + 254 + 1) can be looked up without an explicit
// modulo, which keeps gf256Mul branch-free on the hot path.
//
// logTable[v] = i such that α^i == v, for v in 1..255. logTable[0] is
// undefined; callers must guard against multiplying or dividing by zero.
var (
	expTable [512]uint8
	logTable [256]uint8
)

func init() {
	x := uint16(1)
	for i := 0; i < 255; i++ {
		expTable[i] = uint8(x)
		logTable[uint8(x)] = uint8(i)
		// Multiply x by α (= 0x02 = the monomial 'x'), reducing modulo p(x).
		x <<= 1
		if x&0x100 != 0 {
			x ^= primitivePolynomial
		}
	}
	// Duplicate the cyclic exponent table so exp[i + 255] = exp[i] for i in 0..254.
	for i := 0; i < 255; i++ {
		expTable[i+255] = expTable[i]
	}
}

// gf256Mul multiplies two elements of GF(256). Returns 0 if either input is 0.
// See docs/theory/03-galois-field.md.
func gf256Mul(a, b uint8) uint8 {
	if a == 0 || b == 0 {
		return 0
	}
	return expTable[int(logTable[a])+int(logTable[b])]
}

// polyMul multiplies two polynomials over GF(256). Coefficients are stored
// highest-degree first; the result has length len(a) + len(b) - 1. Empty input
// returns nil.
func polyMul(a, b []uint8) []uint8 {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	result := make([]uint8, len(a)+len(b)-1)
	for i, av := range a {
		if av == 0 {
			continue
		}
		for j, bv := range b {
			result[i+j] ^= gf256Mul(av, bv)
		}
	}
	return result
}

// polyMod computes dividend mod divisor over GF(256). The divisor must be
// monic (leading coefficient = 1); the QR generator polynomial always is.
// Coefficients are highest-degree first. The returned remainder has length
// len(divisor) - 1.
func polyMod(dividend, divisor []uint8) []uint8 {
	if len(divisor) <= 1 {
		return nil
	}
	rem := append([]uint8(nil), dividend...)
	for i := 0; i <= len(rem)-len(divisor); i++ {
		coef := rem[i]
		if coef == 0 {
			continue
		}
		for j, dv := range divisor {
			rem[i+j] ^= gf256Mul(coef, dv)
		}
	}
	return rem[len(rem)-(len(divisor)-1):]
}

// gf256Inverse returns the multiplicative inverse of a in GF(256). Panics on
// a == 0; the zero element has no inverse. Used by Reed–Solomon decoding (D3)
// and by polynomial division when the divisor is not monic.
func gf256Inverse(a uint8) uint8 {
	if a == 0 {
		panic("qrgen: GF(256) inverse of zero")
	}
	return expTable[255-int(logTable[a])]
}

// polyDivQR returns the quotient and remainder of dividend divided by divisor
// over GF(256). Unlike polyMod, the divisor does not have to be monic — the
// leading coefficient is normalised inside this function. Coefficients are
// highest-degree first; quotient has length max(1, len(dividend)-len(divisor)+1)
// and remainder has length len(divisor)-1.
//
// Panics on an empty or zero-leading-coefficient divisor.
func polyDivQR(dividend, divisor []uint8) (quotient, remainder []uint8) {
	if len(divisor) == 0 || divisor[0] == 0 {
		panic("qrgen: polyDivQR: divisor must be non-empty with non-zero leading coefficient")
	}
	if len(dividend) < len(divisor) {
		// Quotient is 0; remainder is the dividend padded to the expected width.
		rem := make([]uint8, len(divisor)-1)
		copy(rem[len(rem)-len(dividend):], dividend)
		return []uint8{0}, rem
	}
	rem := append([]uint8(nil), dividend...)
	leadInv := gf256Inverse(divisor[0])
	qLen := len(dividend) - len(divisor) + 1
	quotient = make([]uint8, qLen)
	for i := 0; i < qLen; i++ {
		coef := gf256Mul(rem[i], leadInv)
		quotient[i] = coef
		if coef == 0 {
			continue
		}
		for j, dv := range divisor {
			rem[i+j] ^= gf256Mul(coef, dv)
		}
	}
	remainder = rem[qLen:]
	return quotient, remainder
}

// polyEval evaluates polynomial p at the point x over GF(256) using Horner's
// method. Coefficients are highest-degree first. Empty polynomial evaluates to 0.
//
// Used heavily by RS decoding (D3): syndromes call this with x = α^i,
// Chien search calls it with each non-zero field element to find roots, and
// Forney's algorithm calls it on Ω and Λ' at the error locators.
func polyEval(p []uint8, x uint8) uint8 {
	if len(p) == 0 {
		return 0
	}
	result := p[0]
	for i := 1; i < len(p); i++ {
		result = gf256Mul(result, x) ^ p[i]
	}
	return result
}

// polyDeriv returns the formal derivative of polynomial p over GF(2⁸).
// Coefficients are highest-degree first. The result has length len(p)-1; a
// constant polynomial returns nil.
//
// In characteristic-2 arithmetic the derivative of x^k is x^(k-1) when k is
// odd and 0 when k is even, because the integer multiplier k collapses
// modulo 2. Used by Forney's algorithm (D3) on the error-locator Λ.
func polyDeriv(p []uint8) []uint8 {
	if len(p) <= 1 {
		return nil
	}
	result := make([]uint8, len(p)-1)
	for i := 0; i < len(p)-1; i++ {
		// Term p[i] has degree (len(p)-1-i) in the original polynomial. The
		// derivative keeps it iff that degree is odd.
		if (len(p)-1-i)%2 == 1 {
			result[i] = p[i]
		}
	}
	return result
}
