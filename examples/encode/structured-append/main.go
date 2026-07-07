// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command structured-append shows the v0.10 EncodeStructuredAppend and
// DecodeStructuredAppend: splitting a long message across a linked set of QR
// symbols, writing each symbol to its own PNG, then reassembling the set back
// into the original text.
//
// Run with:
//
//	go run ./examples/encode/structured-append
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	const message = "Structured append links several QR symbols into one message. " +
		"This paragraph is split across a set so a scanner can read each part and " +
		"reassemble them, in any order, back into the original text. 1234567890"
	const parts = 3

	pages, err := qrgen.EncodeStructuredAppend(message, parts, qrgen.WithECLevel(qrgen.ECLevelM))
	if err != nil {
		log.Fatalf("encode: %v", err)
	}
	for i, png := range pages {
		name := fmt.Sprintf("sa-part-%d.png", i+1)
		if err := os.WriteFile(name, png, 0o644); err != nil {
			log.Fatalf("write %s: %v", name, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", name, len(png))
	}

	// Reassemble from the symbols. Order does not matter to the decoder, so this
	// works even if the symbols were scanned out of sequence.
	text, err := qrgen.DecodeStructuredAppend(pages)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("reassembled %d symbols, matches original: %v\n", parts, text == message)
}
