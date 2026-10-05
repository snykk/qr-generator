// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command micro shows Micro QR Code encoding: the smallest symbol for a short
// number, a forced version and level, a terminal preview, and a round trip
// through DecodeMatrix.
//
//	go run ./examples/encode/micro
package main

import (
	"fmt"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	// The smallest symbol that fits: five digits need only M1 (11x11 modules).
	if err := qrgen.EncodeToFile("12345", "micro-m1.png", qrgen.WithMicro()); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote micro-m1.png")

	// A forced version and level: M3 at error-correction level L.
	err := qrgen.EncodeToFile("HELLO MICRO", "micro-m3l.png",
		qrgen.WithMicroVersion(qrgen.M3), qrgen.WithECLevel(qrgen.ECLevelL))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote micro-m3l.png")

	// A terminal preview; the quiet zone defaults to Micro's 2 modules.
	term, err := qrgen.EncodeTerminal("HELLO", qrgen.WithMicro())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(term)

	// DecodeMatrix recognises a Micro grid by its size.
	grid, err := qrgen.Matrix("MICRO QR", qrgen.WithMicro())
	if err != nil {
		log.Fatal(err)
	}
	text, err := qrgen.DecodeMatrix(grid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%dx%d Micro symbol decodes to %q\n", len(grid), len(grid), text)
}
