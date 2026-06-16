// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command svg shows how to render a QR code to a scalable SVG document
// instead of a PNG raster. SVG stays crisp at any size and embeds directly
// into HTML, so it is a good fit for web pages and print pipelines.
//
// Run with:
//
//	go run ./examples/encode/svg
//
// then open qr.svg in a browser or drop it into an <img> tag.
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
		out  = "qr.svg"
	)
	navy := color.RGBA{R: 0x10, G: 0x2E, B: 0x57, A: 0xFF}
	cream := color.RGBA{R: 0xFF, G: 0xF8, B: 0xE7, A: 0xFF}

	if err := qrgen.EncodeSVGToFile(text, out,
		qrgen.WithECLevel(qrgen.ECLevelQ), // 25% recovery — robust to wear/glare
		qrgen.WithModuleSize(10),          // nominal pixels per module
		qrgen.WithQuietZone(4),            // spec-default margin
		qrgen.WithColors(navy, cream),
	); err != nil {
		log.Fatalf("encode svg: %v", err)
	}
	fmt.Printf("wrote %s\n", out)
}
