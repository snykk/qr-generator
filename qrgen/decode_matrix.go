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
	"strings"
)

// ErrCorruptedPayload is returned when the decoded byte stream contains a
// value that cannot represent a valid QR payload (unknown mode indicator,
// a character count exceeding capacity, an alphanumeric pair out of range,
// etc.).
var ErrCorruptedPayload = errors.New("qrgen: corrupted payload bit stream")

// matrixFromGrid constructs a *matrix from a [][]bool input, marking
// functional and reserved areas the same way the encoder does. The input
// side length must be 21 + 4·(v − 1) for some v in [1, 40]. Functional
// patterns in the input are overwritten with their canonical values; this is
// a no-op on a clean grid and silently corrects mild corruption on a noisy
// one. Format-info and version-info areas are left untouched (only marked
// reserved) so the downstream readers see the original bits.
func matrixFromGrid(grid [][]bool) (*matrix, error) {
	n := len(grid)
	if n < 21 || (n-21)%4 != 0 {
		return nil, fmt.Errorf("qrgen: matrixFromGrid: invalid matrix side %d", n)
	}
	v := Version((n-21)/4 + 1)
	if !v.IsValid() {
		return nil, fmt.Errorf("qrgen: matrixFromGrid: side %d gives invalid version %d", n, v)
	}
	for r, row := range grid {
		if len(row) != n {
			return nil, fmt.Errorf("qrgen: matrixFromGrid: row %d has %d cols, want %d", r, len(row), n)
		}
	}

	m := newMatrix(v)
	for r := 0; r < n; r++ {
		copy(m.modules[r], grid[r])
	}
	// placeFunctionalPatterns overwrites the finder/timing/alignment/dark-module
	// cells with their canonical values and marks them as reserved. It does NOT
	// touch format-info or version-info cells (those are only reserved, not
	// written), so the bits the decoder needs to read stay intact.
	m.placeFunctionalPatterns()
	return m, nil
}

// readCodewordStream reverses the M5 zig-zag walk: it visits the same
// sequence of cells as placeData and reads them into a bit stream, XOR-ing
// the chosen mask out of each data module along the way. Trailing remainder
// bits (per Version.RemainderBits) are discarded, leaving a byte stream
// whose length equals total codewords for the version. Functional patterns
// and format/version-info reserved cells are skipped, matching the encoder.
//
// The returned slice is the interleaved data || EC stream that the
// deinterleaving + RS stages (D6) consume.
//
// See docs/theory/05-matrix-construction.md and docs/theory/13-decoder-pipeline.md.
func readCodewordStream(m *matrix, mask int) []byte {
	n := m.size

	// Pre-count unreserved cells so we can size the output exactly.
	totalBits := 0
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			if !m.reserved[r][c] {
				totalBits++
			}
		}
	}

	bits := make([]bool, 0, totalBits)

	upward := true
	for col := n - 1; col > 0; col -= 2 {
		if col == 6 {
			col--
		}
		for i := 0; i < n; i++ {
			row := i
			if upward {
				row = n - 1 - i
			}
			for c := 0; c < 2; c++ {
				cc := col - c
				if m.reserved[row][cc] {
					continue
				}
				bit := m.modules[row][cc]
				if maskCondition(mask, row, cc) {
					bit = !bit
				}
				bits = append(bits, bit)
			}
		}
		upward = !upward
	}

	rem := m.version.RemainderBits()
	byteCount := (len(bits) - rem) / 8
	out := make([]byte, byteCount)
	for i := 0; i < byteCount*8; i++ {
		if bits[i] {
			out[i/8] |= 1 << uint(7-(i%8))
		}
	}
	return out
}

// deinterleaveBlocks reverses the column-major interleave performed by
// interleaveBlocks (M4): it consumes the codeword stream and splits it back
// into per-block (data, EC) pairs, sized according to v.ECBlocks(ec). Data
// blocks may have one of two lengths (group 1 vs group 2); EC blocks always
// share a uniform length.
//
// See docs/theory/04-reed-solomon.md and docs/theory/13-decoder-pipeline.md.
func deinterleaveBlocks(stream []byte, v Version, ec ECLevel) (dataBlocks, ecBlocks [][]byte, err error) {
	spec := v.ECBlocks(ec)
	blockCount := spec.TotalBlocks()
	if blockCount == 0 {
		return nil, nil, fmt.Errorf("qrgen: deinterleaveBlocks: V%d-%s has no blocks", v, ec)
	}

	dataLengths := make([]int, 0, blockCount)
	for range spec.Group1.Count {
		dataLengths = append(dataLengths, spec.Group1.DataPerBlock)
	}
	for range spec.Group2.Count {
		dataLengths = append(dataLengths, spec.Group2.DataPerBlock)
	}

	maxDataLen := max(spec.Group1.DataPerBlock, spec.Group2.DataPerBlock)
	ecLen := spec.ECPerBlock

	expected := spec.TotalDataCodewords() + spec.TotalECCodewords()
	if len(stream) != expected {
		return nil, nil, fmt.Errorf("qrgen: deinterleaveBlocks: stream has %d bytes, want %d", len(stream), expected)
	}

	dataBlocks = make([][]byte, blockCount)
	ecBlocks = make([][]byte, blockCount)
	for i := range blockCount {
		dataBlocks[i] = make([]byte, dataLengths[i])
		ecBlocks[i] = make([]byte, ecLen)
	}

	pos := 0
	for col := range maxDataLen {
		for b := range blockCount {
			if col < len(dataBlocks[b]) {
				dataBlocks[b][col] = stream[pos]
				pos++
			}
		}
	}
	for col := range ecLen {
		for b := range blockCount {
			ecBlocks[b][col] = stream[pos]
			pos++
		}
	}
	return dataBlocks, ecBlocks, nil
}

// bitReader walks a byte slice one bit at a time, MSB first within each
// byte. Mirrors the encoder's bitBuffer in reverse: the encoder wrote with
// AppendBits, the decoder reads back with ReadBits.
type bitReader struct {
	data []byte
	pos  int // bit position
}

func newBitReader(data []byte) *bitReader { return &bitReader{data: data} }

func (b *bitReader) bitsRemaining() int { return len(b.data)*8 - b.pos }

// readBits consumes n bits (1..32) and returns them as the low n bits of the
// result. Returns ErrCorruptedPayload when there aren't enough bits left.
func (b *bitReader) readBits(n int) (uint32, error) {
	if n < 0 || n > 32 {
		return 0, fmt.Errorf("qrgen: bitReader.readBits: invalid n=%d", n)
	}
	if b.pos+n > len(b.data)*8 {
		return 0, ErrCorruptedPayload
	}
	var result uint32
	for range n {
		bytePos := b.pos / 8
		bitPos := 7 - (b.pos % 8)
		bit := uint32((b.data[bytePos] >> uint(bitPos)) & 1)
		result = (result << 1) | bit
		b.pos++
	}
	return result, nil
}

// alphanumericTable maps an alphanumeric value (0..44) back to its character,
// the inverse of alphanumericValue in encode.go.
var alphanumericTable = []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:")

// decodeNumeric reads `count` numeric characters from br, in groups of 3
// digits (10 bits) with a 7-bit or 4-bit tail for 2 or 1 trailing digits.
func decodeNumeric(br *bitReader, count int) (string, error) {
	var out strings.Builder
	for count >= 3 {
		v, err := br.readBits(10)
		if err != nil {
			return "", err
		}
		if v > 999 {
			return "", ErrCorruptedPayload
		}
		fmt.Fprintf(&out, "%03d", v)
		count -= 3
	}
	if count == 2 {
		v, err := br.readBits(7)
		if err != nil {
			return "", err
		}
		if v > 99 {
			return "", ErrCorruptedPayload
		}
		fmt.Fprintf(&out, "%02d", v)
	} else if count == 1 {
		v, err := br.readBits(4)
		if err != nil {
			return "", err
		}
		if v > 9 {
			return "", ErrCorruptedPayload
		}
		fmt.Fprintf(&out, "%d", v)
	}
	return out.String(), nil
}

// decodeAlphanumeric reads `count` characters from br, in pairs of 11 bits
// (a*45 + b) plus an optional 6-bit single-character tail.
func decodeAlphanumeric(br *bitReader, count int) (string, error) {
	var out strings.Builder
	for count >= 2 {
		v, err := br.readBits(11)
		if err != nil {
			return "", err
		}
		if v >= 45*45 {
			return "", ErrCorruptedPayload
		}
		out.WriteByte(alphanumericTable[v/45])
		out.WriteByte(alphanumericTable[v%45])
		count -= 2
	}
	if count == 1 {
		v, err := br.readBits(6)
		if err != nil {
			return "", err
		}
		if v >= 45 {
			return "", ErrCorruptedPayload
		}
		out.WriteByte(alphanumericTable[v])
	}
	return out.String(), nil
}

// decodeByteMode reads `count` raw 8-bit bytes from br and returns them as a
// string. The bytes are assumed to be UTF-8 (the encoder's convention); we do
// not validate UTF-8 here since the spec allows arbitrary 8-bit payloads.
func decodeByteMode(br *bitReader, count int) (string, error) {
	buf := make([]byte, count)
	for i := range buf {
		v, err := br.readBits(8)
		if err != nil {
			return "", err
		}
		buf[i] = byte(v)
	}
	return string(buf), nil
}

// decodeText parses the data codeword stream produced by D6 back into the
// original text. Multiple segments are supported (in theory): the loop
// continues until a terminator or until fewer than 4 bits remain.
//
// See docs/theory/02-data-encoding.md and docs/theory/13-decoder-pipeline.md.
func decodeText(data []byte, v Version) (string, error) {
	br := newBitReader(data)
	var out strings.Builder

	for br.bitsRemaining() >= 4 {
		modeBits, err := br.readBits(4)
		if err != nil {
			return "", err
		}
		switch modeBits {
		case 0b0000:
			// Terminator.
			return out.String(), nil
		case 0b0001:
			countWidth := ModeNumeric.CharCountBits(v)
			count, err := br.readBits(countWidth)
			if err != nil {
				return "", err
			}
			seg, err := decodeNumeric(br, int(count))
			if err != nil {
				return "", err
			}
			out.WriteString(seg)
		case 0b0010:
			countWidth := ModeAlphanumeric.CharCountBits(v)
			count, err := br.readBits(countWidth)
			if err != nil {
				return "", err
			}
			seg, err := decodeAlphanumeric(br, int(count))
			if err != nil {
				return "", err
			}
			out.WriteString(seg)
		case 0b0100:
			countWidth := ModeByte.CharCountBits(v)
			count, err := br.readBits(countWidth)
			if err != nil {
				return "", err
			}
			seg, err := decodeByteMode(br, int(count))
			if err != nil {
				return "", err
			}
			out.WriteString(seg)
		default:
			return "", fmt.Errorf("%w: unknown mode indicator 0b%04b", ErrCorruptedPayload, modeBits)
		}
	}
	return out.String(), nil
}

// deinterleaveAndCorrect runs deinterleaveBlocks then rsDecode on every
// block, returning the concatenated corrected data codewords in their
// natural per-group order. If any block exceeds RS capacity, the wrapped
// error is returned (ErrTooManyErrors via errors.Is).
func deinterleaveAndCorrect(stream []byte, v Version, ec ECLevel) ([]byte, error) {
	dataBlocks, ecBlocks, err := deinterleaveBlocks(stream, v, ec)
	if err != nil {
		return nil, err
	}
	var out []byte
	for i, dataBlock := range dataBlocks {
		ecBlock := ecBlocks[i]
		combined := make([]byte, 0, len(dataBlock)+len(ecBlock))
		combined = append(combined, dataBlock...)
		combined = append(combined, ecBlock...)
		corrected, err := rsDecode(combined, len(ecBlock))
		if err != nil {
			return nil, fmt.Errorf("qrgen: block %d: %w", i, err)
		}
		out = append(out, corrected...)
	}
	return out, nil
}
