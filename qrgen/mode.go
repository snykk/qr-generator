// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// Mode identifies an encoding mode. The data modes are Numeric, Alphanumeric,
// Byte, and Kanji (v0.11); ModeECI marks the ECI header (v0.9) and
// ModeStructuredAppend the structured-append header (v0.10), which declare
// interpretation rather than carrying data.
// See docs/theory/02-data-encoding.md, docs/theory/20-eci-segments.md, and
// docs/theory/22-kanji-mode.md.
type Mode int

const (
	ModeNumeric Mode = iota
	ModeAlphanumeric
	ModeByte
	ModeECI
	ModeStructuredAppend
	ModeKanji
)

// String returns a short human-readable mode name.
func (m Mode) String() string {
	switch m {
	case ModeNumeric:
		return "Numeric"
	case ModeAlphanumeric:
		return "Alphanumeric"
	case ModeByte:
		return "Byte"
	case ModeECI:
		return "ECI"
	case ModeStructuredAppend:
		return "StructuredAppend"
	case ModeKanji:
		return "Kanji"
	}
	return "Mode(?)"
}

// Indicator returns the 4-bit mode indicator emitted at the start of a segment.
// Numeric=0001, Alphanumeric=0010, Byte=0100, ECI=0111.
// See docs/theory/09-data-tables.md section 1 and docs/theory/20-eci-segments.md.
func (m Mode) Indicator() uint8 {
	switch m {
	case ModeNumeric:
		return 0b0001
	case ModeAlphanumeric:
		return 0b0010
	case ModeByte:
		return 0b0100
	case ModeECI:
		return 0b0111
	case ModeStructuredAppend:
		return 0b0011
	case ModeKanji:
		return 0b1000
	}
	return 0
}

// CharCountBits returns the number of bits used to encode the character count
// indicator for this mode at the given version. Result is 0 for an invalid
// version.
// See docs/theory/09-data-tables.md §3.
func (m Mode) CharCountBits(v Version) int {
	if !v.IsValid() {
		return 0
	}
	// Kanji has its own count widths (8/10/12), narrower than any other mode.
	if m == ModeKanji {
		switch {
		case v <= 9:
			return 8
		case v <= 26:
			return 10
		default:
			return 12
		}
	}
	// Numeric, Alphanumeric, and Byte are the table's three rows; the header
	// modes (ECI, structured append) are not in the table and have no count.
	if int(m) < 0 || int(m) >= len(charCountBitsTable) {
		return 0
	}
	var idx int
	switch {
	case v <= 9:
		idx = 0
	case v <= 26:
		idx = 1
	default:
		idx = 2
	}
	return charCountBitsTable[m][idx]
}

// charCountBitsTable[mode][versionRange]
// versionRange: 0=1..9, 1=10..26, 2=27..40
var charCountBitsTable = [3][3]int{
	{10, 12, 14}, // Numeric
	{9, 11, 13},  // Alphanumeric
	{8, 16, 16},  // Byte
}
