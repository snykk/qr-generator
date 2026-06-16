// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "os"

// Encode encodes text as a QR code and returns the PNG bytes. The encoder
// picks the smallest version that fits the payload at the chosen EC level
// (ECLevelM by default) and the mask with the lowest penalty score, unless
// overridden via [WithVersion] or [WithMask]. Other rendering controls are
// available through [WithModuleSize], [WithQuietZone], and [WithColors].
//
// Example:
//
//	data, err := qrgen.Encode("https://example.com",
//	    qrgen.WithECLevel(qrgen.ECLevelM),
//	    qrgen.WithModuleSize(8),
//	)
//	if err != nil { log.Fatal(err) }
//	os.WriteFile("qr.png", data, 0o644)
func Encode(text string, opts ...Option) ([]byte, error) {
	o := resolveOptions(opts...)
	if err := o.validate(); err != nil {
		return nil, err
	}
	m, _, err := buildMatrix(text, o)
	if err != nil {
		return nil, err
	}
	return renderPNG(m, renderOptions{
		moduleSize: o.moduleSize,
		quietZone:  o.quietZone,
		foreground: o.foreground,
		background: o.background,
	})
}

// EncodeToFile encodes text and writes the resulting PNG to path. The file
// is created with mode 0644 and any existing file at path is overwritten.
// All options accepted by [Encode] also work here.
func EncodeToFile(text, path string, opts ...Option) error {
	data, err := Encode(text, opts...)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// EncodeSVG encodes text as a QR code and returns a standalone SVG document
// as bytes. It runs the exact same encoding pipeline as [Encode] — the same
// version selection, mask choice, and option handling — and differs only in
// the final render step, which emits a scalable vector document instead of a
// PNG raster. All options accepted by [Encode] also work here;
// [WithModuleSize], [WithQuietZone], and [WithColors] all take effect on the
// SVG output.
//
// The output is resolution-independent: its viewBox is in module units so it
// scales to any size, while its width/height default to the same nominal
// pixel dimensions a PNG would have for the same options. See
// docs/theory/16-svg-rendering.md.
//
// Example:
//
//	data, err := qrgen.EncodeSVG("https://example.com", qrgen.WithModuleSize(8))
//	if err != nil { log.Fatal(err) }
//	os.WriteFile("qr.svg", data, 0o644)
func EncodeSVG(text string, opts ...Option) ([]byte, error) {
	o := resolveOptions(opts...)
	if err := o.validate(); err != nil {
		return nil, err
	}
	m, _, err := buildMatrix(text, o)
	if err != nil {
		return nil, err
	}
	return renderSVG(m, renderOptions{
		moduleSize: o.moduleSize,
		quietZone:  o.quietZone,
		foreground: o.foreground,
		background: o.background,
	})
}

// EncodeSVGToFile encodes text and writes the resulting SVG document to path.
// The file is created with mode 0644 and any existing file at path is
// overwritten. All options accepted by [Encode] also work here.
func EncodeSVGToFile(text, path string, opts ...Option) error {
	data, err := EncodeSVG(text, opts...)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// EncodeTerminal encodes text as a QR code and returns it as a multi-line
// string of block characters, ready to print to a terminal and scan from the
// screen. It runs the exact same encoding pipeline as [Encode] and [EncodeSVG]
// — the same version selection, mask choice, and option handling — and differs
// only in the final render step, which emits text instead of a raster or
// vector image.
//
// By default the symbol is drawn with Unicode half-block glyphs, packing two
// module rows per text row so modules stay near-square on a terminal. The
// output targets a light-background terminal, where a block glyph reads as a
// dark module; on a dark-background terminal pass [WithTerminalInvert] so the
// dark modules still read as dark to a scanner. [WithTerminalASCII] falls back
// to a portable double-width ASCII rendering, and [WithQuietZone] controls the
// light border. [WithModuleSize] and [WithColors] have no effect on the text
// output, the same way they do not affect [Matrix]. See
// docs/theory/19-terminal-rendering.md.
//
// Example:
//
//	s, err := qrgen.EncodeTerminal("https://example.com")
//	if err != nil { log.Fatal(err) }
//	fmt.Print(s)
func EncodeTerminal(text string, opts ...Option) (string, error) {
	o := resolveOptions(opts...)
	if err := o.validate(); err != nil {
		return "", err
	}
	m, _, err := buildMatrix(text, o)
	if err != nil {
		return "", err
	}
	return renderTerminal(m, terminalOptions{
		quietZone: o.quietZone,
		invert:    o.terminalInvert,
		ascii:     o.terminalASCII,
	}), nil
}

// Matrix returns the underlying boolean module grid of the encoded QR
// symbol, where true means a dark module. The matrix is square with side
// length 21 + 4*(version-1) and already includes functional patterns, data
// bits, the chosen mask, and format/version info — it is the final scannable
// symbol, ready to be rendered to any target (SVG, terminal, custom raster,
// …) by the caller.
//
// All options accepted by [Encode] also work here, although the rendering
// options ([WithModuleSize], [WithQuietZone], [WithColors]) have no effect
// because nothing is rasterised.
func Matrix(text string, opts ...Option) ([][]bool, error) {
	o := resolveOptions(opts...)
	if err := o.validate(); err != nil {
		return nil, err
	}
	m, _, err := buildMatrix(text, o)
	if err != nil {
		return nil, err
	}
	out := make([][]bool, m.size)
	for i := range out {
		out[i] = append([]bool(nil), m.modules[i]...)
	}
	return out, nil
}
