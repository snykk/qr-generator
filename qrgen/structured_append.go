// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// saHeaderBits is the fixed size of a structured-append header: the 4-bit mode
// indicator, the 8-bit symbol sequence indicator, and the 8-bit parity byte.
// See docs/theory/21-structured-append.md.
const saHeaderBits = 4 + 8 + 8

// maxStructuredAppendSymbols is the spec cap: the count nibble is 4 bits.
const maxStructuredAppendSymbols = 16

// saHeaderInfo carries the structured-append metadata a decoder recovers from a
// symbol's header: its 0-based position, the total symbol count, and the parity
// byte shared across the set. decodeText returns a nil *saHeaderInfo for a
// symbol with no structured-append header.
type saHeaderInfo struct {
	position int
	total    int
	parity   byte
}

// saSequenceIndicator packs the 0-based symbol position and the total symbol
// count into the 8-bit sequence indicator: high nibble = position, low nibble =
// count minus one. Callers guarantee position and total-1 each fit 4 bits.
func saSequenceIndicator(position, total int) uint8 {
	return uint8(position)<<4 | uint8(total-1)&0x0F
}

// saParseSequence unpacks a sequence indicator into the 0-based position and the
// total symbol count (the count nibble stores total minus one).
func saParseSequence(b uint8) (position, total int) {
	return int(b >> 4), int(b&0x0F) + 1
}

// saParity is the structured-append parity: the XOR of every byte of the
// original (undivided) message. Every symbol in a set carries this value.
func saParity(text string) byte {
	var p byte
	for i := 0; i < len(text); i++ {
		p ^= text[i]
	}
	return p
}

// splitRunes divides text into parts contiguous chunks on rune boundaries, as
// evenly as possible; the first (rune count mod parts) chunks get one extra
// rune. Concatenating the chunks reproduces text exactly, and no multi-byte
// rune is ever split.
func splitRunes(text string, parts int) []string {
	runes := []rune(text)
	n := len(runes)
	out := make([]string, parts)
	base, rem := n/parts, n%parts
	idx := 0
	for i := 0; i < parts; i++ {
		size := base
		if i < rem {
			size++
		}
		out[i] = string(runes[idx : idx+size])
		idx += size
	}
	return out
}

// selectCommonVersion returns the smallest version whose data-codeword capacity
// holds every chunk (each with its structured-append header) at ec. All symbols
// in a set share one version so the set is uniform. Like selectVersion it walks
// versions but recomputes the segmentation only per version group.
func selectCommonVersion(chunks []string, ec ECLevel) (Version, error) {
	group := -1
	needed := 0 // max over chunks of header + segment bits at the current group
	for v := MinVersion; v <= MaxVersion; v++ {
		if g := versionGroup(v); g != group {
			group = g
			needed = 0
			for _, chunk := range chunks {
				n := saHeaderBits + segmentsBitLength(segmentText(chunk, v), v)
				if n > needed {
					needed = n
				}
			}
		}
		if needed <= v.DataCodewords(ec)*8 {
			return v, nil
		}
	}
	return 0, ErrCapacityExceeded
}

// encodeSASymbol encodes one symbol of a structured-append set at the forced
// common version v: the structured-append header (mode 0011 + sequence
// indicator + parity) followed by the chunk's mixed-mode segments, padded to
// the data-codeword capacity. It returns the data codewords ready for
// matrixFromData.
func encodeSASymbol(chunk string, ec ECLevel, v Version, position, total int, parity byte) ([]byte, error) {
	capacityBits := v.DataCodewords(ec) * 8
	bb := &bitBuffer{}
	// Structured-append header, first in the symbol.
	bb.appendBits(uint32(ModeStructuredAppend.Indicator()), 4)
	bb.appendBits(uint32(saSequenceIndicator(position, total)), 8)
	bb.appendBits(uint32(parity), 8)
	// The chunk's own mixed-mode segments (default UTF-8; no ECI in v0.10).
	for _, s := range segmentText(chunk, v) {
		bb.appendBits(uint32(s.mode.Indicator()), 4)
		bb.appendBits(uint32(len(s.text)), s.mode.CharCountBits(v))
		writePayload(bb, s.mode, s.text)
	}
	return padToCapacity(bb, capacityBits)
}
