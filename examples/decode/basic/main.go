// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command basic shows the full encode → save → decode round-trip using only
// the qrgen public API. Pass a PNG path as the first argument to decode a
// pre-existing file instead of the built-in demo payload:
//
//	go run ./examples/decode/basic                 # built-in demo
//	go run ./examples/decode/basic path/to/qr.png  # decode an existing file
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	if len(os.Args) > 1 {
		decodeFile(os.Args[1])
		return
	}
	runRoundTripDemo()
}

func decodeFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	text, err := qrgen.DecodeBytes(data)
	if err != nil {
		log.Fatalf("decode %s: %v", path, err)
	}
	fmt.Println(text)
}

func runRoundTripDemo() {
	const (
		payload = "HELLO QR DECODER"
		out     = "qr-decode-demo.png"
	)
	if err := qrgen.EncodeToFile(payload, out); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Printf("wrote %s\n", out)

	data, err := os.ReadFile(out)
	if err != nil {
		log.Fatalf("read: %v", err)
	}
	got, err := qrgen.DecodeBytes(data)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}
	fmt.Printf("decoded: %q\n", got)
	if got != payload {
		log.Fatalf("round-trip mismatch (got %q, want %q)", got, payload)
	}
}
