// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

// formatInfo returns the 15-bit format-information codeword (already XOR-masked
// with 0x5412) for the given EC level and mask index. Mask is expected to be
// in [0, 7]; out-of-range values return 0.
// See docs/theory/07-format-version-info.md and docs/theory/09-data-tables.md §11.
func formatInfo(ec ECLevel, mask int) uint16 {
	if mask < 0 || mask > 7 {
		return 0
	}
	if ec < ECLevelL || ec > ECLevelH {
		return 0
	}
	return formatInfoTable[ec][mask]
}

// versionInfo returns the 18-bit version-information codeword for versions
// 7..40. Returns 0 for any version less than 7 (version info is omitted from
// those symbols).
// See docs/theory/07-format-version-info.md and docs/theory/09-data-tables.md §12.
func versionInfo(v Version) uint32 {
	if v < 7 || v > MaxVersion {
		return 0
	}
	return versionInfoTable[v-7]
}

// formatInfoTable[ec level][mask 0..7] = 15-bit codeword post-XOR.
// Reproduced from ISO/IEC 18004:2015 Annex C; see docs/theory/09-data-tables.md §11.
var formatInfoTable = [4][8]uint16{
	// mask 0,    1,      2,      3,      4,      5,      6,      7
	{0x77C4, 0x72F3, 0x7DAA, 0x789D, 0x662F, 0x6318, 0x6C41, 0x6976}, // L
	{0x5412, 0x5125, 0x5E7C, 0x5B4B, 0x45F9, 0x40CE, 0x4F97, 0x4AA0}, // M
	{0x355F, 0x3068, 0x3F31, 0x3A06, 0x24B4, 0x2183, 0x2EDA, 0x2BED}, // Q
	{0x1689, 0x13BE, 0x1CE7, 0x19D0, 0x0762, 0x0255, 0x0D0C, 0x083B}, // H
}

// versionInfoTable[v-7] = 18-bit codeword for versions 7..40.
// Reproduced from ISO/IEC 18004:2015 Annex D; see docs/theory/09-data-tables.md §12.
var versionInfoTable = [34]uint32{
	0x07C94, 0x085BC, 0x09A99, 0x0A4D3, 0x0BBF6, 0x0C762, 0x0D847, 0x0E60D, 0x0F928, // V7..V15
	0x10B78, 0x1145D, 0x12A17, 0x13532, 0x149A6, 0x15683, 0x168C9, 0x177EC, 0x18EC4, // V16..V24
	0x191E1, 0x1AFAB, 0x1B08E, 0x1CC1A, 0x1D33F, 0x1ED75, 0x1F250, 0x209D5, 0x216F0, // V25..V33
	0x228BA, 0x2379F, 0x24B0B, 0x2542E, 0x26A64, 0x27541, 0x28C69,                   // V34..V40
}
