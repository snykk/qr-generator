// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command kanji shows that Japanese text is encoded in Kanji mode automatically.
// It writes a Japanese payload to PNG, then decodes it back to confirm the
// round-trip. No option selects Kanji: the segmenter uses it wherever a run of
// JIS X 0208 characters is cheaper than byte mode, mixing with the ASCII and
// numeric parts.
//
// Run with:
//
//	go run ./examples/encode/kanji
package main

import (
	"fmt"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	const text = "こんにちは世界 QR 2026"

	if err := qrgen.EncodeToFile(text, "kanji.png"); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Println("wrote kanji.png")

	// The round-trip recovers the exact text through the image pipeline.
	data, err := qrgen.Encode(text)
	if err != nil {
		log.Fatalf("encode: %v", err)
	}
	got, err := qrgen.DecodeBytes(data)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("round-trip %q -> %q (match: %v)\n", text, got, got == text)
}
