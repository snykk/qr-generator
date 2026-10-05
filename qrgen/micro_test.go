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

// TestMicroSymbolTable checks the table's internal arithmetic: the final
// stream exactly fills the n^2 - 2n - 63 data modules, the codeword counts
// match the bit capacities, the half codeword appears in M1 and M3 only, and
// the terminator is the mode indicator plus the numeric count field.
func TestMicroSymbolTable(t *testing.T) {
	for i, s := range microSymbols {
		n := s.version.Size()
		if s.number != i {
			t.Errorf("%v: symbol number %d at index %d", s, s.number, i)
		}
		if got, want := s.streamBits(), n*n-2*n-63; got != want {
			t.Errorf("%v: stream %d bits, want %d data modules", s, got, want)
		}
		if got, want := s.dataWords, (s.dataBits+7)/8; got != want {
			t.Errorf("%v: %d data codewords for %d bits", s, got, s.dataBits)
		}
		if got, want := s.hasHalfCodeword(), s.version == M1 || s.version == M3; got != want {
			t.Errorf("%v: hasHalfCodeword = %v", s, got)
		}
		if got, want := s.version.terminatorBits(), s.version.modeIndicatorBits()+microCountBits(ModeNumeric, s.version); got != want {
			t.Errorf("%v: terminator %d bits, want %d", s, got, want)
		}
	}
	for v, want := range map[MicroVersion]int{M1: 11, M2: 13, M3: 15, M4: 17} {
		if got := v.Size(); got != want {
			t.Errorf("%v.Size() = %d, want %d", v, got, want)
		}
	}
}

// TestMicroWorkedExamples encodes the worked examples of
// docs/theory/26-micro-qr.md section 11, derived by hand from the rules, and
// compares the data portion of the stream bit for bit. The empty input is the
// one case zint cannot confirm (it refuses empty input): M1 with a zero count,
// where the second codeword becomes a full pad codeword.
func TestMicroWorkedExamples(t *testing.T) {
	cases := []struct {
		text  string
		force MicroVersion
		ec    ECLevel
		ecSet bool
		want  string // data bits, codewords separated by '|'
	}{
		{"12345", M1, 0, false, "10100011|11011010|1101"},
		{"1", M1, 0, false, "00100010|00000000|0000"},
		{"", M1, 0, false, "00000000|11101100|0000"},
		{"0123456789", M2, ECLevelL, true, "01010000|00011000|10101100|11010100|11010010"},
		{"1234", M4, ECLevelQ, true, "00000010|00001111|01101000|00000000|11101100|00010001|11101100|00010001|11101100|00010001"},
		{"HELLO WORLD", M3, ECLevelM, true, "01101101|10000101|10111100|01101000|10111001|01101110|00100110|10100001|1010"},
		{"01234567", M2, ECLevelL, true, "01000000|00011000|10101100|11000011|00000000"},
	}
	for _, c := range cases {
		sym, stream, err := encodeMicroText(c.text, c.force, c.ec, c.ecSet)
		if err != nil {
			t.Errorf("%q: %v", c.text, err)
			continue
		}
		want := strings.ReplaceAll(c.want, "|", "")
		if got := bitString(stream)[:sym.dataBits]; got != want {
			t.Errorf("%v %q data bits\n got %s\nwant %s", sym, c.text, got, want)
		}
	}
}

// TestMicroStandardExampleEC checks the EC codewords of the standard's own
// example, "01234567" in M2-L, computed independently and matched against zint.
func TestMicroStandardExampleEC(t *testing.T) {
	sym, stream, err := encodeMicroText("01234567", M2, ECLevelL, true)
	if err != nil {
		t.Fatal(err)
	}
	bits := bitString(stream)
	var got []string
	for i := sym.dataBits; i < len(bits); i += 8 {
		var b byte
		for _, c := range bits[i : i+8] {
			b = b<<1 | byte(c-'0')
		}
		got = append(got, fmt.Sprintf("%02X", b))
	}
	if g, want := strings.Join(got, " "), "86 0D 22 AE 30"; g != want {
		t.Errorf("EC codewords = %s, want %s", g, want)
	}
}

// TestMicroSegmentFeasibility checks that the cost model honours each
// version's modes, including reporting text a version cannot encode at all.
func TestMicroSegmentFeasibility(t *testing.T) {
	cases := []struct {
		text string
		v    MicroVersion
		ok   bool
		mode Mode
	}{
		{"123", M1, true, ModeNumeric},
		{"A", M1, false, 0},
		{"ABC", M2, true, ModeAlphanumeric},
		{"abc", M2, false, 0},
		{"abc", M3, true, ModeByte},
		{"点茗", M2, false, 0},
		{"点茗", M3, true, ModeKanji},
	}
	for _, c := range cases {
		segs, _, ok := segmentWith(c.text, microSegmentCost(c.v))
		if ok != c.ok {
			t.Errorf("%v %q: ok = %v, want %v", c.v, c.text, ok, c.ok)
			continue
		}
		if ok && (len(segs) != 1 || segs[0].mode != c.mode) {
			t.Errorf("%v %q: segments %+v, want one %v segment", c.v, c.text, segs, c.mode)
		}
	}
}

// TestMicroEncodeErrors covers the cases encodeMicroText must refuse.
func TestMicroEncodeErrors(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		force    MicroVersion
		ec       ECLevel
		ecSet    bool
		capacity bool   // must match ErrCapacityExceeded
		contains string // expected message fragment
	}{
		{"letters in M1", "A", M1, 0, false, true, "numeric digits only"},
		{"lowercase in M2", "abc", M2, 0, false, true, "numeric and alphanumeric"},
		{"too long for M4", strings.Repeat("0", 36), 0, 0, false, true, "M4-L"},
		{"too long at Q", strings.Repeat("0", 22), 0, ECLevelQ, true, true, "M4-Q"},
		{"level H", "1", 0, ECLevelH, true, false, "no error-correction level H"},
		{"Q below M4", "1", M2, ECLevelQ, true, false, "M2 has no error-correction level Q"},
		{"explicit level on M1", "1", M1, ECLevelL, true, false, "M1 has no error-correction level"},
		{"invalid version", "1", MicroVersion(7), 0, false, false, "invalid Micro QR version"},
	}
	for _, c := range cases {
		_, _, err := encodeMicroText(c.text, c.force, c.ec, c.ecSet)
		if err == nil {
			t.Errorf("%s: no error", c.name)
			continue
		}
		if got := errors.Is(err, ErrCapacityExceeded); got != c.capacity {
			t.Errorf("%s: errors.Is(ErrCapacityExceeded) = %v, want %v (%v)", c.name, got, c.capacity, err)
		}
		if !strings.Contains(err.Error(), c.contains) {
			t.Errorf("%s: error %q does not mention %q", c.name, err, c.contains)
		}
	}
}

// TestMicroCandidatesOrder pins the EC policy of microCandidates.
func TestMicroCandidatesOrder(t *testing.T) {
	names := func(ss []microSymbol) string {
		var out []string
		for _, s := range ss {
			out = append(out, s.String())
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		force MicroVersion
		ec    ECLevel
		ecSet bool
		want  string
	}{
		{0, 0, false, "M1 M2-M M2-L M3-M M3-L M4-M M4-L"},
		{0, ECLevelL, true, "M2-L M3-L M4-L"},
		{0, ECLevelM, true, "M2-M M3-M M4-M"},
		{0, ECLevelQ, true, "M4-Q"},
		{M3, 0, false, "M3-M M3-L"},
		{M4, ECLevelQ, true, "M4-Q"},
		{M1, ECLevelL, true, ""},
	}
	for _, c := range cases {
		if got := names(microCandidates(c.force, c.ec, c.ecSet)); got != c.want {
			t.Errorf("force=%v ec=%v set=%v: %q, want %q", c.force, c.ec, c.ecSet, got, c.want)
		}
	}
}
