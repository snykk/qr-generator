// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command micro shows Micro QR Code symbols being found in images: a Micro PNG
// read back with DecodeBytes, and a scene mixing QR and Micro symbols read with
// DecodeAll. Pass an image path to run DecodeAll over a file of your own:
//
//	go run ./examples/decode/micro                    # built-in demo
//	go run ./examples/decode/micro path/to/scene.png  # every symbol in a file
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	if len(os.Args) > 1 {
		data, err := os.ReadFile(os.Args[1])
		if err != nil {
			log.Fatalf("read %s: %v", os.Args[1], err)
		}
		report(qrgen.DecodeAllBytes(data))
		return
	}

	// A Micro symbol decodes through DecodeBytes like any QR code: the decoder
	// finds it from its single finder pattern.
	pngBytes, err := qrgen.Encode("MICRO 42", qrgen.WithMicro())
	if err != nil {
		log.Fatal(err)
	}
	text, err := qrgen.DecodeBytes(pngBytes)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DecodeBytes: %q\n", text)

	// DecodeAll reports Micro symbols next to QR symbols.
	report(qrgen.DecodeAll(scene()))
}

func report(results []qrgen.Result, err error) {
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("DecodeAll found %d symbol(s):\n", len(results))
	for _, r := range results {
		kind := fmt.Sprintf("QR v%d", r.Version)
		if r.MicroVersion != 0 {
			kind = "Micro " + r.MicroVersion.String()
		}
		fmt.Printf("  %-12q %-9s at %v\n", r.Text, kind, r.Bounds())
	}
}

// scene composites a QR symbol and two Micro symbols onto one white canvas,
// centred on a common line so DecodeAll reads them as one row.
func scene() image.Image {
	symbols := []struct {
		text string
		opts []qrgen.Option
	}{
		{"QR CODE", nil},
		{"12345", []qrgen.Option{qrgen.WithMicro()}},
		{"HELLO M3", []qrgen.Option{qrgen.WithMicroVersion(qrgen.M3)}},
	}
	const gap = 30
	var tiles []image.Image
	width, height := gap, 0
	for _, s := range symbols {
		pngBytes, err := qrgen.Encode(s.text, s.opts...)
		if err != nil {
			log.Fatalf("encode %q: %v", s.text, err)
		}
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			log.Fatalf("decode png: %v", err)
		}
		tiles = append(tiles, img)
		width += img.Bounds().Dx() + gap
		height = max(height, img.Bounds().Dy())
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, height+2*gap))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	x := gap
	for _, tile := range tiles {
		b := tile.Bounds()
		y := gap + (height-b.Dy())/2
		draw.Draw(canvas, image.Rect(x, y, x+b.Dx(), y+b.Dy()), tile, b.Min, draw.Src)
		x += b.Dx() + gap
	}
	return canvas
}
