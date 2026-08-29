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
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// TestRoundTripWithThirdPartyDecoder generates PNGs across modes, EC levels,
// and version sizes, then asks an independent QR decoder (a ZXing port) to
// read them back. The decoder is a test-only dependency: it never appears in
// the public package import graph at runtime. This is M10's primary quality
// gate — it proves the encoder produces real, spec-compliant symbols rather
// than something that merely round-trips through our own bit shuffler.
func TestRoundTripWithThirdPartyDecoder(t *testing.T) {
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
		// UTF-8 multi-byte (forces byte mode and exercises the implicit-UTF8
		// assumption documented in the README).
		{"byte utf8", "café résumé", []Option{WithECLevel(ECLevelM)}},
		// Larger versions exercise multi-block Reed–Solomon interleaving and
		// alignment-pattern placement.
		{"V5 multi-block Q", strings.Repeat("ABC123", 10), []Option{WithECLevel(ECLevelQ)}},
		{"V10 long byte L", strings.Repeat("The quick brown fox. ", 12), []Option{WithECLevel(ECLevelL)}},
		// Force version + mask exercises the override paths in buildMatrix.
		{"forced V2 mask 3", "HELLO WORLD", []Option{WithVersion(2), WithMask(3)}},
		// Mixed-mode segmentation (v0.6): payloads that split across modes must
		// still read on an independent decoder, proving the multi-segment bit
		// stream is spec-conformant and not just self-consistent.
		{"seg byte+numeric", "Order #1234567890", []Option{WithECLevel(ECLevelM)}},
		{"seg invoice", "Invoice INV-2026 000123456789 total", []Option{WithECLevel(ECLevelQ)}},
		{"seg utf8+numeric", "café☕ 1234567890", []Option{WithECLevel(ECLevelM)}},
		{"seg long digit run", "ID:" + strings.Repeat("0", 60) + " END", []Option{WithECLevel(ECLevelL)}},
		// Convenience payload builders (v0.7): an independent decoder must read
		// the exact built string, confirming the escaping/format is sound.
		{"payload wifi", WiFiPayload(WiFi{SSID: "Cafe Net", Password: "p@ss;word", Security: WiFiWPA}), nil},
		{"payload vcard", VCardPayload(VCard{Name: "Ada Lovelace", Org: "Analytical Engine, Ltd", Phones: []string{"+15551234567"}}), []Option{WithECLevel(ECLevelQ)}},
		{"payload mailto", MailtoPayload("ada@example.com", "Hello there", "Hi & bye"), nil},
		{"payload geo", GeoPayload(37.422, -122.084), nil},
		// ECI segments (v0.9): an explicit ECI header must read on an independent
		// decoder, confirming the designator and charset handling are spec-conformant.
		{"eci utf8", "Héllo 世界 ECI", []Option{WithECI(ECIUTF8)}},
		{"eci latin1", "Café £5 ECI", []Option{WithECI(ECILatin1)}},
		// Kanji mode (v0.11): an independent decoder must read the JIS X 0208
		// characters our 13-bit encoding produces, mixed with ASCII/numeric.
		{"kanji", "日本語のテスト", []Option{WithECLevel(ECLevelM)}},
		{"kanji mixed", "QRコード 2026 世界", nil},
		// Logo embedding (v0.12): a centred logo occludes modules that error
		// correction must recover; an independent decoder must still read it.
		{"logo", "https://github.com/snykk/qr-generator", []Option{WithLogo(Logo{Image: solidLogo(96, color.RGBA{0xC0, 0x10, 0x10, 0xFF})})}},
	}

	runThirdPartyRoundTrip(t, cases)
}

// TestStructuredAppendThirdPartyDecoder confirms an independent decoder reads
// each symbol of a structured-append set. gozxing (like ZXing) returns each
// symbol's own chunk from a single Decode, so concatenating the chunks in emit
// order must reconstruct the original message — proving our structured-append
// header (mode 0011 + sequence indicator) is spec-conformant and interoperable,
// not merely self-consistent.
func TestStructuredAppendThirdPartyDecoder(t *testing.T) {
	const text = "structured append cross-check: the quick brown fox 1234567890 café"
	const parts = 3
	pages, err := EncodeStructuredAppend(text, parts)
	if err != nil {
		t.Fatalf("EncodeStructuredAppend: %v", err)
	}
	chunks := splitRunes(text, parts)

	reader := qrcode.NewQRCodeReader()
	var reassembled strings.Builder
	for i, pngBytes := range pages {
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("symbol %d png.Decode: %v", i+1, err)
		}
		bmp, err := gozxing.NewBinaryBitmapFromImage(img)
		if err != nil {
			t.Fatalf("symbol %d bitmap: %v", i+1, err)
		}
		result, err := reader.Decode(bmp, nil)
		if err != nil {
			t.Fatalf("symbol %d third-party decode: %v", i+1, err)
		}
		if got := result.GetText(); got != chunks[i] {
			t.Errorf("symbol %d: gozxing read %q, want chunk %q", i+1, got, chunks[i])
		}
		reassembled.WriteString(result.GetText())
	}
	if got := reassembled.String(); got != text {
		t.Errorf("reassembled gozxing chunks = %q, want %q", got, text)
	}
}

func runThirdPartyRoundTrip(t *testing.T, cases []struct {
	name string
	text string
	opts []Option
}) {
	t.Helper()
	reader := qrcode.NewQRCodeReader()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pngBytes, err := Encode(c.text, c.opts...)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, err := png.Decode(bytes.NewReader(pngBytes))
			if err != nil {
				t.Fatalf("png.Decode: %v", err)
			}
			bmp, err := gozxing.NewBinaryBitmapFromImage(img)
			if err != nil {
				t.Fatalf("NewBinaryBitmapFromImage: %v", err)
			}
			result, err := reader.Decode(bmp, nil)
			if err != nil {
				t.Fatalf("third-party decoder failed: %v", err)
			}
			if got := result.GetText(); got != c.text {
				t.Errorf("round-trip mismatch:\n got  %q\n want %q", got, c.text)
			}
		})
	}
}
