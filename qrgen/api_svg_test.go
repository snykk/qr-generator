// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"encoding/xml"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// parseSVGPathToGrid reconstructs the boolean module grid from a rendered SVG.
// It reads the viewBox to recover the canvas dimension D, derives the quiet
// zone q = (D - n) / 2 from the known matrix side n, then walks every "M x y"
// subpath in the path data and sets grid[y-q][x-q]. This closes a loop
// analogous to the decoder round-trip: encode -> SVG -> grid must equal the
// matrix the encoder produced.
func parseSVGPathToGrid(t *testing.T, data []byte, n int) [][]bool {
	t.Helper()
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("SVG not well-formed: %v", err)
	}
	// viewBox is "0 0 D D".
	fields := strings.Fields(doc.ViewBox)
	if len(fields) != 4 {
		t.Fatalf("viewBox = %q, want four fields", doc.ViewBox)
	}
	dim, err := strconv.Atoi(fields[2])
	if err != nil {
		t.Fatalf("viewBox dim %q not an int: %v", fields[2], err)
	}
	if (dim-n)%2 != 0 {
		t.Fatalf("dim %d and n %d imply a non-integer quiet zone", dim, n)
	}
	q := (dim - n) / 2

	grid := make([][]bool, n)
	for i := range grid {
		grid[i] = make([]bool, n)
	}
	for _, seg := range strings.Split(doc.Path.D, "M") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		// seg looks like "4 4h1v1h-1z"; the coordinates precede the first 'h'.
		hi := strings.IndexByte(seg, 'h')
		if hi < 0 {
			t.Fatalf("malformed subpath %q", seg)
		}
		parts := strings.Fields(seg[:hi])
		if len(parts) != 2 {
			t.Fatalf("subpath coords %q not two numbers", seg[:hi])
		}
		x, err1 := strconv.Atoi(parts[0])
		y, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			t.Fatalf("subpath coords %q not integers", seg[:hi])
		}
		col, row := x-q, y-q
		if row < 0 || row >= n || col < 0 || col >= n {
			t.Fatalf("module (%d,%d) outside the %dx%d grid (quiet zone %d)", row, col, n, n, q)
		}
		grid[row][col] = true
	}
	return grid
}

func TestEncodeSVGRoundTripGrid(t *testing.T) {
	cases := []struct {
		name string
		text string
		opts []Option
	}{
		{"v1-m default", "HELLO WORLD", nil},
		{"url ec-q", "https://github.com/snykk/qr-generator", []Option{WithECLevel(ECLevelQ)}},
		{"numeric small quiet", "12345678", []Option{WithQuietZone(2)}},
		{"multiblock ec-h", strings.Repeat("ABC123", 10), []Option{WithECLevel(ECLevelH)}},
		{"custom colours", "STYLED", []Option{WithModuleSize(12), WithColors(rgb(0x10, 0x2E, 0x57), rgb(0xFF, 0xF8, 0xE7))}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want, err := Matrix(c.text, c.opts...)
			if err != nil {
				t.Fatalf("Matrix: %v", err)
			}
			data, err := EncodeSVG(c.text, c.opts...)
			if err != nil {
				t.Fatalf("EncodeSVG: %v", err)
			}
			got := parseSVGPathToGrid(t, data, len(want))
			for r := range want {
				for col := range want[r] {
					if got[r][col] != want[r][col] {
						t.Fatalf("module (%d,%d): SVG has dark=%v, matrix has %v", r, col, got[r][col], want[r][col])
					}
				}
			}
		})
	}
}

func TestEncodeSVGModuleSizePropagates(t *testing.T) {
	// Default quiet zone 4; V1 side 21 -> dim 29. moduleSize 16 -> width 464.
	data, err := EncodeSVG("HI", WithModuleSize(16))
	if err != nil {
		t.Fatalf("EncodeSVG: %v", err)
	}
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed: %v", err)
	}
	if doc.Width != "464" || doc.Height != "464" {
		t.Errorf("width/height = %s/%s, want 464/464", doc.Width, doc.Height)
	}
}

func TestEncodeSVGToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "qr.svg")
	if err := EncodeSVGToFile("HELLO", path); err != nil {
		t.Fatalf("EncodeSVGToFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("written file is not well-formed SVG: %v", err)
	}
	if doc.Path.Fill == "" {
		t.Error("written SVG has no foreground path fill")
	}
}

func TestEncodeSVGInvalidOptionErrors(t *testing.T) {
	// An out-of-range version is rejected at validate(), before rendering.
	if _, err := EncodeSVG("HELLO", WithVersion(99)); err == nil {
		t.Error("expected error for invalid version, got nil")
	}
}

// rgb is a tiny helper for opaque RGBA colours in tests.
func rgb(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 0xFF} }
