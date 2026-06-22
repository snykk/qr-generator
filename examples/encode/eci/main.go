// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command eci shows the v0.9 WithECI option, which declares the character set
// of a QR code's byte data with an explicit ECI header. It writes an
// explicit-UTF-8 QR to a file, then encodes an ISO-8859-1 (Latin-1) QR and
// decodes it back to confirm the text survives the round-trip through the
// declared charset.
//
// Run with:
//
//	go run ./examples/encode/eci
package main

import (
	"fmt"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	// Explicit UTF-8 (ECI 26): the charset is declared, not merely assumed, so
	// a strict decoder does not have to guess.
	const unicode = "Héllo, 世界! 😀"
	if err := qrgen.EncodeToFile(unicode, "eci-utf8.png", qrgen.WithECI(qrgen.ECIUTF8)); err != nil {
		log.Fatalf("utf8: %v", err)
	}
	fmt.Println("wrote eci-utf8.png")

	// ISO-8859-1 / Latin-1 (ECI 3): the byte payload is genuine Latin-1, one
	// byte per rune. A rune above U+00FF would be rejected with an error.
	const latin1 = "Café résumé £5"
	data, err := qrgen.Encode(latin1, qrgen.WithECI(qrgen.ECILatin1))
	if err != nil {
		log.Fatalf("latin1: %v", err)
	}

	// Decode it back: the ECI header tells the decoder to read the bytes as
	// Latin-1, so the original text returns exactly.
	got, err := qrgen.DecodeBytes(data)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("latin1 round-trip: %q -> %q (match: %v)\n", latin1, got, got == latin1)
}
