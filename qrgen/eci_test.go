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
	"testing"
)

// The designator codec round-trips through the 1/2/3-codeword boundaries, and
// each value lands in the shortest form.
func TestECIDesignatorRoundTrip(t *testing.T) {
	cases := []struct {
		eci      ECI
		wantBits int
	}{
		{0, 8}, {3, 8}, {26, 8}, {127, 8},
		{128, 16}, {16383, 16},
		{16384, 24}, {999999, 24},
	}
	for _, c := range cases {
		bb := &bitBuffer{}
		appendECIDesignator(bb, c.eci)
		if got := bb.bits(); got != c.wantBits {
			t.Errorf("appendECIDesignator(%d): %d bits, want %d", c.eci, got, c.wantBits)
		}
		br := newBitReader(bb.bytes())
		got, err := readECIDesignator(br)
		if err != nil {
			t.Fatalf("readECIDesignator(%d): %v", c.eci, err)
		}
		if got != c.eci {
			t.Errorf("designator round-trip: ECI %d -> %d", c.eci, got)
		}
	}
}

// A conformant decoder must accept a non-minimal designator even though the
// encoder never emits one: ECI 5 written in the 1-, 2-, and 3-codeword forms.
func TestECIDesignatorNonMinimal(t *testing.T) {
	cases := [][]byte{
		{0x05},             // 1-codeword: 0 + 0000101
		{0x80, 0x05},       // 2-codeword: 10 + 14-bit 5
		{0xC0, 0x00, 0x05}, // 3-codeword: 110 + 21-bit 5
	}
	for _, data := range cases {
		got, err := readECIDesignator(newBitReader(data))
		if err != nil {
			t.Fatalf("readECIDesignator(% x): %v", data, err)
		}
		if got != 5 {
			t.Errorf("readECIDesignator(% x) = %d, want 5", data, got)
		}
	}
}

// A 111xxxxx prefix is not a valid designator.
func TestECIDesignatorInvalidPrefix(t *testing.T) {
	if _, err := readECIDesignator(newBitReader([]byte{0xE0, 0x00, 0x00})); err == nil {
		t.Error("readECIDesignator with 111 prefix: want error, got nil")
	}
}

func TestECITranscode(t *testing.T) {
	// Latin-1 round-trip over representable runes (é U+00E9, ÿ U+00FF, © U+00A9).
	const latin = "café ÿ©"
	b, err := transcodeTo(latin, ECILatin1)
	if err != nil {
		t.Fatalf("transcodeTo Latin-1: %v", err)
	}
	if len(b) != len([]rune(latin)) {
		t.Errorf("Latin-1 byte length = %d, want %d (one byte per rune)", len(b), len([]rune(latin)))
	}
	if got := transcodeFrom(b, ECILatin1); got != latin {
		t.Errorf("Latin-1 round-trip: got %q, want %q", got, latin)
	}

	// A rune above U+00FF cannot be represented (euro sign U+20AC).
	if _, err := transcodeTo("€", ECILatin1); err == nil {
		t.Error("transcodeTo Latin-1 of euro sign: want error, got nil")
	}

	// UTF-8 is a passthrough in both directions.
	const uni = "日本語\U0001f600" // 日本語😀
	if got, _ := transcodeTo(uni, ECIUTF8); !bytes.Equal(got, []byte(uni)) {
		t.Error("transcodeTo UTF-8 should be a passthrough")
	}
	if got := transcodeFrom([]byte(uni), ECIUTF8); got != uni {
		t.Error("transcodeFrom UTF-8 should be a passthrough")
	}
}

// WithECI(ECINone) must be a no-op end to end: identical PNG bytes to a plain
// encode, so the ECI machinery never perturbs the default path.
func TestECINoneMatchesPlainEncode(t *testing.T) {
	for _, text := range []string{"HELLO WORLD", "12345", "https://example.com", "Order #1234567890"} {
		plain, err := Encode(text)
		if err != nil {
			t.Fatalf("Encode(%q): %v", text, err)
		}
		none, err := Encode(text, WithECI(ECINone))
		if err != nil {
			t.Fatalf("Encode(%q, ECINone): %v", text, err)
		}
		if !bytes.Equal(plain, none) {
			t.Errorf("WithECI(ECINone) changed the encoding of %q", text)
		}
	}
}

// An ECI-26 encode prepends a readable ECI header (mode 0111 + designator)
// ahead of the data segments.
func TestECIHeaderEmitted(t *testing.T) {
	data, _, err := encodeTextECI("12345", ECLevelM, 0, ECIUTF8)
	if err != nil {
		t.Fatalf("encodeTextECI: %v", err)
	}
	br := newBitReader(data)
	mode, err := br.readBits(4)
	if err != nil || mode != 0b0111 {
		t.Fatalf("first mode indicator = 0b%04b (err %v), want 0111 (ECI)", mode, err)
	}
	eci, err := readECIDesignator(br)
	if err != nil {
		t.Fatalf("readECIDesignator: %v", err)
	}
	if eci != ECIUTF8 {
		t.Errorf("designator = %d, want %d (UTF-8)", eci, ECIUTF8)
	}
}

// Unsupported ECI values and non-representable Latin-1 input are rejected.
func TestECIValidationErrors(t *testing.T) {
	if _, err := Encode("hello", WithECI(20)); err == nil { // Shift-JIS: unsupported
		t.Error("WithECI(20): want error, got nil")
	}
	if _, err := Encode("€", WithECI(ECILatin1)); err == nil { // euro: not in Latin-1
		t.Error("Latin-1 with euro sign: want error, got nil")
	}
}

// Declaring an ECI on a pure-numeric payload still selects a valid version and
// fits within capacity (the header overhead is accounted for).
func TestECIWithNumericFits(t *testing.T) {
	if _, _, err := encodeTextECI("1234567890", ECLevelM, 0, ECIUTF8); err != nil {
		t.Errorf("encodeTextECI numeric with ECI: %v", err)
	}
}

// Full round-trip through the image pipeline: encode with an ECI, decode, and
// confirm the exact text returns for both UTF-8 and Latin-1, including payloads
// that mix a numeric run between byte segments.
func TestECIRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		text string
		eci  ECI
	}{
		{"utf8-cjk", "Héllo, 世界!", ECIUTF8},
		{"utf8-emoji", "tea🍵42", ECIUTF8},
		{"utf8-ascii", "plain ascii 123", ECIUTF8},
		{"latin1-accents", "Café résumé £5", ECILatin1},
		{"latin1-mixed", "Jürgen 1234567 Müller", ECILatin1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := Encode(c.text, WithECI(c.eci))
			if err != nil {
				t.Fatalf("Encode(%q, ECI %d): %v", c.text, c.eci, err)
			}
			got, err := DecodeBytes(data)
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}
			if got != c.text {
				t.Errorf("round-trip: got %q, want %q", got, c.text)
			}
		})
	}
}
