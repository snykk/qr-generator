// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command multi shows DecodeAll reading several QR symbols from one image. With
// no argument it composites three codes onto one canvas and decodes them all;
// pass an image path to run DecodeAll over a file of your own:
//
//	go run ./examples/decode/multi                    # built-in three-code demo
//	go run ./examples/decode/multi path/to/scene.png  # decode every code in a file
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
	report(qrgen.DecodeAll(demoScene()))
}

func report(results []qrgen.Result, err error) {
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("found %d symbol(s):\n", len(results))
	for _, r := range results {
		fmt.Printf("  %-16q v%-2d at %v\n", r.Text, r.Version, r.Bounds())
	}
}

// demoScene composites three QR codes onto one white canvas so the command has
// something to decode without any input file.
func demoScene() image.Image {
	payloads := []string{"LEFT", "MIDDLE", "RIGHT"}
	tiles := make([]image.Image, len(payloads))
	tw, th := 0, 0
	for i, p := range payloads {
		pngBytes, err := qrgen.Encode(p)
		if err != nil {
			log.Fatalf("encode %q: %v", p, err)
		}
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			log.Fatalf("decode png: %v", err)
		}
		tiles[i] = img
		tw, th = img.Bounds().Dx(), img.Bounds().Dy()
	}

	const gap = 40
	canvas := image.NewRGBA(image.Rect(0, 0, len(tiles)*tw+(len(tiles)+1)*gap, th+2*gap))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	for i, tile := range tiles {
		x := gap + i*(tw+gap)
		draw.Draw(canvas, image.Rect(x, gap, x+tw, gap+th), tile, tile.Bounds().Min, draw.Src)
	}
	return canvas
}
