// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// Version is a QR code version number, 1..40 inclusive.
// See docs/theory/01-qr-overview.md for what a version is.
type Version int

const (
	MinVersion Version = 1
	MaxVersion Version = 40
)

// Size returns the matrix side length, in modules.
// side(v) = 21 + 4 * (v - 1).
func (v Version) Size() int {
	return 21 + 4*(int(v)-1)
}

// IsValid reports whether v is in the inclusive range [MinVersion, MaxVersion].
func (v Version) IsValid() bool {
	return v >= MinVersion && v <= MaxVersion
}

// ECLevel is one of the four QR error-correction levels.
// Listed in order of increasing redundancy: L, M, Q, H.
// See docs/theory/01-qr-overview.md.
type ECLevel int

const (
	ECLevelL ECLevel = iota
	ECLevelM
	ECLevelQ
	ECLevelH
)

// String returns the single-letter name of the EC level.
func (e ECLevel) String() string {
	switch e {
	case ECLevelL:
		return "L"
	case ECLevelM:
		return "M"
	case ECLevelQ:
		return "Q"
	case ECLevelH:
		return "H"
	}
	return fmt.Sprintf("ECLevel(%d)", int(e))
}

// formatBits returns the 2-bit encoding of this level as used in the
// format-information codeword (see docs/theory/09-data-tables.md §11).
// The mapping is L=01, M=00, Q=11, H=10 — deliberately non-monotonic.
func (e ECLevel) formatBits() uint8 {
	switch e {
	case ECLevelL:
		return 0b01
	case ECLevelM:
		return 0b00
	case ECLevelQ:
		return 0b11
	case ECLevelH:
		return 0b10
	}
	return 0
}

// ECBlockGroup describes one group of equally-sized EC blocks.
type ECBlockGroup struct {
	// Count is the number of blocks in this group.
	Count int
	// DataPerBlock is the number of data codewords per block in this group.
	DataPerBlock int
}

// ECBlockSpec describes how a (version, EC level) pair splits the data and EC
// codewords into blocks. Some pairs use only Group1; others split data into
// two block sizes that differ by 1 codeword (Group2.Count == 0 indicates the
// single-group case).
type ECBlockSpec struct {
	// ECPerBlock is the number of error-correction codewords per block,
	// the same for every block in this (version, level).
	ECPerBlock int
	Group1     ECBlockGroup
	Group2     ECBlockGroup
}

// TotalDataCodewords returns the sum of data codewords across both groups.
func (s ECBlockSpec) TotalDataCodewords() int {
	return s.Group1.Count*s.Group1.DataPerBlock +
		s.Group2.Count*s.Group2.DataPerBlock
}

// TotalBlocks returns the total number of EC blocks (Group1.Count + Group2.Count).
func (s ECBlockSpec) TotalBlocks() int {
	return s.Group1.Count + s.Group2.Count
}

// TotalECCodewords returns the total number of EC codewords across all blocks.
func (s ECBlockSpec) TotalECCodewords() int {
	return s.TotalBlocks() * s.ECPerBlock
}

// DataCodewords returns the number of data codewords available for the given
// (version, EC level). Multiply by 8 to get bit capacity.
// Source: ISO/IEC 18004:2015 Annex A; see docs/theory/09-data-tables.md §6.
func (v Version) DataCodewords(ec ECLevel) int {
	return dataCodewordCount[v-1][ec]
}

// ECBlocks returns the EC block layout for the given (version, EC level).
// Source: ISO/IEC 18004:2015 Table 9; see docs/theory/09-data-tables.md §7.
func (v Version) ECBlocks(ec ECLevel) ECBlockSpec {
	return ecBlocks[v-1][ec]
}

// RemainderBits returns the number of zero bits appended after the codeword
// stream so it fills the data area exactly.
// Source: ISO/IEC 18004:2015 Table 1; see docs/theory/09-data-tables.md §8.
func (v Version) RemainderBits() int {
	return remainderBitsTable[v-1]
}

// AlignmentCenters returns the row/column centre coordinates for alignment
// patterns in this version. The actual patterns sit at every pair where both
// coordinates appear in the returned slice, *except* positions that overlap a
// finder pattern. Returns nil for Version 1 (no alignment patterns).
// Source: ISO/IEC 18004:2015 Annex E; see docs/theory/09-data-tables.md §9.
func (v Version) AlignmentCenters() []int {
	return alignmentCentersTable[v-1]
}

// dataCodewordCount[version-1][ec level] = data codewords.
// Order of EC levels: L, M, Q, H (matches the ECLevel constants).
var dataCodewordCount = [40][4]int{
	{19, 16, 13, 9},          // V1
	{34, 28, 22, 16},         // V2
	{55, 44, 34, 26},         // V3
	{80, 64, 48, 36},         // V4
	{108, 86, 62, 46},        // V5
	{136, 108, 76, 60},       // V6
	{156, 124, 88, 66},       // V7
	{194, 154, 110, 86},      // V8
	{232, 182, 132, 100},     // V9
	{274, 216, 154, 122},     // V10
	{324, 254, 180, 140},     // V11
	{370, 290, 206, 158},     // V12
	{428, 334, 244, 180},     // V13
	{461, 365, 261, 197},     // V14
	{523, 415, 295, 223},     // V15
	{589, 453, 325, 253},     // V16
	{647, 507, 367, 283},     // V17
	{721, 563, 397, 313},     // V18
	{795, 627, 445, 341},     // V19
	{861, 669, 485, 385},     // V20
	{932, 714, 512, 406},     // V21
	{1006, 782, 568, 442},    // V22
	{1094, 860, 614, 464},    // V23
	{1174, 914, 664, 514},    // V24
	{1276, 1000, 718, 538},   // V25
	{1370, 1062, 754, 596},   // V26
	{1468, 1128, 808, 628},   // V27
	{1531, 1193, 871, 661},   // V28
	{1631, 1267, 911, 701},   // V29
	{1735, 1373, 985, 745},   // V30
	{1843, 1455, 1033, 793},  // V31
	{1955, 1541, 1115, 845},  // V32
	{2071, 1631, 1171, 901},  // V33
	{2191, 1725, 1231, 961},  // V34
	{2306, 1812, 1286, 986},  // V35
	{2434, 1914, 1354, 1054}, // V36
	{2566, 1992, 1426, 1096}, // V37
	{2702, 2102, 1502, 1142}, // V38
	{2812, 2216, 1582, 1222}, // V39
	{2956, 2334, 1666, 1276}, // V40
}

// ecBlocks[version-1][ec level] = ECBlockSpec.
// Encoded as {ECPerBlock, {g1Count, g1Data}, {g2Count, g2Data}}.
// Group2 with Count=0 means a single-group layout.
var ecBlocks = [40][4]ECBlockSpec{
	// V1
	{
		{7, ECBlockGroup{1, 19}, ECBlockGroup{0, 0}},
		{10, ECBlockGroup{1, 16}, ECBlockGroup{0, 0}},
		{13, ECBlockGroup{1, 13}, ECBlockGroup{0, 0}},
		{17, ECBlockGroup{1, 9}, ECBlockGroup{0, 0}},
	},
	// V2
	{
		{10, ECBlockGroup{1, 34}, ECBlockGroup{0, 0}},
		{16, ECBlockGroup{1, 28}, ECBlockGroup{0, 0}},
		{22, ECBlockGroup{1, 22}, ECBlockGroup{0, 0}},
		{28, ECBlockGroup{1, 16}, ECBlockGroup{0, 0}},
	},
	// V3
	{
		{15, ECBlockGroup{1, 55}, ECBlockGroup{0, 0}},
		{26, ECBlockGroup{1, 44}, ECBlockGroup{0, 0}},
		{18, ECBlockGroup{2, 17}, ECBlockGroup{0, 0}},
		{22, ECBlockGroup{2, 13}, ECBlockGroup{0, 0}},
	},
	// V4
	{
		{20, ECBlockGroup{1, 80}, ECBlockGroup{0, 0}},
		{18, ECBlockGroup{2, 32}, ECBlockGroup{0, 0}},
		{26, ECBlockGroup{2, 24}, ECBlockGroup{0, 0}},
		{16, ECBlockGroup{4, 9}, ECBlockGroup{0, 0}},
	},
	// V5
	{
		{26, ECBlockGroup{1, 108}, ECBlockGroup{0, 0}},
		{24, ECBlockGroup{2, 43}, ECBlockGroup{0, 0}},
		{18, ECBlockGroup{2, 15}, ECBlockGroup{2, 16}},
		{22, ECBlockGroup{2, 11}, ECBlockGroup{2, 12}},
	},
	// V6
	{
		{18, ECBlockGroup{2, 68}, ECBlockGroup{0, 0}},
		{16, ECBlockGroup{4, 27}, ECBlockGroup{0, 0}},
		{24, ECBlockGroup{4, 19}, ECBlockGroup{0, 0}},
		{28, ECBlockGroup{4, 15}, ECBlockGroup{0, 0}},
	},
	// V7
	{
		{20, ECBlockGroup{2, 78}, ECBlockGroup{0, 0}},
		{18, ECBlockGroup{4, 31}, ECBlockGroup{0, 0}},
		{18, ECBlockGroup{2, 14}, ECBlockGroup{4, 15}},
		{26, ECBlockGroup{4, 13}, ECBlockGroup{1, 14}},
	},
	// V8
	{
		{24, ECBlockGroup{2, 97}, ECBlockGroup{0, 0}},
		{22, ECBlockGroup{2, 38}, ECBlockGroup{2, 39}},
		{22, ECBlockGroup{4, 18}, ECBlockGroup{2, 19}},
		{26, ECBlockGroup{4, 14}, ECBlockGroup{2, 15}},
	},
	// V9
	{
		{30, ECBlockGroup{2, 116}, ECBlockGroup{0, 0}},
		{22, ECBlockGroup{3, 36}, ECBlockGroup{2, 37}},
		{20, ECBlockGroup{4, 16}, ECBlockGroup{4, 17}},
		{24, ECBlockGroup{4, 12}, ECBlockGroup{4, 13}},
	},
	// V10
	{
		{18, ECBlockGroup{2, 68}, ECBlockGroup{2, 69}},
		{26, ECBlockGroup{4, 43}, ECBlockGroup{1, 44}},
		{24, ECBlockGroup{6, 19}, ECBlockGroup{2, 20}},
		{28, ECBlockGroup{6, 15}, ECBlockGroup{2, 16}},
	},
	// V11
	{
		{20, ECBlockGroup{4, 81}, ECBlockGroup{0, 0}},
		{30, ECBlockGroup{1, 50}, ECBlockGroup{4, 51}},
		{28, ECBlockGroup{4, 22}, ECBlockGroup{4, 23}},
		{24, ECBlockGroup{3, 12}, ECBlockGroup{8, 13}},
	},
	// V12
	{
		{24, ECBlockGroup{2, 92}, ECBlockGroup{2, 93}},
		{22, ECBlockGroup{6, 36}, ECBlockGroup{2, 37}},
		{26, ECBlockGroup{4, 20}, ECBlockGroup{6, 21}},
		{28, ECBlockGroup{7, 14}, ECBlockGroup{4, 15}},
	},
	// V13
	{
		{26, ECBlockGroup{4, 107}, ECBlockGroup{0, 0}},
		{22, ECBlockGroup{8, 37}, ECBlockGroup{1, 38}},
		{24, ECBlockGroup{8, 20}, ECBlockGroup{4, 21}},
		{22, ECBlockGroup{12, 11}, ECBlockGroup{4, 12}},
	},
	// V14
	{
		{30, ECBlockGroup{3, 115}, ECBlockGroup{1, 116}},
		{24, ECBlockGroup{4, 40}, ECBlockGroup{5, 41}},
		{20, ECBlockGroup{11, 16}, ECBlockGroup{5, 17}},
		{24, ECBlockGroup{11, 12}, ECBlockGroup{5, 13}},
	},
	// V15
	{
		{22, ECBlockGroup{5, 87}, ECBlockGroup{1, 88}},
		{24, ECBlockGroup{5, 41}, ECBlockGroup{5, 42}},
		{30, ECBlockGroup{5, 24}, ECBlockGroup{7, 25}},
		{24, ECBlockGroup{11, 12}, ECBlockGroup{7, 13}},
	},
	// V16
	{
		{24, ECBlockGroup{5, 98}, ECBlockGroup{1, 99}},
		{28, ECBlockGroup{7, 45}, ECBlockGroup{3, 46}},
		{24, ECBlockGroup{15, 19}, ECBlockGroup{2, 20}},
		{30, ECBlockGroup{3, 15}, ECBlockGroup{13, 16}},
	},
	// V17
	{
		{28, ECBlockGroup{1, 107}, ECBlockGroup{5, 108}},
		{28, ECBlockGroup{10, 46}, ECBlockGroup{1, 47}},
		{28, ECBlockGroup{1, 22}, ECBlockGroup{15, 23}},
		{28, ECBlockGroup{2, 14}, ECBlockGroup{17, 15}},
	},
	// V18
	{
		{30, ECBlockGroup{5, 120}, ECBlockGroup{1, 121}},
		{26, ECBlockGroup{9, 43}, ECBlockGroup{4, 44}},
		{28, ECBlockGroup{17, 22}, ECBlockGroup{1, 23}},
		{28, ECBlockGroup{2, 14}, ECBlockGroup{19, 15}},
	},
	// V19
	{
		{28, ECBlockGroup{3, 113}, ECBlockGroup{4, 114}},
		{26, ECBlockGroup{3, 44}, ECBlockGroup{11, 45}},
		{26, ECBlockGroup{17, 21}, ECBlockGroup{4, 22}},
		{26, ECBlockGroup{9, 13}, ECBlockGroup{16, 14}},
	},
	// V20
	{
		{28, ECBlockGroup{3, 107}, ECBlockGroup{5, 108}},
		{26, ECBlockGroup{3, 41}, ECBlockGroup{13, 42}},
		{30, ECBlockGroup{15, 24}, ECBlockGroup{5, 25}},
		{28, ECBlockGroup{15, 15}, ECBlockGroup{10, 16}},
	},
	// V21
	{
		{28, ECBlockGroup{4, 116}, ECBlockGroup{4, 117}},
		{26, ECBlockGroup{17, 42}, ECBlockGroup{0, 0}},
		{28, ECBlockGroup{17, 22}, ECBlockGroup{6, 23}},
		{30, ECBlockGroup{19, 16}, ECBlockGroup{6, 17}},
	},
	// V22
	{
		{28, ECBlockGroup{2, 111}, ECBlockGroup{7, 112}},
		{28, ECBlockGroup{17, 46}, ECBlockGroup{0, 0}},
		{30, ECBlockGroup{7, 24}, ECBlockGroup{16, 25}},
		{24, ECBlockGroup{34, 13}, ECBlockGroup{0, 0}},
	},
	// V23
	{
		{30, ECBlockGroup{4, 121}, ECBlockGroup{5, 122}},
		{28, ECBlockGroup{4, 47}, ECBlockGroup{14, 48}},
		{30, ECBlockGroup{11, 24}, ECBlockGroup{14, 25}},
		{30, ECBlockGroup{16, 15}, ECBlockGroup{14, 16}},
	},
	// V24
	{
		{30, ECBlockGroup{6, 117}, ECBlockGroup{4, 118}},
		{28, ECBlockGroup{6, 45}, ECBlockGroup{14, 46}},
		{30, ECBlockGroup{11, 24}, ECBlockGroup{16, 25}},
		{30, ECBlockGroup{30, 16}, ECBlockGroup{2, 17}},
	},
	// V25
	{
		{26, ECBlockGroup{8, 106}, ECBlockGroup{4, 107}},
		{28, ECBlockGroup{8, 47}, ECBlockGroup{13, 48}},
		{30, ECBlockGroup{7, 24}, ECBlockGroup{22, 25}},
		{30, ECBlockGroup{22, 15}, ECBlockGroup{13, 16}},
	},
	// V26
	{
		{28, ECBlockGroup{10, 114}, ECBlockGroup{2, 115}},
		{28, ECBlockGroup{19, 46}, ECBlockGroup{4, 47}},
		{28, ECBlockGroup{28, 22}, ECBlockGroup{6, 23}},
		{30, ECBlockGroup{33, 16}, ECBlockGroup{4, 17}},
	},
	// V27
	{
		{30, ECBlockGroup{8, 122}, ECBlockGroup{4, 123}},
		{28, ECBlockGroup{22, 45}, ECBlockGroup{3, 46}},
		{30, ECBlockGroup{8, 23}, ECBlockGroup{26, 24}},
		{30, ECBlockGroup{12, 15}, ECBlockGroup{28, 16}},
	},
	// V28
	{
		{30, ECBlockGroup{3, 117}, ECBlockGroup{10, 118}},
		{28, ECBlockGroup{3, 45}, ECBlockGroup{23, 46}},
		{30, ECBlockGroup{4, 24}, ECBlockGroup{31, 25}},
		{30, ECBlockGroup{11, 15}, ECBlockGroup{31, 16}},
	},
	// V29
	{
		{30, ECBlockGroup{7, 116}, ECBlockGroup{7, 117}},
		{28, ECBlockGroup{21, 45}, ECBlockGroup{7, 46}},
		{30, ECBlockGroup{1, 23}, ECBlockGroup{37, 24}},
		{30, ECBlockGroup{19, 15}, ECBlockGroup{26, 16}},
	},
	// V30
	{
		{30, ECBlockGroup{5, 115}, ECBlockGroup{10, 116}},
		{28, ECBlockGroup{19, 47}, ECBlockGroup{10, 48}},
		{30, ECBlockGroup{15, 24}, ECBlockGroup{25, 25}},
		{30, ECBlockGroup{23, 15}, ECBlockGroup{25, 16}},
	},
	// V31
	{
		{30, ECBlockGroup{13, 115}, ECBlockGroup{3, 116}},
		{28, ECBlockGroup{2, 46}, ECBlockGroup{29, 47}},
		{30, ECBlockGroup{42, 24}, ECBlockGroup{1, 25}},
		{30, ECBlockGroup{23, 15}, ECBlockGroup{28, 16}},
	},
	// V32
	{
		{30, ECBlockGroup{17, 115}, ECBlockGroup{0, 0}},
		{28, ECBlockGroup{10, 46}, ECBlockGroup{23, 47}},
		{30, ECBlockGroup{10, 24}, ECBlockGroup{35, 25}},
		{30, ECBlockGroup{19, 15}, ECBlockGroup{35, 16}},
	},
	// V33
	{
		{30, ECBlockGroup{17, 115}, ECBlockGroup{1, 116}},
		{28, ECBlockGroup{14, 46}, ECBlockGroup{21, 47}},
		{30, ECBlockGroup{29, 24}, ECBlockGroup{19, 25}},
		{30, ECBlockGroup{11, 15}, ECBlockGroup{46, 16}},
	},
	// V34
	{
		{30, ECBlockGroup{13, 115}, ECBlockGroup{6, 116}},
		{28, ECBlockGroup{14, 46}, ECBlockGroup{23, 47}},
		{30, ECBlockGroup{44, 24}, ECBlockGroup{7, 25}},
		{30, ECBlockGroup{59, 16}, ECBlockGroup{1, 17}},
	},
	// V35
	{
		{30, ECBlockGroup{12, 121}, ECBlockGroup{7, 122}},
		{28, ECBlockGroup{12, 47}, ECBlockGroup{26, 48}},
		{30, ECBlockGroup{39, 24}, ECBlockGroup{14, 25}},
		{30, ECBlockGroup{22, 15}, ECBlockGroup{41, 16}},
	},
	// V36
	{
		{30, ECBlockGroup{6, 121}, ECBlockGroup{14, 122}},
		{28, ECBlockGroup{6, 47}, ECBlockGroup{34, 48}},
		{30, ECBlockGroup{46, 24}, ECBlockGroup{10, 25}},
		{30, ECBlockGroup{2, 15}, ECBlockGroup{64, 16}},
	},
	// V37
	{
		{30, ECBlockGroup{17, 122}, ECBlockGroup{4, 123}},
		{28, ECBlockGroup{29, 46}, ECBlockGroup{14, 47}},
		{30, ECBlockGroup{49, 24}, ECBlockGroup{10, 25}},
		{30, ECBlockGroup{24, 15}, ECBlockGroup{46, 16}},
	},
	// V38
	{
		{30, ECBlockGroup{4, 122}, ECBlockGroup{18, 123}},
		{28, ECBlockGroup{13, 46}, ECBlockGroup{32, 47}},
		{30, ECBlockGroup{48, 24}, ECBlockGroup{14, 25}},
		{30, ECBlockGroup{42, 15}, ECBlockGroup{32, 16}},
	},
	// V39
	{
		{30, ECBlockGroup{20, 117}, ECBlockGroup{4, 118}},
		{28, ECBlockGroup{40, 47}, ECBlockGroup{7, 48}},
		{30, ECBlockGroup{43, 24}, ECBlockGroup{22, 25}},
		{30, ECBlockGroup{10, 15}, ECBlockGroup{67, 16}},
	},
	// V40
	{
		{30, ECBlockGroup{19, 118}, ECBlockGroup{6, 119}},
		{28, ECBlockGroup{18, 47}, ECBlockGroup{31, 48}},
		{30, ECBlockGroup{34, 24}, ECBlockGroup{34, 25}},
		{30, ECBlockGroup{20, 15}, ECBlockGroup{61, 16}},
	},
}

// remainderBitsTable[version-1] = zero bits appended after codewords.
var remainderBitsTable = [40]int{
	0,                      // V1
	7, 7, 7, 7, 7,          // V2..V6
	0, 0, 0, 0, 0, 0, 0,    // V7..V13
	3, 3, 3, 3, 3, 3, 3,    // V14..V20
	4, 4, 4, 4, 4, 4, 4,    // V21..V27
	3, 3, 3, 3, 3, 3, 3,    // V28..V34
	0, 0, 0, 0, 0, 0,       // V35..V40
}

// alignmentCentersTable[version-1] = row/column centre coordinates.
// V1 has no alignment patterns.
var alignmentCentersTable = [40][]int{
	nil,                                  // V1
	{6, 18},                              // V2
	{6, 22},                              // V3
	{6, 26},                              // V4
	{6, 30},                              // V5
	{6, 34},                              // V6
	{6, 22, 38},                          // V7
	{6, 24, 42},                          // V8
	{6, 26, 46},                          // V9
	{6, 28, 50},                          // V10
	{6, 30, 54},                          // V11
	{6, 32, 58},                          // V12
	{6, 34, 62},                          // V13
	{6, 26, 46, 66},                      // V14
	{6, 26, 48, 70},                      // V15
	{6, 26, 50, 74},                      // V16
	{6, 30, 54, 78},                      // V17
	{6, 30, 56, 82},                      // V18
	{6, 30, 58, 86},                      // V19
	{6, 34, 62, 90},                      // V20
	{6, 28, 50, 72, 94},                  // V21
	{6, 26, 50, 74, 98},                  // V22
	{6, 30, 54, 78, 102},                 // V23
	{6, 28, 54, 80, 106},                 // V24
	{6, 32, 58, 84, 110},                 // V25
	{6, 30, 58, 86, 114},                 // V26
	{6, 34, 62, 90, 118},                 // V27
	{6, 26, 50, 74, 98, 122},             // V28
	{6, 30, 54, 78, 102, 126},            // V29
	{6, 26, 52, 78, 104, 130},            // V30
	{6, 30, 56, 82, 108, 134},            // V31
	{6, 34, 60, 86, 112, 138},            // V32
	{6, 30, 58, 86, 114, 142},            // V33
	{6, 34, 62, 90, 118, 146},            // V34
	{6, 30, 54, 78, 102, 126, 150},       // V35
	{6, 24, 50, 76, 102, 128, 154},       // V36
	{6, 28, 54, 80, 106, 132, 158},       // V37
	{6, 32, 58, 84, 110, 136, 162},       // V38
	{6, 26, 54, 82, 110, 138, 166},       // V39
	{6, 30, 58, 86, 114, 142, 170},       // V40
}
