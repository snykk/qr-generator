// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// genPoly returns the Reed–Solomon generator polynomial for n EC codewords:
//
//	g(x) = (x − α^0)(x − α^1) … (x − α^(n-1))
//
// multiplied out over GF(256). The result has length n+1, with the leading
// (x^n) coefficient at index 0 and a constant term at index n. Because
// subtraction equals addition in GF(2^8), we multiply by (x + α^i).
//
// See docs/theory/04-reed-solomon.md and docs/theory/09-data-tables.md §10.
func genPoly(n int) []uint8 {
	g := []uint8{1}
	for i := 0; i < n; i++ {
		g = polyMul(g, []uint8{1, expTable[i]})
	}
	return g
}

// encodeBlock returns the n EC codewords for a single block of data codewords,
// using the Reed–Solomon generator polynomial of degree n. The block is treated
// as the high-order part of a polynomial; the result is the remainder after
// polynomial long division by g(x).
//
// See docs/theory/04-reed-solomon.md.
func encodeBlock(data []uint8, n int) []uint8 {
	g := genPoly(n)
	dividend := make([]uint8, len(data)+n)
	copy(dividend, data)
	return polyMod(dividend, g)
}

// splitAndEncodeBlocks consumes the data-codeword byte stream for a given
// (version, EC level), splits it into the per-group blocks defined by the spec,
// and runs encodeBlock on each. It returns the data blocks in their natural
// per-group order followed by the matching EC blocks (one EC block per data
// block, in the same order).
//
// See docs/theory/04-reed-solomon.md and docs/theory/09-data-tables.md §7.
func splitAndEncodeBlocks(data []byte, v Version, ec ECLevel) (dataBlocks, ecBlocks [][]byte) {
	spec := v.ECBlocks(ec)
	totalBlocks := spec.TotalBlocks()

	dataBlocks = make([][]byte, 0, totalBlocks)
	ecBlocks = make([][]byte, 0, totalBlocks)

	offset := 0
	for _, group := range [2]ECBlockGroup{spec.Group1, spec.Group2} {
		for i := 0; i < group.Count; i++ {
			block := data[offset : offset+group.DataPerBlock]
			offset += group.DataPerBlock
			dataBlocks = append(dataBlocks, block)
			ecBlocks = append(ecBlocks, encodeBlock(block, spec.ECPerBlock))
		}
	}
	return dataBlocks, ecBlocks
}

// interleaveBlocks merges data blocks and EC blocks in column-major order, as
// required for placement in the QR matrix: column 0 of every data block first,
// then column 1 of every data block, and so on (skipping positions past a
// block's length when block sizes differ); afterwards the EC blocks are
// interleaved the same way.
//
// See docs/theory/04-reed-solomon.md.
func interleaveBlocks(dataBlocks, ecBlocks [][]byte) []byte {
	maxData := 0
	totalLen := 0
	for _, b := range dataBlocks {
		if len(b) > maxData {
			maxData = len(b)
		}
		totalLen += len(b)
	}
	maxEC := 0
	for _, b := range ecBlocks {
		if len(b) > maxEC {
			maxEC = len(b)
		}
		totalLen += len(b)
	}

	out := make([]byte, 0, totalLen)
	for col := 0; col < maxData; col++ {
		for _, block := range dataBlocks {
			if col < len(block) {
				out = append(out, block[col])
			}
		}
	}
	for col := 0; col < maxEC; col++ {
		for _, block := range ecBlocks {
			if col < len(block) {
				out = append(out, block[col])
			}
		}
	}
	return out
}

// rsEncode is the top-level Reed–Solomon stage: data codewords in, interleaved
// data + EC codewords out, ready to be written into the matrix. The input must
// have length v.DataCodewords(ec); the output has length equal to the
// version's total codeword count.
func rsEncode(data []byte, v Version, ec ECLevel) []byte {
	dataBlocks, ecBlocks := splitAndEncodeBlocks(data, v, ec)
	return interleaveBlocks(dataBlocks, ecBlocks)
}
