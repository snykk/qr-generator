// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command rmqr shows rMQR Code (Rectangular Micro QR Code) encoding: the
// smallest symbol for a text, a strip of fixed height, a forced version and
// level, a terminal preview, and a round trip through DecodeMatrix.
//
//	go run ./examples/encode/rmqr
package main

import (
	"fmt"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	// The smallest-area symbol that holds the text.
	if err := qrgen.EncodeToFile("HELLO RMQR", "rmqr.png", qrgen.WithRMQR()); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote rmqr.png")

	// A strip 7 modules tall, the case rMQR exists for: the encoder picks the
	// narrowest width of that height that holds the text.
	err := qrgen.EncodeToFile("https://example.com/rmqr", "rmqr-strip.png", qrgen.WithRMQRHeight(7))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote rmqr-strip.png")

	// A forced version and level: R13x43 at error-correction level H.
	err = qrgen.EncodeToFile("BATCH 2026-11", "rmqr-r13x43h.png",
		qrgen.WithRMQRVersion(qrgen.R13x43), qrgen.WithECLevel(qrgen.ECLevelH))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote rmqr-r13x43h.png")

	// A terminal preview; the quiet zone defaults to rMQR's 2 modules.
	term, err := qrgen.EncodeTerminal("12345", qrgen.WithRMQR())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(term)

	// DecodeMatrix recognises an rMQR grid by its rectangular shape.
	grid, err := qrgen.Matrix("RECTANGULAR", qrgen.WithRMQR())
	if err != nil {
		log.Fatal(err)
	}
	text, err := qrgen.DecodeMatrix(grid)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%dx%d rMQR symbol decodes to %q\n", len(grid), len(grid[0]), text)
}
