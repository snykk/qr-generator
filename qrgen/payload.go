// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"net/url"
	"strconv"
	"strings"
)

// This file holds convenience builders that format structured input into the
// string payloads scanner apps recognise (Wi-Fi join, vCard, mailto, tel, SMS,
// geo). Each builder returns a string so it composes with Encode, EncodeSVG,
// and Matrix alike. The builders escape but do not validate their input. See
// docs/theory/18-payload-formats.md.

// WiFiSecurity is the authentication type of a Wi-Fi network.
type WiFiSecurity string

const (
	// WiFiWPA covers WPA / WPA2 / WPA3 networks.
	WiFiWPA WiFiSecurity = "WPA"
	// WiFiWEP is the legacy WEP scheme.
	WiFiWEP WiFiSecurity = "WEP"
	// WiFiNoPass is an open network with no password.
	WiFiNoPass WiFiSecurity = "nopass"
)

// WiFi describes a Wi-Fi network for WiFiPayload.
type WiFi struct {
	SSID     string
	Password string
	// Security defaults to WiFiWPA when empty.
	Security WiFiSecurity
	// Hidden marks an SSID that is not broadcast.
	Hidden bool
}

// WiFiPayload builds the WIFI: join string for cfg. The password is omitted
// for an open (nopass) network or when empty. SSID and password are escaped
// per the WIFI: convention. See docs/theory/18-payload-formats.md section 1.
func WiFiPayload(cfg WiFi) string {
	sec := cfg.Security
	if sec == "" {
		sec = WiFiWPA
	}
	var b strings.Builder
	b.WriteString("WIFI:T:")
	b.WriteString(string(sec))
	b.WriteString(";S:")
	b.WriteString(escapeWiFi(cfg.SSID))
	b.WriteString(";")
	if sec != WiFiNoPass && cfg.Password != "" {
		b.WriteString("P:")
		b.WriteString(escapeWiFi(cfg.Password))
		b.WriteString(";")
	}
	if cfg.Hidden {
		b.WriteString("H:true;")
	}
	b.WriteString(";")
	return b.String()
}

// VCard describes a contact for VCardPayload. Every field is optional; empty
// fields are omitted from the output. Phones and Emails emit one line each.
type VCard struct {
	Name       string // FN — formatted display name
	FamilyName string // N family component
	GivenName  string // N given component
	Org        string // ORG
	Title      string // TITLE
	Phones     []string
	Emails     []string
	URL        string
	Address    string // ADR, placed in the street component
	Note       string
}

// VCardPayload builds a vCard 3.0 string for cfg with CRLF line breaks and
// RFC 6350 text escaping. Lines are emitted unfolded. See
// docs/theory/18-payload-formats.md section 2.
func VCardPayload(cfg VCard) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCARD\r\nVERSION:3.0\r\n")

	if cfg.Name != "" {
		writeVCardLine(&b, "FN", escapeVCard(cfg.Name))
	}
	if cfg.FamilyName != "" || cfg.GivenName != "" {
		// N is structured: family;given;additional;prefix;suffix. The
		// component separators are literal; only values are escaped.
		writeVCardLine(&b, "N", escapeVCard(cfg.FamilyName)+";"+escapeVCard(cfg.GivenName)+";;;")
	}
	if cfg.Org != "" {
		writeVCardLine(&b, "ORG", escapeVCard(cfg.Org))
	}
	if cfg.Title != "" {
		writeVCardLine(&b, "TITLE", escapeVCard(cfg.Title))
	}
	for _, tel := range cfg.Phones {
		writeVCardLine(&b, "TEL", escapeVCard(tel))
	}
	for _, email := range cfg.Emails {
		writeVCardLine(&b, "EMAIL", escapeVCard(email))
	}
	if cfg.URL != "" {
		writeVCardLine(&b, "URL", escapeVCard(cfg.URL))
	}
	if cfg.Address != "" {
		// ADR is structured: pobox;ext;street;city;region;postcode;country.
		// A free-form address goes in the street component.
		writeVCardLine(&b, "ADR", ";;"+escapeVCard(cfg.Address)+";;;;")
	}
	if cfg.Note != "" {
		writeVCardLine(&b, "NOTE", escapeVCard(cfg.Note))
	}

	b.WriteString("END:VCARD")
	return b.String()
}

func writeVCardLine(b *strings.Builder, field, value string) {
	b.WriteString(field)
	b.WriteString(":")
	b.WriteString(value)
	b.WriteString("\r\n")
}

// MailtoPayload builds an RFC 6068 mailto: string. The subject and body are
// percent-encoded; the query is omitted entirely when both are empty. The
// address is emitted as given. See docs/theory/18-payload-formats.md section 3.
func MailtoPayload(addr, subject, body string) string {
	out := "mailto:" + addr
	var params []string
	if subject != "" {
		params = append(params, "subject="+mailtoEscape(subject))
	}
	if body != "" {
		params = append(params, "body="+mailtoEscape(body))
	}
	if len(params) > 0 {
		out += "?" + strings.Join(params, "&")
	}
	return out
}

// TelPayload builds an RFC 3966 tel: string. The number is emitted as given.
func TelPayload(number string) string {
	return "tel:" + number
}

// SMSPayload builds an SMSTO: string. An empty message yields "SMSTO:<number>:".
// See docs/theory/18-payload-formats.md section 5.
func SMSPayload(number, message string) string {
	return "SMSTO:" + number + ":" + message
}

// GeoPayload builds an RFC 5870 geo: string. Coordinates use the shortest
// exact decimal form, avoiding scientific notation.
func GeoPayload(lat, lon float64) string {
	return "geo:" + formatCoord(lat) + "," + formatCoord(lon)
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// escapeWiFi backslash-escapes the characters special to the WIFI: scheme.
var wifiEscaper = strings.NewReplacer(
	`\`, `\\`,
	`;`, `\;`,
	`,`, `\,`,
	`:`, `\:`,
	`"`, `\"`,
)

func escapeWiFi(s string) string { return wifiEscaper.Replace(s) }

// escapeVCard escapes a vCard 3.0 text value per RFC 6350 section 3.4:
// backslash, semicolon, and comma are backslash-escaped, and any newline
// becomes the two-character sequence \n.
var vcardEscaper = strings.NewReplacer(
	`\`, `\\`,
	`;`, `\;`,
	`,`, `\,`,
	"\r\n", `\n`,
	"\n", `\n`,
	"\r", `\n`,
)

func escapeVCard(s string) string { return vcardEscaper.Replace(s) }

// mailtoEscape percent-encodes a mailto query component. net/url's query
// escaping renders a space as "+", but mailto: expects "%20", so we convert.
func mailtoEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
