// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command basic shows the simplest qrgen usage: encode a string and write
// the resulting QR PNG to disk.
//
// Run with:
//
//	go run ./examples/encode/basic
//
// then open qr.png and scan it with any phone camera.
package main

import (
	"fmt"
	"log"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	const (
		text = "HELLO WORLD"
		out  = "qr.png"
	)
	if err := qrgen.EncodeToFile(text, out); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Printf("wrote %s\n", out)
}
