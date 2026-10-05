// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// MicroVersion is a Micro QR Code version, M1 to M4. It is a separate type
// from Version on purpose: Micro symbols have their own size formula, tables,
// and layout, and Version's QR-only methods must never see one.
// See docs/theory/26-micro-qr.md.
type MicroVersion int

// The four Micro QR Code versions.
const (
	M1 MicroVersion = iota + 1
	M2
	M3
	M4
)

// IsValid reports whether v is one of M1 to M4.
func (v MicroVersion) IsValid() bool { return v >= M1 && v <= M4 }

// Size returns the side length of a Micro symbol in modules: 2k + 9 for Mk,
// so 11, 13, 15, and 17.
func (v MicroVersion) Size() int { return 2*int(v) + 9 }

// String returns "M1".."M4", or "Micro(n)" for an invalid value.
func (v MicroVersion) String() string {
	if !v.IsValid() {
		return fmt.Sprintf("Micro(%d)", int(v))
	}
	return fmt.Sprintf("M%d", int(v))
}

// modeIndicatorBits is the mode-indicator width: 0, 1, 2, or 3 bits for M1
// to M4. M1 needs none because numeric is its only mode.
func (v MicroVersion) modeIndicatorBits() int { return int(v) - 1 }

// terminatorBits is the terminator length: the mode indicator plus the numeric
// count field (3, 5, 7, 9 bits), so a reader parses "numeric, zero characters".
func (v MicroVersion) terminatorBits() int { return 2*int(v) + 1 }

// microCountBitsTable holds the character-count widths per mode, indexed by
// version (M1..M4 at 1..4). Zero means the mode is not available.
var microCountBitsTable = map[Mode][5]int{
	ModeNumeric:      {0, 3, 4, 5, 6},
	ModeAlphanumeric: {0, 0, 3, 4, 5},
	ModeByte:         {0, 0, 0, 4, 5},
	ModeKanji:        {0, 0, 0, 3, 4},
}

// microCountBits returns the character-count width for mode m at version v,
// or 0 if v does not support m.
func microCountBits(m Mode, v MicroVersion) int {
	if !v.IsValid() {
		return 0
	}
	return microCountBitsTable[m][v]
}

// microModeValue is the Micro mode-indicator value of a data mode: numeric 0,
// alphanumeric 1, byte 2, Kanji 3. These differ from the QR indicators, and
// Kanji is not Mode's own ordinal, so the mapping is explicit.
func microModeValue(m Mode) uint32 {
	switch m {
	case ModeNumeric:
		return 0
	case ModeAlphanumeric:
		return 1
	case ModeByte:
		return 2
	case ModeKanji:
		return 3
	}
	panic(fmt.Sprintf("qrgen: microModeValue: mode %d has no Micro indicator", m))
}

// microSymbol is one Micro symbol type: a version at an error-correction
// level. M1 has no correction level, only error detection.
type microSymbol struct {
	version    MicroVersion
	ec         ECLevel // the level; unused for M1
	detectOnly bool    // M1: the EC codewords detect errors but never correct
	number     int     // 3-bit symbol number carried in the format information
	dataBits   int     // data capacity in bits
	dataWords  int     // data codewords, counting M1/M3's final 4-bit codeword as one
	ecWords    int     // error-correction codewords (a single Reed-Solomon block)
}

// microSymbols lists the eight Micro symbol types in symbol-number order.
// Values from ISO/IEC 18004:2015, verified against zint and segno.
var microSymbols = [8]microSymbol{
	{version: M1, detectOnly: true, number: 0, dataBits: 20, dataWords: 3, ecWords: 2},
	{version: M2, ec: ECLevelL, number: 1, dataBits: 40, dataWords: 5, ecWords: 5},
	{version: M2, ec: ECLevelM, number: 2, dataBits: 32, dataWords: 4, ecWords: 6},
	{version: M3, ec: ECLevelL, number: 3, dataBits: 84, dataWords: 11, ecWords: 6},
	{version: M3, ec: ECLevelM, number: 4, dataBits: 68, dataWords: 9, ecWords: 8},
	{version: M4, ec: ECLevelL, number: 5, dataBits: 128, dataWords: 16, ecWords: 8},
	{version: M4, ec: ECLevelM, number: 6, dataBits: 112, dataWords: 14, ecWords: 10},
	{version: M4, ec: ECLevelQ, number: 7, dataBits: 80, dataWords: 10, ecWords: 14},
}

// hasHalfCodeword reports whether the final data codeword is 4 bits long
// (M1 and M3), which is when the data capacity is not a multiple of 8.
func (s microSymbol) hasHalfCodeword() bool { return s.dataBits%8 != 0 }

// streamBits is the length of the final message: the data bits (the half
// codeword counted as 4) plus the EC codewords. It equals the symbol's data
// module count, 36 / 80 / 132 / 192, so Micro needs no remainder bits.
func (s microSymbol) streamBits() int { return s.dataBits + 8*s.ecWords }

// String returns "M1" or "M2-L" style names.
func (s microSymbol) String() string {
	if s.detectOnly {
		return s.version.String()
	}
	return s.version.String() + "-" + s.ec.String()
}

// microSymbolFor returns the symbol type for version v at level ec. M1 is
// returned for M1 regardless of ec, since it has no level; ok is false when v
// does not offer ec.
func microSymbolFor(v MicroVersion, ec ECLevel) (microSymbol, bool) {
	for _, s := range microSymbols {
		if s.version == v && (s.detectOnly || s.ec == ec) {
			return s, true
		}
	}
	return microSymbol{}, false
}

// microCandidates lists the symbol types to try, smallest first, for a forced
// version (0 = any) and an EC level that may be unset.
//
// With the level unset, the order is M1, M2-M, M2-L, M3-M, M3-L, M4-M, M4-L:
// the smallest symbol first and, within a version, M before L, never Q. This
// is exactly zint's automatic choice. An explicit level selects only the types
// that offer it, and never M1: M1 corrects nothing, so it cannot honour a
// requested correction level (zint also allows M1 for an explicit L; this
// library deliberately does not).
func microCandidates(force MicroVersion, ec ECLevel, ecSet bool) []microSymbol {
	var out []microSymbol
	for v := M1; v <= M4; v++ {
		if force != 0 && v != force {
			continue
		}
		if !ecSet {
			if v == M1 {
				out = append(out, microSymbols[0])
				continue
			}
			for _, lvl := range [2]ECLevel{ECLevelM, ECLevelL} {
				if s, ok := microSymbolFor(v, lvl); ok {
					out = append(out, s)
				}
			}
			continue
		}
		if v == M1 {
			continue
		}
		if s, ok := microSymbolFor(v, ec); ok {
			out = append(out, s)
		}
	}
	return out
}
