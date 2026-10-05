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

// microGolden is one Micro QR Code symbol produced by zint, loaded from
// testdata/thirdparty/micro_symbols.txt. Micro symbols have a single format
// copy, so a layout mistake shared by our encoder and decoder would make
// every symbol unreadable by real scanners while passing every self-made
// round trip; these fixtures are the external reference that prevents it.
type microGolden struct {
	typ     string // "M1", "M2-L", ...
	version MicroVersion
	ec      ECLevel // meaningless when m1
	m1      bool
	mask    int
	ecReq   string // the level asked of zint: "-" (unset), "L", "M", or "Q"
	text    string
	stream  string // final message bits, data then EC, as '0'/'1'
	grid    [][]bool
}

var ecLevelByName = map[string]ECLevel{"L": ECLevelL, "M": ECLevelM, "Q": ECLevelQ, "H": ECLevelH}

// loadMicroGoldens parses the fixture file: a header line
// "@ source=.. type=.. mask=.. ec_req=.. text=..", a "stream=" line, then the
// module rows.
func loadMicroGoldens(t *testing.T) []microGolden {
	t.Helper()
	f, err := os.Open("testdata/thirdparty/micro_symbols.txt")
	if err != nil {
		t.Fatalf("open Micro fixtures: %v", err)
	}
	defer f.Close()

	var gs []microGolden
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "@ "):
			head, text, ok := strings.Cut(strings.TrimPrefix(line, "@ "), " text=")
			if !ok {
				t.Fatalf("header without text=: %q", line)
			}
			g := microGolden{text: text}
			for _, kv := range strings.Fields(head) {
				k, v, _ := strings.Cut(kv, "=")
				switch k {
				case "type":
					g.typ = v
					ver, lvl, hasLevel := strings.Cut(v, "-")
					n, _ := strconv.Atoi(strings.TrimPrefix(ver, "M"))
					g.version = MicroVersion(n)
					g.m1 = !hasLevel
					if hasLevel {
						g.ec = ecLevelByName[lvl]
					}
				case "mask":
					g.mask, _ = strconv.Atoi(v)
				case "ec_req":
					g.ecReq = v
				}
			}
			gs = append(gs, g)
		case strings.HasPrefix(line, "stream="):
			gs[len(gs)-1].stream = strings.TrimPrefix(line, "stream=")
		default:
			row := make([]bool, len(line))
			for i := range line {
				row[i] = line[i] == '1'
			}
			cur := &gs[len(gs)-1]
			cur.grid = append(cur.grid, row)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan Micro fixtures: %v", err)
	}
	if len(gs) == 0 {
		t.Fatal("no Micro fixtures loaded")
	}
	return gs
}

// bitString renders a bit buffer as '0'/'1' characters.
func bitString(bb *bitBuffer) string {
	var b strings.Builder
	for i := 0; i < bb.bits(); i++ {
		if bb.buf[i>>3]>>(7-uint(i&7))&1 == 1 {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// TestMicroGoldenStreams pins our bit stream to zint's: forced to each
// fixture's symbol type, the final message (segment headers, payload,
// terminator, padding including M1/M3's 4-bit final codeword, and the
// Reed-Solomon codewords) must match bit for bit.
func TestMicroGoldenStreams(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		sym, stream, err := encodeMicroText(g.text, g.version, g.ec, !g.m1)
		if err != nil {
			t.Errorf("%s %q: %v", g.typ, g.text, err)
			continue
		}
		if sym.String() != g.typ {
			t.Errorf("%s %q: encoded as %v", g.typ, g.text, sym)
			continue
		}
		if got := bitString(stream); got != g.stream {
			t.Errorf("%s %q: stream differs from zint\n got %s\nwant %s", g.typ, g.text, got, g.stream)
		}
	}
}

// TestMicroAutoSelect checks the automatic choice of symbol type against
// zint's, given the same EC request. With the level unset both pick the
// smallest symbol, M before L within a version. The one deliberate difference
// is an explicit L on data that fits M1: zint still picks M1, which corrects
// nothing, while this library picks the smallest type that provides L.
func TestMicroAutoSelect(t *testing.T) {
	divergent := map[string]string{"L/7": "M2-L"}
	for _, g := range loadMicroGoldens(t) {
		ecSet := g.ecReq != "-"
		sym, _, err := encodeMicroText(g.text, 0, ecLevelByName[g.ecReq], ecSet)
		if err != nil {
			t.Errorf("ec=%s %q: %v", g.ecReq, g.text, err)
			continue
		}
		want := g.typ
		if d, ok := divergent[g.ecReq+"/"+g.text]; ok {
			want = d
		}
		if sym.String() != want {
			t.Errorf("ec=%s %q: chose %v, want %s (zint chose %s)", g.ecReq, g.text, sym, want, g.typ)
		}
	}
}

// forceOpts returns options that force a fixture's symbol type, and its mask
// when withMask is set. M1 takes no level: an explicit one is an error there.
func (g microGolden) forceOpts(withMask bool) []Option {
	opts := []Option{WithMicroVersion(g.version)}
	if !g.m1 {
		opts = append(opts, WithECLevel(g.ec))
	}
	if withMask {
		opts = append(opts, WithMask(g.mask))
	}
	return opts
}

// diffModules counts the modules where two grids differ, or -1 if their sizes
// differ.
func diffModules(a, b [][]bool) int {
	if len(a) != len(b) {
		return -1
	}
	d := 0
	for r := range a {
		if len(a[r]) != len(b[r]) {
			return -1
		}
		for c := range a[r] {
			if a[r][c] != b[r][c] {
				d++
			}
		}
	}
	return d
}

// TestMicroGoldenMatrices forces each fixture's symbol type and mask and
// requires our symbol to match zint's module for module: finder, separator,
// timing, the single format copy, placement, and masking.
func TestMicroGoldenMatrices(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		got, err := Matrix(g.text, g.forceOpts(true)...)
		if err != nil {
			t.Errorf("%s mask %d %q: %v", g.typ, g.mask, g.text, err)
			continue
		}
		if d := diffModules(got, g.grid); d != 0 {
			t.Errorf("%s mask %d %q: %d modules differ from zint", g.typ, g.mask, g.text, d)
		}
	}
}

// TestMicroGoldenAutoMask forces only the symbol type and checks that our mask
// evaluation (highest edge score, lowest mask on a tie) picks zint's mask.
func TestMicroGoldenAutoMask(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		m, mask, err := buildMatrix(g.text, resolveOptions(g.forceOpts(false)...))
		if err != nil {
			t.Errorf("%s %q: %v", g.typ, g.text, err)
			continue
		}
		if mask != g.mask {
			t.Errorf("%s %q: chose mask %d, zint chose %d", g.typ, g.text, mask, g.mask)
			continue
		}
		if d := diffModules(m.modules, g.grid); d != 0 {
			t.Errorf("%s %q: same mask but %d modules differ", g.typ, g.text, d)
		}
	}
}

// TestMicroGoldenFullAuto encodes with WithMicro alone (plus the EC level asked
// of zint) and requires zint's exact symbol, so type selection, bit stream,
// layout, and mask choice all agree end to end. The explicit-L M1 case is the
// documented divergence and is skipped here (TestMicroAutoSelect pins it).
func TestMicroGoldenFullAuto(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		opts := []Option{WithMicro()}
		if g.ecReq != "-" {
			if g.m1 {
				continue
			}
			opts = append(opts, WithECLevel(ecLevelByName[g.ecReq]))
		}
		got, err := Matrix(g.text, opts...)
		if err != nil {
			t.Errorf("ec=%s %q: %v", g.ecReq, g.text, err)
			continue
		}
		if d := diffModules(got, g.grid); d != 0 {
			t.Errorf("ec=%s %q: differs from zint's %s (%d modules)", g.ecReq, g.text, g.typ, d)
		}
	}
}

// TestMicroGoldenDecode decodes every zint symbol with DecodeMatrix: our
// decoder must read symbols it did not produce, which is the check that would
// have caught the v0.14.1 format-orientation bug in QR.
func TestMicroGoldenDecode(t *testing.T) {
	for _, g := range loadMicroGoldens(t) {
		got, err := DecodeMatrix(g.grid)
		if err != nil {
			t.Errorf("%s %q: %v", g.typ, g.text, err)
			continue
		}
		if got != g.text {
			t.Errorf("%s: decoded %q, want %q", g.typ, got, g.text)
		}
	}
}
