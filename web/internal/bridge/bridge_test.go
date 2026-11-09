// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package bridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"strings"
	"testing"

	"github.com/snykk/qr-generator/qrgen"
)

// call runs an operation and decodes its result into out, failing the test
// on an error response.
func call(t *testing.T, method string, req any, blobs [][]byte, out any) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(Call(method, body, blobs), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" {
		t.Fatalf("%s: %s", method, resp.Error)
	}
	if out != nil {
		if err := json.Unmarshal(resp.Result, out); err != nil {
			t.Fatal(err)
		}
	}
}

// callError runs an operation that must fail and returns its message.
func callError(t *testing.T, method string, req string, blobs [][]byte) string {
	t.Helper()
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(Call(method, []byte(req), blobs), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatalf("%s %s: no error", method, req)
	}
	return resp.Error
}

func settings(edit func(*Settings)) Settings {
	s := defaultSettings()
	if edit != nil {
		edit(&s)
	}
	return s
}

func TestEncodeFormats(t *testing.T) {
	var res encodeResult
	call(t, "encode", map[string]any{"text": "HELLO WORLD", "settings": settings(nil), "formats": []string{"svg", "png", "terminal"}}, nil, &res)
	if res.Info.Symbology != "QR Code" || res.Info.Version != "1" || res.Info.ECLevel != "M" || res.Info.Rows != 21 {
		t.Errorf("info %+v", res.Info)
	}
	if !strings.HasPrefix(res.SVG, "<svg") && !strings.Contains(res.SVG, "<svg") {
		t.Errorf("svg %.40q", res.SVG)
	}
	raw, err := base64.StdEncoding.DecodeString(res.PNG)
	if err != nil {
		t.Fatal(err)
	}
	if text, err := qrgen.DecodeBytes(raw); err != nil || text != "HELLO WORLD" {
		t.Errorf("PNG decodes to (%q, %v)", text, err)
	}
	if res.Terminal == "" {
		t.Error("no terminal rendering")
	}
}

// TestSettingsOptions checks that each setting reaches the library: the
// encoded symbol matches one built with the equivalent options directly.
func TestSettingsOptions(t *testing.T) {
	for _, c := range []struct {
		s    Settings
		text string
		opts []qrgen.Option
	}{
		{settings(func(s *Settings) { s.Version, s.ECLevel, s.Mask = 5, "Q", 3 }), "abc", []qrgen.Option{qrgen.WithVersion(5), qrgen.WithECLevel(qrgen.ECLevelQ), qrgen.WithMask(3)}},
		{settings(func(s *Settings) { s.Symbology = "micro" }), "12345", []qrgen.Option{qrgen.WithMicro()}},
		{settings(func(s *Settings) { s.Symbology, s.MicroVersion, s.ECLevel = "micro", "m3", "L" }), "HELLO", []qrgen.Option{qrgen.WithMicroVersion(qrgen.M3), qrgen.WithECLevel(qrgen.ECLevelL)}},
		{settings(func(s *Settings) { s.Symbology, s.RMQRHeight = "rmqr", 9 }), "rect", []qrgen.Option{qrgen.WithRMQRHeight(9)}},
		{settings(func(s *Settings) { s.Symbology, s.RMQRVersion, s.ECLevel = "rmqr", "R13x43", "H" }), "rect", []qrgen.Option{qrgen.WithRMQRVersion(qrgen.R13x43), qrgen.WithECLevel(qrgen.ECLevelH)}},
		{settings(func(s *Settings) { s.ECI = "latin1" }), "café", []qrgen.Option{qrgen.WithECI(qrgen.ECILatin1)}},
	} {
		var in Inspection
		call(t, "inspect", map[string]any{"text": c.text, "settings": c.s}, nil, &in)
		grid, err := qrgen.Matrix(c.text, c.opts...)
		if err != nil {
			t.Fatal(err)
		}
		if in.Modules != bits(grid) {
			t.Errorf("%+v: symbol differs from the direct options", c.s)
		}
	}
}

func TestSettingsErrors(t *testing.T) {
	for req, want := range map[string]string{
		`{"text":"x","settings":{"symbology":"aztec"}}`:                     "unknown symbology",
		`{"text":"x","settings":{"ecLevel":"Z"}}`:                           "unknown error-correction level",
		`{"text":"x","settings":{"symbology":"micro","microVersion":"M9"}}`: "unknown Micro QR version",
		`{"text":"x","settings":{"symbology":"rmqr","rmqrVersion":"R1x1"}}`: "unknown rMQR version",
		`{"text":"x","settings":{"foreground":"red"}}`:                      "not #rrggbb",
		`{"text":"x","settings":{"eci":"ebcdic"}}`:                          "unknown ECI",
		`{"text":"x","settings":{"logo":true}}`:                             "no logo image",
		`{"text":"x","settings":{"symbology":"rmqr","mask":2}}`:             "WithMask is not supported with rMQR",
		`not json`: "bad request",
	} {
		if got := callError(t, "inspect", req, nil); !strings.Contains(got, want) {
			t.Errorf("%s: %q, want it to mention %q", req, got, want)
		}
	}
	if got := callError(t, "nope", "{}", nil); !strings.Contains(got, "unknown operation") {
		t.Errorf("unknown operation: %q", got)
	}
}

// TestInspectFlattening checks the flattened layers and the codeword
// locations against the blocks they came from.
func TestInspectFlattening(t *testing.T) {
	for _, s := range []Settings{
		settings(func(s *Settings) { s.ECLevel = "Q" }),
		settings(func(s *Settings) { s.Symbology = "micro" }),
		settings(func(s *Settings) { s.Symbology = "rmqr" }),
	} {
		text := map[string]string{
			"":      strings.Repeat("Interleaved blocks 0123456789 ", 6),
			"micro": "MICRO",
			"rmqr":  strings.Repeat("Interleaved blocks 0123456789 ", 2),
		}[s.Symbology]
		var in Inspection
		call(t, "inspect", map[string]any{"text": text, "settings": s}, nil, &in)
		n := in.Info.Rows * in.Info.Cols
		if len(in.Modules) != n || len(in.Unmasked) != n || len(in.MaskPattern) != n || len(in.Roles) != n || len(in.ModuleCodeword) != n || len(in.ModuleBit) != n {
			t.Fatalf("%s: layer lengths do not match %d modules", s.Symbology, n)
		}
		if len(in.CodewordBlock) != len(in.Codewords) {
			t.Fatalf("%s: %d locations for %d codewords", s.Symbology, len(in.CodewordBlock), len(in.Codewords))
		}
		for i, cw := range in.Codewords {
			blk := in.Blocks[in.CodewordBlock[i]]
			part := blk.Data
			if in.CodewordIsEC[i] {
				part = blk.EC
			}
			if part[in.CodewordIndex[i]] != cw {
				t.Fatalf("%s: codeword %d is not where its location says", s.Symbology, i)
			}
		}
	}
}

func TestMasks(t *testing.T) {
	for symbology, want := range map[string]int{"qr": 8, "micro": 4, "rmqr": 0} {
		s := settings(func(s *Settings) { s.Symbology = symbology })
		var variants []MaskVariant
		call(t, "masks", map[string]any{"text": "MASKS", "settings": s}, nil, &variants)
		if len(variants) != want {
			t.Errorf("%s: %d variants, want %d", symbology, len(variants), want)
		}
		for _, v := range variants {
			opts, _ := s.Options(nil)
			grid, err := qrgen.Matrix("MASKS", append(opts, qrgen.WithMask(v.Mask))...)
			if err != nil || bits(grid) != v.Modules {
				t.Errorf("%s mask %d: variant differs (%v)", symbology, v.Mask, err)
			}
		}
	}
}

func TestPayload(t *testing.T) {
	var got map[string]string
	call(t, "payload", map[string]any{"kind": "wifi", "ssid": "Home;Net", "password": "p@ss", "security": "WPA"}, nil, &got)
	if want := qrgen.WiFiPayload(qrgen.WiFi{SSID: "Home;Net", Password: "p@ss", Security: qrgen.WiFiWPA}); got["text"] != want {
		t.Errorf("wifi %q, want %q", got["text"], want)
	}
	call(t, "payload", map[string]any{"kind": "vcard", "name": "Ada", "phones": []string{" +1 555 ", ""}}, nil, &got)
	if want := qrgen.VCardPayload(qrgen.VCard{Name: "Ada", Phones: []string{"+1 555"}}); got["text"] != want {
		t.Errorf("vcard %q, want %q", got["text"], want)
	}
	call(t, "payload", map[string]any{"kind": "geo", "lat": -6.2, "lon": 106.8}, nil, &got)
	if want := qrgen.GeoPayload(-6.2, 106.8); got["text"] != want {
		t.Errorf("geo %q, want %q", got["text"], want)
	}
	if msg := callError(t, "payload", `{"kind":"fax"}`, nil); !strings.Contains(msg, "unknown payload kind") {
		t.Errorf("unknown kind: %q", msg)
	}
}

func TestStructuredAppendRoundTrip(t *testing.T) {
	text := strings.Repeat("structured append across symbols ", 8)
	var res map[string][]string
	call(t, "structuredAppend", map[string]any{"text": text, "parts": 3, "settings": settings(nil)}, nil, &res)
	if len(res["pngs"]) != 3 {
		t.Fatalf("%d symbols, want 3", len(res["pngs"]))
	}
	var blobs [][]byte
	for i := len(res["pngs"]) - 1; i >= 0; i-- { // any order
		raw, err := base64.StdEncoding.DecodeString(res["pngs"][i])
		if err != nil {
			t.Fatal(err)
		}
		blobs = append(blobs, raw)
	}
	var got map[string]string
	call(t, "decodeStructuredAppend", map[string]any{}, blobs, &got)
	if got["text"] != text {
		t.Errorf("reassembled %q", got["text"])
	}
}

// scene renders two symbols, a QR and a Micro QR, side by side on a white
// canvas and returns it.
func scene(t *testing.T) *image.RGBA {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 520, 260))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	for i, sym := range []struct {
		text string
		opts []qrgen.Option
	}{{"LEFT QR", nil}, {"12345", []qrgen.Option{qrgen.WithMicro()}}} {
		raw, err := qrgen.Encode(sym.text, append(sym.opts, qrgen.WithModuleSize(6))...)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		draw.Draw(canvas, img.Bounds().Add(image.Pt(20+i*270, 20)), img, img.Bounds().Min, draw.Src)
	}
	return canvas
}

func TestDecodeImageFile(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, scene(t)); err != nil {
		t.Fatal(err)
	}
	var res DecodeResult
	call(t, "decode", map[string]any{}, [][]byte{buf.Bytes()}, &res)
	if res.Width != 520 || res.Height != 260 || len(res.Symbols) != 2 {
		t.Fatalf("got %+v", res)
	}
	kinds := map[string]string{}
	for _, s := range res.Symbols {
		kinds[s.Text] = s.Symbology + " " + s.Version
		if len(s.Corners) != 4 || len(s.Finders) == 0 {
			t.Errorf("%q: corners %v, finders %v", s.Text, s.Corners, s.Finders)
		}
	}
	if kinds["LEFT QR"] != "QR Code 1" || kinds["12345"] != "Micro QR Code M1" {
		t.Errorf("found %v", kinds)
	}
}

// TestDecodeRMQRImage: an rMQR symbol in an image file is reported as one,
// with its version, outline, and finder.
func TestDecodeRMQRImage(t *testing.T) {
	raw, err := qrgen.Encode("RECTANGULAR MICRO QR", qrgen.WithRMQRHeight(7), qrgen.WithModuleSize(4))
	if err != nil {
		t.Fatal(err)
	}
	var res DecodeResult
	call(t, "decode", map[string]any{}, [][]byte{raw}, &res)
	if len(res.Symbols) != 1 {
		t.Fatalf("found %+v", res)
	}
	s := res.Symbols[0]
	if s.Text != "RECTANGULAR MICRO QR" || s.Symbology != "rMQR Code" || s.Version != "R7x77" || len(s.Corners) != 4 || len(s.Finders) != 1 {
		t.Errorf("got %+v", s)
	}
}

func TestDecodePixels(t *testing.T) {
	img := scene(t)
	var res DecodeResult
	call(t, "decodePixels", map[string]any{"width": 520, "height": 260}, [][]byte{img.Pix}, &res)
	if len(res.Symbols) != 2 {
		t.Fatalf("found %+v", res)
	}
	blank := make([]byte, 4*40*30)
	for i := range blank {
		blank[i] = 0xFF
	}
	call(t, "decodePixels", map[string]any{"width": 40, "height": 30}, [][]byte{blank}, &res)
	if len(res.Symbols) != 0 || res.Failure == "" {
		t.Errorf("blank frame: %+v", res)
	}
	if msg := callError(t, "decodePixels", `{"width":10,"height":10}`, [][]byte{make([]byte, 7)}); !strings.Contains(msg, "RGBA pixels") {
		t.Errorf("short frame: %q", msg)
	}
}

// TestRGBAToGray checks the conversion against color.GrayModel.
func TestRGBAToGray(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pix := make([]byte, 4*64)
	rng.Read(pix)
	g := rgbaToGray(pix, 8, 8)
	for i := range 64 {
		c := color.NRGBA{R: pix[4*i], G: pix[4*i+1], B: pix[4*i+2], A: 0xFF}
		if want := color.GrayModel.Convert(c).(color.Gray).Y; g.Pix[i] != want {
			t.Fatalf("pixel %d: %d, want %d", i, g.Pix[i], want)
		}
	}
}

func TestLogo(t *testing.T) {
	logo := image.NewRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(logo, logo.Bounds(), &image.Uniform{color.RGBA{R: 200, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, logo); err != nil {
		t.Fatal(err)
	}
	s := settings(func(s *Settings) { s.Logo, s.LogoScale = true, 0.2 })
	var res encodeResult
	call(t, "encode", map[string]any{"text": "https://example.com", "settings": s, "formats": []string{"png"}}, [][]byte{buf.Bytes()}, &res)
	if res.Info.ECLevel != "H" {
		t.Errorf("a logo should raise the level to H, got %s", res.Info.ECLevel)
	}
	raw, _ := base64.StdEncoding.DecodeString(res.PNG)
	if text, err := qrgen.DecodeBytes(raw); err != nil || text != "https://example.com" {
		t.Errorf("logo symbol decodes to (%q, %v)", text, err)
	}
	if msg := callError(t, "encode", `{"text":"x","settings":{"logo":true}}`, [][]byte{[]byte("not an image")}); !strings.Contains(msg, "cannot read the logo") {
		t.Errorf("bad logo: %q", msg)
	}
}

// BenchmarkDecodePixels measures one camera-sized frame (640 by 480) holding
// a symbol, the work the page does per scanned frame.
func BenchmarkDecodePixels(b *testing.B) {
	raw, err := qrgen.Encode("https://example.com/camera-frame", qrgen.WithModuleSize(7))
	if err != nil {
		b.Fatal(err)
	}
	sym, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		b.Fatal(err)
	}
	frame := image.NewRGBA(image.Rect(0, 0, 640, 480))
	draw.Draw(frame, frame.Bounds(), &image.Uniform{color.RGBA{R: 180, G: 170, B: 160, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(frame, sym.Bounds().Add(image.Pt(160, 80)), sym, sym.Bounds().Min, draw.Src)
	req := []byte(`{"width":640,"height":480}`)
	b.ResetTimer()
	for range b.N {
		var res struct {
			Result DecodeResult `json:"result"`
		}
		if err := json.Unmarshal(Call("decodePixels", req, [][]byte{frame.Pix}), &res); err != nil || len(res.Result.Symbols) != 1 {
			b.Fatalf("frame not read: %v %+v", err, res)
		}
	}
}

// BenchmarkDecodePixelsEmpty measures a frame with no symbol, the common case
// while the camera is still being aimed: a lit background with dark objects
// and a little sensor noise. (Pure per-pixel noise is far slower, since every
// speck becomes a finder candidate; the page keeps frames small for that.)
func BenchmarkDecodePixelsEmpty(b *testing.B) {
	frame := image.NewRGBA(image.Rect(0, 0, 640, 480))
	rng := rand.New(rand.NewSource(2))
	for y := range 480 {
		for x := range 640 {
			v := 120 + 60*float64(y)/480 + rng.NormFloat64()*5
			if (x/40+y/55)%5 == 0 {
				v -= 70
			}
			i := frame.PixOffset(x, y)
			frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2], frame.Pix[i+3] = uint8(v), uint8(v), uint8(v), 255
		}
	}
	req := []byte(`{"width":640,"height":480}`)
	b.ResetTimer()
	for range b.N {
		Call("decodePixels", req, [][]byte{frame.Pix})
	}
}
