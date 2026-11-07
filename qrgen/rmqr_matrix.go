// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// rmqrQuietZone is the quiet zone rMQR requires: 2 modules, as for Micro QR.
const rmqrQuietZone = 2

// rmqrMask is the QR mask pattern every rMQR symbol uses; there is no mask
// selection (docs/theory/28-rmqr.md section 9).
const rmqrMask = 4

// newRMQRMatrix returns an empty rMQR symbol of version v, v.Height() rows by
// v.Width() columns. The size and version fields stay zero: they describe
// square QR symbols, and the code rMQR shares with QR reads rows and cols.
func newRMQRMatrix(v RMQRVersion) *matrix {
	h, w := v.Height(), v.Width()
	m := &matrix{modules: make([][]bool, h), reserved: make([][]bool, h)}
	for i := range h {
		m.modules[i] = make([]bool, w)
		m.reserved[i] = make([]bool, w)
	}
	return m
}

// setFunction sets module (r, c) and reserves it as a function module.
func (m *matrix) setFunction(r, c int, dark bool) {
	m.set(r, c, dark)
	m.reserve(r, c)
}

// placeRMQRFunctionPatterns draws the rMQR function patterns and reserves both
// format areas (docs/theory/28-rmqr.md section 6): the finder with its
// separator, which placeSingleFinder clips to column 7 and, from height 9,
// row 7; the 5 by 5 finder sub-pattern; the corner patterns; the alignment
// patterns at the top and bottom of each alignment column; and the timing
// patterns on all four edges and down each alignment column, dark at even
// indices wherever no other pattern sits.
func (m *matrix) placeRMQRFunctionPatterns() {
	h, w := m.rows(), m.cols()
	m.placeSingleFinder(0, 0)
	for i := range 5 {
		for j := range 5 {
			ring := i == 0 || i == 4 || j == 0 || j == 4
			m.setFunction(h-5+i, w-5+j, ring || (i == 2 && j == 2))
		}
	}
	m.setFunction(0, w-2, true)
	m.setFunction(0, w-1, true)
	m.setFunction(1, w-1, true)
	m.setFunction(1, w-2, false)
	m.setFunction(h-1, 0, true)
	m.setFunction(h-1, 1, true)
	m.setFunction(h-1, 2, true)
	if h >= 11 {
		m.setFunction(h-2, 0, true)
		m.setFunction(h-2, 1, false)
	}
	cols := rmqrAlignmentColumns(w)
	for _, cx := range cols {
		for i := range 3 {
			for j := range 3 {
				dark := i != 1 || j != 1
				m.setFunction(i, cx-1+j, dark)
				m.setFunction(h-1-i, cx-1+j, dark)
			}
		}
	}
	for c := range w {
		for _, r := range [2]int{0, h - 1} {
			if !m.isReserved(r, c) {
				m.setFunction(r, c, c%2 == 0)
			}
		}
	}
	for _, c := range append([]int{0, w - 1}, cols...) {
		for r := range h {
			if !m.isReserved(r, c) {
				m.setFunction(r, c, r%2 == 0)
			}
		}
	}
	for i := range 18 {
		m.reserve(rmqrFormatModuleFinder(i))
		m.reserve(rmqrFormatModuleSub(i, h, w))
	}
}

// rmqrFormatModuleFinder returns the (row, col) of bit i (0..17) of the format
// copy beside the finder: bits 0 to 14 down columns 8, 9, and 10 from row 1,
// five per column, and bits 15 to 17 down column 11 from row 1.
func rmqrFormatModuleFinder(i int) (row, col int) {
	if i < 15 {
		return 1 + i%5, 8 + i/5
	}
	return i - 14, 11
}

// rmqrFormatModuleSub returns the (row, col) of bit i (0..17) of the format
// copy beside the finder sub-pattern of an h by w symbol: bits 0 to 14 down
// columns w-8, w-7, and w-6 from row h-6, five per column, and bits 15 to 17
// along row h-6 at columns w-5, w-4, and w-3.
func rmqrFormatModuleSub(i, h, w int) (row, col int) {
	if i < 15 {
		return h - 6 + i%5, w - 8 + i/5
	}
	return h - 6, w - 20 + i
}

// writeRMQRFormatInfo writes both format copies of sym: the BCH(18,6) word of
// its level and version, XORed with each copy's own mask.
func (m *matrix) writeRMQRFormatInfo(sym rmqrSymbol) {
	h, w := m.rows(), m.cols()
	word := bch18Word(rmqrFormatData(sym.version, sym.lvl))
	finder, sub := word^rmqrFormatMaskFinder, word^rmqrFormatMaskSub
	for i := range 18 {
		r, c := rmqrFormatModuleFinder(i)
		m.modules[r][c] = bitOf(finder, i)
		r, c = rmqrFormatModuleSub(i, h, w)
		m.modules[r][c] = bitOf(sub, i)
	}
}

// buildRMQRMatrix encodes text as an rMQR symbol: it chooses the symbol and its
// codeword stream (encodeRMQRText), draws the function patterns, places the
// stream and the remainder bits from the second column on the right, applies
// the fixed mask, and writes both format copies. It returns the mask used,
// always rmqrMask.
func buildRMQRMatrix(text string, opts *options) (*matrix, int, error) {
	sym, stream, err := encodeRMQRText(text, opts.rmqrVersion, opts.rmqrHeight, opts.ec, opts.ecSet, opts.eci)
	if err != nil {
		return nil, 0, err
	}
	m := newRMQRMatrix(sym.version)
	m.placeRMQRFunctionPatterns()
	if err := m.placeBits(stream, 8*len(stream)+sym.info().remainder, -1, m.cols()-2); err != nil {
		return nil, 0, err
	}
	m.applyMask(rmqrMask)
	m.writeRMQRFormatInfo(sym)
	return m, rmqrMask, nil
}
