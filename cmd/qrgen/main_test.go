// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package main

import (
	"bytes"
	"encoding/xml"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/snykk/qr-generator/qrgen"
)

func TestParseECLevel(t *testing.T) {
	cases := []struct {
		in      string
		want    qrgen.ECLevel
		wantErr bool
	}{
		{"L", qrgen.ECLevelL, false},
		{"M", qrgen.ECLevelM, false},
		{"Q", qrgen.ECLevelQ, false},
		{"H", qrgen.ECLevelH, false},
		{"l", qrgen.ECLevelL, false},    // lowercase
		{"  q  ", qrgen.ECLevelQ, false}, // whitespace
		{"X", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseECLevel(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("parseECLevel(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("parseECLevel(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		fb      color.Color
		want    color.RGBA
		wantErr bool
	}{
		{"empty falls back", "", color.White, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, false},
		{"6 digits hash", "#000000", color.White, color.RGBA{0, 0, 0, 0xFF}, false},
		{"6 digits no hash", "FFFFFF", color.Black, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, false},
		{"6 digits mixed case", "#10aB80", color.Black, color.RGBA{0x10, 0xAB, 0x80, 0xFF}, false},
		{"8 digits with alpha", "#10204080", color.Black, color.RGBA{0x10, 0x20, 0x40, 0x80}, false},
		{"bad length", "#12345", color.Black, color.RGBA{}, true},
		{"bad digit", "#zz0000", color.Black, color.RGBA{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseHexColor(c.in, c.fb)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			// For empty input fallback, just compare to the fallback.
			if c.in == "" {
				if got != c.fb {
					t.Errorf("empty input returned %v, want fallback %v", got, c.fb)
				}
				return
			}
			gotRGBA, ok := got.(color.RGBA)
			if !ok {
				t.Fatalf("got %T, want color.RGBA", got)
			}
			if gotRGBA != c.want {
				t.Errorf("got %v, want %v", gotRGBA, c.want)
			}
		})
	}
}

func TestRunBasicWritesPNG(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "qr.png")
	cfg := cliConfig{
		text:       "HELLO WORLD",
		out:        out,
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 232 || b.Dy() != 232 {
		t.Errorf("got %dx%d, want 232x232", b.Dx(), b.Dy())
	}
}

func TestRunReadsStdinWhenTextEmpty(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "qr.png")
	cfg := cliConfig{
		out:        out,
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	stdin := strings.NewReader("HELLO WORLD\n")
	if err := run(cfg, stdin, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	// PNG file must exist and decode.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("png decode: %v", err)
	}
}

func TestRunStdoutOutput(t *testing.T) {
	cfg := cliConfig{
		text:       "HELLO",
		out:        "-",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	var stdout bytes.Buffer
	if err := run(cfg, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := png.Decode(&stdout); err != nil {
		t.Fatalf("stdout was not a valid PNG: %v", err)
	}
}

func TestRunInvalidECLevel(t *testing.T) {
	cfg := cliConfig{
		text:       "HELLO",
		out:        "/tmp/should-not-exist.png",
		moduleSize: 8,
		ec:         "Z",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Error("expected error for invalid EC level, got nil")
	}
}

func TestRunDecodeFromFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "qr.png")
	// Encode a known payload to disk first via the library.
	const want = "HELLO WORLD"
	if err := qrgen.EncodeToFile(want, in); err != nil {
		t.Fatalf("EncodeToFile: %v", err)
	}
	cfg := cliConfig{
		decode: true,
		in:     in,
	}
	var stdout bytes.Buffer
	if err := run(cfg, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunDecodeFromStdin(t *testing.T) {
	const want = "PIPE ME"
	pngBytes, err := qrgen.Encode(want)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	cfg := cliConfig{decode: true}
	var stdout bytes.Buffer
	if err := run(cfg, bytes.NewReader(pngBytes), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRunDecodeMicro: -decode reads a Micro QR symbol, located in the image
// from its single finder pattern (docs/theory/27-micro-qr-detection.md).
func TestRunDecodeMicro(t *testing.T) {
	const want = "MICRO 42"
	pngBytes, err := qrgen.Encode(want, qrgen.WithMicro())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var stdout bytes.Buffer
	if err := run(cliConfig{decode: true}, bytes.NewReader(pngBytes), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := strings.TrimRight(stdout.String(), "\n"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunDecodeWritesOutputFile(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "qr.png")
	out := filepath.Join(dir, "text.txt")
	const want = "writes to file"
	if err := qrgen.EncodeToFile(want, in); err != nil {
		t.Fatalf("EncodeToFile: %v", err)
	}
	cfg := cliConfig{decode: true, in: in, out: out}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("file contents = %q, want %q", string(got), want)
	}
}

func TestRunDecodeInvalidInput(t *testing.T) {
	cfg := cliConfig{decode: true}
	stdin := strings.NewReader("not a png")
	if err := run(cfg, stdin, &bytes.Buffer{}); err == nil {
		t.Error("expected error for non-image input, got nil")
	}
}

func TestRunDecodeMissingFile(t *testing.T) {
	cfg := cliConfig{decode: true, in: "/tmp/qrgen-test-does-not-exist.png"}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Error("expected error for missing -in file, got nil")
	}
}

func TestRunCustomColors(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "qr.png")
	cfg := cliConfig{
		text:       "HELLO",
		out:        out,
		moduleSize: 8,
		ec:         "M",
		fg:         "#102E57",
		bg:         "#FFF8E7",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	// Quiet-zone pixel should be the custom cream background.
	gr, gg, gb, _ := img.At(0, 0).RGBA()
	if gr>>8 != 0xFF || gg>>8 != 0xF8 || gb>>8 != 0xE7 {
		t.Errorf("quiet-zone pixel = (%x %x %x), want (FF F8 E7)", gr>>8, gg>>8, gb>>8)
	}
}

// svgRoot is a minimal parse target proving CLI output is well-formed SVG.
type svgRoot struct {
	XMLName xml.Name `xml:"svg"`
}

func assertSVG(t *testing.T, data []byte) {
	t.Helper()
	var root svgRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatalf("output is not well-formed SVG: %v\n%s", err, data)
	}
	if root.XMLName.Local != "svg" {
		t.Errorf("root element = %q, want svg", root.XMLName.Local)
	}
}

func TestRunEncodeSVGFormatFlag(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt") // non-.svg name; -format forces SVG
	cfg := cliConfig{
		text:       "HELLO WORLD",
		out:        out,
		format:     "svg",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	assertSVG(t, data)
}

func TestRunEncodeSVGExtensionInference(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "qr.svg") // .svg extension, no -format
	cfg := cliConfig{
		text:       "HELLO WORLD",
		out:        out,
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	assertSVG(t, data)
}

func TestRunEncodeSVGToStdout(t *testing.T) {
	cfg := cliConfig{
		text:       "HELLO",
		out:        "-",
		format:     "svg",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	var stdout bytes.Buffer
	if err := run(cfg, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertSVG(t, stdout.Bytes())
}

func TestRunEncodeTerminalToStdout(t *testing.T) {
	cfg := cliConfig{
		text:       "HELLO WORLD",
		out:        "-",
		format:     "terminal",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	var stdout bytes.Buffer
	if err := run(cfg, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := stdout.String()
	if !strings.ContainsRune(out, '█') && !strings.ContainsRune(out, '▀') && !strings.ContainsRune(out, '▄') {
		t.Errorf("terminal output has no block glyphs:\n%s", out)
	}
}

func TestRunEncodeTerminalDefaultsToStdout(t *testing.T) {
	// No -out and -format terminal must write to stdout, not a file.
	cfg := cliConfig{
		text:       "HELLO",
		format:     "terminal",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	var stdout bytes.Buffer
	if err := run(cfg, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("terminal format with empty -out wrote nothing to stdout")
	}
}

func TestRunEncodeASCIIToStdout(t *testing.T) {
	cfg := cliConfig{
		text:       "HELLO",
		out:        "-",
		format:     "ascii",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	var stdout bytes.Buffer
	if err := run(cfg, strings.NewReader(""), &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "##") {
		t.Error("ascii output missing ## glyph")
	}
}

func TestRunEncodeInvalidFormat(t *testing.T) {
	cfg := cliConfig{
		text:       "HELLO",
		out:        "/tmp/should-not-exist.gif",
		format:     "gif",
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Error("expected error for invalid -format, got nil")
	}
}

func TestRunEncodeDefaultRemainsPNG(t *testing.T) {
	// No -format and a non-.svg -out must still produce a PNG.
	dir := t.TempDir()
	out := filepath.Join(dir, "qr.png")
	cfg := cliConfig{
		text:       "HELLO",
		out:        out,
		moduleSize: 8,
		ec:         "M",
		quietZone:  4,
		mask:       -1,
	}
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("default output was not a PNG: %v", err)
	}
}

// runPNG runs the encoder with cfg into a temporary PNG and returns its bytes.
func runPNG(t *testing.T, cfg cliConfig) []byte {
	t.Helper()
	cfg.out = filepath.Join(t.TempDir(), "out.png")
	if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(cfg.out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func pngSide(t *testing.T, data []byte) int {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	return img.Bounds().Dx()
}

// TestRunUnsetDefaultsKeepQROutput pins the meaning of the new flag defaults:
// an empty -ec and a -quiet-zone of -1 must give exactly the PNG that the old
// explicit defaults (M and 4) gave, so QR output is unchanged.
func TestRunUnsetDefaultsKeepQROutput(t *testing.T) {
	unset := runPNG(t, cliConfig{text: "HELLO WORLD", moduleSize: 8, ec: "", quietZone: -1, mask: -1})
	explicit := runPNG(t, cliConfig{text: "HELLO WORLD", moduleSize: 8, ec: "M", quietZone: 4, mask: -1})
	if !bytes.Equal(unset, explicit) {
		t.Error("unset -ec/-quiet-zone produced a different PNG from explicit M and 4")
	}
}

// TestRunMicroDefaults: -micro alone picks M1 for a short number and Micro's
// 2-module quiet zone, so the PNG is (11 + 2*2) * 8 = 120 pixels wide.
func TestRunMicroDefaults(t *testing.T) {
	data := runPNG(t, cliConfig{text: "12345", moduleSize: 8, quietZone: -1, mask: -1, micro: true})
	if got := pngSide(t, data); got != 120 {
		t.Errorf("width %d px, want 120 (M1 with a 2-module quiet zone)", got)
	}
}

// TestRunMicroVersion: -micro-version (any case) forces the version, here M3
// with level L: (15 + 2*2) * 8 = 152 pixels.
func TestRunMicroVersion(t *testing.T) {
	data := runPNG(t, cliConfig{text: "HELLO", moduleSize: 8, ec: "L", quietZone: -1, mask: -1, microVersion: "m3"})
	if got := pngSide(t, data); got != 152 {
		t.Errorf("width %d px, want 152 (M3 with a 2-module quiet zone)", got)
	}
}

// TestRunMicroErrors covers an unknown -micro-version and a level Micro QR
// does not have.
func TestRunMicroErrors(t *testing.T) {
	cases := map[string]cliConfig{
		"bad version": {text: "1", moduleSize: 8, quietZone: -1, mask: -1, microVersion: "M5"},
		"level H":     {text: "1", moduleSize: 8, quietZone: -1, mask: -1, micro: true, ec: "H"},
	}
	for name, cfg := range cases {
		cfg.out = filepath.Join(t.TempDir(), "x.png")
		if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// pngDims returns the width and height of a PNG.
func pngDims(t *testing.T, data []byte) (int, int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

// TestRunRMQR: -rmqr picks the smallest rMQR symbol, -rmqr-version (any case)
// forces one, and -rmqr-height fixes the height; each PNG is the symbol plus
// rMQR's 2-module quiet zone on every side, at 2 pixels per module.
func TestRunRMQR(t *testing.T) {
	cases := []struct {
		name string
		cfg  cliConfig
		w, h int // modules, quiet zone included
	}{
		{"smallest", cliConfig{text: "12345", rmqr: true}, 27 + 4, 11 + 4},
		{"forced version", cliConfig{text: "HELLO", rmqrVersion: "r7X59"}, 59 + 4, 7 + 4},
		{"fixed height", cliConfig{text: "12345", rmqrHeight: 7}, 43 + 4, 7 + 4},
	}
	for _, c := range cases {
		c.cfg.moduleSize, c.cfg.quietZone, c.cfg.mask = 2, -1, -1
		w, h := pngDims(t, runPNG(t, c.cfg))
		if w != 2*c.w || h != 2*c.h {
			t.Errorf("%s: %dx%d px, want %dx%d", c.name, w, h, 2*c.w, 2*c.h)
		}
	}
}

// TestRunRMQRErrors covers an unknown -rmqr-version, a height rMQR does not
// have, a level it does not have, a mask (rMQR's is fixed), and a version and
// height that disagree.
func TestRunRMQRErrors(t *testing.T) {
	cases := map[string]cliConfig{
		"bad version":     {text: "1", rmqrVersion: "R8x43"},
		"bad height":      {text: "1", rmqrHeight: 8},
		"level L":         {text: "1", rmqr: true, ec: "L"},
		"mask":            {text: "1", rmqr: true, mask: 2},
		"version, height": {text: "1", rmqrVersion: "R11x27", rmqrHeight: 7},
	}
	for name, cfg := range cases {
		cfg.moduleSize, cfg.quietZone = 8, -1
		if name != "mask" {
			cfg.mask = -1
		}
		cfg.out = filepath.Join(t.TempDir(), "x.png")
		if err := run(cfg, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
