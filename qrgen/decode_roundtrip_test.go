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
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// fillColor returns an opaque RGB grey at the given luminance, used by the
// low-contrast and inverted-feel robustness tests.
func fillColor(v uint8) color.Color {
	return color.RGBA{R: v, G: v, B: v, A: 0xFF}
}

// encodeBlankPNG returns a side×side all-white PNG byte slice. Used to drive
// the decoder past the binarise stage with no finders present.
func encodeBlankPNG(side int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TestRoundTripWithOwnDecoder mirrors the 12-case matrix in roundtrip_test.go
// (which uses gozxing) but exercises our own DecodeBytes — closing the loop
// without any third-party dependency. This is D13's primary cross-validation.
func TestRoundTripWithOwnDecoder(t *testing.T) {
	cases := []struct {
		name string
		text string
		opts []Option
	}{
		// Alphanumeric mode at V1, every EC level.
		{"alphanumeric L", "HELLO WORLD", []Option{WithECLevel(ECLevelL)}},
		{"alphanumeric M", "HELLO WORLD", []Option{WithECLevel(ECLevelM)}},
		{"alphanumeric Q", "HELLO WORLD", []Option{WithECLevel(ECLevelQ)}},
		{"alphanumeric H", "HELLO WORLD", []Option{WithECLevel(ECLevelH)}},
		// Numeric mode.
		{"numeric short", "12345", []Option{WithECLevel(ECLevelM)}},
		{"numeric 20 digits", "01234567890123456789", []Option{WithECLevel(ECLevelL)}},
		// Byte mode: lowercase / punctuation forces byte.
		{"byte mixed case", "Hello, World!", []Option{WithECLevel(ECLevelM)}},
		{"byte URL", "https://github.com/snykk/qr-generator", []Option{WithECLevel(ECLevelM)}},
		// UTF-8 multi-byte exercises the implicit-UTF8 assumption.
		{"byte utf8", "café résumé", []Option{WithECLevel(ECLevelM)}},
		// Larger versions exercise multi-block Reed-Solomon and alignment.
		{"V5 multi-block Q", strings.Repeat("ABC123", 10), []Option{WithECLevel(ECLevelQ)}},
		{"V10 long byte L", strings.Repeat("The quick brown fox. ", 12), []Option{WithECLevel(ECLevelL)}},
		// Forced version + mask exercises the override paths in buildMatrix.
		{"forced V2 mask 3", "HELLO WORLD", []Option{WithVersion(2), WithMask(3)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pngBytes, err := Encode(c.text, c.opts...)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := DecodeBytes(pngBytes)
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}
			if got != c.text {
				t.Errorf("round-trip mismatch:\n got  %q\n want %q", got, c.text)
			}
		})
	}
}

// TestRoundTripRobustnessFlippedBits randomly flips a handful of modules in
// the encoded matrix and confirms DecodeMatrix still recovers the source text.
// The flip count stays well below the per-block RS budget at EC-Q.
func TestRoundTripRobustnessFlippedBits(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		opts      []Option
		flipCells [][2]int // (row, col) cells to flip in the matrix
	}{
		{
			name: "V1-Q tolerates 3 data flips",
			text: "HELLO WORLD",
			opts: []Option{WithECLevel(ECLevelQ)},
			// Flip 3 cells in the data area (rows 9..13, away from finders/format).
			flipCells: [][2]int{{9, 9}, {11, 11}, {13, 12}},
		},
		{
			name: "V1-H tolerates 5 data flips",
			text: "HELLO WORLD",
			opts: []Option{WithECLevel(ECLevelH)},
			flipCells: [][2]int{{9, 9}, {11, 11}, {13, 12}, {9, 13}, {11, 15}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grid, err := Matrix(c.text, c.opts...)
			if err != nil {
				t.Fatalf("Matrix: %v", err)
			}
			for _, cell := range c.flipCells {
				grid[cell[0]][cell[1]] = !grid[cell[0]][cell[1]]
			}
			got, err := DecodeMatrix(grid)
			if err != nil {
				t.Fatalf("DecodeMatrix: %v", err)
			}
			if got != c.text {
				t.Errorf("got %q, want %q", got, c.text)
			}
		})
	}
}

// TestRoundTripImageRobustness exercises the image stage with non-default
// render knobs to confirm the decoder copes with varied module sizes, quiet
// zones, and colour pairs.
func TestRoundTripImageRobustness(t *testing.T) {
	const text = "qrgen v0.2 image robustness"
	cases := []struct {
		name string
		opts []Option
	}{
		{"default", nil},
		{"small modules", []Option{WithModuleSize(4)}},
		{"large modules", []Option{WithModuleSize(16)}},
		{"large quiet zone", []Option{WithQuietZone(10)}},
		{"low-contrast greys", []Option{WithColors(
			fillColor(0x30),
			fillColor(0xD0),
		)}},
		{"reversed-feeling colours", []Option{WithColors(
			fillColor(0x10),
			fillColor(0xEF),
		)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pngBytes, err := Encode(text, c.opts...)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := DecodeBytes(pngBytes)
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}
			if got != text {
				t.Errorf("got %q, want %q", got, text)
			}
		})
	}
}

// TestRoundTripPNGOnlyDecoderConsumesOurOutput sanity-checks that the
// standard library's png.Decode happily reads our output before DecodeBytes
// runs the image pipeline. Cheap defence against rendering regressions.
func TestRoundTripPNGOnlyDecoderConsumesOurOutput(t *testing.T) {
	pngBytes, err := Encode("HELLO WORLD")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(pngBytes)); err != nil {
		t.Errorf("standard library png.Decode rejected our output: %v", err)
	}
}

// TestDecodeBytesSurfacesFinderError validates that an image with no QR at
// all bubbles up ErrFinderNotFound rather than panicking or returning empty
// text.
func TestDecodeBytesSurfacesFinderError(t *testing.T) {
	// Render a tiny all-white PNG so binarise produces no finders.
	pngBytes, err := encodeBlankPNG(40)
	if err != nil {
		t.Fatalf("encodeBlankPNG: %v", err)
	}
	_, err = DecodeBytes(pngBytes)
	if !errors.Is(err, ErrFinderNotFound) {
		t.Errorf("got %v, want ErrFinderNotFound", err)
	}
}
