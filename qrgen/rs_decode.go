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
	"fmt"
)

// ErrTooManyErrors is returned by Reed–Solomon decoding when the received
// block contains more corrupted codewords than the code can correct
// (floor(n/2) where n is the number of EC codewords in the block).
var ErrTooManyErrors = errors.New("qrgen: too many errors to correct")

// rsDecode recovers the original data codewords from a received Reed–Solomon
// block laid out as [data || EC] with `n` EC codewords trailing. Up to
// floor(n/2) corrupted bytes anywhere in the block are recoverable; beyond
// that the decoder returns ErrTooManyErrors rather than guessing.
//
// See docs/theory/11-rs-decoding.md for the algorithm walkthrough.
func rsDecode(received []byte, n int) ([]byte, error) {
	dataLen := len(received) - n
	if dataLen < 0 {
		return nil, fmt.Errorf("qrgen: rsDecode: block (%d bytes) shorter than n=%d EC codewords", len(received), n)
	}

	// Stage 1 — syndromes.
	synd := make([]byte, n)
	for i := 0; i < n; i++ {
		synd[i] = polyEval(received, expTable[i])
	}

	// Fast path: all syndromes zero → no errors.
	allZero := true
	for _, s := range synd {
		if s != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return append([]byte(nil), received[:dataLen]...), nil
	}

	// Stages 2 & 3 — Berlekamp–Massey.
	lambda, L := berlekampMassey(synd)
	if L == 0 || L > n/2 {
		// L == 0 with non-zero syndromes is internally inconsistent.
		return nil, ErrTooManyErrors
	}

	// Stage 4 — Chien search.
	positions, locators := chienSearch(lambda, len(received))
	if len(positions) != L {
		return nil, ErrTooManyErrors
	}

	// Stage 5 — Forney.
	magnitudes := forneyMagnitudes(lambda, synd, locators)
	if magnitudes == nil {
		return nil, ErrTooManyErrors
	}

	// Apply corrections.
	corrected := append([]byte(nil), received...)
	for k, pos := range positions {
		if pos < 0 || pos >= len(corrected) {
			return nil, ErrTooManyErrors
		}
		corrected[pos] ^= magnitudes[k]
	}
	return corrected[:dataLen], nil
}

// berlekampMassey runs the Berlekamp–Massey algorithm on the syndrome sequence
// and returns the error-locator polynomial Λ (in highest-degree-first form,
// compatible with polyEval / polyDeriv) along with L, the number of detected
// errors. See docs/theory/11-rs-decoding.md §3.
//
// The algorithm itself works most naturally in lowest-degree-first form; the
// reversal at the end is purely a presentation choice so downstream stages
// can reuse the existing polynomial helpers.
func berlekampMassey(syndromes []byte) (lambda []byte, errorCount int) {
	// Work in lowest-degree-first form internally.
	Λ := []byte{1}
	B := []byte{1}
	L := 0
	m := 1
	b := byte(1)

	for n := 0; n < len(syndromes); n++ {
		// Discrepancy δ = S[n] + Σ_{i=1..L} Λ[i] · S[n-i].
		δ := syndromes[n]
		for i := 1; i < len(Λ); i++ {
			if n-i >= 0 {
				δ ^= gf256Mul(Λ[i], syndromes[n-i])
			}
		}

		if δ == 0 {
			m++
			continue
		}

		T := append([]byte(nil), Λ...)
		scale := gf256Mul(δ, gf256Inverse(b))

		// Λ = Λ + scale · x^m · B (XOR is GF(2^8) addition).
		// "x^m · B" in lowest-degree-first means prepend m zeros to B.
		if m+len(B) > len(Λ) {
			extended := make([]byte, m+len(B))
			copy(extended, Λ)
			Λ = extended
		}
		for i, bv := range B {
			Λ[m+i] ^= gf256Mul(scale, bv)
		}

		if 2*L <= n {
			L = n + 1 - L
			B = T
			b = δ
			m = 1
		} else {
			m++
		}
	}

	return reverseBytes(Λ), L
}

// reverseBytes returns a copy of p with the element order flipped, used for
// swapping between lowest-degree-first and highest-degree-first polynomial
// conventions.
func reverseBytes(p []byte) []byte {
	r := make([]byte, len(p))
	for i, v := range p {
		r[len(p)-1-i] = v
	}
	return r
}

// chienSearch evaluates Λ at α^(-j) for j in [0, codewordLen) and reports
// every root. lambda must be in highest-degree-first form. Returns parallel
// slices: positions[k] is the array index into the received codeword where
// the k-th error sits, and locators[k] is the matching error locator
// X_k = α^j (the polynomial-degree representation of that position).
func chienSearch(lambda []byte, codewordLen int) (positions []int, locators []byte) {
	for j := 0; j < codewordLen; j++ {
		// x = α^(-j). For j == 0, that's α^0 = 1.
		var x byte
		if j == 0 {
			x = 1
		} else {
			x = expTable[255-(j%255)]
		}
		if polyEval(lambda, x) != 0 {
			continue
		}
		// Root found. The error is at polynomial degree j; in our high-degree-
		// first array convention that is array index codewordLen-1-j.
		var locator byte
		if j == 0 {
			locator = 1
		} else {
			locator = expTable[j%255]
		}
		positions = append(positions, codewordLen-1-j)
		locators = append(locators, locator)
	}
	return positions, locators
}

// forneyMagnitudes computes the error magnitude Y_k for each error position.
// Inputs: Λ (highest-degree-first), the syndrome vector, and the per-error
// locators returned by chienSearch. Returns nil if the formula degenerates
// (Λ'(X^{-1}) = 0), which indicates an inconsistent solution.
//
// For QR's RS code with generator roots starting at α^0, Forney's formula is
// Y_k = X_k · Ω(X_k^{-1}) / Λ'(X_k^{-1}) over GF(256).
func forneyMagnitudes(lambda []byte, syndromes []byte, locators []byte) []byte {
	n := len(syndromes)

	// S(x) = Σ S_i · x^i. In high-degree-first form sPoly[0] = S_{n-1}, …, sPoly[n-1] = S_0.
	sPoly := make([]byte, n)
	for i := 0; i < n; i++ {
		sPoly[n-1-i] = syndromes[i]
	}

	// Ω(x) = (S(x) · Λ(x)) mod x^n. In high-degree-first form, "mod x^n" drops
	// the leading high-degree terms and keeps the trailing n entries.
	product := polyMul(sPoly, lambda)
	var omega []byte
	if len(product) > n {
		omega = product[len(product)-n:]
	} else {
		omega = product
	}

	lambdaDeriv := polyDeriv(lambda)

	magnitudes := make([]byte, len(locators))
	for k, X := range locators {
		Xinv := gf256Inverse(X)
		omegaVal := polyEval(omega, Xinv)
		derivVal := polyEval(lambdaDeriv, Xinv)
		if derivVal == 0 {
			return nil
		}
		magnitudes[k] = gf256Mul(X, gf256Mul(omegaVal, gf256Inverse(derivVal)))
	}
	return magnitudes
}
