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
	"image/color"
)

// Option configures a call to Encode, EncodeToFile, or Matrix. Use the
// WithXxx helpers below to construct one; the zero Option does nothing.
type Option func(*options)

// options carries the resolved settings used by the encoding pipeline.
// All fields have spec-conformant defaults so the zero-options case is
// correct out of the box.
type options struct {
	ec         ECLevel
	version    Version     // 0 = auto-select smallest fitting version
	mask       int         // -1 = auto-select lowest-penalty mask
	moduleSize int         // pixels per QR module
	quietZone  int         // modules of background around the symbol
	foreground color.Color // dark module colour
	background color.Color // light module colour

	terminalInvert bool // EncodeTerminal: swap dark/light polarity for dark backgrounds
	terminalASCII  bool // EncodeTerminal: ASCII double-width fallback instead of half-block
}

// defaultOptions returns the spec-conformant defaults.
func defaultOptions() options {
	return options{
		ec:         ECLevelM,
		version:    0,
		mask:       -1,
		moduleSize: defaultModuleSize,
		quietZone:  defaultQuietZone,
		foreground: color.Black,
		background: color.White,
	}
}

// resolveOptions builds an options struct by starting from the defaults and
// applying each functional option in order.
func resolveOptions(opts ...Option) *options {
	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return &o
}

// defaultsForEC returns an options struct with the given EC level and all
// other fields at their defaults. Convenience for internal callers and tests
// that do not need the full functional-option ceremony.
func defaultsForEC(ec ECLevel) *options {
	o := defaultOptions()
	o.ec = ec
	return &o
}

// validate reports the first error in o (or nil). Called by the public API
// before doing any work so users get fast, clear feedback on bad inputs.
func (o *options) validate() error {
	if o.ec < ECLevelL || o.ec > ECLevelH {
		return fmt.Errorf("qrgen: invalid EC level %d", o.ec)
	}
	if o.version != 0 && !o.version.IsValid() {
		return fmt.Errorf("qrgen: invalid version %d (want 0 for auto, or 1..40)", o.version)
	}
	if o.mask < -1 || o.mask >= numMasks {
		return fmt.Errorf("qrgen: invalid mask %d (want -1 for auto, or 0..7)", o.mask)
	}
	if o.moduleSize <= 0 {
		return fmt.Errorf("qrgen: invalid module size %d (must be > 0)", o.moduleSize)
	}
	if o.quietZone < 0 {
		return fmt.Errorf("qrgen: invalid quiet zone %d (must be >= 0)", o.quietZone)
	}
	if o.foreground == nil {
		o.foreground = color.Black
	}
	if o.background == nil {
		o.background = color.White
	}
	return nil
}

// WithECLevel sets the error-correction level. Defaults to ECLevelM (~15%
// recovery). Higher levels (Q, H) survive more damage at the cost of less
// payload capacity per version.
func WithECLevel(ec ECLevel) Option {
	return func(o *options) { o.ec = ec }
}

// WithVersion forces the QR version (1..40). The default (0) auto-picks the
// smallest version whose data-codeword capacity fits the payload at the
// chosen EC level. Use this to pin output size for fixed-layout printing.
func WithVersion(v Version) Option {
	return func(o *options) { o.version = v }
}

// WithMask forces a specific mask pattern (0..7). The default (-1) tries all
// eight masks and picks the lowest-penalty one. Pinning a mask is useful for
// reproducible test fixtures.
func WithMask(mask int) Option {
	return func(o *options) { o.mask = mask }
}

// WithModuleSize sets the pixel side length of each QR module in the
// rendered PNG. Default 8. Larger values produce larger, easier-to-scan
// images at the cost of file size.
func WithModuleSize(px int) Option {
	return func(o *options) { o.moduleSize = px }
}

// WithQuietZone sets the number of background modules around the symbol.
// Default 4, which is the spec minimum. Larger quiet zones improve scan
// reliability when the QR is printed close to other elements.
func WithQuietZone(modules int) Option {
	return func(o *options) { o.quietZone = modules }
}

// WithColors sets the foreground (dark) and background (light) module
// colours. Default is black on white. Pick colours with a luminance contrast
// ratio of at least 3:1 so scanners can still read the symbol.
func WithColors(foreground, background color.Color) Option {
	return func(o *options) {
		o.foreground = foreground
		o.background = background
	}
}

// WithTerminalInvert swaps the dark/light polarity of [EncodeTerminal] output.
// The default targets a light-background terminal, where a block glyph reads as
// a dark module; set this on a dark-background terminal so the rendered dark
// modules still read as dark to a scanner. It has no effect on PNG or SVG
// output.
func WithTerminalInvert(invert bool) Option {
	return func(o *options) { o.terminalInvert = invert }
}

// WithTerminalASCII renders [EncodeTerminal] output with a portable
// double-width ASCII form ("##" per dark module) instead of Unicode half-block
// glyphs, for terminals or fonts without block-element support. It is roughly
// twice as tall as the default half-block rendering. It has no effect on PNG or
// SVG output.
func WithTerminalASCII(ascii bool) Option {
	return func(o *options) { o.terminalASCII = ascii }
}
