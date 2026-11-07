// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"fmt"
	"math/bits"
	"strings"
)

// rmqrFormatBudget is the bit-error budget for each format copy: BCH(18,6)'s
// minimum distance of 8 guarantees unique decoding up to 3 errors.
const rmqrFormatBudget = 3

// rmqrVersionForShape returns the rMQR version of an h-row, w-column symbol,
// or 0 if no version has that shape. No rMQR shape is square.
func rmqrVersionForShape(h, w int) RMQRVersion {
	for v := R7x43; v <= R17x139; v++ {
		if v.Height() == h && v.Width() == w {
			return v
		}
	}
	return 0
}

// decodeRMQRMatrix recovers the text of an rMQR symbol from its module grid:
// the version comes from the grid's shape, the level from the nearer of the
// two format copies, and then the data modules are unmasked and read in
// placement order, deinterleaved, corrected block by block, and parsed with
// rMQR's 3-bit mode indicators and per-version count widths.
// See docs/theory/28-rmqr.md.
func decodeRMQRMatrix(grid [][]bool) (string, error) {
	if len(grid) == 0 {
		return "", fmt.Errorf("qrgen: decodeRMQRMatrix: empty grid")
	}
	h, w := len(grid), len(grid[0])
	v := rmqrVersionForShape(h, w)
	if v == 0 {
		return "", fmt.Errorf("qrgen: decodeRMQRMatrix: no rMQR symbol is %d by %d modules", h, w)
	}
	for r, row := range grid {
		if len(row) != w {
			return "", fmt.Errorf("qrgen: decodeRMQRMatrix: row %d has %d cols, want %d", r, len(row), w)
		}
	}
	m := newRMQRMatrix(v)
	m.placeRMQRFunctionPatterns() // marks the function modules as reserved
	for r := range h {
		copy(m.modules[r], grid[r])
	}

	sym, err := readRMQRFormatInfo(m, v)
	if err != nil {
		return "", err
	}
	data, err := rmqrCorrect(readRMQRStream(m, sym), sym)
	if err != nil {
		return "", err
	}
	return decodeRMQRText(data, v)
}

// readRMQRFormatInfo reads both format copies and matches each, under its own
// mask, against the two words of version v, one per level. The version is
// already known from the grid's shape, so the format information only has to
// name the level, and restricting each copy to v's words keeps a damaged copy
// from naming another version. The nearer of the two copies wins, within the
// 3-bit budget, so either copy alone is enough.
func readRMQRFormatInfo(m *matrix, v RMQRVersion) (rmqrSymbol, error) {
	h, w := m.rows(), m.cols()
	var finder, sub uint32
	for i := range 18 {
		if r, c := rmqrFormatModuleFinder(i); m.modules[r][c] {
			finder |= 1 << uint(i)
		}
		if r, c := rmqrFormatModuleSub(i, h, w); m.modules[r][c] {
			sub |= 1 << uint(i)
		}
	}
	best, bestDist := 0, 19
	for lvl := range 2 {
		word := bch18Word(rmqrFormatData(v, lvl))
		for _, d := range [2]int{
			bits.OnesCount32(finder ^ word ^ rmqrFormatMaskFinder),
			bits.OnesCount32(sub ^ word ^ rmqrFormatMaskSub),
		} {
			if d < bestDist {
				best, bestDist = lvl, d
			}
		}
	}
	if bestDist > rmqrFormatBudget {
		return rmqrSymbol{}, ErrFormatUnreadable
	}
	return rmqrSymbol{version: v, lvl: best}, nil
}

// readRMQRStream walks the data modules in placement order, column pairs from
// the second column on the right, removing the fixed mask, and returns the
// symbol's codewords; the remainder bits after them are not read.
func readRMQRStream(m *matrix, sym rmqrSymbol) []byte {
	n := sym.info().codewords
	out := make([]byte, 0, n)
	var cur byte
	nbits := 0
	h, w := m.rows(), m.cols()
	upward := true
	for col := w - 2; col > 0; col -= 2 {
		for i := range h {
			row := i
			if upward {
				row = h - 1 - i
			}
			for c := range 2 {
				cc := col - c
				if m.isReserved(row, cc) || len(out) == n {
					continue
				}
				cur <<= 1
				if m.modules[row][cc] != maskCondition(rmqrMask, row, cc) {
					cur |= 1
				}
				if nbits++; nbits == 8 {
					out = append(out, cur)
					cur, nbits = 0, 0
				}
			}
		}
		upward = !upward
	}
	return out
}

// rmqrCorrect splits the interleaved codeword stream back into its blocks, the
// shorter ones first, corrects each with Reed-Solomon, and returns the data
// codewords in order.
func rmqrCorrect(stream []byte, sym rmqrSymbol) ([]byte, error) {
	info := sym.info()
	nb, nd := info.blocks[sym.lvl], info.data[sym.lvl]
	ecPer := (info.codewords - nd) / nb
	short, nShort := nd/nb, nb-nd%nb
	if len(stream) != info.codewords {
		return nil, fmt.Errorf("qrgen: rmqrCorrect: %d codewords, want %d for %v", len(stream), info.codewords, sym)
	}
	blocks := make([][]byte, nb)
	k := 0
	for i := 0; i <= short; i++ {
		for b := range nb {
			if i < short || b >= nShort {
				blocks[b] = append(blocks[b], stream[k])
				k++
			}
		}
	}
	for range ecPer {
		for b := range nb {
			blocks[b] = append(blocks[b], stream[k])
			k++
		}
	}
	out := make([]byte, 0, nd)
	for b, block := range blocks {
		data, err := rsDecode(block, ecPer)
		if err != nil {
			return nil, fmt.Errorf("qrgen: block %d: %w", b, err)
		}
		out = append(out, data...)
	}
	return out, nil
}

// decodeRMQRText parses the data codewords of an rMQR symbol at version v:
// segments with 3-bit mode indicators and v's count widths, an ECI header
// switching the charset of later byte segments, until the terminator or until
// fewer than 3 bits remain, which is how a terminator cut short by a full
// symbol ends. Byte segments are read as UTF-8 unless an ECI says otherwise.
func decodeRMQRText(data []byte, v RMQRVersion) (string, error) {
	br := newBitReader(data)
	var out strings.Builder
	activeECI := ECINone
	for br.bitsRemaining() >= 3 {
		mode, err := br.readBits(3)
		if err != nil {
			return "", err
		}
		var seg string
		switch mode {
		case 0:
			return out.String(), nil
		case 1, 2, 3, 4:
			m := [5]Mode{0, ModeNumeric, ModeAlphanumeric, ModeByte, ModeKanji}[mode]
			count, err := br.readBits(rmqrCountBits(m, v))
			if err != nil {
				return "", err
			}
			switch m {
			case ModeNumeric:
				seg, err = decodeNumeric(br, int(count))
			case ModeAlphanumeric:
				seg, err = decodeAlphanumeric(br, int(count))
			case ModeByte:
				var raw []byte
				raw, err = decodeByteModeRaw(br, int(count))
				seg = transcodeFrom(raw, activeECI)
			case ModeKanji:
				seg, err = decodeKanji(br, int(count))
			}
			if err != nil {
				return "", err
			}
		case 7:
			if activeECI, err = readECIDesignator(br); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("%w: rMQR mode indicator %03b (FNC1) is not supported", ErrCorruptedPayload, mode)
		}
		out.WriteString(seg)
	}
	return out.String(), nil
}
