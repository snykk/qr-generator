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
	"strings"
)

// ECI is an Extended Channel Interpretation assignment number, declaring the
// character set of the byte data in a symbol. The zero value, ECINone, means no
// ECI is declared and byte mode is treated as UTF-8, matching the library's
// behaviour before v0.9. Only the two charsets transcodable with the standard
// library are supported; see docs/theory/20-eci-segments.md.
type ECI int

const (
	ECINone   ECI = 0  // no ECI header; byte mode treated as UTF-8
	ECILatin1 ECI = 3  // ISO-8859-1
	ECIUTF8   ECI = 26 // UTF-8
)

// designatorBits returns the width of the ECI designator for this assignment
// number in the shortest of the 1/2/3-codeword forms (8, 16, or 24 bits).
func (e ECI) designatorBits() int {
	switch n := uint32(e); {
	case n <= 127:
		return 8
	case n <= 16383:
		return 16
	default:
		return 24
	}
}

// headerBits returns the total bit cost of the ECI header (the 4-bit mode
// indicator plus the designator), or 0 when no ECI is declared.
func (e ECI) headerBits() int {
	if e == ECINone {
		return 0
	}
	return 4 + e.designatorBits()
}

// appendECIDesignator writes the ECI assignment number to bb in the shortest of
// the three forms defined by ISO/IEC 18004 Table 4: 0bbbbbbb (0..127),
// 10bbbbbb bbbbbbbb (0..16383), or 110bbbbb bbbbbbbb bbbbbbbb (0..999999). The
// encoder always emits the shortest form; see docs/theory/20-eci-segments.md.
func appendECIDesignator(bb *bitBuffer, e ECI) {
	n := uint32(e)
	switch {
	case n <= 127:
		bb.appendBits(n, 8) // 0 + 7-bit value
	case n <= 16383:
		bb.appendBits(0x8000|n, 16) // 10 + 14-bit value
	default:
		bb.appendBits(0xC00000|n, 24) // 110 + 21-bit value
	}
}

// readECIDesignator reads an ECI designator from br, deriving its 1/2/3-codeword
// length from the leading bits (0 / 10 / 110). It accepts non-minimal encodings
// because the spec allows a low assignment number to be written in a longer
// form, even though this library's encoder never produces one.
func readECIDesignator(br *bitReader) (ECI, error) {
	first, err := br.readBits(8)
	if err != nil {
		return 0, err
	}
	switch {
	case first&0x80 == 0: // 0bbbbbbb
		return ECI(first), nil
	case first&0xC0 == 0x80: // 10bbbbbb bbbbbbbb
		second, err := br.readBits(8)
		if err != nil {
			return 0, err
		}
		return ECI((first&0x3F)<<8 | second), nil
	case first&0xE0 == 0xC0: // 110bbbbb bbbbbbbb bbbbbbbb
		rest, err := br.readBits(16)
		if err != nil {
			return 0, err
		}
		return ECI((first&0x1F)<<16 | rest), nil
	default: // 111xxxxx is not a valid designator prefix
		return 0, fmt.Errorf("%w: invalid ECI designator prefix 0x%02X", ErrCorruptedPayload, first)
	}
}

// transcodeTo converts a UTF-8 Go string into the byte payload for the given
// ECI charset. UTF-8 (and the no-ECI default) pass through as the string's own
// bytes; ISO-8859-1 narrows each rune to one byte, erroring on any rune above
// U+00FF. See docs/theory/20-eci-segments.md section 6.
func transcodeTo(s string, e ECI) ([]byte, error) {
	if e != ECILatin1 {
		return []byte(s), nil
	}
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			return nil, fmt.Errorf("qrgen: ECILatin1: rune %q (U+%04X) is not representable in ISO-8859-1", r, r)
		}
		out = append(out, byte(r))
	}
	return out, nil
}

// transcodeFrom converts a byte payload in the given ECI charset back to a
// UTF-8 Go string. ISO-8859-1 widens each byte to its identical code point;
// everything else (UTF-8, the no-ECI default, an unsupported ECI) is read as
// UTF-8.
func transcodeFrom(b []byte, e ECI) string {
	if e != ECILatin1 {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}
