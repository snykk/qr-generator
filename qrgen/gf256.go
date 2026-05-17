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
