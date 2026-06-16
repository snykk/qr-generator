// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "testing"

func TestWiFiPayload(t *testing.T) {
	cases := []struct {
		name string
		cfg  WiFi
		want string
	}{
		{
			"wpa basic",
			WiFi{SSID: "mynet", Password: "secret", Security: WiFiWPA},
			"WIFI:T:WPA;S:mynet;P:secret;;",
		},
		{
			"default security is wpa",
			WiFi{SSID: "mynet", Password: "secret"},
			"WIFI:T:WPA;S:mynet;P:secret;;",
		},
		{
			"nopass omits password",
			WiFi{SSID: "guest", Security: WiFiNoPass},
			"WIFI:T:nopass;S:guest;;",
		},
		{
			"hidden flag",
			WiFi{SSID: "net", Password: "p", Hidden: true},
			"WIFI:T:WPA;S:net;P:p;H:true;;",
		},
		{
			"escapes special chars",
			WiFi{SSID: `Cafe; Wifi`, Password: `p:w,d\1`, Security: WiFiWPA},
			`WIFI:T:WPA;S:Cafe\; Wifi;P:p\:w\,d\\1;;`,
		},
		{
			"escapes quote",
			WiFi{SSID: `a"b`, Password: "x", Security: WiFiWEP},
			`WIFI:T:WEP;S:a\"b;P:x;;`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WiFiPayload(c.cfg); got != c.want {
				t.Errorf("WiFiPayload(%+v)\n got  %q\n want %q", c.cfg, got, c.want)
			}
		})
	}
}

func TestVCardPayload(t *testing.T) {
	cases := []struct {
		name string
		cfg  VCard
		want string
	}{
		{
			"name and org with comma escaped",
			VCard{Name: "Ada Lovelace", FamilyName: "Lovelace", GivenName: "Ada", Org: "Analytical Engine, Ltd"},
			"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\nORG:Analytical Engine\\, Ltd\r\nEND:VCARD",
		},
		{
			"multiple phones and emails",
			VCard{Name: "Bob", Phones: []string{"+1", "+2"}, Emails: []string{"a@x.com", "b@x.com"}},
			"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Bob\r\nTEL:+1\r\nTEL:+2\r\nEMAIL:a@x.com\r\nEMAIL:b@x.com\r\nEND:VCARD",
		},
		{
			"address and note with newline escaped",
			VCard{Name: "C", Address: "1 Main St", Note: "line1\nline2"},
			"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:C\r\nADR:;;1 Main St;;;;\r\nNOTE:line1\\nline2\r\nEND:VCARD",
		},
		{
			"empty optionals omitted",
			VCard{Name: "D"},
			"BEGIN:VCARD\r\nVERSION:3.0\r\nFN:D\r\nEND:VCARD",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := VCardPayload(c.cfg); got != c.want {
				t.Errorf("VCardPayload(%+v)\n got  %q\n want %q", c.cfg, got, c.want)
			}
		})
	}
}

func TestMailtoPayload(t *testing.T) {
	cases := []struct {
		name                string
		addr, subject, body string
		want                string
	}{
		{"address only", "ada@example.com", "", "", "mailto:ada@example.com"},
		{"subject only", "ada@example.com", "Hello there", "", "mailto:ada@example.com?subject=Hello%20there"},
		{
			"subject and body percent-encoded",
			"ada@example.com", "Hello there", "Hi & bye",
			"mailto:ada@example.com?subject=Hello%20there&body=Hi%20%26%20bye",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MailtoPayload(c.addr, c.subject, c.body); got != c.want {
				t.Errorf("MailtoPayload = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTelPayload(t *testing.T) {
	if got := TelPayload("+15551234567"); got != "tel:+15551234567" {
		t.Errorf("TelPayload = %q, want tel:+15551234567", got)
	}
}

func TestSMSPayload(t *testing.T) {
	if got := SMSPayload("+15551234567", "on my way"); got != "SMSTO:+15551234567:on my way" {
		t.Errorf("SMSPayload = %q, want SMSTO:+15551234567:on my way", got)
	}
	if got := SMSPayload("+1", ""); got != "SMSTO:+1:" {
		t.Errorf("SMSPayload empty message = %q, want SMSTO:+1:", got)
	}
}

func TestGeoPayload(t *testing.T) {
	cases := []struct {
		lat, lon float64
		want     string
	}{
		{37.422, -122.084, "geo:37.422,-122.084"},
		{0, 0, "geo:0,0"},
		// Shortest-exact formatting must not emit scientific notation.
		{0.0000001, 100, "geo:0.0000001,100"},
	}
	for _, c := range cases {
		if got := GeoPayload(c.lat, c.lon); got != c.want {
			t.Errorf("GeoPayload(%v, %v) = %q, want %q", c.lat, c.lon, got, c.want)
		}
	}
}

func TestEscapeWiFi(t *testing.T) {
	// Every special char escaped; backslash not double-escaped beyond its own.
	if got := escapeWiFi(`\;,:"`); got != `\\\;\,\:\"` {
		t.Errorf("escapeWiFi = %q, want %q", got, `\\\;\,\:\"`)
	}
	if got := escapeWiFi("plain"); got != "plain" {
		t.Errorf("escapeWiFi(plain) = %q, want plain", got)
	}
}

func TestEscapeVCard(t *testing.T) {
	if got := escapeVCard(`\;,`); got != `\\\;\,` {
		t.Errorf("escapeVCard = %q, want %q", got, `\\\;\,`)
	}
	if got := escapeVCard("a\r\nb\nc\rd"); got != `a\nb\nc\nd` {
		t.Errorf("escapeVCard newlines = %q, want %q", got, `a\nb\nc\nd`)
	}
}

// TestPayloadRoundTrip confirms every builder's output survives the full
// encode -> decode pipeline intact. The digit-heavy tel/SMS/geo payloads also
// exercise the v0.6 segmentation on the way through.
func TestPayloadRoundTrip(t *testing.T) {
	payloads := []string{
		WiFiPayload(WiFi{SSID: "Cafe; Wifi", Password: `p:w,d\1`, Security: WiFiWPA, Hidden: true}),
		VCardPayload(VCard{Name: "Ada Lovelace", FamilyName: "Lovelace", GivenName: "Ada",
			Org: "Analytical Engine, Ltd", Phones: []string{"+15551234567"}, Emails: []string{"ada@example.com"}}),
		MailtoPayload("ada@example.com", "Hello there", "Hi & bye"),
		TelPayload("+15551234567"),
		SMSPayload("+15551234567", "on my way"),
		GeoPayload(37.422, -122.084),
	}
	for _, p := range payloads {
		t.Run(p, func(t *testing.T) {
			data, err := Encode(p)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := DecodeBytes(data)
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}
			if got != p {
				t.Errorf("round-trip mismatch:\n got  %q\n want %q", got, p)
			}
		})
	}
}
