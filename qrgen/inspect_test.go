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
	"image"
	"image/color"
	"reflect"
	"strings"
	"testing"
)

// inspectCase is one encoding Inspect is checked against.
type inspectCase struct {
	name string
	text string
	opts []Option
}

func inspectCases() []inspectCase {
	logo := image.NewRGBA(image.Rect(0, 0, 4, 4))
	cases := []inspectCase{
		{"QR HELLO WORLD", "HELLO WORLD", nil},
		{"QR empty", "", nil},
		{"QR V7 forced mask", "version information", []Option{WithVersion(7), WithMask(3)}},
		{"QR multi-block", strings.Repeat("Reed-Solomon blocks ", 20), []Option{WithECLevel(ECLevelQ)}},
		{"QR mixed modes", "ABC12345678901234abc", nil},
		{"QR Kanji", "点茗 漢字テスト", nil},
		{"QR ECI Latin-1", "café crème", []Option{WithECI(ECILatin1)}},
		{"QR ECI UTF-8", "naïve", []Option{WithECI(ECIUTF8)}},
		{"QR logo", "https://example.com", []Option{WithLogo(Logo{Image: logo})}},
		{"QR V40", strings.Repeat("x", 2000), []Option{WithECLevel(ECLevelL)}},
		{"rMQR auto", "HELLO RMQR", []Option{WithRMQR()}},
		{"rMQR strip", "https://example.com/rmqr", []Option{WithRMQRHeight(7)}},
		{"rMQR R17x139 H", strings.Repeat("rectangular ", 6), []Option{WithRMQRVersion(R17x139), WithECLevel(ECLevelH)}},
		{"rMQR ECI", "über", []Option{WithRMQR(), WithECI(ECIUTF8)}},
		{"rMQR Kanji", "点茗", []Option{WithRMQR()}},
	}
	for _, sym := range microSymbols {
		opts := []Option{WithMicroVersion(sym.version)}
		if !sym.detectOnly {
			opts = append(opts, WithECLevel(sym.ec))
		}
		cases = append(cases, inspectCase{"Micro " + sym.String(), "1234", opts})
	}
	cases = append(cases,
		inspectCase{"Micro auto", "HELLO", []Option{WithMicro()}},
		inspectCase{"Micro forced mask", "MICRO", []Option{WithMicro(), WithMask(2)}},
		inspectCase{"Micro Kanji", "点", []Option{WithMicroVersion(M3)}},
	)
	return cases
}

// TestInspectMatchesMatrix checks that Inspect reports the very symbol Matrix
// produces, for every symbology and the options that change the encoding.
func TestInspectMatchesMatrix(t *testing.T) {
	for _, c := range inspectCases() {
		want, err := Matrix(c.text, c.opts...)
		if err != nil {
			t.Fatalf("%s: Matrix: %v", c.name, err)
		}
		in, err := Inspect(c.text, c.opts...)
		if err != nil {
			t.Fatalf("%s: Inspect: %v", c.name, err)
		}
		if !reflect.DeepEqual(in.Modules, want) {
			t.Errorf("%s: Inspect's modules differ from Matrix", c.name)
		}
	}
}

// TestInspectLayersAreConsistent checks the module layers against each other
// and against the codeword stream: every data module carries the bit of
// Codewords it claims, remainder modules carry zero, function modules are
// unmasked, and the role counts add up.
func TestInspectLayersAreConsistent(t *testing.T) {
	for _, c := range inspectCases() {
		in, err := Inspect(c.text, c.opts...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		seen := make(map[[2]int]bool)
		counts := make(map[ModuleRole]int)
		for r, row := range in.Roles {
			for col, role := range row {
				counts[role]++
				dark, raw := in.Modules[r][col], in.Unmasked[r][col]
				cw, bit := in.ModuleCodeword[r][col], in.ModuleBit[r][col]
				switch {
				case role.IsFunction():
					if dark != raw || cw != -1 || bit != -1 {
						t.Fatalf("%s: function module (%d,%d) is masked or carries a bit", c.name, r, col)
					}
				case role == RoleRemainder:
					if raw || cw != -1 {
						t.Fatalf("%s: remainder module (%d,%d) is dark before masking or carries a bit", c.name, r, col)
					}
					if dark != in.MaskPattern[r][col] {
						t.Fatalf("%s: remainder module (%d,%d) is not masked", c.name, r, col)
					}
				default:
					if dark != (raw != in.MaskPattern[r][col]) {
						t.Fatalf("%s: data module (%d,%d) is not masked by the pattern", c.name, r, col)
					}
					if want := in.Codewords[cw]>>uint(bit)&1 == 1; raw != want {
						t.Fatalf("%s: data module (%d,%d) is %v, but bit %d of codeword %d is %v", c.name, r, col, raw, bit, cw, want)
					}
					if seen[[2]int{cw, bit}] {
						t.Fatalf("%s: codeword %d bit %d placed twice", c.name, cw, bit)
					}
					seen[[2]int{cw, bit}] = true
				}
			}
		}
		wantBits := 8 * len(in.Codewords)
		if in.Symbology == "Micro QR Code" && in.CapacityBits%8 != 0 {
			wantBits -= 4 // the half codeword
		}
		if counts[RoleData] != wantBits {
			t.Errorf("%s: %d data modules, want %d", c.name, counts[RoleData], wantBits)
		}
		wantFormat := map[string]int{"QR Code": 30, "Micro QR Code": 15, "rMQR Code": 36}[in.Symbology]
		if counts[RoleFormat] != wantFormat {
			t.Errorf("%s: %d format modules, want %d", c.name, counts[RoleFormat], wantFormat)
		}
	}
}

// TestInspectRoleCounts pins the module census of a few symbols, worked out
// by hand from the layouts in docs/theory/05-matrix-construction.md, 26, and 28.
func TestInspectRoleCounts(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
		opts []Option
		want map[ModuleRole]int
	}{
		{"V1", "HELLO WORLD", nil, map[ModuleRole]int{
			RoleFinder: 147, RoleSeparator: 45, RoleTiming: 10, RoleDarkModule: 1, RoleFormat: 30, RoleData: 208,
		}},
		{"V7", "x", []Option{WithVersion(7)}, map[ModuleRole]int{
			// Six alignment patterns of 25 modules; the two centred on row 6
			// and column 6 take over five timing modules each.
			RoleFinder: 147, RoleSeparator: 45, RoleTiming: 2*(45-16) - 10, RoleAlignment: 6 * 25,
			RoleDarkModule: 1, RoleFormat: 30, RoleVersion: 36, RoleData: 8 * 196,
		}},
		{"M1", "1", []Option{WithMicroVersion(M1)}, map[ModuleRole]int{
			RoleFinder: 49, RoleSeparator: 15, RoleTiming: 6, RoleFormat: 15, RoleData: 36,
		}},
		{"R7x43", "1", []Option{WithRMQRVersion(R7x43)}, map[ModuleRole]int{
			// The finder fills the height: its separator is column 7 alone,
			// and the bottom-left corner pattern is its bottom edge.
			RoleFinder: 49, RoleSeparator: 7, RoleFinderSubPattern: 25, RoleCornerPattern: 4,
			RoleAlignment: 18, RoleTiming: 58, RoleFormat: 36, RoleData: 8 * 13,
		}},
		{"R11x27", "1", []Option{WithRMQRVersion(R11x27)}, map[ModuleRole]int{
			RoleFinder: 49, RoleSeparator: 15, RoleFinderSubPattern: 25, RoleCornerPattern: 9,
			RoleTiming: 41, RoleFormat: 36, RoleData: 8 * 15, RoleRemainder: 2,
		}},
	} {
		in, err := Inspect(c.text, c.opts...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := make(map[ModuleRole]int)
		for _, row := range in.Roles {
			for _, role := range row {
				got[role]++
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: role census %v, want %v", c.name, got, c.want)
		}
	}
}

// TestInspectHelloWorld checks the worked example of
// docs/theory/10-worked-example.md: one alphanumeric segment and the data and
// error-correction codewords of V1-M.
func TestInspectHelloWorld(t *testing.T) {
	in, err := Inspect("HELLO WORLD")
	if err != nil {
		t.Fatal(err)
	}
	if in.Symbology != "QR Code" || in.Version != "1" || in.ECLevel != ECLevelM || in.DetectionOnly {
		t.Errorf("symbol %s %s-%v", in.Symbology, in.Version, in.ECLevel)
	}
	wantSeg := []InspectSegment{{Mode: ModeAlphanumeric, Text: "HELLO WORLD", Count: 11, IndicatorBits: 4, CountBits: 9, PayloadBits: 61}}
	if !reflect.DeepEqual(in.Segments, wantSeg) {
		t.Errorf("segments %+v, want %+v", in.Segments, wantSeg)
	}
	if in.DataBits != 74 || in.TerminatorBits != 4 || in.CapacityBits != 128 {
		t.Errorf("bits: data %d, terminator %d, capacity %d; want 74, 4, 128", in.DataBits, in.TerminatorBits, in.CapacityBits)
	}
	data := []byte{0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D, 0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11}
	ec := []byte{0xC4, 0x23, 0x27, 0x77, 0xEB, 0xD7, 0xE7, 0xE2, 0x5D, 0x17}
	if len(in.Blocks) != 1 || !bytes.Equal(in.Blocks[0].Data, data) || !bytes.Equal(in.Blocks[0].EC, ec) {
		t.Errorf("blocks %X", in.Blocks)
	}
	if !bytes.Equal(in.Codewords, append(append([]byte(nil), data...), ec...)) {
		t.Errorf("codewords % X", in.Codewords)
	}
}

// TestInspectRMQRWorkedExamples checks two R11x27-M streams of
// docs/theory/28-rmqr.md section 11, which zint produces too: "HELLO" with
// pad codewords and two remainder bits, and fourteen digits that cut the
// terminator to two bits.
func TestInspectRMQRWorkedExamples(t *testing.T) {
	for _, c := range []struct {
		text       string
		data, ec   []byte
		terminator int
	}{
		{"HELLO", []byte{0x4A, 0xC2, 0xDE, 0x33, 0x00, 0xEC, 0x11}, []byte{0xF8, 0x99, 0x00, 0xE4, 0xB0, 0xCB, 0x71, 0x17}, 3},
		{"12345678901234", []byte{0x3C, 0x3D, 0xB9, 0x18, 0xA8, 0x18, 0x88}, []byte{0x43, 0x4E, 0xCF, 0xF5, 0xDB, 0xF2, 0x66, 0xE0}, 2},
	} {
		in, err := Inspect(c.text, WithRMQRVersion(R11x27), WithECLevel(ECLevelM))
		if err != nil {
			t.Fatal(err)
		}
		want := append(append([]byte(nil), c.data...), c.ec...)
		if in.Symbology != "rMQR Code" || in.Version != "R11x27" || !bytes.Equal(in.Codewords, want) {
			t.Errorf("%q: %s %s, codewords % X, want % X", c.text, in.Symbology, in.Version, in.Codewords, want)
		}
		if in.TerminatorBits != c.terminator {
			t.Errorf("%q: terminator %d bits, want %d", c.text, in.TerminatorBits, c.terminator)
		}
		remainder := 0
		for _, row := range in.Roles {
			for _, role := range row {
				if role == RoleRemainder {
					remainder++
				}
			}
		}
		if remainder != 2 {
			t.Errorf("%q: %d remainder modules, want 2", c.text, remainder)
		}
		if in.Mask != 4 || in.MaskScores != nil || in.MaskPenalties != nil {
			t.Errorf("%q: rMQR mask %d, scores %v, penalties %v", c.text, in.Mask, in.MaskScores, in.MaskPenalties)
		}
	}
}

// TestInspectMaskScores checks that the reported scores are the ones the
// encoder chose by: the automatic mask has the lowest QR penalty, or the
// highest Micro score with the lowest index winning ties, and forcing a mask
// leaves the scores as they were.
func TestInspectMaskScores(t *testing.T) {
	for _, text := range []string{"HELLO WORLD", "https://example.com/inspect", "0123456789"} {
		in, err := Inspect(text)
		if err != nil {
			t.Fatal(err)
		}
		best := 0
		for k, s := range in.MaskScores {
			if s < in.MaskScores[best] {
				best = k
			}
			p := in.MaskPenalties[k]
			if p[0]+p[1]+p[2]+p[3] != s {
				t.Errorf("%q mask %d: rules %v do not add up to %d", text, k, p, s)
			}
		}
		if in.Mask != best {
			t.Errorf("%q: mask %d, but the lowest penalty is mask %d (%v)", text, in.Mask, best, in.MaskScores)
		}
		forced, err := Inspect(text, WithMask((best+1)%8))
		if err != nil {
			t.Fatal(err)
		}
		if forced.Mask != (best+1)%8 || !reflect.DeepEqual(forced.MaskScores, in.MaskScores) {
			t.Errorf("%q forced: mask %d, scores %v; want %d, %v", text, forced.Mask, forced.MaskScores, (best+1)%8, in.MaskScores)
		}
	}
	for _, text := range []string{"1", "HELLO", "micro qr"} {
		in, err := Inspect(text, WithMicro())
		if err != nil {
			t.Fatal(err)
		}
		best := 0
		for k, s := range in.MaskScores {
			if s > in.MaskScores[best] {
				best = k
			}
		}
		if len(in.MaskScores) != 4 || in.Mask != best || in.MaskPenalties != nil {
			t.Errorf("Micro %q: mask %d, scores %v, penalties %v", text, in.Mask, in.MaskScores, in.MaskPenalties)
		}
	}
}

// TestInspectSegments checks the field widths reported for each symbology,
// an ECI header, Kanji counted in characters, and a Latin-1 byte segment
// counted in transcoded bytes.
func TestInspectSegments(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
		opts []Option
		want []InspectSegment
	}{
		{"QR Latin-1", "é", []Option{WithECI(ECILatin1)}, []InspectSegment{
			{Mode: ModeECI, Count: 3, IndicatorBits: 4, PayloadBits: 8},
			{Mode: ModeByte, Text: "é", Count: 1, IndicatorBits: 4, CountBits: 8, PayloadBits: 8},
		}},
		{"QR Kanji", "点茗", nil, []InspectSegment{
			{Mode: ModeKanji, Text: "点茗", Count: 2, IndicatorBits: 4, CountBits: 8, PayloadBits: 26},
		}},
		{"Micro M2", "12A", []Option{WithMicroVersion(M2)}, []InspectSegment{
			{Mode: ModeAlphanumeric, Text: "12A", Count: 3, IndicatorBits: 1, CountBits: 3, PayloadBits: 17},
		}},
		{"rMQR UTF-8", "ü", []Option{WithRMQR(), WithECI(ECIUTF8)}, []InspectSegment{
			{Mode: ModeECI, Count: 26, IndicatorBits: 3, PayloadBits: 8},
			{Mode: ModeByte, Text: "ü", Count: 2, IndicatorBits: 3, CountBits: 3, PayloadBits: 16},
		}},
	} {
		in, err := Inspect(c.text, c.opts...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !reflect.DeepEqual(in.Segments, c.want) {
			t.Errorf("%s: segments %+v, want %+v", c.name, in.Segments, c.want)
		}
		total := 0
		for _, s := range c.want {
			total += s.Bits()
		}
		if in.DataBits != total {
			t.Errorf("%s: DataBits %d, want %d", c.name, in.DataBits, total)
		}
	}
}

// TestInspectMicroDetectionOnly checks M1's reporting: no level, a 4-bit last
// data codeword in the high nibble, and every stream bit placed once.
func TestInspectMicroDetectionOnly(t *testing.T) {
	in, err := Inspect("12345", WithMicro())
	if err != nil {
		t.Fatal(err)
	}
	if in.Version != "M1" || !in.DetectionOnly || len(in.Codewords) != 5 || in.CapacityBits != 20 {
		t.Fatalf("got %s detection-only=%v, %d codewords, capacity %d", in.Version, in.DetectionOnly, len(in.Codewords), in.CapacityBits)
	}
	if in.Codewords[2]&0x0F != 0 {
		t.Errorf("half codeword %02X has bits in its low nibble", in.Codewords[2])
	}
}

// TestInspectErrors checks that Inspect fails exactly as Matrix does.
func TestInspectErrors(t *testing.T) {
	for _, c := range []struct {
		text string
		opts []Option
	}{
		{"x", []Option{WithVersion(41)}},
		{"x", []Option{WithRMQR(), WithMask(1)}},
		{"x", []Option{WithMicro(), WithECLevel(ECLevelH)}},
		{strings.Repeat("9", 100), []Option{WithMicro()}},
		{"€", []Option{WithECI(ECILatin1)}},
	} {
		_, want := Matrix(c.text, c.opts...)
		_, got := Inspect(c.text, c.opts...)
		if want == nil || got == nil || got.Error() != want.Error() {
			t.Errorf("Inspect error %v, Matrix error %v", got, want)
		}
	}
	if _, err := Inspect(strings.Repeat("9", 100), WithMicro()); !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("capacity error %v does not wrap ErrCapacityExceeded", err)
	}
}

// TestInspectLeavesEncodersAlone checks that tracing is off for the encoders:
// a traced matrix is only ever built by Inspect.
func TestInspectLeavesEncodersAlone(t *testing.T) {
	o := resolveOptions(WithColors(color.Black, color.White))
	for _, build := range []func() (*matrix, int, error){
		func() (*matrix, int, error) { return buildMatrix("plain", o) },
		func() (*matrix, int, error) { return buildMatrix("1", resolveOptions(WithMicro())) },
		func() (*matrix, int, error) { return buildMatrix("1", resolveOptions(WithRMQR())) },
	} {
		m, _, err := build()
		if err != nil {
			t.Fatal(err)
		}
		if m.trace != nil {
			t.Errorf("an encoder built a traced matrix")
		}
	}
}

func TestModuleRoleString(t *testing.T) {
	for role, want := range map[ModuleRole]string{
		RoleData: "data", RoleDarkModule: "dark module", RoleFinderSubPattern: "finder sub-pattern", ModuleRole(99): "ModuleRole(99)",
	} {
		if got := role.String(); got != want {
			t.Errorf("%d: %q, want %q", role, got, want)
		}
	}
	if RoleData.IsFunction() || RoleRemainder.IsFunction() || !RoleTiming.IsFunction() {
		t.Error("IsFunction misclassifies data, remainder, or timing")
	}
}
