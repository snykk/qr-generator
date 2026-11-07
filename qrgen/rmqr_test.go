// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRMQRVersionTable(t *testing.T) {
	if R17x139 != 32 || R11x27 != 11 {
		t.Fatalf("version numbering: R11x27=%d R17x139=%d", R11x27, R17x139)
	}
	for v := R7x43; v <= R17x139; v++ {
		info := rmqrVersions[v-1]
		if want := fmt.Sprintf("R%dx%d", info.height, info.width); v.String() != want {
			t.Errorf("version %d: String %q, want %q", int(v), v.String(), want)
		}
		for lvl := range 2 {
			if info.data[lvl] >= info.codewords {
				t.Errorf("%v: %d data codewords at level %d of %d", v, info.data[lvl], lvl, info.codewords)
			}
			if (info.codewords-info.data[lvl])%info.blocks[lvl] != 0 {
				t.Errorf("%v level %d: %d EC codewords do not divide into %d blocks", v, lvl, info.codewords-info.data[lvl], info.blocks[lvl])
			}
			// Every count width must hold the most characters the capacity
			// allows, so a segment never needs splitting for its count.
			capBits := 8 * info.data[lvl]
			for _, m := range qrModes {
				maxCount := 0
				for n := 1; ; n++ {
					if 3+rmqrCountBits(m, v)+payloadCost(m, n) > capBits {
						break
					}
					maxCount = n
				}
				if maxCount > 1<<rmqrCountBits(m, v)-1 {
					t.Errorf("%v level %d: %d %v characters fit but the count holds %d bits", v, lvl, maxCount, m, rmqrCountBits(m, v))
				}
			}
		}
	}
	if R1 := RMQRVersion(0); R1.IsValid() || RMQRVersion(33).IsValid() || R1.String() != "RMQR(0)" {
		t.Error("versions 0 and 33 must be invalid")
	}
}

// payloadCost is the payload length in bits of n characters in mode m.
func payloadCost(m Mode, n int) int {
	switch m {
	case ModeNumeric:
		return 10*(n/3) + [3]int{0, 4, 7}[n%3]
	case ModeAlphanumeric:
		return 11*(n/2) + 6*(n%2)
	case ModeByte:
		return 8 * n
	}
	return 13 * n
}

// TestBCH18MatchesQRVersionInfo: QR's version information is the same
// BCH(18,6) code with no mask, so bch18Word must reproduce its table.
func TestBCH18MatchesQRVersionInfo(t *testing.T) {
	for v := 7; v <= 40; v++ {
		if got, want := bch18Word(uint32(v)), versionInfoTable[v-7]; got != want {
			t.Errorf("version %d: %05X, want %05X", v, got, want)
		}
	}
}

// rmqrFormatFinder and rmqrFormatSub are ISO/IEC 23941 Table C.1 as both zint
// and zxing-cpp transcribe it: the 64 masked format words of each copy.
var (
	rmqrFormatFinder = [64]uint32{
		0x1FAB2, 0x1E597, 0x1DBDD, 0x1C4F8, 0x1B86C, 0x1A749, 0x19903, 0x18626, 0x17F0E, 0x1602B, 0x15E61, 0x14144, 0x13DD0, 0x122F5, 0x11CBF, 0x1039A,
		0x0F1CA, 0x0EEEF, 0x0D0A5, 0x0CF80, 0x0B314, 0x0AC31, 0x0927B, 0x08D5E, 0x07476, 0x06B53, 0x05519, 0x04A3C, 0x036A8, 0x0298D, 0x017C7, 0x008E2,
		0x3F367, 0x3EC42, 0x3D208, 0x3CD2D, 0x3B1B9, 0x3AE9C, 0x390D6, 0x38FF3, 0x376DB, 0x369FE, 0x357B4, 0x34891, 0x33405, 0x32B20, 0x3156A, 0x30A4F,
		0x2F81F, 0x2E73A, 0x2D970, 0x2C655, 0x2BAC1, 0x2A5E4, 0x29BAE, 0x2848B, 0x27DA3, 0x26286, 0x25CCC, 0x243E9, 0x23F7D, 0x22058, 0x21E12, 0x20137,
	}
	rmqrFormatSub = [64]uint32{
		0x20A7B, 0x2155E, 0x22B14, 0x23431, 0x248A5, 0x25780, 0x269CA, 0x276EF, 0x28FC7, 0x290E2, 0x2AEA8, 0x2B18D, 0x2CD19, 0x2D23C, 0x2EC76, 0x2F353,
		0x30103, 0x31E26, 0x3206C, 0x33F49, 0x343DD, 0x35CF8, 0x362B2, 0x37D97, 0x384BF, 0x39B9A, 0x3A5D0, 0x3BAF5, 0x3C661, 0x3D944, 0x3E70E, 0x3F82B,
		0x003AE, 0x01C8B, 0x022C1, 0x03DE4, 0x04170, 0x05E55, 0x0601F, 0x07F3A, 0x08612, 0x09937, 0x0A77D, 0x0B858, 0x0C4CC, 0x0DBE9, 0x0E5A3, 0x0FA86,
		0x108D6, 0x117F3, 0x129B9, 0x1369C, 0x14A08, 0x1552D, 0x16B67, 0x17442, 0x18D6A, 0x1924F, 0x1AC05, 0x1B320, 0x1CFB4, 0x1D091, 0x1EEDB, 0x1F1FE,
	}
)

func TestRMQRFormatWords(t *testing.T) {
	for d := range uint32(64) {
		if got := bch18Word(d) ^ rmqrFormatMaskFinder; got != rmqrFormatFinder[d] {
			t.Errorf("finder copy %d: %05X, want %05X", d, got, rmqrFormatFinder[d])
		}
		if got := bch18Word(d) ^ rmqrFormatMaskSub; got != rmqrFormatSub[d] {
			t.Errorf("sub-pattern copy %d: %05X, want %05X", d, got, rmqrFormatSub[d])
		}
	}
	if d := rmqrFormatData(R17x139, 1); d != 63 {
		t.Errorf("R17x139-H format data %d, want 63", d)
	}
}

// TestRMQRWorkedExamples pins the worked examples of docs/theory/28-rmqr.md
// section 11: data and EC codewords derived by hand and checked against zint.
func TestRMQRWorkedExamples(t *testing.T) {
	cases := []struct {
		text      string
		v         RMQRVersion
		ec        ECLevel
		data, ec2 string
	}{
		{"123", R7x43, ECLevelH, "263D80", "A97FE556038446AC4CDF"},
		{"HELLO", R11x27, ECLevelM, "4AC2DE3300EC11", "F89900E4B0CB7117"},
		{"12345678901234", R11x27, ECLevelM, "3C3DB918A81888", "434ECFF5DBF266E0"},
	}
	for _, c := range cases {
		sym, cw, err := encodeRMQRText(c.text, c.v, 0, c.ec, true, ECINone)
		if err != nil {
			t.Fatalf("%q: %v", c.text, err)
		}
		got := fmt.Sprintf("%X", cw)
		if want := c.data + c.ec2; got != want {
			t.Errorf("%v %q:\n got  %s\n want %s", sym, c.text, got, want)
		}
	}
}

func TestRMQREncodeErrors(t *testing.T) {
	long := strings.Repeat("9", 400)
	cases := []struct {
		name   string
		text   string
		force  RMQRVersion
		height int
		ec     ECLevel
		ecSet  bool
		capErr bool
	}{
		{"too long for any symbol", long, 0, 0, ECLevelM, false, true},
		{"too long for a forced version", "12345678", R7x43, 0, ECLevelH, true, true},
		{"too long for a height", strings.Repeat("9", 200), 0, 7, ECLevelM, false, true},
		{"level L", "1", 0, 0, ECLevelL, true, false},
		{"level Q", "1", 0, 0, ECLevelQ, true, false},
		{"height 8", "1", 0, 8, ECLevelM, false, false},
		{"version and height disagree", "1", R11x27, 7, ECLevelM, false, false},
		{"version 33", "1", 33, 0, ECLevelM, false, false},
	}
	for _, c := range cases {
		_, _, err := encodeRMQRText(c.text, c.force, c.height, c.ec, c.ecSet, ECINone)
		if err == nil {
			t.Errorf("%s: no error", c.name)
			continue
		}
		if errors.Is(err, ErrCapacityExceeded) != c.capErr {
			t.Errorf("%s: %v, capacity error %v", c.name, err, c.capErr)
		}
	}
	// A fixed height chooses the narrowest width that fits at that height.
	sym, _, err := encodeRMQRText(strings.Repeat("9", 30), 0, 7, ECLevelM, true, ECINone)
	if err != nil || sym.version != R7x77 {
		t.Errorf("30 digits at height 7: %v, %v; want R7x77-M", sym, err)
	}
}
