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

// microFormatBudget is the bit-error budget for Micro's single format copy:
// the full correction capacity of BCH(15,5), since there is no second copy.
const microFormatBudget = 3

// microVersionForSize returns the Micro version of an n-module symbol, or 0 if
// n is not a Micro size (11, 13, 15, 17). No Micro size is a QR size (21+4k).
func microVersionForSize(n int) MicroVersion {
	if n < 11 || n > 17 || n%2 == 0 {
		return 0
	}
	return MicroVersion((n - 9) / 2)
}

// decodeMicroMatrix recovers the text of a Micro QR symbol from its module
// grid: it reads the single format copy, walks and unmasks the data modules in
// placement order, checks the single Reed-Solomon block (detection only in
// M1), and parses the segments with Micro's field widths.
// See docs/theory/26-micro-qr.md.
func decodeMicroMatrix(grid [][]bool) (string, error) {
	n := len(grid)
	v := microVersionForSize(n)
	if v == 0 {
		return "", fmt.Errorf("qrgen: decodeMicroMatrix: invalid Micro side %d", n)
	}
	for r, row := range grid {
		if len(row) != n {
			return "", fmt.Errorf("qrgen: decodeMicroMatrix: row %d has %d cols, want %d", r, len(row), n)
		}
	}
	m := newMicroMatrix(v)
	m.placeMicroFunctionPatterns() // marks the function modules as reserved
	for r := 0; r < n; r++ {
		copy(m.modules[r], grid[r])
	}

	sym, mask, err := readMicroFormatInfo(m, v)
	if err != nil {
		return "", err
	}
	data, err := microCorrect(readMicroStream(m, sym, mask), sym)
	if err != nil {
		return "", err
	}
	return decodeMicroText(data, sym)
}

// readMicroFormatInfo reads the single format copy and decodes it to the
// nearest valid word among the symbol types of version v, within the 3-bit
// budget. Restricting the search to v's types is how the decoder makes the
// symbol number agree with the grid size: an all-light strip is within 3 bits
// of M4-L mask 1 (0x1202), so trusting the nearest of all 32 words regardless
// of size would misread damaged smaller symbols. BCH(15,5) has minimum
// distance 7, so two words can never both lie within the budget.
func readMicroFormatInfo(m *matrix, v MicroVersion) (microSymbol, int, error) {
	var word uint32
	for i := 0; i < 15; i++ {
		if r, c := microFormatModule(i); m.modules[r][c] {
			word |= 1 << uint(i)
		}
	}
	sym, mask, dist := nearestMicroFormat(word, v)
	if dist > microFormatBudget {
		return microSymbol{}, 0, ErrFormatUnreadable
	}
	return sym, mask, nil
}

// nearestMicroFormat returns the symbol type and mask whose format word is
// nearest to word, with the Hamming distance between them. With v set, only
// v's symbol types are considered, which is how the matrix reader makes the
// symbol number agree with the grid size; with v zero, all 32 words are,
// which is what the image-side reader needs before it knows the size. Ties go
// to the first word in symbol-number, then mask, order.
func nearestMicroFormat(word uint32, v MicroVersion) (microSymbol, int, int) {
	bestDist, bestMask := 16, 0
	var best microSymbol
	for _, s := range microSymbols {
		if v != 0 && s.version != v {
			continue
		}
		for mask := 0; mask < numMicroMasks; mask++ {
			if d := bits.OnesCount32(word ^ microFormatInfo(s.number, mask)); d < bestDist {
				bestDist, best, bestMask = d, s, mask
			}
		}
	}
	return best, bestMask, bestDist
}

// readMicroStream walks the data modules in placement order (the walk of
// placeBits with no skipped column), removing the mask, and returns the final
// message bits. A Micro symbol has exactly as many data modules as stream
// bits, so the walk reads the whole stream.
func readMicroStream(m *matrix, sym microSymbol, mask int) *bitBuffer {
	qrMask := microMaskToQR[mask]
	out := &bitBuffer{}
	n := m.size
	upward := true
	for col := n - 1; col > 0; col -= 2 {
		for i := 0; i < n; i++ {
			row := i
			if upward {
				row = n - 1 - i
			}
			for c := 0; c < 2; c++ {
				cc := col - c
				if m.isReserved(row, cc) || out.bits() >= sym.streamBits() {
					continue
				}
				var bit uint32
				if m.modules[row][cc] != maskCondition(qrMask, row, cc) {
					bit = 1
				}
				out.appendBits(bit, 1)
			}
		}
		upward = !upward
	}
	return out
}

// microCorrect splits the stream into codewords, rebuilding M1/M3's 4-bit
// final data codeword as a byte with its bits in the high nibble, and checks
// the single Reed-Solomon block. M1 is detection-only, so any non-zero
// syndrome fails the decode instead of being corrected. Other types correct up
// to half their EC codewords; a correction that sets bits in the half
// codeword's always-zero low nibble is a miscorrection and fails too.
func microCorrect(stream *bitBuffer, sym microSymbol) ([]byte, error) {
	raw := stream.bytes()
	pos := 0
	read := func(n int) byte {
		var b byte
		for i := 0; i < n; i++ {
			b <<= 1
			if bitAt(raw, pos) {
				b |= 1
			}
			pos++
		}
		return b
	}
	words := make([]byte, 0, sym.dataWords+sym.ecWords)
	full := sym.dataWords
	if sym.hasHalfCodeword() {
		full--
	}
	for i := 0; i < full; i++ {
		words = append(words, read(8))
	}
	if sym.hasHalfCodeword() {
		words = append(words, read(4)<<4)
	}
	for i := 0; i < sym.ecWords; i++ {
		words = append(words, read(8))
	}

	if sym.detectOnly {
		for i := 0; i < sym.ecWords; i++ {
			if polyEval(words, expTable[i]) != 0 {
				return nil, fmt.Errorf("%w: Micro QR M1 detects errors but cannot correct them", ErrTooManyErrors)
			}
		}
		return words[:sym.dataWords], nil
	}
	data, err := rsDecode(words, sym.ecWords)
	if err != nil {
		return nil, err
	}
	if sym.hasHalfCodeword() && data[len(data)-1]&0x0F != 0 {
		return nil, fmt.Errorf("%w: correction set bits outside the 4-bit final codeword", ErrTooManyErrors)
	}
	return data, nil
}

// microModeFromValue maps a Micro mode-indicator value back to a mode, and
// rejects modes version v does not support.
func microModeFromValue(val uint32, v MicroVersion) (Mode, error) {
	modes := [4]Mode{ModeNumeric, ModeAlphanumeric, ModeByte, ModeKanji}
	if val < uint32(len(modes)) && microCountBits(modes[val], v) > 0 {
		return modes[val], nil
	}
	return 0, fmt.Errorf("%w: mode indicator %d is not valid in Micro QR %v", ErrCorruptedPayload, val, v)
}

// decodeMicroText parses the data bits of a Micro symbol: segments with
// Micro's mode-indicator and count widths, ending at the terminator (numeric
// mode with a zero count) or when fewer bits remain than a terminator needs,
// which is how a terminator cut short by a full symbol ends. Every real
// segment needs more bits than the terminator, so that rule never drops data.
// Byte segments are read as UTF-8, as in QR without an ECI header.
func decodeMicroText(data []byte, sym microSymbol) (string, error) {
	v := sym.version
	br := newBitReader(data)
	var out strings.Builder
	for sym.dataBits-br.pos >= v.terminatorBits() {
		mode := ModeNumeric // M1 has no indicator: numeric is its only mode
		if w := v.modeIndicatorBits(); w > 0 {
			val, err := br.readBits(w)
			if err != nil {
				return "", err
			}
			if mode, err = microModeFromValue(val, v); err != nil {
				return "", err
			}
		}
		count, err := br.readBits(microCountBits(mode, v))
		if err != nil {
			return "", err
		}
		if mode == ModeNumeric && count == 0 {
			break
		}
		var seg string
		switch mode {
		case ModeNumeric:
			seg, err = decodeNumeric(br, int(count))
		case ModeAlphanumeric:
			seg, err = decodeAlphanumeric(br, int(count))
		case ModeByte:
			var raw []byte
			raw, err = decodeByteModeRaw(br, int(count))
			seg = transcodeFrom(raw, ECINone)
		case ModeKanji:
			seg, err = decodeKanji(br, int(count))
		}
		if err != nil {
			return "", err
		}
		if br.pos > sym.dataBits {
			return "", fmt.Errorf("%w: segment runs past the %d data bits of %v", ErrCorruptedPayload, sym.dataBits, sym)
		}
		out.WriteString(seg)
	}
	return out.String(), nil
}
