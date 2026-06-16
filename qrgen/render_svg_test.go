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
	"math"
	"strconv"
	"strings"
	"testing"
)

// svgDoc is a minimal parse target proving the renderer output is well-formed
// XML with the expected element/attribute shape.
type svgDoc struct {
	XMLName xml.Name `xml:"svg"`
	Width   string   `xml:"width,attr"`
	Height  string   `xml:"height,attr"`
	ViewBox string   `xml:"viewBox,attr"`
	Shape   string   `xml:"shape-rendering,attr"`
	Rect    struct {
		Width   string `xml:"width,attr"`
		Height  string `xml:"height,attr"`
		Fill    string `xml:"fill,attr"`
		Opacity string `xml:"fill-opacity,attr"`
	} `xml:"rect"`
	Path struct {
		D       string `xml:"d,attr"`
		Fill    string `xml:"fill,attr"`
		Opacity string `xml:"fill-opacity,attr"`
	} `xml:"path"`
}

// testMatrix builds a tiny hand-specified matrix for isolated renderer tests.
func testMatrix(rows [][]bool) *matrix {
	return &matrix{size: len(rows), modules: rows}
}

func countDark(m *matrix) int {
	n := 0
	for _, row := range m.modules {
		for _, dark := range row {
			if dark {
				n++
			}
		}
	}
	return n
}

func TestRenderSVGWellFormedAndModuleCount(t *testing.T) {
	m := testMatrix([][]bool{
		{true, false, true},
		{false, true, false},
		{true, false, true},
	})
	// defaultRenderOptions mirrors what EncodeSVG passes after resolveOptions
	// fills the spec defaults; a bare renderOptions{} would keep quietZone 0
	// because withDefaults only promotes a negative quiet zone, not a zero one
	// (a zero quiet zone is a legitimate caller choice).
	data, err := renderSVG(m, defaultRenderOptions())
	if err != nil {
		t.Fatalf("renderSVG: %v", err)
	}

	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("output is not well-formed XML: %v\n%s", err, data)
	}

	// Default options: moduleSize 8, quietZone 4 -> dim = 3 + 8 = 11,
	// side = 8 * 11 = 88.
	if doc.ViewBox != "0 0 11 11" {
		t.Errorf("viewBox = %q, want %q", doc.ViewBox, "0 0 11 11")
	}
	if doc.Width != "88" || doc.Height != "88" {
		t.Errorf("width/height = %s/%s, want 88/88", doc.Width, doc.Height)
	}
	if doc.Shape != "crispEdges" {
		t.Errorf("shape-rendering = %q, want crispEdges", doc.Shape)
	}
	if doc.Rect.Fill != "#FFFFFF" {
		t.Errorf("background fill = %q, want #FFFFFF", doc.Rect.Fill)
	}
	if doc.Path.Fill != "#000000" {
		t.Errorf("foreground fill = %q, want #000000", doc.Path.Fill)
	}
	// One "M" subpath per dark module.
	if got, want := strings.Count(doc.Path.D, "M"), countDark(m); got != want {
		t.Errorf("path has %d move commands, want %d (one per dark module)", got, want)
	}
}

func TestRenderSVGCustomModuleAndQuietZone(t *testing.T) {
	m := testMatrix([][]bool{
		{true, false, true},
		{false, true, false},
		{true, false, true},
	})
	data, err := renderSVG(m, renderOptions{moduleSize: 10, quietZone: 2})
	if err != nil {
		t.Fatalf("renderSVG: %v", err)
	}
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
	// dim = 3 + 2*2 = 7, side = 10 * 7 = 70.
	if doc.ViewBox != "0 0 7 7" {
		t.Errorf("viewBox = %q, want %q", doc.ViewBox, "0 0 7 7")
	}
	if doc.Width != "70" || doc.Height != "70" {
		t.Errorf("width/height = %s/%s, want 70/70", doc.Width, doc.Height)
	}
	// First dark module is matrix (0,0); with quietZone 2 its square starts at
	// module-unit (2, 2), so the path must begin "M2 2".
	if !strings.HasPrefix(doc.Path.D, "M2 2h1v1h-1z") {
		t.Errorf("path starts %q, want it to begin M2 2h1v1h-1z", doc.Path.D[:min(16, len(doc.Path.D))])
	}
}

func TestRenderSVGCustomColorsOpaque(t *testing.T) {
	m := testMatrix([][]bool{{true}})
	navy := color.RGBA{R: 0x10, G: 0x2E, B: 0x57, A: 0xFF}
	cream := color.RGBA{R: 0xFF, G: 0xF8, B: 0xE7, A: 0xFF}
	data, err := renderSVG(m, renderOptions{foreground: navy, background: cream})
	if err != nil {
		t.Fatalf("renderSVG: %v", err)
	}
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
	if doc.Path.Fill != "#102E57" {
		t.Errorf("foreground = %q, want #102E57", doc.Path.Fill)
	}
	if doc.Rect.Fill != "#FFF8E7" {
		t.Errorf("background = %q, want #FFF8E7", doc.Rect.Fill)
	}
	// Opaque colours emit no fill-opacity.
	if doc.Path.Opacity != "" || doc.Rect.Opacity != "" {
		t.Errorf("opaque colours emitted fill-opacity (path=%q rect=%q)", doc.Path.Opacity, doc.Rect.Opacity)
	}
}

func TestRenderSVGAlphaForegroundEmitsOpacity(t *testing.T) {
	m := testMatrix([][]bool{{true}})
	// Half-transparent foreground (non-premultiplied input).
	fg := color.NRGBA{R: 0x10, G: 0x20, B: 0x40, A: 0x80}
	data, err := renderSVG(m, renderOptions{foreground: fg})
	if err != nil {
		t.Fatalf("renderSVG: %v", err)
	}
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
	if doc.Path.Opacity == "" {
		t.Fatal("non-opaque foreground did not emit fill-opacity")
	}
	op, err := strconv.ParseFloat(doc.Path.Opacity, 64)
	if err != nil {
		t.Fatalf("fill-opacity %q not a float: %v", doc.Path.Opacity, err)
	}
	if math.Abs(op-0x80/255.0) > 0.01 {
		t.Errorf("fill-opacity = %v, want ~%.4f", op, 0x80/255.0)
	}
}

func TestRenderSVGEmptyMatrixHasNoPath(t *testing.T) {
	// All-light matrix: background only, no foreground path element.
	m := testMatrix([][]bool{
		{false, false},
		{false, false},
	})
	data, err := renderSVG(m, renderOptions{})
	if err != nil {
		t.Fatalf("renderSVG: %v", err)
	}
	if strings.Contains(string(data), "<path") {
		t.Errorf("all-light matrix should emit no <path>:\n%s", data)
	}
	var doc svgDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
}

func TestRenderSVGNilMatrix(t *testing.T) {
	if _, err := renderSVG(nil, renderOptions{}); err == nil {
		t.Error("expected error for nil matrix, got nil")
	}
}

func TestColorToHex(t *testing.T) {
	cases := []struct {
		name        string
		in          color.Color
		wantHex     string
		wantOpacity float64
	}{
		{"black", color.Black, "#000000", 1},
		{"white", color.White, "#FFFFFF", 1},
		{"navy opaque", color.RGBA{R: 0x10, G: 0x2E, B: 0x57, A: 0xFF}, "#102E57", 1},
		{"fully transparent", color.RGBA{R: 0, G: 0, B: 0, A: 0}, "#000000", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hex, op := colorToHex(c.in)
			if hex != c.wantHex {
				t.Errorf("hex = %q, want %q", hex, c.wantHex)
			}
			if math.Abs(op-c.wantOpacity) > 1e-9 {
				t.Errorf("opacity = %v, want %v", op, c.wantOpacity)
			}
		})
	}
}
