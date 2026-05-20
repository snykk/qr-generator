// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command styled shows that the qrgen decoder still recovers the payload
// from a "branded" QR code — non-default colours, a larger module size,
// extra quiet zone, and a higher EC level. It encodes a URL with the same
// styling as ./examples/encode/styled, then decodes the resulting PNG back
// to text using only the qrgen public API.
//
// Run with:
//
//	go run ./examples/decode/styled
package main

import (
	"fmt"
	"image/color"
	"log"
	"os"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	const (
		payload = "https://github.com/snykk/qr-generator"
		out     = "qr-decode-styled.png"
	)
	navy := color.RGBA{R: 0x10, G: 0x2E, B: 0x57, A: 0xFF}
	cream := color.RGBA{R: 0xFF, G: 0xF8, B: 0xE7, A: 0xFF}

	if err := qrgen.EncodeToFile(payload, out,
		qrgen.WithECLevel(qrgen.ECLevelQ),
		qrgen.WithModuleSize(12),
		qrgen.WithQuietZone(6),
		qrgen.WithColors(navy, cream),
	); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Printf("wrote %s\n", out)

	data, err := os.ReadFile(out)
	if err != nil {
		log.Fatalf("read: %v", err)
	}
	got, err := qrgen.DecodeBytes(data)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("decoded: %q\n", got)
	if got != payload {
		log.Fatalf("round-trip mismatch (got %q, want %q)", got, payload)
	}
}
