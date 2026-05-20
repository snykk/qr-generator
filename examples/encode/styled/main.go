// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command styled shows how to customise a qrgen output: larger modules,
// extra quiet zone, a higher EC level for resilience, and brand colours.
//
// Run with:
//
//	go run ./examples/encode/styled
package main

import (
	"fmt"
	"image/color"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	const (
		text = "https://github.com/snykk/qr-generator"
		out  = "qr-styled.png"
	)
	navy := color.RGBA{R: 0x10, G: 0x2E, B: 0x57, A: 0xFF}
	cream := color.RGBA{R: 0xFF, G: 0xF8, B: 0xE7, A: 0xFF}

	if err := qrgen.EncodeToFile(text, out,
		qrgen.WithECLevel(qrgen.ECLevelQ), // 25% recovery — more robust to wear/glare
		qrgen.WithModuleSize(12),          // bigger pixels per module
		qrgen.WithQuietZone(6),            // extra breathing room
		qrgen.WithColors(navy, cream),
	); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Printf("wrote %s\n", out)
}
