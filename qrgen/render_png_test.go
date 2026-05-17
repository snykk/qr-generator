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
	"image/color"
	"image/png"
	"testing"
)

func TestRenderPNGHelloWorld(t *testing.T) {
	m, _, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	out, err := renderPNG(m, defaultRenderOptions())
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty PNG output")
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png.Decode round-trip: %v", err)
	}

	// V1 (21 modules) + 2 * 4 quiet zone = 29 modules. 8 px each = 232 px side.
	wantSide := 232
	b := img.Bounds()
	if b.Dx() != wantSide || b.Dy() != wantSide {
		t.Errorf("image size = %dx%d, want %dx%d", b.Dx(), b.Dy(), wantSide, wantSide)
	}

	// Top-left quiet-zone pixel must be white.
	if !pixelEqual(img.At(0, 0), color.White) {
		t.Errorf("quiet-zone pixel (0,0) = %v, want white", img.At(0, 0))
	}
	// Top-right quiet-zone pixel must be white.
	if !pixelEqual(img.At(wantSide-1, 0), color.White) {
		t.Errorf("quiet-zone pixel (top-right) = %v, want white", img.At(wantSide-1, 0))
	}

	// The top-left finder's outer corner sits at matrix (0,0); in pixels this
	// is the cell starting at (quietZone*moduleSize, quietZone*moduleSize).
	// Module (0,0) is dark (the outer ring of the finder), so the centre of
	// the pixel block must be foreground.
	finderX := 4*8 + 4 // quietZone(4)*moduleSize(8) + half a module
	finderY := finderX
	if !pixelEqual(img.At(finderX, finderY), color.Black) {
		t.Errorf("finder corner pixel = %v, want black", img.At(finderX, finderY))
	}
}

func TestRenderPNGCustomColors(t *testing.T) {
	m, _, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	opts := defaultRenderOptions()
	opts.foreground = color.RGBA{R: 0x10, G: 0x40, B: 0x80, A: 0xFF} // navy-ish
	opts.background = color.RGBA{R: 0xFF, G: 0xF8, B: 0xE0, A: 0xFF} // cream
	out, err := renderPNG(m, opts)
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}

	// Quiet-zone pixel must match the custom background.
	got := img.At(0, 0)
	if !pixelEqual(got, opts.background) {
		t.Errorf("quiet-zone pixel = %v, want %v", got, opts.background)
	}

	// Finder corner pixel must match the custom foreground.
	finderX := 4*8 + 4
	finderY := finderX
	got = img.At(finderX, finderY)
	if !pixelEqual(got, opts.foreground) {
		t.Errorf("finder corner pixel = %v, want %v", got, opts.foreground)
	}
}

func TestRenderPNGModuleSizeAndQuietZone(t *testing.T) {
	m, _, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	opts := defaultRenderOptions()
	opts.moduleSize = 12
	opts.quietZone = 2
	out, err := renderPNG(m, opts)
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	// 21 + 2*2 = 25 modules, × 12 px = 300 px side.
	want := 300
	b := img.Bounds()
	if b.Dx() != want || b.Dy() != want {
		t.Errorf("image size = %dx%d, want %dx%d", b.Dx(), b.Dy(), want, want)
	}
}

// TestRenderPNGMatrixDarkPixelsMatchModules samples every module's centre pixel
// against the matrix entry, verifying the renderer's coordinate mapping for a
// V1 symbol.
func TestRenderPNGMatrixDarkPixelsMatchModules(t *testing.T) {
	m, _, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	opts := defaultRenderOptions()
	out, err := renderPNG(m, opts)
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	for r := 0; r < m.size; r++ {
		for c := 0; c < m.size; c++ {
			cx := (c+opts.quietZone)*opts.moduleSize + opts.moduleSize/2
			cy := (r+opts.quietZone)*opts.moduleSize + opts.moduleSize/2
			pix := img.At(cx, cy)
			wantDark := m.modules[r][c]
			isDark := pixelEqual(pix, color.Black)
			if isDark != wantDark {
				t.Errorf("module (%d,%d) dark=%v but pixel (%d,%d)=%v",
					r, c, wantDark, cx, cy, pix)
			}
		}
	}
}

func TestRenderPNGUsesGrayForDefault(t *testing.T) {
	m, _, err := buildMatrix("HELLO WORLD", defaultsForEC(ECLevelM))
	if err != nil {
		t.Fatalf("buildMatrix: %v", err)
	}
	out, err := renderPNG(m, defaultRenderOptions())
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if _, ok := img.(*image.Gray); !ok {
		t.Errorf("default render produced %T, want *image.Gray for smaller PNG", img)
	}
}

// pixelEqual compares two colours after converting both to RGBA64, the
// canonical form used by image/color comparisons.
func pixelEqual(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
