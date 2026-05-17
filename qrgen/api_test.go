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
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeDefault(t *testing.T) {
	data, err := Encode("HELLO WORLD")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	// V1 + 4 quiet * 2 = 29 modules * 8 px = 232 px side.
	if b := img.Bounds(); b.Dx() != 232 || b.Dy() != 232 {
		t.Errorf("default image = %dx%d, want 232x232", b.Dx(), b.Dy())
	}
}

func TestEncodeOptions(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
		want int // expected pixel side
	}{
		{
			name: "L bigger modules",
			opts: []Option{WithECLevel(ECLevelL), WithModuleSize(12)},
			want: 12 * (21 + 2*4), // V1 still, 12 px modules
		},
		{
			name: "no quiet zone",
			opts: []Option{WithQuietZone(0)},
			want: 8 * 21,
		},
		{
			name: "force V2",
			opts: []Option{WithVersion(2)},
			want: 8 * (25 + 2*4),
		},
		{
			name: "force mask 3",
			opts: []Option{WithMask(3)},
			want: 8 * (21 + 2*4),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := Encode("HELLO WORLD", c.opts...)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("png.Decode: %v", err)
			}
			if b := img.Bounds(); b.Dx() != c.want || b.Dy() != c.want {
				t.Errorf("got %dx%d, want %dx%d", b.Dx(), b.Dy(), c.want, c.want)
			}
		})
	}
}

func TestEncodeRejectsBadOptions(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
	}{
		{"invalid EC", []Option{WithECLevel(ECLevel(99))}},
		{"version out of range", []Option{WithVersion(Version(99))}},
		{"mask out of range", []Option{WithMask(99)}},
		{"zero module size", []Option{WithModuleSize(0)}},
		{"negative module size", []Option{WithModuleSize(-1)}},
		{"negative quiet zone", []Option{WithQuietZone(-1)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Encode("HELLO", c.opts...); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestEncodeForcedVersionTooSmall(t *testing.T) {
	// 600 alphanumeric characters cannot fit in V1.
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'A'
	}
	if _, err := Encode(string(long), WithVersion(1), WithECLevel(ECLevelM)); err == nil {
		t.Error("expected error for oversize payload at forced V1, got nil")
	}
}

func TestEncodeToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	if err := EncodeToFile("HELLO WORLD", path); err != nil {
		t.Fatalf("EncodeToFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
	// Round-trip decode to confirm it really is a PNG.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("written file is not a valid PNG: %v", err)
	}
}

func TestMatrix(t *testing.T) {
	out, err := Matrix("HELLO WORLD")
	if err != nil {
		t.Fatalf("Matrix: %v", err)
	}
	// V1 is 21x21.
	if len(out) != 21 {
		t.Fatalf("matrix side = %d, want 21", len(out))
	}
	for r, row := range out {
		if len(row) != 21 {
			t.Errorf("row %d length = %d, want 21", r, len(row))
		}
	}
	// Top-left finder corner must be dark (outer ring).
	if !out[0][0] {
		t.Error("(0,0) should be dark for top-left finder corner")
	}
}

func TestEncodeCustomColors(t *testing.T) {
	fg := color.RGBA{R: 0x00, G: 0x33, B: 0x99, A: 0xFF}
	bg := color.RGBA{R: 0xFF, G: 0xFF, B: 0xE0, A: 0xFF}
	data, err := Encode("HELLO WORLD", WithColors(fg, bg))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	// Quiet zone pixel must be the custom background.
	gr, gg, gb, ga := img.At(0, 0).RGBA()
	wr, wg, wb, wa := bg.RGBA()
	if gr != wr || gg != wg || gb != wb || ga != wa {
		t.Errorf("quiet zone (0,0) = %v, want %v", img.At(0, 0), bg)
	}
}
