// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
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

// TestMicroFormatInfoTable checks every Micro format word against the table in
// docs/theory/26-micro-qr.md section 7, which matches segno's precomputed
// FORMAT_INFO_MICRO.
func TestMicroFormatInfoTable(t *testing.T) {
	want := [8][4]uint32{
		{0x4445, 0x4172, 0x4E2B, 0x4B1C}, // M1
		{0x55AE, 0x5099, 0x5FC0, 0x5AF7}, // M2-L
		{0x6793, 0x62A4, 0x6DFD, 0x68CA}, // M2-M
		{0x7678, 0x734F, 0x7C16, 0x7921}, // M3-L
		{0x06DE, 0x03E9, 0x0CB0, 0x0987}, // M3-M
		{0x1735, 0x1202, 0x1D5B, 0x186C}, // M4-L
		{0x2508, 0x203F, 0x2F66, 0x2A51}, // M4-M
		{0x34E3, 0x31D4, 0x3E8D, 0x3BBA}, // M4-Q
	}
	for num := range want {
		for mask := range want[num] {
			if got := microFormatInfo(num, mask); got != want[num][mask] {
				t.Errorf("%v mask %d: %04X, want %04X", microSymbols[num], mask, got, want[num][mask])
			}
		}
	}
}

// TestBCHFormatWordMatchesQRTable runs the BCH routine Micro uses through QR's
// payloads and XOR mask, and requires it to reproduce QR's precomputed table:
// one implementation, checked against data it did not produce.
func TestBCHFormatWordMatchesQRTable(t *testing.T) {
	for ec := ECLevelL; ec <= ECLevelH; ec++ {
		for mask := 0; mask < numMasks; mask++ {
			got := bchFormatWord(uint32(ec.formatBits())<<3|uint32(mask)) ^ 0x5412
			if want := uint32(formatInfo(ec, mask)); got != want {
				t.Errorf("%v mask %d: %04X, want %04X", ec, mask, got, want)
			}
		}
	}
}

// TestMicroMatrixLayout checks the function patterns of every Micro version.
func TestMicroMatrixLayout(t *testing.T) {
	for v := M1; v <= M4; v++ {
		m := newMicroMatrix(v)
		m.placeMicroFunctionPatterns()
		n := v.Size()
		if got, want := m.dataAreaCells(), n*n-2*n-63; got != want {
			t.Errorf("%v: %d data cells, want %d", v, got, want)
		}
		// Finder corner, ring, and centre; separator on row 7 and column 7.
		for _, c := range []struct {
			r, col int
			dark   bool
		}{{0, 0, true}, {1, 1, false}, {3, 3, true}, {7, 0, false}, {0, 7, false}, {7, 7, false}} {
			if m.get(c.r, c.col) != c.dark || !m.isReserved(c.r, c.col) {
				t.Errorf("%v: finder/separator module (%d,%d) wrong", v, c.r, c.col)
			}
		}
		// Timing along row 0 and column 0, dark at even indices from 8.
		for i := 8; i < n; i++ {
			if m.get(0, i) != (i%2 == 0) || m.get(i, 0) != (i%2 == 0) {
				t.Errorf("%v: timing module %d wrong", v, i)
			}
		}
		// The 15 format cells are reserved; (0,8) and (8,0) belong to timing.
		for i := 1; i <= 8; i++ {
			if !m.isReserved(i, 8) || !m.isReserved(8, i) {
				t.Errorf("%v: format cell at index %d not reserved", v, i)
			}
		}
	}
}

// TestMicroOptionValidation covers the options Micro QR rejects.
func TestMicroOptionValidation(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
		want string
	}{
		{"WithVersion", []Option{WithMicro(), WithVersion(2)}, "use WithMicroVersion"},
		{"bad version", []Option{WithMicroVersion(MicroVersion(5))}, "invalid Micro QR version"},
		{"ECI", []Option{WithMicro(), WithECI(ECIUTF8)}, "does not support ECI"},
		{"logo", []Option{WithMicro(), WithLogo(Logo{Image: nil})}, "WithLogo is not supported"},
		{"level H", []Option{WithMicro(), WithECLevel(ECLevelH)}, "no error-correction level H"},
		{"mask 4", []Option{WithMicro(), WithMask(4)}, "0..3"},
	}
	for _, c := range cases {
		_, err := Matrix("1", c.opts...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want mention of %q", c.name, err, c.want)
		}
	}
	if _, err := EncodeStructuredAppend("12345", 2, WithMicro()); err == nil || !strings.Contains(err.Error(), "structured append") {
		t.Errorf("structured append: err = %v", err)
	}
}

// TestMicroQuietZone checks the 2-module default and that WithQuietZone wins.
func TestMicroQuietZone(t *testing.T) {
	cases := []struct {
		opts []Option
		want int // pixels per side at the default 8 px per module
	}{
		{[]Option{WithMicro()}, (11 + 2*2) * 8},
		{[]Option{WithMicro(), WithQuietZone(4)}, (11 + 2*4) * 8},
		{nil, (21 + 2*4) * 8}, // QR keeps its 4-module default
	}
	for _, c := range cases {
		data, err := Encode("1", c.opts...)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if got := img.Bounds().Dx(); got != c.want {
			t.Errorf("opts %d: width %d px, want %d", len(c.opts), got, c.want)
		}
	}
}

// TestMicroRenderers checks that the SVG and terminal renderers accept Micro
// symbols unchanged, since they only read the module grid.
func TestMicroRenderers(t *testing.T) {
	svg, err := EncodeSVG("HELLO", WithMicro())
	if err != nil || !bytes.Contains(svg, []byte("<svg")) {
		t.Errorf("EncodeSVG: %v", err)
	}
	term, err := EncodeTerminal("12345", WithMicro())
	if err != nil {
		t.Fatalf("EncodeTerminal: %v", err)
	}
	// M1 is 11 modules plus 2 quiet modules each side; the half-block renderer
	// packs two module rows per line.
	if got, want := strings.Count(term, "\n"), (11+2*2+1)/2; got != want {
		t.Errorf("terminal output has %d lines, want %d", got, want)
	}
}

// microDataPositions lists the data modules of a Micro symbol in placement
// order, so position i holds stream bit i.
func microDataPositions(v MicroVersion) [][2]int {
	m := newMicroMatrix(v)
	m.placeMicroFunctionPatterns()
	n := m.size
	var out [][2]int
	upward := true
	for col := n - 1; col > 0; col -= 2 {
		for i := 0; i < n; i++ {
			row := i
			if upward {
				row = n - 1 - i
			}
			for c := 0; c < 2; c++ {
				if !m.isReserved(row, col-c) {
					out = append(out, [2]int{row, col - c})
				}
			}
		}
		upward = !upward
	}
	return out
}

// TestMicroRoundTrip encodes across every symbol type, every mask, and every
// mode (including mixed text, Kanji, and the empty M1), and decodes it back.
func TestMicroRoundTrip(t *testing.T) {
	texts := map[MicroVersion][]string{
		M1: {"", "7", "12345"},
		M2: {"0123456789", "AB-12", "HELLO"},
		M3: {"abc", "点茗", "Ab1"},
		M4: {"hello, micro!", "ABC123def", "点茗ABC", "0123456789012345678901"},
	}
	for _, s := range microSymbols {
		ok := 0
		for _, text := range texts[s.version] {
			for mask := 0; mask < numMicroMasks; mask++ {
				opts := []Option{WithMicroVersion(s.version), WithMask(mask)}
				if !s.detectOnly {
					opts = append(opts, WithECLevel(s.ec))
				}
				grid, err := Matrix(text, opts...)
				if err != nil {
					continue // the text does not fit this level, e.g. a long text at Q
				}
				got, err := DecodeMatrix(grid)
				if err != nil || got != text {
					t.Errorf("%v mask %d %q: decoded %q, %v", s, mask, text, got, err)
					continue
				}
				ok++
			}
		}
		if ok == 0 {
			t.Errorf("%v: no text fit, so nothing was round-tripped", s)
		}
	}
}

// TestMicroErrorCorrection flips modules in distinct codewords, up to each
// type's correction capacity, and requires the original text back. M1 is the
// exception: it only detects errors, so a single flipped module must fail.
func TestMicroErrorCorrection(t *testing.T) {
	for _, s := range microSymbols {
		opts := []Option{WithMicroVersion(s.version)}
		if !s.detectOnly {
			opts = append(opts, WithECLevel(s.ec))
		}
		const text = "1"
		grid, err := Matrix(text, opts...)
		if err != nil {
			t.Fatalf("%v: %v", s, err)
		}
		pos := microDataPositions(s.version)
		flips := s.ecWords / 2
		if s.detectOnly {
			flips = 1
		}
		for k := 0; k < flips; k++ {
			p := pos[k*8] // stream bit 8k lies in data codeword k
			grid[p[0]][p[1]] = !grid[p[0]][p[1]]
		}
		got, err := DecodeMatrix(grid)
		if s.detectOnly {
			if !errors.Is(err, ErrTooManyErrors) {
				t.Errorf("M1 with one flipped module: got %q, %v; want a detection error, never a correction", got, err)
			}
			continue
		}
		if err != nil || got != text {
			t.Errorf("%v with %d corrupted codewords: decoded %q, %v", s, flips, got, err)
		}
	}
}

// TestMicroFormatDamage checks the single format copy's 3-bit budget, and
// that a blank strip is rejected for M1, whose format words all weigh at
// least 5; only the size restriction stops a blank strip on a smaller symbol
// from being read as M4-L mask 1, the one word within 3 bits of all-light.
func TestMicroFormatDamage(t *testing.T) {
	format := [15][2]int{}
	for i := 0; i < 8; i++ {
		format[i] = [2]int{i + 1, 8}
	}
	for i := 8; i < 15; i++ {
		format[i] = [2]int{8, 15 - i}
	}
	for _, text := range []string{"12345", "HELLO", "abc", "hello, micro!"} {
		grid, err := Matrix(text, WithMicro())
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range []int{0, 7, 14} { // three errors across both arms
			p := format[i]
			grid[p[0]][p[1]] = !grid[p[0]][p[1]]
		}
		if got, err := DecodeMatrix(grid); err != nil || got != text {
			t.Errorf("%q with 3 format errors: %q, %v", text, got, err)
		}
	}
	grid, _ := Matrix("12345", WithMicro())
	for _, p := range format {
		grid[p[0]][p[1]] = false
	}
	if _, err := DecodeMatrix(grid); !errors.Is(err, ErrFormatUnreadable) {
		t.Errorf("M1 with a blank format strip: err = %v, want ErrFormatUnreadable", err)
	}
}

// TestMicroDecodeRejectsBadMode feeds the segment parser an M4 data stream
// whose mode indicator is 5, which no Micro version defines.
func TestMicroDecodeRejectsBadMode(t *testing.T) {
	sym, _ := microSymbolFor(M4, ECLevelL)
	data := make([]byte, sym.dataWords)
	data[0] = 0b101_00000
	if _, err := decodeMicroText(data, sym); !errors.Is(err, ErrCorruptedPayload) {
		t.Errorf("err = %v, want ErrCorruptedPayload", err)
	}
}
