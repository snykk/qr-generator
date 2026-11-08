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

	eci ECI // ECINone = no ECI header (byte mode treated as UTF-8)

	logo *Logo // nil = no logo overlay

	micro        bool         // encode a Micro QR Code instead of a QR Code
	microVersion MicroVersion // 0 = auto-select the smallest fitting Micro symbol

	rmqr        bool        // encode an rMQR Code instead of a QR Code
	rmqrVersion RMQRVersion // 0 = auto-select the smallest fitting rMQR symbol
	rmqrHeight  int         // 0 = any height; else the fixed rMQR height in modules

	// ecSet and quietZoneSet record whether the caller chose these explicitly.
	// QR ignores them; Micro QR needs them because its defaults differ: an
	// unset level lets the encoder pick the smallest symbol (including the
	// detection-only M1), and an unset quiet zone is Micro's 2 modules.
	ecSet        bool
	quietZoneSet bool

	// trace makes the encoder record each module's role and stream bit for
	// Inspect. No option sets it.
	trace bool
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
	// A logo occludes modules the decoder recovers through error correction, so
	// force at least EC-H (the ~30% recovery level) when one is present. Applied
	// after all options so it is order-independent.
	if o.logo != nil && o.ec < ECLevelH {
		o.ec = ECLevelH
	}
	// Micro QR's and rMQR's spec quiet zone is 2 modules; an explicit
	// WithQuietZone wins.
	if o.micro && !o.quietZoneSet {
		o.quietZone = microQuietZone
	}
	if o.rmqr && !o.quietZoneSet {
		o.quietZone = rmqrQuietZone
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
	if o.eci != ECINone && o.eci != ECILatin1 && o.eci != ECIUTF8 {
		return fmt.Errorf("qrgen: unsupported ECI %d (only ECIUTF8=26 and ECILatin1=3 are supported)", int(o.eci))
	}
	if o.rmqr {
		return o.validateRMQR()
	}
	if o.micro {
		return o.validateMicro()
	}
	return nil
}

// validateRMQR rejects the options rMQR cannot honour, each with its own
// message rather than a confusing failure further down.
func (o *options) validateRMQR() error {
	switch {
	case o.micro:
		return fmt.Errorf("qrgen: WithMicro and WithRMQR select different symbologies; use one")
	case o.version != 0:
		return fmt.Errorf("qrgen: WithVersion selects a QR Code version; use WithRMQRVersion for rMQR")
	case o.rmqrVersion != 0 && !o.rmqrVersion.IsValid():
		return fmt.Errorf("qrgen: invalid rMQR version %v (want R7x43..R17x139)", o.rmqrVersion)
	case o.rmqrHeight != 0 && !validRMQRHeight(o.rmqrHeight):
		return fmt.Errorf("qrgen: invalid rMQR height %d (want 7, 9, 11, 13, 15, or 17)", o.rmqrHeight)
	case o.rmqrVersion != 0 && o.rmqrHeight != 0 && o.rmqrVersion.Height() != o.rmqrHeight:
		return fmt.Errorf("qrgen: rMQR version %v is not %d modules tall", o.rmqrVersion, o.rmqrHeight)
	case o.logo != nil:
		// Checked before the EC level, which a logo forces to H.
		return fmt.Errorf("qrgen: WithLogo is not supported with rMQR (a symbol 7 to 17 modules tall has no room for a logo)")
	case o.ecSet && o.ec != ECLevelM && o.ec != ECLevelH:
		return fmt.Errorf("qrgen: rMQR has no error-correction level %v (only M and H)", o.ec)
	case o.mask != -1:
		return fmt.Errorf("qrgen: WithMask is not supported with rMQR, which always uses one fixed mask")
	}
	return nil
}

// validateMicro rejects the options Micro QR cannot honour, each with its own
// message rather than a confusing failure further down.
func (o *options) validateMicro() error {
	switch {
	case o.version != 0:
		return fmt.Errorf("qrgen: WithVersion selects a QR Code version; use WithMicroVersion for Micro QR")
	case o.microVersion != 0 && !o.microVersion.IsValid():
		return fmt.Errorf("qrgen: invalid Micro QR version %v (want M1..M4)", o.microVersion)
	case o.eci != ECINone:
		return fmt.Errorf("qrgen: Micro QR does not support ECI")
	case o.logo != nil:
		// Checked before the EC level: a logo forces level H, which Micro lacks.
		return fmt.Errorf("qrgen: WithLogo is not supported with Micro QR (a logo needs EC level H, which Micro QR does not have)")
	case o.ecSet && o.ec == ECLevelH:
		return fmt.Errorf("qrgen: Micro QR has no error-correction level H")
	case o.mask >= numMicroMasks:
		return fmt.Errorf("qrgen: invalid mask %d for Micro QR (want -1 for auto, or 0..3)", o.mask)
	}
	return nil
}

// WithECLevel sets the error-correction level. Defaults to ECLevelM (~15%
// recovery). Higher levels (Q, H) survive more damage at the cost of less
// payload capacity per version.
func WithECLevel(ec ECLevel) Option {
	return func(o *options) { o.ec, o.ecSet = ec, true }
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
	return func(o *options) { o.quietZone, o.quietZoneSet = modules, true }
}

// WithMicro makes the encoder produce a Micro QR Code (versions M1 to M4)
// instead of a QR Code, choosing the smallest Micro symbol that holds the
// text. It applies to Encode, EncodeToFile, EncodeSVG, EncodeSVGToFile,
// EncodeTerminal, and Matrix.
//
// With no WithECLevel, the encoder picks the smallest symbol and prefers level
// M over L within a version, which may give the detection-only M1 for short
// numeric text. An explicit level restricts the choice to symbols offering it
// (L and M from M2, Q only in M4; Micro QR has no H) and never yields M1. The
// quiet zone defaults to Micro's 2 modules. Micro QR does not support
// WithVersion, WithECI, WithLogo, or structured append, and its masks are
// 0..3. See docs/theory/26-micro-qr.md.
func WithMicro() Option {
	return func(o *options) { o.micro = true }
}

// WithMicroVersion is WithMicro with the version forced to v (M1..M4). The
// encoder fails if the text does not fit v at the requested level.
func WithMicroVersion(v MicroVersion) Option {
	return func(o *options) { o.micro, o.microVersion = true, v }
}

// WithRMQR makes the encoder produce an rMQR Code (Rectangular Micro QR Code,
// ISO/IEC 23941) instead of a QR Code: one of 32 rectangular symbols, 7 to 17
// modules tall and 27 to 139 wide, choosing the smallest-area symbol that holds
// the text. It applies to Encode, EncodeToFile, EncodeSVG, EncodeSVGToFile,
// EncodeTerminal, and Matrix.
//
// rMQR has two error-correction levels, M and H. With no WithECLevel, the
// encoder takes the smallest symbol that fits at M and uses H instead when the
// text also fits at H in that symbol, as zint does; an explicit M or H picks
// the smallest symbol at that level, and L or Q is an error. The quiet zone
// defaults to rMQR's 2 modules. WithECI is supported; WithVersion, WithMask
// (rMQR has one fixed mask), WithLogo, WithMicro, and structured append are
// not. See docs/theory/28-rmqr.md.
func WithRMQR() Option {
	return func(o *options) { o.rmqr = true }
}

// WithRMQRVersion is WithRMQR with the version forced to v (R7x43..R17x139).
// The encoder fails if the text does not fit v at the requested level.
func WithRMQRVersion(v RMQRVersion) Option {
	return func(o *options) { o.rmqr, o.rmqrVersion = true, v }
}

// WithRMQRHeight is WithRMQR with the height fixed to rows modules (7, 9, 11,
// 13, 15, or 17): the encoder picks the narrowest symbol of that height that
// holds the text, the case rMQR exists for, such as a label edge of a known
// height.
func WithRMQRHeight(rows int) Option {
	return func(o *options) { o.rmqr, o.rmqrHeight = true, rows }
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

// WithECI declares an Extended Channel Interpretation, emitting an ECI header
// that tells a decoder the character set of the byte data. The default
// ([ECINone]) emits no header and treats byte mode as UTF-8, which keeps the
// output byte-for-byte identical to earlier versions. [ECIUTF8] makes that
// UTF-8 assumption explicit and standards-conformant; [ECILatin1] encodes the
// byte payload as ISO-8859-1 (and errors if the text contains a rune above
// U+00FF). Numeric and alphanumeric data are unaffected. See
// docs/theory/20-eci-segments.md.
func WithECI(eci ECI) Option {
	return func(o *options) { o.eci = eci }
}

// WithLogo overlays a centred logo on the rendered symbol. It applies to
// [Encode] (PNG) and [EncodeSVG]; [EncodeTerminal] and [Matrix] ignore it.
// Requesting a logo raises the error-correction level to at least H so
// Reed-Solomon recovers the modules the logo covers. Keep the logo modest
// (the default covers 20% of the symbol width) and clear of the corners so the
// finder patterns stay intact. See docs/theory/23-logo-embedding.md.
func WithLogo(logo Logo) Option {
	return func(o *options) { o.logo = &logo }
}
