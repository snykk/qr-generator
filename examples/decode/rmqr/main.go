// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command rmqr shows rMQR Code symbols being found in images: an rMQR PNG read
// back with DecodeBytes, and a scene mixing QR, Micro QR, and rMQR symbols read
// with DecodeAll. Pass an image path to run DecodeAll over a file of your own:
//
//	go run ./examples/decode/rmqr                    # built-in demo
//	go run ./examples/decode/rmqr path/to/scene.png  # every symbol in a file
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

	// An rMQR symbol decodes through DecodeBytes like any QR code: the decoder
	// finds it from its finder and the finder sub-pattern at the far corner.
	pngBytes, err := qrgen.Encode("https://example.com/rmqr", qrgen.WithRMQRHeight(7))
	if err != nil {
		log.Fatal(err)
	}
	text, err := qrgen.DecodeBytes(pngBytes)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("DecodeBytes: %q\n", text)

	// DecodeAll reports rMQR symbols next to QR and Micro QR symbols.
	report(qrgen.DecodeAll(scene()))
}

func report(results []qrgen.Result, err error) {
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("DecodeAll found %d symbol(s):\n", len(results))
	for _, r := range results {
		kind := fmt.Sprintf("QR v%d", r.Version)
		switch {
		case r.MicroVersion != 0:
			kind = "Micro " + r.MicroVersion.String()
		case r.RMQRVersion != 0:
			kind = "rMQR " + r.RMQRVersion.String()
		}
		fmt.Printf("  %-28q %-12s at %v\n", r.Text, kind, r.Bounds())
	}
}

// scene stacks a QR symbol, a Micro QR symbol, and two rMQR symbols on one
// white canvas.
func scene() image.Image {
	symbols := []struct {
		text string
		opts []qrgen.Option
	}{
		{"QR CODE", nil},
		{"12345", []qrgen.Option{qrgen.WithMicro()}},
		{"RECTANGULAR", []qrgen.Option{qrgen.WithRMQR()}},
		{"BATCH 2026-11", []qrgen.Option{qrgen.WithRMQRVersion(qrgen.R13x43), qrgen.WithECLevel(qrgen.ECLevelH)}},
	}
	const gap = 30
	var tiles []image.Image
	width, height := 0, gap
	for _, s := range symbols {
		pngBytes, err := qrgen.Encode(s.text, append(s.opts, qrgen.WithModuleSize(6))...)
		if err != nil {
			log.Fatalf("encode %q: %v", s.text, err)
		}
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			log.Fatalf("decode png: %v", err)
		}
		tiles = append(tiles, img)
		width = max(width, img.Bounds().Dx())
		height += img.Bounds().Dy() + gap
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width+2*gap, height))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	y := gap
	for _, tile := range tiles {
		b := tile.Bounds()
		draw.Draw(canvas, image.Rect(gap, y, gap+b.Dx(), y+b.Dy()), tile, b.Min, draw.Src)
		y += b.Dy() + gap
	}
	return canvas
}
