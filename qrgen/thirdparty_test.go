// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// thirdPartySymbol is one QR Code symbol produced by an independent encoder
// (segno or zint), loaded from testdata/thirdparty/qr_symbols.txt.
type thirdPartySymbol struct {
	source  string
	version int
	ec      ECLevel
	mask    int
	text    string
	grid    [][]bool
}

// loadThirdPartySymbols parses the fixture file: a header line
// "@ source=.. version=.. ec=.. mask=.. text=.." followed by the module rows.
func loadThirdPartySymbols(t *testing.T) []thirdPartySymbol {
	t.Helper()
	f, err := os.Open("testdata/thirdparty/qr_symbols.txt")
	if err != nil {
		t.Fatalf("open fixtures: %v", err)
	}
	defer f.Close()

	ecOf := map[string]ECLevel{"L": ECLevelL, "M": ECLevelM, "Q": ECLevelQ, "H": ECLevelH}
	var syms []thirdPartySymbol
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "@ "):
			// text= is last and may contain spaces, so split it off first.
			head, text, ok := strings.Cut(strings.TrimPrefix(line, "@ "), " text=")
			if !ok {
				t.Fatalf("header without text=: %q", line)
			}
			s := thirdPartySymbol{text: text}
			for _, kv := range strings.Fields(head) {
				k, v, _ := strings.Cut(kv, "=")
				switch k {
				case "source":
					s.source = v
				case "version":
					s.version, _ = strconv.Atoi(v)
				case "ec":
					s.ec = ecOf[v]
				case "mask":
					s.mask, _ = strconv.Atoi(v)
				}
			}
			syms = append(syms, s)
		default:
			if len(syms) == 0 {
				t.Fatalf("module row before any header: %q", line)
			}
			row := make([]bool, len(line))
			for i := range line {
				row[i] = line[i] == '1'
			}
			cur := &syms[len(syms)-1]
			cur.grid = append(cur.grid, row)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan fixtures: %v", err)
	}
	if len(syms) == 0 {
		t.Fatal("no fixtures loaded")
	}
	return syms
}

// TestThirdPartyQRDecode reads symbols made by independent encoders. Every
// earlier decoder test fed the decoder our own encoder's output, so a layout
// mistake shared by both sides could never surface; that is how the
// bit-reversed first format-information copy survived until v0.14.1, when
// this decoder could not read 14 of the 32 format words from any other
// encoder.
func TestThirdPartyQRDecode(t *testing.T) {
	for _, s := range loadThirdPartySymbols(t) {
		got, err := DecodeMatrix(s.grid)
		if err != nil {
			t.Errorf("%s v%d %s mask %d %q: %v", s.source, s.version, s.ec, s.mask, s.text, err)
			continue
		}
		if got != s.text {
			t.Errorf("%s v%d %s mask %d: decoded %q, want %q", s.source, s.version, s.ec, s.mask, got, s.text)
		}
	}
}

// TestThirdPartyQREncode checks that, pinned to the same version, EC level and
// mask, our encoder reproduces segno's symbols module for module: function
// patterns, both format-information copies, version information, and the
// data region. The zint fixtures are decode-only, because zint's segmentation
// of mixed-mode text may legitimately differ from ours.
func TestThirdPartyQREncode(t *testing.T) {
	for _, s := range loadThirdPartySymbols(t) {
		if s.source != "segno" {
			continue
		}
		got, err := Matrix(s.text, WithVersion(Version(s.version)), WithECLevel(s.ec), WithMask(s.mask))
		if err != nil {
			t.Fatalf("Matrix(v%d %s mask %d): %v", s.version, s.ec, s.mask, err)
		}
		if len(got) != len(s.grid) {
			t.Fatalf("v%d %s mask %d: size %d, want %d", s.version, s.ec, s.mask, len(got), len(s.grid))
		}
		diff := 0
		for r := range got {
			for c := range got[r] {
				if got[r][c] != s.grid[r][c] {
					diff++
				}
			}
		}
		if diff != 0 {
			t.Errorf("v%d %s mask %d: %d modules differ from segno", s.version, s.ec, s.mask, diff)
		}
	}
}
