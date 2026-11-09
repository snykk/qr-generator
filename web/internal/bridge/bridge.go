// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Package bridge is the demo page's view of the qrgen library: a small set of
// operations that take JSON requests and binary blobs and return JSON, so the
// WebAssembly entry point in web/wasm stays a thin shim and everything here
// can be tested with the ordinary Go toolchain.
package bridge

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif" // logos and scanned images may be GIF, JPEG, or PNG
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/snykk/qr-generator/qrgen"
)

// Call runs one operation for the page: method names it, req is its JSON
// request, and blobs carry its binary inputs, such as an image file or raw
// pixels. The response is a JSON object, {"result": ...} on success or
// {"error": "..."} on failure; Call never panics on bad input.
func Call(method string, req []byte, blobs [][]byte) (resp []byte) {
	defer func() {
		if r := recover(); r != nil {
			resp = errorResponse(fmt.Errorf("internal error in %s: %v", method, r))
		}
	}()
	op, ok := operations[method]
	if !ok {
		return errorResponse(fmt.Errorf("unknown operation %q", method))
	}
	result, err := op(req, blobs)
	if err != nil {
		return errorResponse(err)
	}
	out, err := json.Marshal(map[string]any{"result": result})
	if err != nil {
		return errorResponse(err)
	}
	return out
}

func errorResponse(err error) []byte {
	out, _ := json.Marshal(map[string]string{"error": strings.TrimPrefix(err.Error(), "qrgen: ")})
	return out
}

var operations map[string]func(req []byte, blobs [][]byte) (any, error)

func init() {
	operations = map[string]func([]byte, [][]byte) (any, error){
		"encode":                 encode,
		"inspect":                inspect,
		"masks":                  masks,
		"payload":                payload,
		"structuredAppend":       structuredAppend,
		"decode":                 decode,
		"decodePixels":           decodePixels,
		"decodeStructuredAppend": decodeStructuredAppend,
	}
}

// Settings are the symbol and rendering choices of the page, mapped onto
// qrgen options by Options. Zero values mean the library's defaults, except
// Mask and QuietZone, where -1 does.
type Settings struct {
	Symbology      string  `json:"symbology"`    // "qr" (default), "micro", or "rmqr"
	ECLevel        string  `json:"ecLevel"`      // "", "L", "M", "Q", or "H"
	Version        int     `json:"version"`      // QR version 1..40, or 0 for automatic
	MicroVersion   string  `json:"microVersion"` // "M1".."M4", or "" for automatic
	RMQRVersion    string  `json:"rmqrVersion"`  // "R7x43".."R17x139", or "" for automatic
	RMQRHeight     int     `json:"rmqrHeight"`   // 7..17, or 0 for any height
	Mask           int     `json:"mask"`         // 0..7 (0..3 for Micro QR), or -1 for automatic
	ECI            string  `json:"eci"`          // "", "utf8", or "latin1"
	QuietZone      int     `json:"quietZone"`    // modules, or -1 for the symbology's default
	ModuleSize     int     `json:"moduleSize"`   // pixels per module, or 0 for the default
	Foreground     string  `json:"foreground"`   // "#rrggbb", or "" for black
	Background     string  `json:"background"`   // "#rrggbb", or "" for white
	Logo           bool    `json:"logo"`         // the first blob holds the logo image
	LogoScale      float64 `json:"logoScale"`    // fraction of the symbol width, or 0 for the default
	TerminalInvert bool    `json:"terminalInvert"`
}

func defaultSettings() Settings { return Settings{Mask: -1, QuietZone: -1} }

// Options turns s into qrgen options. logo is the decoded logo image when
// s.Logo is set.
func (s Settings) Options(logo image.Image) ([]qrgen.Option, error) {
	var opts []qrgen.Option
	switch s.Symbology {
	case "", "qr":
		if s.Version != 0 {
			opts = append(opts, qrgen.WithVersion(qrgen.Version(s.Version)))
		}
	case "micro":
		if s.MicroVersion == "" {
			opts = append(opts, qrgen.WithMicro())
		} else {
			v, err := parseMicroVersion(s.MicroVersion)
			if err != nil {
				return nil, err
			}
			opts = append(opts, qrgen.WithMicroVersion(v))
		}
	case "rmqr":
		opts = append(opts, qrgen.WithRMQR())
		if s.RMQRVersion != "" {
			v, err := parseRMQRVersion(s.RMQRVersion)
			if err != nil {
				return nil, err
			}
			opts = append(opts, qrgen.WithRMQRVersion(v))
		}
		if s.RMQRHeight != 0 {
			opts = append(opts, qrgen.WithRMQRHeight(s.RMQRHeight))
		}
	default:
		return nil, fmt.Errorf("unknown symbology %q", s.Symbology)
	}
	if s.ECLevel != "" {
		ec, err := parseECLevel(s.ECLevel)
		if err != nil {
			return nil, err
		}
		opts = append(opts, qrgen.WithECLevel(ec))
	}
	if s.Mask != -1 {
		opts = append(opts, qrgen.WithMask(s.Mask))
	}
	switch s.ECI {
	case "":
	case "utf8":
		opts = append(opts, qrgen.WithECI(qrgen.ECIUTF8))
	case "latin1":
		opts = append(opts, qrgen.WithECI(qrgen.ECILatin1))
	default:
		return nil, fmt.Errorf("unknown ECI %q", s.ECI)
	}
	if s.QuietZone != -1 {
		opts = append(opts, qrgen.WithQuietZone(s.QuietZone))
	}
	if s.ModuleSize != 0 {
		opts = append(opts, qrgen.WithModuleSize(s.ModuleSize))
	}
	if s.Foreground != "" || s.Background != "" {
		fg, err := parseColor(s.Foreground, color.Black)
		if err != nil {
			return nil, err
		}
		bg, err := parseColor(s.Background, color.White)
		if err != nil {
			return nil, err
		}
		opts = append(opts, qrgen.WithColors(fg, bg))
	}
	if s.Logo {
		if logo == nil {
			return nil, errors.New("no logo image was given")
		}
		opts = append(opts, qrgen.WithLogo(qrgen.Logo{Image: logo, Scale: s.LogoScale}))
	}
	if s.TerminalInvert {
		opts = append(opts, qrgen.WithTerminalInvert(true))
	}
	return opts, nil
}

func parseECLevel(s string) (qrgen.ECLevel, error) {
	switch s {
	case "L":
		return qrgen.ECLevelL, nil
	case "M":
		return qrgen.ECLevelM, nil
	case "Q":
		return qrgen.ECLevelQ, nil
	case "H":
		return qrgen.ECLevelH, nil
	}
	return 0, fmt.Errorf("unknown error-correction level %q", s)
}

func parseMicroVersion(s string) (qrgen.MicroVersion, error) {
	for v := qrgen.M1; v <= qrgen.M4; v++ {
		if strings.EqualFold(v.String(), s) {
			return v, nil
		}
	}
	return 0, fmt.Errorf("unknown Micro QR version %q", s)
}

func parseRMQRVersion(s string) (qrgen.RMQRVersion, error) {
	for v := qrgen.R7x43; v <= qrgen.R17x139; v++ {
		if strings.EqualFold(v.String(), s) {
			return v, nil
		}
	}
	return 0, fmt.Errorf("unknown rMQR version %q", s)
}

// parseColor reads "#rrggbb", or returns def for the empty string.
func parseColor(s string, def color.Color) (color.Color, error) {
	if s == "" {
		return def, nil
	}
	var r, g, b uint8
	if len(s) != 7 || s[0] != '#' {
		return nil, fmt.Errorf("colour %q is not #rrggbb", s)
	}
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return nil, fmt.Errorf("colour %q is not #rrggbb", s)
	}
	return color.RGBA{R: r, G: g, B: b, A: 0xFF}, nil
}

// symbolRequest is the request of every operation that builds a symbol.
type symbolRequest struct {
	Text     string   `json:"text"`
	Settings Settings `json:"settings"`
	Formats  []string `json:"formats"` // encode: any of "svg", "png", and "terminal"
	Parts    int      `json:"parts"`   // structuredAppend: the number of symbols
}

// readSymbolRequest decodes a symbol request and turns its settings into
// options; the first blob is the logo image when the settings ask for one.
func readSymbolRequest(req []byte, blobs [][]byte) (symbolRequest, []qrgen.Option, error) {
	r := symbolRequest{Settings: defaultSettings()}
	if err := json.Unmarshal(req, &r); err != nil {
		return r, nil, fmt.Errorf("bad request: %w", err)
	}
	var logo image.Image
	if r.Settings.Logo {
		if len(blobs) == 0 {
			return r, nil, errors.New("no logo image was given")
		}
		img, _, err := image.Decode(bytes.NewReader(blobs[0]))
		if err != nil {
			return r, nil, fmt.Errorf("cannot read the logo image: %w", err)
		}
		logo = img
	}
	opts, err := r.Settings.Options(logo)
	return r, opts, err
}

// SymbolInfo summarises a symbol for the page's status line.
type SymbolInfo struct {
	Symbology     string `json:"symbology"`
	Version       string `json:"version"`
	ECLevel       string `json:"ecLevel"` // "" for Micro QR M1, which only detects errors
	Mask          int    `json:"mask"`
	Rows          int    `json:"rows"`
	Cols          int    `json:"cols"`
	DataBits      int    `json:"dataBits"`
	CapacityBits  int    `json:"capacityBits"`
	Codewords     int    `json:"codewords"`
	FixedMask     bool   `json:"fixedMask"` // rMQR: one mask, nothing chosen
	DetectionOnly bool   `json:"detectionOnly"`
}

func symbolInfo(in *qrgen.Inspection) SymbolInfo {
	info := SymbolInfo{
		Symbology:     in.Symbology,
		Version:       in.Version,
		ECLevel:       in.ECLevel.String(),
		Mask:          in.Mask,
		Rows:          len(in.Modules),
		Cols:          len(in.Modules[0]),
		DataBits:      in.DataBits,
		CapacityBits:  in.CapacityBits,
		Codewords:     len(in.Codewords),
		FixedMask:     in.MaskScores == nil,
		DetectionOnly: in.DetectionOnly,
	}
	if in.DetectionOnly {
		info.ECLevel = ""
	}
	return info
}

type encodeResult struct {
	Info     SymbolInfo `json:"info"`
	SVG      string     `json:"svg,omitempty"`
	PNG      string     `json:"png,omitempty"` // base64
	Terminal string     `json:"terminal,omitempty"`
}

func encode(req []byte, blobs [][]byte) (any, error) {
	r, opts, err := readSymbolRequest(req, blobs)
	if err != nil {
		return nil, err
	}
	in, err := qrgen.Inspect(r.Text, opts...)
	if err != nil {
		return nil, err
	}
	res := encodeResult{Info: symbolInfo(in)}
	for _, f := range r.Formats {
		switch f {
		case "svg":
			svg, err := qrgen.EncodeSVG(r.Text, opts...)
			if err != nil {
				return nil, err
			}
			res.SVG = string(svg)
		case "png":
			png, err := qrgen.Encode(r.Text, opts...)
			if err != nil {
				return nil, err
			}
			res.PNG = base64.StdEncoding.EncodeToString(png)
		case "terminal":
			term, err := qrgen.EncodeTerminal(r.Text, opts...)
			if err != nil {
				return nil, err
			}
			res.Terminal = term
		default:
			return nil, fmt.Errorf("unknown format %q", f)
		}
	}
	return res, nil
}

// Inspection is qrgen.Inspection in a form the page draws from: the module
// layers flattened row by row, and every codeword's block and position.
type Inspection struct {
	Info           SymbolInfo `json:"info"`
	MaskScores     []int      `json:"maskScores"`
	MaskPenalties  [][4]int   `json:"maskPenalties"`
	Segments       []Segment  `json:"segments"`
	TerminatorBits int        `json:"terminatorBits"`
	Blocks         []Block    `json:"blocks"`
	Codewords      []int      `json:"codewords"`
	// CodewordBlock, CodewordIndex, and CodewordIsEC locate each codeword of
	// Codewords: its block, its position in that block's data or EC part, and
	// which part.
	CodewordBlock []int  `json:"codewordBlock"`
	CodewordIndex []int  `json:"codewordIndex"`
	CodewordIsEC  []bool `json:"codewordIsEC"`
	// The module layers, row by row: "0" and "1" strings for the boolean
	// ones, and numbers for the rest.
	Modules        string `json:"modules"`
	Unmasked       string `json:"unmasked"`
	MaskPattern    string `json:"maskPattern"`
	Roles          []int  `json:"roles"`
	ModuleCodeword []int  `json:"moduleCodeword"`
	ModuleBit      []int  `json:"moduleBit"`
}

// Segment is one segment of the message.
type Segment struct {
	Mode          string `json:"mode"`
	Text          string `json:"text"`
	Count         int    `json:"count"`
	IndicatorBits int    `json:"indicatorBits"`
	CountBits     int    `json:"countBits"`
	PayloadBits   int    `json:"payloadBits"`
}

// Block is one Reed-Solomon block.
type Block struct {
	Data []int `json:"data"`
	EC   []int `json:"ec"`
}

func ints(b []byte) []int {
	out := make([]int, len(b))
	for i, x := range b {
		out[i] = int(x)
	}
	return out
}

func bits(grid [][]bool) string {
	var sb strings.Builder
	for _, row := range grid {
		for _, v := range row {
			if v {
				sb.WriteByte('1')
			} else {
				sb.WriteByte('0')
			}
		}
	}
	return sb.String()
}

func flatten[T any](grid [][]T, conv func(T) int) []int {
	var out []int
	for _, row := range grid {
		for _, v := range row {
			out = append(out, conv(v))
		}
	}
	return out
}

// newInspection converts a qrgen.Inspection.
func newInspection(in *qrgen.Inspection) Inspection {
	out := Inspection{
		Info:           symbolInfo(in),
		MaskScores:     in.MaskScores,
		MaskPenalties:  in.MaskPenalties,
		TerminatorBits: in.TerminatorBits,
		Codewords:      ints(in.Codewords),
		Modules:        bits(in.Modules),
		Unmasked:       bits(in.Unmasked),
		MaskPattern:    bits(in.MaskPattern),
		Roles:          flatten(in.Roles, func(r qrgen.ModuleRole) int { return int(r) }),
		ModuleCodeword: flatten(in.ModuleCodeword, func(v int) int { return v }),
		ModuleBit:      flatten(in.ModuleBit, func(v int) int { return v }),
	}
	for _, s := range in.Segments {
		out.Segments = append(out.Segments, Segment{
			Mode: s.Mode.String(), Text: s.Text, Count: s.Count,
			IndicatorBits: s.IndicatorBits, CountBits: s.CountBits, PayloadBits: s.PayloadBits,
		})
	}
	maxData, maxEC := 0, 0
	for _, b := range in.Blocks {
		out.Blocks = append(out.Blocks, Block{Data: ints(b.Data), EC: ints(b.EC)})
		maxData, maxEC = max(maxData, len(b.Data)), max(maxEC, len(b.EC))
	}
	// The codewords are interleaved column by column across the blocks, the
	// data first and then the error correction, skipping positions past the
	// end of a shorter block; replaying that order locates each one.
	for _, ec := range []bool{false, true} {
		n := maxData
		if ec {
			n = maxEC
		}
		for i := range n {
			for b, blk := range in.Blocks {
				part := blk.Data
				if ec {
					part = blk.EC
				}
				if i < len(part) {
					out.CodewordBlock = append(out.CodewordBlock, b)
					out.CodewordIndex = append(out.CodewordIndex, i)
					out.CodewordIsEC = append(out.CodewordIsEC, ec)
				}
			}
		}
	}
	return out
}

func inspect(req []byte, blobs [][]byte) (any, error) {
	r, opts, err := readSymbolRequest(req, blobs)
	if err != nil {
		return nil, err
	}
	in, err := qrgen.Inspect(r.Text, opts...)
	if err != nil {
		return nil, err
	}
	return newInspection(in), nil
}

// MaskVariant is the symbol with one candidate mask applied.
type MaskVariant struct {
	Mask    int    `json:"mask"`
	Modules string `json:"modules"`
}

// masks returns the symbol under every candidate mask, for comparing them
// side by side; rMQR, with its single mask, returns none.
func masks(req []byte, blobs [][]byte) (any, error) {
	r, opts, err := readSymbolRequest(req, blobs)
	if err != nil {
		return nil, err
	}
	n := 8
	switch r.Settings.Symbology {
	case "micro":
		n = 4
	case "rmqr":
		n = 0
	}
	out := []MaskVariant{}
	for k := range n {
		grid, err := qrgen.Matrix(r.Text, append(append([]qrgen.Option(nil), opts...), qrgen.WithMask(k))...)
		if err != nil {
			return nil, err
		}
		out = append(out, MaskVariant{Mask: k, Modules: bits(grid)})
	}
	return out, nil
}

type payloadRequest struct {
	Kind string `json:"kind"` // "wifi", "vcard", "mailto", "tel", "sms", or "geo"

	SSID     string `json:"ssid"`
	Password string `json:"password"`
	Security string `json:"security"`
	Hidden   bool   `json:"hidden"`

	Name       string   `json:"name"`
	FamilyName string   `json:"familyName"`
	GivenName  string   `json:"givenName"`
	Org        string   `json:"org"`
	Title      string   `json:"title"`
	Phones     []string `json:"phones"`
	Emails     []string `json:"emails"`
	URL        string   `json:"url"`
	Address    string   `json:"address"`
	Note       string   `json:"note"`

	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Number  string `json:"number"`
	Message string `json:"message"`

	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// payload builds structured payload text with the library's builders.
func payload(req []byte, _ [][]byte) (any, error) {
	var r payloadRequest
	if err := json.Unmarshal(req, &r); err != nil {
		return nil, fmt.Errorf("bad request: %w", err)
	}
	var text string
	switch r.Kind {
	case "wifi":
		text = qrgen.WiFiPayload(qrgen.WiFi{SSID: r.SSID, Password: r.Password, Security: qrgen.WiFiSecurity(r.Security), Hidden: r.Hidden})
	case "vcard":
		text = qrgen.VCardPayload(qrgen.VCard{
			Name: r.Name, FamilyName: r.FamilyName, GivenName: r.GivenName, Org: r.Org, Title: r.Title,
			Phones: nonEmpty(r.Phones), Emails: nonEmpty(r.Emails), URL: r.URL, Address: r.Address, Note: r.Note,
		})
	case "mailto":
		text = qrgen.MailtoPayload(r.To, r.Subject, r.Body)
	case "tel":
		text = qrgen.TelPayload(r.Number)
	case "sms":
		text = qrgen.SMSPayload(r.Number, r.Message)
	case "geo":
		text = qrgen.GeoPayload(r.Lat, r.Lon)
	default:
		return nil, fmt.Errorf("unknown payload kind %q", r.Kind)
	}
	return map[string]string{"text": text}, nil
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// structuredAppend splits the text across a structured-append set and
// returns the PNG of each symbol, base64-encoded.
func structuredAppend(req []byte, blobs [][]byte) (any, error) {
	r, opts, err := readSymbolRequest(req, blobs)
	if err != nil {
		return nil, err
	}
	pngs, err := qrgen.EncodeStructuredAppend(r.Text, r.Parts, opts...)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(pngs))
	for i, p := range pngs {
		out[i] = base64.StdEncoding.EncodeToString(p)
	}
	return map[string][]string{"pngs": out}, nil
}

// Found is one symbol read from an image. Corners and Finders are in image
// pixels; both are absent when only the single-symbol fallback read it.
type Found struct {
	Text      string   `json:"text"`
	Symbology string   `json:"symbology"`
	Version   string   `json:"version"`
	Corners   [][2]int `json:"corners,omitempty"`
	Finders   [][2]int `json:"finders,omitempty"`
}

// DecodeResult is what a decode operation returns: the symbols found, and
// when there are none, why the decoder gave up.
type DecodeResult struct {
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	Symbols []Found `json:"symbols"`
	Failure string  `json:"failure,omitempty"`
}

// readImage finds every symbol in img with DecodeAll and, when that finds
// none, tries the single-symbol Decode, whose line scanner reads some blurry
// symbols the multi-symbol detector misses.
func readImage(img image.Image) DecodeResult {
	b := img.Bounds()
	res := DecodeResult{Width: b.Dx(), Height: b.Dy(), Symbols: []Found{}}
	results, err := qrgen.DecodeAll(img)
	if err == nil && len(results) > 0 {
		for _, r := range results {
			f := Found{Text: r.Text}
			switch {
			case r.MicroVersion != 0:
				f.Symbology, f.Version = "Micro QR Code", r.MicroVersion.String()
				f.Finders = [][2]int{{r.Finders[0].X - b.Min.X, r.Finders[0].Y - b.Min.Y}}
			case r.RMQRVersion != 0:
				f.Symbology, f.Version = "rMQR Code", r.RMQRVersion.String()
				f.Finders = [][2]int{{r.Finders[0].X - b.Min.X, r.Finders[0].Y - b.Min.Y}}
			default:
				f.Symbology, f.Version = "QR Code", fmt.Sprint(int(r.Version))
				for _, p := range r.Finders {
					f.Finders = append(f.Finders, [2]int{p.X - b.Min.X, p.Y - b.Min.Y})
				}
			}
			for _, p := range r.Corners {
				f.Corners = append(f.Corners, [2]int{p.X - b.Min.X, p.Y - b.Min.Y})
			}
			res.Symbols = append(res.Symbols, f)
		}
		return res
	}
	text, err := qrgen.Decode(img)
	if err != nil {
		res.Failure = strings.TrimPrefix(err.Error(), "qrgen: ")
		return res
	}
	res.Symbols = append(res.Symbols, Found{Text: text})
	return res
}

// decode reads every symbol in an image file (PNG, JPEG, or GIF).
func decode(_ []byte, blobs [][]byte) (any, error) {
	if len(blobs) == 0 {
		return nil, errors.New("no image was given")
	}
	img, _, err := image.Decode(bytes.NewReader(blobs[0]))
	if err != nil {
		return nil, fmt.Errorf("cannot read the image: %w", err)
	}
	return readImage(img), nil
}

type pixelsRequest struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// decodePixels reads every symbol in a frame of raw RGBA pixels, such as a
// canvas grab of the camera. The frame is converted to grey first with the
// luma weights of color.GrayModel, so the decoder reads the same grey levels
// it would from the RGBA image itself.
func decodePixels(req []byte, blobs [][]byte) (any, error) {
	var r pixelsRequest
	if err := json.Unmarshal(req, &r); err != nil {
		return nil, fmt.Errorf("bad request: %w", err)
	}
	if len(blobs) == 0 || r.Width <= 0 || r.Height <= 0 || len(blobs[0]) != 4*r.Width*r.Height {
		return nil, fmt.Errorf("want %d by %d RGBA pixels", r.Width, r.Height)
	}
	return readImage(rgbaToGray(blobs[0], r.Width, r.Height)), nil
}

// rgbaToGray converts non-premultiplied RGBA pixels to grey with
// color.GrayModel's weights, treating the frame as opaque.
func rgbaToGray(pix []byte, w, h int) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, w, h))
	for i := range w * h {
		r, gr, b := uint32(pix[4*i])*0x101, uint32(pix[4*i+1])*0x101, uint32(pix[4*i+2])*0x101
		g.Pix[i] = uint8((19595*r + 38470*gr + 7471*b + 1<<15) >> 24)
	}
	return g
}

// decodeStructuredAppend reassembles a structured-append set from its
// images, in any order.
func decodeStructuredAppend(_ []byte, blobs [][]byte) (any, error) {
	if len(blobs) < 2 {
		return nil, errors.New("a structured-append set needs at least two images")
	}
	text, err := qrgen.DecodeStructuredAppend(blobs)
	if err != nil {
		return nil, err
	}
	return map[string]string{"text": text}, nil
}
