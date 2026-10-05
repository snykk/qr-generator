// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// microQuietZone is the quiet zone Micro QR requires: 2 modules, against QR's 4.
const microQuietZone = 2

// numMicroMasks is the number of Micro QR mask patterns.
const numMicroMasks = 4

// microMaskToQR maps Micro mask 0..3 to the QR mask pattern it reuses: QR
// masks 1, 4, 6, and 7 (docs/theory/26-micro-qr.md section 9).
var microMaskToQR = [numMicroMasks]int{1, 4, 6, 7}

// microFormatMask is XORed into every Micro format word, as 0x5412 is in QR.
const microFormatMask = 0x4445

// newMicroMatrix returns an empty symbol of Micro version v. The version field
// stays zero: it describes QR versions, and no Micro code path reads it.
func newMicroMatrix(v MicroVersion) *matrix {
	n := v.Size()
	m := &matrix{size: n, modules: make([][]bool, n), reserved: make([][]bool, n)}
	for i := 0; i < n; i++ {
		m.modules[i] = make([]bool, n)
		m.reserved[i] = make([]bool, n)
	}
	return m
}

// placeMicroFunctionPatterns draws the Micro function patterns: the single
// finder at the top-left, whose separator placeSingleFinder clips to the right
// and bottom edges of the 8 by 8 block; the timing patterns along row 0 and
// column 0 from index 8, dark at even indices; and the reserved format area,
// column 8 rows 1..8 plus row 8 columns 1..7. There is no alignment pattern,
// version information, or dark module.
func (m *matrix) placeMicroFunctionPatterns() {
	m.placeSingleFinder(0, 0)
	for i := 8; i < m.size; i++ {
		dark := i%2 == 0
		m.set(0, i, dark)
		m.reserve(0, i)
		m.set(i, 0, dark)
		m.reserve(i, 0)
	}
	for i := 1; i <= 8; i++ {
		m.reserve(i, 8)
		m.reserve(8, i)
	}
}

// microFormatInfo returns the 15-bit format word for symbol number (0..7) and
// mask (0..3): the BCH(15,5) code over number<<2 | mask with QR's generator
// 0x537, XORed with 0x4445.
func microFormatInfo(number, mask int) uint32 {
	return bchFormatWord(uint32(number<<2|mask)) ^ microFormatMask
}

// bchFormatWord returns the BCH(15,5) codeword for a 5-bit payload: the
// payload followed by the remainder of payload * x^10 modulo the generator
// x^10 + x^8 + x^5 + x^4 + x^2 + x + 1 (0x537). QR and Micro QR share it and
// differ only in the XOR mask applied afterwards.
func bchFormatWord(payload uint32) uint32 {
	rem := payload << 10
	for bit := 14; bit >= 10; bit-- {
		if rem&(1<<uint(bit)) != 0 {
			rem ^= 0x537 << uint(bit-10)
		}
	}
	return payload<<10 | rem
}

// writeMicroFormatInfo places the single format copy: bit i (0..7) at
// (i+1, 8), down column 8, and bit i (8..14) at (8, 15-i), left along row 8,
// so bit 14 lands at (8, 1). This is QR's first-copy L with the same corner
// but no timing gaps; getting it backwards would still round-trip internally
// while every other reader failed (docs/theory/26-micro-qr.md section 7).
func (m *matrix) writeMicroFormatInfo(word uint32) {
	for i := 0; i < 8; i++ {
		m.modules[i+1][8] = bitOf(word, i)
	}
	for i := 8; i < 15; i++ {
		m.modules[8][15-i] = bitOf(word, i)
	}
}

// microMaskScore is the Micro mask evaluation: SUM1 counts dark modules in the
// last column and SUM2 in the last row, each skipping the timing module at
// index 0; the score is SUM1*16 + SUM2 when SUM1 <= SUM2, else SUM2*16 + SUM1.
// Higher is better. The format area never touches those edges.
func (m *matrix) microMaskScore() int {
	n := m.size
	sum1, sum2 := 0, 0
	for i := 1; i < n; i++ {
		if m.modules[i][n-1] {
			sum1++
		}
		if m.modules[n-1][i] {
			sum2++
		}
	}
	if sum1 <= sum2 {
		return sum1*16 + sum2
	}
	return sum2*16 + sum1
}

// selectMicroMask returns the Micro mask with the highest score, trying each
// on a copy; on a tie the lowest-numbered mask wins. m is left unmasked.
func (m *matrix) selectMicroMask() int {
	best, bestScore := 0, -1
	for k := 0; k < numMicroMasks; k++ {
		trial := m.clone()
		trial.applyMask(microMaskToQR[k])
		if s := trial.microMaskScore(); s > bestScore {
			best, bestScore = k, s
		}
	}
	return best
}

// buildMicroMatrix encodes text as a Micro QR symbol: it picks the symbol type
// and bit stream (encodeMicroText), draws the function patterns, places the
// stream without QR's column-6 skip, applies the requested or best-scoring
// mask, and writes the single format copy. It returns the mask used.
func buildMicroMatrix(text string, opts *options) (*matrix, int, error) {
	sym, stream, err := encodeMicroText(text, opts.microVersion, opts.ec, opts.ecSet)
	if err != nil {
		return nil, 0, err
	}
	m := newMicroMatrix(sym.version)
	m.placeMicroFunctionPatterns()
	if err := m.placeBits(stream.bytes(), sym.streamBits(), -1); err != nil {
		return nil, 0, err
	}
	mask := opts.mask
	if mask < 0 {
		mask = m.selectMicroMask()
	}
	m.applyMask(microMaskToQR[mask])
	m.writeMicroFormatInfo(microFormatInfo(sym.number, mask))
	return m, mask, nil
}
