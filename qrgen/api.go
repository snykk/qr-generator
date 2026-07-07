// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"fmt"
	"os"
)

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

// EncodeStructuredAppend splits text across a structured-append set of parts
// linked QR symbols (2..16) and returns one PNG per symbol. It carries a
// message too large for a single symbol: a scanner that reads every symbol
// reassembles the original text with [DecodeStructuredAppend], in any scan
// order.
//
// The text is divided into parts contiguous chunks on rune boundaries, all
// symbols are sized to one common version chosen to fit the largest chunk (or
// the version pinned by [WithVersion]), and each symbol carries a header with
// its position, the total count, and a parity byte shared across the set. The
// other options ([WithECLevel], [WithModuleSize], [WithQuietZone],
// [WithColors], [WithMask]) apply to every symbol. Combining structured append
// with [WithECI] is not supported in this version. See
// docs/theory/21-structured-append.md.
//
// Example:
//
//	pages, err := qrgen.EncodeStructuredAppend(longText, 3, qrgen.WithECLevel(qrgen.ECLevelM))
//	if err != nil { log.Fatal(err) }
//	for i, png := range pages {
//	    os.WriteFile(fmt.Sprintf("part-%d.png", i+1), png, 0o644)
//	}
func EncodeStructuredAppend(text string, parts int, opts ...Option) ([][]byte, error) {
	if parts < 2 || parts > maxStructuredAppendSymbols {
		return nil, fmt.Errorf("qrgen: structured append needs 2..%d parts, got %d", maxStructuredAppendSymbols, parts)
	}
	o := resolveOptions(opts...)
	if err := o.validate(); err != nil {
		return nil, err
	}
	if o.eci != ECINone {
		return nil, fmt.Errorf("qrgen: structured append does not support WithECI in this version")
	}

	chunks := splitRunes(text, parts)
	parity := saParity(text)

	v := o.version
	if v == 0 {
		var err error
		if v, err = selectCommonVersion(chunks, o.ec); err != nil {
			return nil, err
		}
	} else {
		capBits := v.DataCodewords(o.ec) * 8
		for i, chunk := range chunks {
			needed := saHeaderBits + segmentsBitLength(segmentText(chunk, v), v)
			if needed > capBits {
				return nil, fmt.Errorf("qrgen: structured-append symbol %d (%d bits) does not fit in V%d-%s (capacity %d bits)", i+1, needed, v, o.ec, capBits)
			}
		}
	}

	out := make([][]byte, parts)
	for i, chunk := range chunks {
		data, err := encodeSASymbol(chunk, o.ec, v, i, parts, parity)
		if err != nil {
			return nil, err
		}
		m, _, err := matrixFromData(data, v, o)
		if err != nil {
			return nil, err
		}
		png, err := renderPNG(m, renderOptions{
			moduleSize: o.moduleSize,
			quietZone:  o.quietZone,
			foreground: o.foreground,
			background: o.background,
		})
		if err != nil {
			return nil, err
		}
		out[i] = png
	}
	return out, nil
}
