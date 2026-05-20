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
	"image"
	_ "image/gif"  // register GIF decoder for DecodeBytes
	_ "image/jpeg" // register JPEG decoder for DecodeBytes
	_ "image/png"  // register PNG decoder for DecodeBytes
)

// Decode reads a QR code from an image and returns the original text. The
// image stage (binarisation, finder-pattern detection, perspective transform,
// alignment refinement, module sampling) runs first; the resulting boolean
// module grid is then handed off to [DecodeMatrix] for the matrix stage.
//
// Any stage may fail with a typed sentinel error: [ErrFinderNotFound] if the
// image's finder patterns can't be located, [ErrInvalidVersion] if the
// finder spacing implies a version outside 1..40, [ErrFormatUnreadable] if
// the format-info strips can't be decoded, [ErrTooManyErrors] if any RS
// block exceeds its correction budget, or [ErrCorruptedPayload] if the
// recovered bit stream contains an unparseable segment header.
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
// some valid v in [1, 40].
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
// Image-based decoding (`Decode`) is planned for later milestones; for now
// callers with a real image must produce the matrix themselves.
func DecodeMatrix(grid [][]bool) (string, error) {
	m, err := matrixFromGrid(grid)
	if err != nil {
		return "", err
	}
	ec, mask, err := readFormatInfo(m)
	if err != nil {
		return "", err
	}
	stream := readCodewordStream(m, mask)
	data, err := deinterleaveAndCorrect(stream, m.version, ec)
	if err != nil {
		return "", err
	}
	return decodeText(data, m.version)
}
