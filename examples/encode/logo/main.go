// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command logo overlays a centred logo on a QR code. It writes a logo QR to
// both PNG and SVG using the same WithLogo option, then decodes the PNG to
// confirm the logo does not stop it from scanning: requesting a logo forces
// EC-H, so error correction recovers the modules the logo covers.
//
// Run with:
//
//	go run ./examples/encode/logo
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"github.com/snykk/qr-generator/qrgen"
)

// makeLogo returns a filled navy circle on a transparent field, a stand-in
// logo. The transparent corners fall onto the white pad the renderer draws.
func makeLogo(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill := color.RGBA{0x10, 0x2E, 0x57, 0xFF}
	r := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-r, float64(y)-r
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, fill)
			}
		}
	}
	return img
}

func main() {
	const url = "https://github.com/snykk/qr-generator"
	logo := qrgen.Logo{Image: makeLogo(240)}

	if err := qrgen.EncodeToFile(url, "logo.png", qrgen.WithLogo(logo)); err != nil {
		log.Fatalf("png: %v", err)
	}
	fmt.Println("wrote logo.png")

	svg, err := qrgen.EncodeSVG(url, qrgen.WithLogo(logo), qrgen.WithModuleSize(10))
	if err != nil {
		log.Fatalf("svg: %v", err)
	}
	if err := os.WriteFile("logo.svg", svg, 0o644); err != nil {
		log.Fatalf("write logo.svg: %v", err)
	}
	fmt.Println("wrote logo.svg")

	// The logo occludes the centre, but EC-H recovers it, so the code still reads.
	data, err := qrgen.Encode(url, qrgen.WithLogo(logo))
	if err != nil {
		log.Fatalf("encode: %v", err)
	}
	got, err := qrgen.DecodeBytes(data)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("round-trip matches: %v\n", got == url)
}
