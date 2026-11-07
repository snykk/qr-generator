// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// RMQRVersion is an rMQR Code (Rectangular Micro QR Code, ISO/IEC 23941)
// version: one of 32 rectangular sizes, named R{height}x{width} after its
// height and width in modules, as the standard names them. It is a separate
// type from Version and MicroVersion on purpose: rMQR has its own sizes,
// tables, and layout. See docs/theory/28-rmqr.md.
type RMQRVersion int

// The 32 rMQR Code versions, in the standard's order: heights 7 to 17, and
// within a height, widths from narrowest to widest.
const (
	R7x43 RMQRVersion = iota + 1
	R7x59
	R7x77
	R7x99
	R7x139
	R9x43
	R9x59
	R9x77
	R9x99
	R9x139
	R11x27
	R11x43
	R11x59
	R11x77
	R11x99
	R11x139
	R13x27
	R13x43
	R13x59
	R13x77
	R13x99
	R13x139
	R15x43
	R15x59
	R15x77
	R15x99
	R15x139
	R17x43
	R17x59
	R17x77
	R17x99
	R17x139
)

// IsValid reports whether v is one of R7x43 to R17x139.
func (v RMQRVersion) IsValid() bool { return v >= R7x43 && v <= R17x139 }

// Height returns the symbol's height in modules (7 to 17), or 0 for an
// invalid version.
func (v RMQRVersion) Height() int {
	if !v.IsValid() {
		return 0
	}
	return rmqrVersions[v-1].height
}

// Width returns the symbol's width in modules (27 to 139), or 0 for an
// invalid version.
func (v RMQRVersion) Width() int {
	if !v.IsValid() {
		return 0
	}
	return rmqrVersions[v-1].width
}

// String returns the standard's name, such as "R11x43", or "RMQR(n)" for an
// invalid value.
func (v RMQRVersion) String() string {
	if !v.IsValid() {
		return fmt.Sprintf("RMQR(%d)", int(v))
	}
	return fmt.Sprintf("R%dx%d", v.Height(), v.Width())
}

// rmqrVersionInfo holds one version's facts (docs/theory/28-rmqr.md section
// 2). Levels are indexed 0 for M and 1 for H; count widths are indexed
// numeric, alphanumeric, byte, Kanji.
type rmqrVersionInfo struct {
	height, width int
	codewords     int    // every codeword in the symbol, data and EC
	data          [2]int // data codewords at M and H
	blocks        [2]int // Reed-Solomon blocks at M and H
	remainder     int    // data modules left after the last codeword
	countBits     [4]int // character-count widths
}

// rmqrVersions lists the 32 versions in order. The values agree across zint,
// zxing-cpp, and (but for two block definitions it gets wrong) rmqrcode, and
// were checked against 64 zint symbols; see docs/theory/28-rmqr.md section 12.
var rmqrVersions = [32]rmqrVersionInfo{
	{7, 43, 13, [2]int{6, 3}, [2]int{1, 1}, 0, [4]int{4, 3, 3, 2}},       // R7x43
	{7, 59, 21, [2]int{12, 7}, [2]int{1, 1}, 3, [4]int{5, 5, 4, 3}},      // R7x59
	{7, 77, 32, [2]int{20, 10}, [2]int{1, 1}, 5, [4]int{6, 5, 5, 4}},     // R7x77
	{7, 99, 44, [2]int{28, 14}, [2]int{1, 1}, 6, [4]int{7, 6, 5, 5}},     // R7x99
	{7, 139, 68, [2]int{44, 24}, [2]int{1, 2}, 1, [4]int{7, 6, 6, 5}},    // R7x139
	{9, 43, 21, [2]int{12, 7}, [2]int{1, 1}, 2, [4]int{5, 5, 4, 3}},      // R9x43
	{9, 59, 33, [2]int{21, 11}, [2]int{1, 1}, 3, [4]int{6, 5, 5, 4}},     // R9x59
	{9, 77, 49, [2]int{31, 17}, [2]int{1, 2}, 1, [4]int{7, 6, 5, 5}},     // R9x77
	{9, 99, 66, [2]int{42, 22}, [2]int{1, 2}, 4, [4]int{7, 6, 6, 5}},     // R9x99
	{9, 139, 99, [2]int{63, 33}, [2]int{2, 3}, 5, [4]int{8, 7, 6, 6}},    // R9x139
	{11, 27, 15, [2]int{7, 5}, [2]int{1, 1}, 2, [4]int{4, 4, 3, 2}},      // R11x27
	{11, 43, 31, [2]int{19, 11}, [2]int{1, 1}, 1, [4]int{6, 5, 5, 4}},    // R11x43
	{11, 59, 47, [2]int{31, 15}, [2]int{1, 2}, 0, [4]int{7, 6, 5, 5}},    // R11x59
	{11, 77, 67, [2]int{43, 23}, [2]int{1, 2}, 2, [4]int{7, 6, 6, 5}},    // R11x77
	{11, 99, 89, [2]int{57, 29}, [2]int{2, 2}, 7, [4]int{8, 7, 6, 6}},    // R11x99
	{11, 139, 132, [2]int{84, 42}, [2]int{2, 3}, 6, [4]int{8, 7, 7, 6}},  // R11x139
	{13, 27, 21, [2]int{12, 7}, [2]int{1, 1}, 4, [4]int{5, 5, 4, 3}},     // R13x27
	{13, 43, 41, [2]int{27, 13}, [2]int{1, 1}, 1, [4]int{6, 6, 5, 5}},    // R13x43
	{13, 59, 60, [2]int{38, 20}, [2]int{1, 2}, 6, [4]int{7, 6, 6, 5}},    // R13x59
	{13, 77, 85, [2]int{53, 29}, [2]int{2, 2}, 4, [4]int{7, 7, 6, 6}},    // R13x77
	{13, 99, 113, [2]int{73, 35}, [2]int{2, 3}, 3, [4]int{8, 7, 7, 6}},   // R13x99
	{13, 139, 166, [2]int{106, 54}, [2]int{3, 4}, 0, [4]int{8, 8, 7, 7}}, // R13x139
	{15, 43, 51, [2]int{33, 15}, [2]int{1, 2}, 1, [4]int{7, 6, 6, 5}},    // R15x43
	{15, 59, 74, [2]int{48, 26}, [2]int{1, 2}, 4, [4]int{7, 7, 6, 5}},    // R15x59
	{15, 77, 103, [2]int{67, 31}, [2]int{2, 3}, 6, [4]int{8, 7, 7, 6}},   // R15x77
	{15, 99, 136, [2]int{88, 48}, [2]int{2, 4}, 7, [4]int{8, 7, 7, 6}},   // R15x99
	{15, 139, 199, [2]int{127, 69}, [2]int{3, 5}, 2, [4]int{9, 8, 7, 7}}, // R15x139
	{17, 43, 61, [2]int{39, 21}, [2]int{1, 2}, 1, [4]int{7, 6, 6, 5}},    // R17x43
	{17, 59, 88, [2]int{56, 28}, [2]int{2, 2}, 2, [4]int{8, 7, 6, 6}},    // R17x59
	{17, 77, 122, [2]int{78, 38}, [2]int{2, 3}, 0, [4]int{8, 7, 7, 6}},   // R17x77
	{17, 99, 160, [2]int{100, 56}, [2]int{3, 4}, 3, [4]int{8, 8, 7, 6}},  // R17x99
	{17, 139, 232, [2]int{152, 76}, [2]int{4, 6}, 4, [4]int{9, 8, 8, 7}}, // R17x139
}

// rmqrAlignmentColumns returns the columns of the alignment patterns and the
// vertical timing patterns between them, which depend on the width alone
// (ISO/IEC 23941 Table D.1); width 27 has none.
func rmqrAlignmentColumns(width int) []int {
	switch width {
	case 43:
		return []int{21}
	case 59:
		return []int{19, 39}
	case 77:
		return []int{25, 51}
	case 99:
		return []int{23, 49, 75}
	case 139:
		return []int{27, 55, 83, 111}
	}
	return nil
}

// rmqrLevel returns the table index of an rMQR error-correction level: 0 for
// M and 1 for H. ok is false for L and Q, which rMQR does not have.
func rmqrLevel(ec ECLevel) (int, bool) {
	switch ec {
	case ECLevelM:
		return 0, true
	case ECLevelH:
		return 1, true
	}
	return 0, false
}

// rmqrCountBits returns the character-count width of mode m at version v.
func rmqrCountBits(m Mode, v RMQRVersion) int {
	i := rmqrVersions[v-1].countBits
	switch m {
	case ModeNumeric:
		return i[0]
	case ModeAlphanumeric:
		return i[1]
	case ModeByte:
		return i[2]
	case ModeKanji:
		return i[3]
	}
	panic(fmt.Sprintf("qrgen: rmqrCountBits: mode %d has no rMQR count", m))
}

// The two format-information masks: one for the copy beside the finder, one
// for the copy beside the finder sub-pattern (docs/theory/28-rmqr.md section 7).
const (
	rmqrFormatMaskFinder = 0x1FAB2
	rmqrFormatMaskSub    = 0x20A7B
)

// bch18Word returns the BCH(18,6) codeword for a 6-bit payload: the payload
// followed by the remainder of payload * x^12 modulo the generator
// x^12 + x^11 + x^10 + x^9 + x^8 + x^5 + x^2 + 1 (0x1F25). QR's version
// information uses the same code; rMQR's format information XORs the result
// with one of two masks.
func bch18Word(payload uint32) uint32 {
	rem := payload << 12
	for bit := 17; bit >= 12; bit-- {
		if rem&(1<<uint(bit)) != 0 {
			rem ^= 0x1F25 << uint(bit-12)
		}
	}
	return payload<<12 | rem
}

// rmqrFormatData is the 6-bit format payload of version v at level index lvl:
// the level above the version number minus one.
func rmqrFormatData(v RMQRVersion, lvl int) uint32 {
	return uint32(lvl)<<5 | uint32(v-1)
}
