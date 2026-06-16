// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command payloads shows the v0.7 convenience builders. Each returns a
// formatted payload string, so it composes with any output: Encode for PNG,
// EncodeSVG for SVG, or Matrix for a raw grid.
//
// Run with:
//
//	go run ./examples/encode/payloads
//
// then scan wifi.png with a phone camera (it should offer to join the network)
// and open contact.svg.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/snykk/qr-generator/qrgen"
)

func main() {
	// A Wi-Fi join QR as PNG.
	wifi := qrgen.WiFiPayload(qrgen.WiFi{
		SSID:     "Cafe Wi-Fi",
		Password: "latte123",
		Security: qrgen.WiFiWPA,
	})
	if err := qrgen.EncodeToFile(wifi, "wifi.png", qrgen.WithECLevel(qrgen.ECLevelQ)); err != nil {
		log.Fatalf("wifi: %v", err)
	}
	fmt.Println("wrote wifi.png")

	// A contact card as SVG — same builder pattern, different output format.
	vcard := qrgen.VCardPayload(qrgen.VCard{
		Name:       "Ada Lovelace",
		FamilyName: "Lovelace",
		GivenName:  "Ada",
		Org:        "Analytical Engine, Ltd",
		Phones:     []string{"+15551234567"},
		Emails:     []string{"ada@example.com"},
		URL:        "https://example.com/ada",
	})
	svg, err := qrgen.EncodeSVG(vcard, qrgen.WithModuleSize(8))
	if err != nil {
		log.Fatalf("vcard: %v", err)
	}
	if err := os.WriteFile("contact.svg", svg, 0o644); err != nil {
		log.Fatalf("write contact.svg: %v", err)
	}
	fmt.Println("wrote contact.svg")

	// The simple URI builders compose the same way.
	fmt.Println(qrgen.TelPayload("+15551234567"))
	fmt.Println(qrgen.GeoPayload(37.422, -122.084))
}
