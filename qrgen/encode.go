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

// ErrCapacityExceeded indicates the payload does not fit any QR version at the
// requested EC level. Returned by encodeText.
var ErrCapacityExceeded = errors.New("qrgen: payload exceeds the capacity of the largest QR version at the requested EC level")

// analyzeMode picks the most compact mode that can encode every character in
// text. Numeric wins if all chars are digits; alphanumeric wins next; byte is
// the fallback. Empty input is reported as Numeric (it costs zero payload
// bits in either of the compact modes, but Numeric keeps later size math
// consistent).
//
// This is a single-segment greedy analyzer. Mixed-mode segmentation could
// produce a smaller bit count for some inputs and is deferred past v0.1.
// See docs/theory/02-data-encoding.md.
func analyzeMode(text string) Mode {
	allNumeric := true
	allAlphanumeric := true
	for _, r := range text {
		if r < '0' || r > '9' {
			allNumeric = false
		}
		if alphanumericValue(r) < 0 {
			allAlphanumeric = false
		}
		if !allNumeric && !allAlphanumeric {
			return ModeByte
		}
	}
	switch {
	case allNumeric:
		return ModeNumeric
	case allAlphanumeric:
		return ModeAlphanumeric
	}
	return ModeByte
}

// alphanumericValue returns the spec-defined value 0..44 for an alphanumeric
// character, or -1 if r is not in the alphanumeric set.
// See docs/theory/09-data-tables.md §2.
func alphanumericValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'A' && r <= 'Z':
		return 10 + int(r-'A')
	case r == ' ':
		return 36
	case r == '$':
		return 37
	case r == '%':
		return 38
	case r == '*':
		return 39
	case r == '+':
		return 40
	case r == '-':
		return 41
	case r == '.':
		return 42
	case r == '/':
		return 43
	case r == ':':
		return 44
	}
	return -1
}

// payloadBitLength returns the number of bits the encoded *payload* takes
// (excludes the 4-bit mode indicator and the character-count indicator).
// For byte mode the count is len(text) bytes — this assumes UTF-8 input, which
// Go's string type guarantees.
func payloadBitLength(mode Mode, text string) int {
	switch mode {
	case ModeNumeric:
		n := len(text)
		bits := (n / 3) * 10
		switch n % 3 {
		case 1:
			bits += 4
		case 2:
			bits += 7
		}
		return bits
	case ModeAlphanumeric:
		n := len(text)
		bits := (n / 2) * 11
		if n%2 == 1 {
			bits += 6
		}
		return bits
	case ModeByte:
		return len(text) * 8
	}
	return 0
}

// selectVersion returns the smallest Version whose data-codeword capacity (in
// bits) is enough to hold the header + payload at the given EC level.
// Returns ErrCapacityExceeded if even V40 cannot hold the payload.
func selectVersion(mode Mode, text string, ec ECLevel) (Version, error) {
	payloadBits := payloadBitLength(mode, text)
	for v := MinVersion; v <= MaxVersion; v++ {
		needed := 4 + mode.CharCountBits(v) + payloadBits
		capacity := v.DataCodewords(ec) * 8
		if needed <= capacity {
			return v, nil
		}
	}
	return 0, ErrCapacityExceeded
}

// writePayload appends the encoded payload bits to bb. The mode must already
// match text (use analyzeMode first); writePayload does not validate.
func writePayload(bb *bitBuffer, mode Mode, text string) {
	switch mode {
	case ModeNumeric:
		writeNumeric(bb, text)
	case ModeAlphanumeric:
		writeAlphanumeric(bb, text)
	case ModeByte:
		writeByte(bb, text)
	default:
		panic(fmt.Sprintf("qrgen: writePayload: unknown mode %d", mode))
	}
}

func writeNumeric(bb *bitBuffer, text string) {
	i := 0
	for i+3 <= len(text) {
		n := int(text[i]-'0')*100 + int(text[i+1]-'0')*10 + int(text[i+2]-'0')
		bb.appendBits(uint32(n), 10)
		i += 3
	}
	switch len(text) - i {
	case 1:
		bb.appendBits(uint32(text[i]-'0'), 4)
	case 2:
		n := int(text[i]-'0')*10 + int(text[i+1]-'0')
		bb.appendBits(uint32(n), 7)
	}
}

func writeAlphanumeric(bb *bitBuffer, text string) {
	i := 0
	for i+2 <= len(text) {
		v := alphanumericValue(rune(text[i]))*45 + alphanumericValue(rune(text[i+1]))
		bb.appendBits(uint32(v), 11)
		i += 2
	}
	if i < len(text) {
		bb.appendBits(uint32(alphanumericValue(rune(text[i]))), 6)
	}
}

func writeByte(bb *bitBuffer, text string) {
	for i := 0; i < len(text); i++ {
		bb.appendBits(uint32(text[i]), 8)
	}
}

// encodeText runs the M3 pipeline end-to-end: it analyzes the mode, picks the
// smallest version that fits, then emits header + payload + terminator + bit
// padding + pad bytes to fill the data-codeword capacity exactly. The
// returned slice has length v.DataCodewords(ec) and is ready to feed Reed–
// Solomon (M4).
//
// See docs/theory/02-data-encoding.md and docs/theory/10-worked-example.md.
func encodeText(text string, ec ECLevel) (data []byte, v Version, m Mode, err error) {
	m = analyzeMode(text)
	v, err = selectVersion(m, text, ec)
	if err != nil {
		return nil, 0, 0, err
	}

	capacityBits := v.DataCodewords(ec) * 8

	bb := &bitBuffer{}
	// Header: 4-bit mode indicator + character count indicator.
	bb.appendBits(uint32(m.Indicator()), 4)
	bb.appendBits(uint32(len(text)), m.CharCountBits(v))
	// Payload.
	writePayload(bb, m, text)

	// Terminator: up to 4 zero bits, truncated if fewer remain before capacity.
	remaining := capacityBits - bb.bits()
	if remaining < 0 {
		return nil, 0, 0, fmt.Errorf("qrgen: internal error: encoded length exceeds capacity by %d bits", -remaining)
	}
	termBits := 4
	if remaining < termBits {
		termBits = remaining
	}
	bb.appendBits(0, termBits)

	// Bit padding: zero-fill to the next byte boundary.
	if pad := (8 - bb.bits()%8) % 8; pad > 0 {
		bb.appendBits(0, pad)
	}

	// Pad bytes: alternate 0xEC, 0x11 until capacity is filled.
	padBytes := [2]uint32{0xEC, 0x11}
	for i := 0; bb.bits() < capacityBits; i++ {
		bb.appendBits(padBytes[i%2], 8)
	}

	return bb.bytes(), v, m, nil
}
