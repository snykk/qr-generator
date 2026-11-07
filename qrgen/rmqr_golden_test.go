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
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"
)

// rmqrGolden is one zint rMQR record from
// testdata/thirdparty/rmqr_symbols.txt. Forced records carry the requested
// version and level, the final codeword stream, and the module rows; auto
// records carry only zint's own choice for a request.
type rmqrGolden struct {
	auto      bool
	version   RMQRVersion // the symbol zint produced
	ec        ECLevel
	heightReq int      // auto: the fixed height requested, 0 if none
	ecReq     string   // "-" (unset), "M", or "H"
	eci       ECI      // forced records only
	note      string   // forced records only
	text      string   // the encoded text
	codewords []byte   // forced: the final codeword stream
	grid      [][]bool // forced: the module rows, no quiet zone
}

// rmqrGoldenDigits is the digit string whose prefixes the auto records encode:
// the first 100 digits of pi, repeated four times.
var rmqrGoldenDigits = strings.Repeat("31415926535897932384626433832795028841971693993751058209749445923078164062862089986280348253421170679", 4)

// rmqrVersionByName maps "R11x43" style names to versions.
func rmqrVersionByName(name string) (RMQRVersion, bool) {
	for v := R7x43; v <= R17x139; v++ {
		if v.String() == name {
			return v, true
		}
	}
	return 0, false
}

// loadRMQRGoldens parses the fixture file.
func loadRMQRGoldens(t *testing.T) []rmqrGolden {
	t.Helper()
	f, err := os.Open("testdata/thirdparty/rmqr_symbols.txt")
	if err != nil {
		t.Fatalf("open rMQR fixtures: %v", err)
	}
	defer f.Close()

	var gs []rmqrGolden
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "@ "):
			g := rmqrGolden{ec: ECLevelM}
			head := strings.TrimPrefix(line, "@ ")
			if h, text, ok := strings.Cut(head, " text="); ok {
				head, g.text = h, text
			}
			for _, kv := range strings.Fields(head) {
				k, val, _ := strings.Cut(kv, "=")
				switch k {
				case "auto":
					g.auto = true
				case "version":
					v, ok := rmqrVersionByName(val)
					if !ok {
						t.Fatalf("unknown rMQR version %q in %q", val, line)
					}
					g.version = v
				case "ec":
					g.ec = ecLevelByName[val]
				case "height_req":
					if val != "-" {
						g.heightReq, _ = strconv.Atoi(val)
					}
				case "ec_req":
					g.ecReq = val
				case "eci":
					n, _ := strconv.Atoi(val)
					g.eci = ECI(n)
				case "note":
					g.note = val
				case "digits":
					n, _ := strconv.Atoi(val)
					g.text = rmqrGoldenDigits[:n]
				}
			}
			gs = append(gs, g)
		case strings.HasPrefix(line, "codewords="):
			cw, err := hex.DecodeString(strings.TrimPrefix(line, "codewords="))
			if err != nil {
				t.Fatalf("bad codewords line: %v", err)
			}
			gs[len(gs)-1].codewords = cw
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
		t.Fatalf("scan rMQR fixtures: %v", err)
	}
	if len(gs) == 0 {
		t.Fatal("no rMQR fixtures loaded")
	}
	return gs
}

// TestRMQRGoldenCodewords: our encoder, forced to each zint symbol's version
// and level, produces zint's final codeword stream byte for byte.
func TestRMQRGoldenCodewords(t *testing.T) {
	forced := 0
	for _, g := range loadRMQRGoldens(t) {
		if g.auto {
			continue
		}
		forced++
		sym, cw, err := encodeRMQRText(g.text, g.version, 0, g.ec, true, g.eci)
		if err != nil {
			t.Errorf("%v-%v %q: %v", g.version, g.ec, g.text, err)
			continue
		}
		if sym.version != g.version || sym.ecLevel() != g.ec {
			t.Errorf("%v-%v %q: encoded as %v", g.version, g.ec, g.text, sym)
		}
		if hex.EncodeToString(cw) != hex.EncodeToString(g.codewords) {
			t.Errorf("%v-%v %s %q:\n got  %X\n want %X", g.version, g.ec, g.note, g.text, cw, g.codewords)
		}
	}
	if forced != 70 {
		t.Errorf("checked %d forced records, want 70", forced)
	}
}

// TestRMQRAutoSelect: for every auto record, our choice of version and level
// for the same request (level unset, M, or H, and height free or fixed)
// matches zint's.
func TestRMQRAutoSelect(t *testing.T) {
	n := 0
	for _, g := range loadRMQRGoldens(t) {
		if !g.auto {
			continue
		}
		n++
		ec, ecSet := ECLevelM, false
		if g.ecReq != "-" {
			ec, ecSet = ecLevelByName[g.ecReq], true
		}
		sym, _, err := encodeRMQRText(g.text, 0, g.heightReq, ec, ecSet, ECINone)
		if err != nil {
			t.Errorf("%d digits, height %d, level %s: %v", len(g.text), g.heightReq, g.ecReq, err)
			continue
		}
		if sym.version != g.version || sym.ecLevel() != g.ec {
			t.Errorf("%d digits, height %d, level %s: chose %v, zint chose %v-%v", len(g.text), g.heightReq, g.ecReq, sym, g.version, g.ec)
		}
	}
	if n != 545 {
		t.Errorf("checked %d auto records, want 545", n)
	}
}
