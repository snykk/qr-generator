// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command terminal prints a QR code straight to the terminal with the v0.8
// EncodeTerminal renderer. It shows the default half-block output, which scans
// on a light-background terminal, then the inverted variant for a
// dark-background terminal. Point a phone camera at whichever one reads as dark
// modules on a light field.
//
// Run with:
//
//	go run ./examples/encode/terminal
package main

import (
	"fmt"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	const url = "https://github.com/snykk/qr-generator"

	// Default half-block rendering. The block glyphs read as dark modules on a
	// light-background terminal.
	s, err := qrgen.EncodeTerminal(url)
	if err != nil {
		log.Fatalf("terminal: %v", err)
	}
	fmt.Println("Half-block (light-background terminal):")
	fmt.Print(s)

	// Inverted rendering for a dark-background terminal, so the dark modules
	// still read as dark to a scanner.
	inv, err := qrgen.EncodeTerminal(url, qrgen.WithTerminalInvert(true))
	if err != nil {
		log.Fatalf("terminal invert: %v", err)
	}
	fmt.Println("\nInverted (dark-background terminal):")
	fmt.Print(inv)
}
