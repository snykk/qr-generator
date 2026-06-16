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

// analyzeMode picks the most compact single mode that can encode every
// character in text. Numeric wins if all chars are digits; alphanumeric wins
// next; byte is the fallback. Empty input is reported as Numeric (it costs
// zero payload bits in either of the compact modes, but Numeric keeps later
// size math consistent).
//
// As of v0.6 the encoder no longer drives sizing from this single-mode choice;
// encodeText uses the DP-optimal segmenter (segmentText) instead, for which a
// homogeneous input yields exactly the segment analyzeMode would pick. This
// helper is retained for that homogeneous fast-reasoning and for tests.
// See docs/theory/02-data-encoding.md and docs/theory/17-optimal-segmentation.md.
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
// bits) is enough to hold the optimal mixed-mode segmentation of text at the
// given EC level. Because the optimal segmentation depends on the version
// (the character-count indicator width varies by version group), the
// segmentation is recomputed for each candidate version. Returns
// ErrCapacityExceeded if even V40 cannot hold the payload.
// See docs/theory/17-optimal-segmentation.md.
func selectVersion(text string, ec ECLevel) (Version, error) {
	// The optimal segmentation and its bit length depend on the version only
	// through CharCountBits, which is constant within each version group
	// (1-9, 10-26, 27-40). So recompute the segmentation only when the group
	// changes — three DP runs at most instead of forty.
	group := -1
	needed := 0
	for v := MinVersion; v <= MaxVersion; v++ {
		if g := versionGroup(v); g != group {
			group = g
			needed = segmentsBitLength(segmentText(text, v), v)
		}
		if needed <= v.DataCodewords(ec)*8 {
			return v, nil
		}
	}
	return 0, ErrCapacityExceeded
}

// versionGroup maps a version to its character-count-indicator group: 0 for
// versions 1-9, 1 for 10-26, 2 for 27-40. CharCountBits is constant within a
// group, so the optimal segmentation is too.
func versionGroup(v Version) int {
	switch {
	case v <= 9:
		return 0
	case v <= 26:
		return 1
	default:
		return 2
	}
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

// buildMatrix runs the M3..M6 pipeline end-to-end using the resolved options:
// encode the text, run Reed–Solomon, place the codewords in the matrix together
// with all functional patterns, write version info (V7+), then pick and apply
// the mask (auto-selected or forced via opts.mask) along with its matching
// format-info codeword. The returned matrix is the final, scannable QR symbol;
// callers only need to render it to pixels.
func buildMatrix(text string, opts *options) (*matrix, int, error) {
	data, v, err := encodeText(text, opts.ec, opts.version)
	if err != nil {
		return nil, 0, err
	}
	stream := rsEncode(data, v, opts.ec)
	m := newMatrix(v)
	m.placeFunctionalPatterns()
	if err := m.placeData(stream, v.RemainderBits()); err != nil {
		return nil, 0, err
	}
	m.writeVersionInfo()

	var mask int
	if opts.mask >= 0 {
		m.applyMask(opts.mask)
		m.writeFormatInfo(opts.ec, opts.mask)
		mask = opts.mask
	} else {
		mask = m.selectAndApplyMask(opts.ec)
	}
	return m, mask, nil
}

// encodeText runs the M3 pipeline end-to-end: it computes the optimal
// mixed-mode segmentation, picks (or accepts) a version that fits, then emits
// each segment's header + payload followed by a single shared terminator +
// bit padding + pad bytes to fill the data-codeword capacity exactly.
// forceVersion = 0 means auto-pick the smallest fitting version; any other
// value is validated against the segmented payload size.
//
// For a homogeneous input the segmentation is a single segment, so the output
// is byte-for-byte identical to the pre-segmentation single-mode encoder.
//
// See docs/theory/02-data-encoding.md, docs/theory/10-worked-example.md, and
// docs/theory/17-optimal-segmentation.md.
func encodeText(text string, ec ECLevel, forceVersion Version) (data []byte, v Version, err error) {
	if forceVersion == 0 {
		v, err = selectVersion(text, ec)
		if err != nil {
			return nil, 0, err
		}
	} else {
		if !forceVersion.IsValid() {
			return nil, 0, fmt.Errorf("qrgen: invalid version %d (want 1..40)", forceVersion)
		}
		needed := segmentsBitLength(segmentText(text, forceVersion), forceVersion)
		capacity := forceVersion.DataCodewords(ec) * 8
		if needed > capacity {
			return nil, 0, fmt.Errorf("qrgen: payload (%d bits) does not fit in V%d-%s (capacity %d bits)", needed, forceVersion, ec, capacity)
		}
		v = forceVersion
	}

	capacityBits := v.DataCodewords(ec) * 8

	bb := &bitBuffer{}
	// Each segment: 4-bit mode indicator + character-count indicator + payload.
	// len(s.text) is the correct count for every mode: numeric/alphanumeric
	// segments hold only single-byte ASCII (so byte length == character
	// count), and byte mode counts bytes by definition.
	for _, s := range segmentText(text, v) {
		bb.appendBits(uint32(s.mode.Indicator()), 4)
		bb.appendBits(uint32(len(s.text)), s.mode.CharCountBits(v))
		writePayload(bb, s.mode, s.text)
	}

	// Terminator: up to 4 zero bits, truncated if fewer remain before capacity.
	remaining := capacityBits - bb.bits()
	if remaining < 0 {
		return nil, 0, fmt.Errorf("qrgen: internal error: encoded length exceeds capacity by %d bits", -remaining)
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

	return bb.bytes(), v, nil
}
