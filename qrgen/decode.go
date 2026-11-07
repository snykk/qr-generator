// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoder for DecodeBytes
	_ "image/jpeg" // register JPEG decoder for DecodeBytes
	_ "image/png"  // register PNG decoder for DecodeBytes
	"strings"
)

// Decode reads a QR code or a Micro QR code from an image and returns the
// original text. The image stage (binarisation, finder-pattern detection,
// perspective transform, alignment refinement, module sampling) runs first;
// the resulting boolean module grid is then handed off to [DecodeMatrix] for
// the matrix stage.
//
// A Micro QR symbol has a single finder pattern. When no QR symbol decodes,
// Decode tries each finder in the image as a Micro finder, using its corners,
// the timing patterns beside it, and its format information to orient and size
// the symbol (see docs/theory/27-micro-qr-detection.md). An image that holds
// both kinds is read as its QR symbol.
//
// Any stage may fail with a typed sentinel error: [ErrFinderNotFound] if the
// image's finder patterns can't be located, [ErrInvalidVersion] if the
// finder spacing implies a version outside 1..40, [ErrFormatUnreadable] if
// the format-info strips can't be decoded, [ErrTooManyErrors] if any RS
// block exceeds its correction budget, or [ErrCorruptedPayload] if the
// recovered bit stream contains an unparseable segment header. For a Micro
// symbol whose timing pattern checks out, its own format or Reed-Solomon error
// is reported rather than [ErrFinderNotFound].
//
// See docs/theory/13-decoder-pipeline.md for the full stage-by-stage flow.
func Decode(img image.Image) (string, error) {
	return decodeImage(img)
}

// DecodeBytes is a convenience wrapper that decodes a PNG, JPEG, or GIF byte
// slice via [image.Decode] and runs [Decode] on the result. Use this when
// you have raw file bytes (e.g. an HTTP body or a freshly-read file) and
// don't want to manage the image decoder yourself.
func DecodeBytes(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return Decode(img)
}

// DecodeMatrix recovers the original text from a clean, top-down QR module
// grid (true = dark). The input must be square with side 21 + 4·(v − 1) for
// some valid v in [1, 40], or 11, 13, 15, or 17 for a Micro QR symbol, which
// is recognised by its size alone and decoded with Micro's own format
// information, single Reed-Solomon block (detection only in M1), and field
// widths (see docs/theory/26-micro-qr.md). A rectangular grid, h rows of w
// modules, is read as an rMQR symbol when h by w is one of its 32 sizes, with
// the version taken from the shape and the level from the nearer of its two
// format copies (see docs/theory/28-rmqr.md).
//
// The full pipeline runs in five stages (see docs/theory/13-decoder-pipeline.md):
//
//  1. Reconstruct the matrix and its functional/reserved areas.
//  2. Read format-info, brute-force BCH-decode → (EC level, mask).
//  3. Reverse the zig-zag walk and un-XOR the mask → interleaved codeword stream.
//  4. Deinterleave the per-block layout and run Reed–Solomon error correction
//     on each block.
//  5. Parse mode indicator + character count + payload per segment back into text.
//
// Any stage may fail with a typed sentinel error: [ErrFormatUnreadable] if
// the format-info strips are too corrupted, [ErrTooManyErrors] if any RS
// block exceeds its correction budget, or [ErrCorruptedPayload] if the
// recovered bit stream contains an unparseable segment header. Plain wrapping
// errors are returned for structural problems (invalid matrix size, etc.).
//
// Callers with an image rather than a module grid use [Decode] or
// [DecodeBytes], which locate and sample QR and Micro QR symbols themselves.
func DecodeMatrix(grid [][]bool) (string, error) {
	text, _, err := decodeMatrixSA(grid)
	return text, err
}

// decodeMatrixSA is DecodeMatrix plus the structured-append metadata: it returns
// the symbol's text and, when the symbol carries a structured-append header,
// its position/total/parity (nil otherwise). DecodeMatrix discards the metadata;
// DecodeStructuredAppend uses it to reassemble a set.
func decodeMatrixSA(grid [][]bool) (string, *saHeaderInfo, error) {
	// rMQR symbols are rectangular and no QR or Micro QR symbol is, so a
	// grid's shape alone routes it. rMQR has no structured append.
	if len(grid) > 0 && len(grid[0]) != len(grid) {
		text, err := decodeRMQRMatrix(grid)
		return text, nil, err
	}
	// Micro QR sides (11..17, odd) never coincide with QR sides (21 + 4k), so
	// the size alone routes the grid. Micro has no structured append.
	if microVersionForSize(len(grid)) != 0 {
		text, err := decodeMicroMatrix(grid)
		return text, nil, err
	}
	m, err := matrixFromGrid(grid)
	if err != nil {
		return "", nil, err
	}
	ec, mask, err := readFormatInfo(m)
	if err != nil {
		return "", nil, err
	}
	stream := readCodewordStream(m, mask)
	data, err := deinterleaveAndCorrect(stream, m.version, ec)
	if err != nil {
		return "", nil, err
	}
	return decodeText(data, m.version)
}

// DecodeStructuredAppend decodes a set of structured-append symbols (as PNG,
// JPEG, or GIF bytes, in any order) and returns the reassembled original text.
// It is the counterpart of [EncodeStructuredAppend]: each symbol is decoded, the
// symbols are ordered by their position, their chunks are concatenated, and the
// shared parity byte is verified against the reassembled text.
//
// It errors if a symbol is not part of a structured-append set, if the symbols
// disagree on the total count or parity, if a position is missing or duplicated,
// or if the parity check fails. The number of symbols supplied must equal the
// set's total count.
func DecodeStructuredAppend(symbols [][]byte) (string, error) {
	if len(symbols) == 0 {
		return "", fmt.Errorf("qrgen: DecodeStructuredAppend: no symbols provided")
	}

	type part struct {
		text string
		info *saHeaderInfo
	}
	decoded := make([]part, 0, len(symbols))
	for i, data := range symbols {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("qrgen: symbol %d: %w", i+1, err)
		}
		grid, err := imageToGrid(img)
		if err != nil {
			return "", fmt.Errorf("qrgen: symbol %d: %w", i+1, err)
		}
		text, info, err := decodeMatrixSA(grid)
		if err != nil {
			return "", fmt.Errorf("qrgen: symbol %d: %w", i+1, err)
		}
		if info == nil {
			return "", fmt.Errorf("qrgen: symbol %d is not a structured-append symbol", i+1)
		}
		decoded = append(decoded, part{text: text, info: info})
	}

	total := decoded[0].info.total
	parity := decoded[0].info.parity
	if len(symbols) != total {
		return "", fmt.Errorf("qrgen: got %d symbols but the set declares %d", len(symbols), total)
	}
	ordered := make([]string, total)
	seen := make([]bool, total)
	for i, p := range decoded {
		if p.info.total != total {
			return "", fmt.Errorf("qrgen: symbol %d reports total %d, expected %d", i+1, p.info.total, total)
		}
		if p.info.parity != parity {
			return "", fmt.Errorf("qrgen: symbol %d parity 0x%02X differs from the set parity 0x%02X", i+1, p.info.parity, parity)
		}
		pos := p.info.position
		if pos < 0 || pos >= total {
			return "", fmt.Errorf("qrgen: symbol %d position %d out of range for a set of %d", i+1, pos, total)
		}
		if seen[pos] {
			return "", fmt.Errorf("qrgen: duplicate structured-append position %d", pos)
		}
		seen[pos] = true
		ordered[pos] = p.text
	}

	result := strings.Join(ordered, "")
	if got := saParity(result); got != parity {
		return "", fmt.Errorf("qrgen: structured-append parity mismatch (reassembled 0x%02X, symbols report 0x%02X)", got, parity)
	}
	return result, nil
}
