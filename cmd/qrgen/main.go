// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command qrgen renders text into a QR code PNG and decodes QR images back to
// text. Run with `-h` for a flag listing. Typical usage:
//
//	qrgen -text "HELLO WORLD"                                # encode → qr.png
//	qrgen -text "https://example.com" -ec Q -size 12 -out url.png
//	qrgen -text "https://example.com" -out url.svg           # SVG (inferred from .svg)
//	qrgen -text "HELLO" -format svg -out - > qr.svg          # SVG to stdout
//	echo -n "HELLO" | qrgen -out hello.png                   # text from stdin
//	qrgen -text "HELLO" -out - > qr.png                      # PNG to stdout
//	qrgen -decode -in qr.png                                 # decode → stdout
//	cat qr.png | qrgen -decode                               # decode from stdin
//	qrgen -decode -in qr.png -out text.txt                   # decode → file
//
// All encoding flags map directly to options in the qrgen library; see the
// README and godoc for the underlying option semantics.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"

	"github.com/snykk/qr-generator/qrgen"
)

// cliConfig collects every CLI flag in one place so the run() function below
// is easy to test without going through the flag package.
type cliConfig struct {
	decode     bool
	in         string
	text       string
	out        string
	format     string
	moduleSize int
	ec         string
	fg         string
	bg         string
	quietZone  int
	version    int
	mask       int
}

func main() {
	cfg := cliConfig{}
	flag.BoolVar(&cfg.decode, "decode", false, "decode mode: read a QR image and print its text")
	flag.StringVar(&cfg.in, "in", "", "decode mode: input image path (default: read from stdin)")
	flag.StringVar(&cfg.text, "text", "", "encode mode: text to encode; if empty, read from stdin")
	flag.StringVar(&cfg.out, "out", "", "output path; default qr.png/qr.svg for encode, stdout for decode. Use \"-\" to force stdout")
	flag.StringVar(&cfg.format, "format", "", "encode mode: output format png or svg; default png, inferred as svg from a .svg -out extension")
	flag.IntVar(&cfg.moduleSize, "size", 8, "encode mode: module size in pixels")
	flag.StringVar(&cfg.ec, "ec", "M", "encode mode: error-correction level L, M, Q, or H")
	flag.StringVar(&cfg.fg, "fg", "", "encode mode: foreground hex colour (e.g. #102E57); default black")
	flag.StringVar(&cfg.bg, "bg", "", "encode mode: background hex colour (e.g. #FFF8E7); default white")
	flag.IntVar(&cfg.quietZone, "quiet-zone", 4, "encode mode: modules of background around the symbol")
	flag.IntVar(&cfg.version, "version", 0, "encode mode: force QR version 1..40 (0 = auto)")
	flag.IntVar(&cfg.mask, "mask", -1, "encode mode: force mask 0..7 (-1 = auto)")

	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: qrgen [flags]")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Encode text into a QR code PNG, or decode a QR image back to text.")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Flags:")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Encode examples:")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -text \"HELLO WORLD\"")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -text \"https://example.com\" -ec Q -size 12 -out url.png")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -text \"https://example.com\" -out url.svg")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -text \"HELLO\" -format svg -out - > qr.svg")
		fmt.Fprintln(flag.CommandLine.Output(), "  echo -n \"HELLO\" | qrgen -out hello.png")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -text \"HELLO\" -out - > qr.png")
		fmt.Fprintln(flag.CommandLine.Output(), "")
		fmt.Fprintln(flag.CommandLine.Output(), "Decode examples:")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -decode -in qr.png")
		fmt.Fprintln(flag.CommandLine.Output(), "  cat qr.png | qrgen -decode")
		fmt.Fprintln(flag.CommandLine.Output(), "  qrgen -decode -in qr.png -out text.txt")
	}
	flag.Parse()

	if err := run(cfg, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "qrgen:", err)
		os.Exit(1)
	}
}

// run dispatches to the encode or decode path based on cfg.decode.
func run(cfg cliConfig, stdin io.Reader, stdout io.Writer) error {
	if cfg.decode {
		return runDecode(cfg, stdin, stdout)
	}
	return runEncode(cfg, stdin, stdout)
}

// runEncode performs the encode pipeline: resolve text from -text or stdin,
// pick the output format, build the option list from the encoding flags,
// encode to PNG or SVG, and write either to -out or stdout (when -out is "-").
func runEncode(cfg cliConfig, stdin io.Reader, stdout io.Writer) error {
	text, err := resolveText(cfg.text, stdin)
	if err != nil {
		return err
	}

	format, err := resolveFormat(cfg.format, cfg.out)
	if err != nil {
		return err
	}

	ec, err := parseECLevel(cfg.ec)
	if err != nil {
		return err
	}

	opts := []qrgen.Option{
		qrgen.WithECLevel(ec),
		qrgen.WithModuleSize(cfg.moduleSize),
		qrgen.WithQuietZone(cfg.quietZone),
	}
	if cfg.version > 0 {
		opts = append(opts, qrgen.WithVersion(qrgen.Version(cfg.version)))
	}
	if cfg.mask >= 0 {
		opts = append(opts, qrgen.WithMask(cfg.mask))
	}
	if cfg.fg != "" || cfg.bg != "" {
		fg, err := parseHexColor(cfg.fg, color.Black)
		if err != nil {
			return fmt.Errorf("invalid -fg: %w", err)
		}
		bg, err := parseHexColor(cfg.bg, color.White)
		if err != nil {
			return fmt.Errorf("invalid -bg: %w", err)
		}
		opts = append(opts, qrgen.WithColors(fg, bg))
	}

	var data []byte
	if format == "svg" {
		data, err = qrgen.EncodeSVG(text, opts...)
	} else {
		data, err = qrgen.Encode(text, opts...)
	}
	if err != nil {
		return err
	}

	out := cfg.out
	if out == "" {
		out = "qr.png"
		if format == "svg" {
			out = "qr.svg"
		}
	}
	if out == "-" {
		if _, err := stdout.Write(data); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

// resolveFormat decides the encode output format. An explicit -format png|svg
// wins; when -format is unset, a .svg extension on -out infers SVG, otherwise
// the default is PNG. Anything else is an error.
func resolveFormat(format, out string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "png", nil
	case "svg":
		return "svg", nil
	case "":
		if strings.HasSuffix(strings.ToLower(out), ".svg") {
			return "svg", nil
		}
		return "png", nil
	}
	return "", fmt.Errorf("invalid -format %q (want png or svg)", format)
}

// runDecode performs the decode pipeline: read PNG/JPEG/GIF bytes from -in
// or stdin, call qrgen.DecodeBytes, and write the decoded text to -out or
// stdout. An empty or "-" -out goes to stdout.
func runDecode(cfg cliConfig, stdin io.Reader, stdout io.Writer) error {
	var data []byte
	if cfg.in != "" {
		b, err := os.ReadFile(cfg.in)
		if err != nil {
			return fmt.Errorf("read %s: %w", cfg.in, err)
		}
		data = b
	} else {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		data = b
	}

	text, err := qrgen.DecodeBytes(data)
	if err != nil {
		return err
	}

	if cfg.out == "" || cfg.out == "-" {
		if _, err := fmt.Fprintln(stdout, text); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(cfg.out, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", cfg.out, err)
	}
	return nil
}

// resolveText returns flagText when non-empty, otherwise reads stdin and trims
// the trailing newline that shells normally append.
func resolveText(flagText string, stdin io.Reader) (string, error) {
	if flagText != "" {
		return flagText, nil
	}
	b, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

// parseECLevel accepts L, M, Q, H (case-insensitive). Anything else is an error.
func parseECLevel(s string) (qrgen.ECLevel, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "L":
		return qrgen.ECLevelL, nil
	case "M":
		return qrgen.ECLevelM, nil
	case "Q":
		return qrgen.ECLevelQ, nil
	case "H":
		return qrgen.ECLevelH, nil
	}
	return 0, fmt.Errorf("invalid EC level %q (want L, M, Q, or H)", s)
}

// parseHexColor accepts #RRGGBB and #RRGGBBAA (with or without the leading
// hash). Empty s returns the fallback so callers can mix-and-match -fg and -bg
// independently.
func parseHexColor(s string, fallback color.Color) (color.Color, error) {
	if s == "" {
		return fallback, nil
	}
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 && len(s) != 8 {
		return nil, fmt.Errorf("hex colour must be 6 (RRGGBB) or 8 (RRGGBBAA) chars, got %q", s)
	}
	var rgba [4]uint8
	rgba[3] = 0xFF
	for i := 0; i < len(s); i += 2 {
		hi, err := hexDigit(s[i])
		if err != nil {
			return nil, err
		}
		lo, err := hexDigit(s[i+1])
		if err != nil {
			return nil, err
		}
		rgba[i/2] = hi*16 + lo
	}
	return color.RGBA{R: rgba[0], G: rgba[1], B: rgba[2], A: rgba[3]}, nil
}

func hexDigit(c byte) (uint8, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	}
	return 0, errors.New("invalid hex digit: " + string(c))
}
