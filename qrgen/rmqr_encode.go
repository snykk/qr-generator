// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"fmt"
	"sort"
)

// rmqrSymbol is one rMQR symbol type: a version at an error-correction level
// (lvl 0 for M, 1 for H).
type rmqrSymbol struct {
	version RMQRVersion
	lvl     int
}

func (s rmqrSymbol) info() rmqrVersionInfo { return rmqrVersions[s.version-1] }

// dataWords is the symbol's number of data codewords.
func (s rmqrSymbol) dataWords() int { return s.info().data[s.lvl] }

// ecLevel is the symbol's error-correction level, ECLevelM or ECLevelH.
func (s rmqrSymbol) ecLevel() ECLevel {
	if s.lvl == 1 {
		return ECLevelH
	}
	return ECLevelM
}

// String returns names like "R11x43-M".
func (s rmqrSymbol) String() string { return s.version.String() + "-" + s.ecLevel().String() }

// rmqrModeValue is the 3-bit rMQR mode indicator of a data mode or of ECI.
func rmqrModeValue(m Mode) uint32 {
	switch m {
	case ModeNumeric:
		return 1
	case ModeAlphanumeric:
		return 2
	case ModeByte:
		return 3
	case ModeKanji:
		return 4
	case ModeECI:
		return 7
	}
	panic(fmt.Sprintf("qrgen: rmqrModeValue: mode %d has no rMQR indicator", m))
}

// rmqrSegmentCost is rMQR's cost model for the segmentation DP at version v:
// all four data modes, each paying the 3-bit mode indicator plus its count
// width at v.
func rmqrSegmentCost(v RMQRVersion) segmentCost {
	return segmentCost{
		modes:      qrModes,
		headerBits: func(m Mode) int { return 3 + rmqrCountBits(m, v) },
	}
}

// rmqrNarrowestCost is the cost model with each mode's narrowest count width
// over all 32 versions: numeric 4, alphanumeric 3, byte 3, Kanji 2 bits. No
// version's segmentation costs less, which makes it a lower bound.
var rmqrNarrowestCost = segmentCost{
	modes: qrModes,
	headerBits: func(m Mode) int {
		narrowest := 0
		for v := R7x43; v <= R17x139; v++ {
			if w := rmqrCountBits(m, v); narrowest == 0 || w < narrowest {
				narrowest = w
			}
		}
		return 3 + narrowest
	},
}

// rmqrHeights are the six rMQR symbol heights.
var rmqrHeights = [6]int{7, 9, 11, 13, 15, 17}

// validRMQRHeight reports whether h is an rMQR symbol height.
func validRMQRHeight(h int) bool {
	for _, x := range rmqrHeights {
		if h == x {
			return true
		}
	}
	return false
}

// rmqrCandidates lists the versions the encoder may choose, smallest area
// first, the lower version number winning a tie: only force when it is set,
// only the versions of the given height when that is set, and otherwise all
// 32.
func rmqrCandidates(force RMQRVersion, height int) []RMQRVersion {
	var out []RMQRVersion
	for v := R7x43; v <= R17x139; v++ {
		if (force != 0 && v != force) || (height != 0 && v.Height() != height) {
			continue
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Height()*out[i].Width() < out[j].Height()*out[j].Width()
	})
	return out
}

// rmqrCapacityError reports that a text fits no rMQR symbol the options
// allow. It matches ErrCapacityExceeded under errors.Is, with an rMQR message.
type rmqrCapacityError struct{ detail string }

func (e *rmqrCapacityError) Error() string { return "qrgen: " + e.detail }
func (e *rmqrCapacityError) Unwrap() error { return ErrCapacityExceeded }

// encodeRMQRText chooses the rMQR symbol for text and returns it with the final
// codeword stream: the data codewords and the error-correction codewords of
// every block, interleaved. force fixes the version and height fixes the
// height (zero leaves either free); ec and ecSet carry the caller's level; eci,
// when set, puts an ECI header ahead of the segments and transcodes the byte
// segments.
//
// The choice follows zint. With the level unset, it takes the smallest-area
// version that fits at M, then uses H instead when the text, without its
// terminator, is shorter than H's capacity in that same version, so the extra
// correction costs nothing (zint upgrades only with at least one bit to spare,
// and so does this, so the two agree); an explicit M or H takes the
// smallest-area version at that level. See docs/plan-rmqr.md section 5.
func encodeRMQRText(text string, force RMQRVersion, height int, ec ECLevel, ecSet bool, eci ECI) (rmqrSymbol, []byte, error) {
	if force != 0 && !force.IsValid() {
		return rmqrSymbol{}, nil, fmt.Errorf("qrgen: invalid rMQR version %v", force)
	}
	if height != 0 && !validRMQRHeight(height) {
		return rmqrSymbol{}, nil, fmt.Errorf("qrgen: invalid rMQR height %d (want 7, 9, 11, 13, 15, or 17)", height)
	}
	if force != 0 && height != 0 && force.Height() != height {
		return rmqrSymbol{}, nil, fmt.Errorf("qrgen: rMQR version %v is not %d modules tall", force, height)
	}
	lvl := 0
	if ecSet {
		l, ok := rmqrLevel(ec)
		if !ok {
			return rmqrSymbol{}, nil, fmt.Errorf("qrgen: rMQR has no error-correction level %v (only M and H)", ec)
		}
		lvl = l
	}
	if eci == ECILatin1 {
		if _, err := transcodeTo(text, eci); err != nil {
			return rmqrSymbol{}, nil, err
		}
	}
	eciBits := 0
	if eci != ECINone {
		eciBits = 3 + eci.designatorBits()
	}
	// The segmentation under the narrowest count widths of any version is a
	// lower bound on every version's cost, so one DP run rules out the versions
	// too small to hold the text without running the DP for each of them.
	_, lowerBound, _ := segmentWith(text, rmqrNarrowestCost)

	for _, v := range rmqrCandidates(force, height) {
		info := rmqrVersions[v-1]
		if lowerBound+eciBits > 8*info.data[lvl] {
			continue
		}
		segs, bits, ok := segmentWith(text, rmqrSegmentCost(v))
		if !ok || bits+eciBits > 8*info.data[lvl] {
			continue
		}
		sym := rmqrSymbol{version: v, lvl: lvl}
		if !ecSet && bits+eciBits < 8*info.data[1] {
			sym.lvl = 1
		}
		data, err := rmqrDataCodewords(segs, sym, eci)
		if err != nil {
			return rmqrSymbol{}, nil, err
		}
		return sym, rmqrFinalCodewords(data, sym), nil
	}

	switch {
	case force != 0:
		return rmqrSymbol{}, nil, &rmqrCapacityError{fmt.Sprintf("payload exceeds the capacity of rMQR %v-%v", force, rmqrSymbol{lvl: lvl}.ecLevel())}
	case height != 0:
		return rmqrSymbol{}, nil, &rmqrCapacityError{fmt.Sprintf("payload exceeds the capacity of every %d-module-tall rMQR symbol at level %v", height, rmqrSymbol{lvl: lvl}.ecLevel())}
	}
	return rmqrSymbol{}, nil, &rmqrCapacityError{fmt.Sprintf("payload exceeds the capacity of the largest rMQR symbol, R17x139, at level %v", rmqrSymbol{lvl: lvl}.ecLevel())}
}

// rmqrDataCodewords writes the ECI header (if any) and the segments with
// rMQR's 3-bit mode indicator and count widths, then the 3-bit terminator,
// padding to the codeword boundary, and the pad codewords, returning exactly
// the symbol's data codewords.
func rmqrDataCodewords(segs []segment, sym rmqrSymbol, eci ECI) ([]byte, error) {
	bb := &bitBuffer{}
	if eci != ECINone {
		bb.appendBits(rmqrModeValue(ModeECI), 3)
		appendECIDesignator(bb, eci)
	}
	for _, s := range segs {
		if err := writeRMQRSegment(bb, s, sym.version, eci); err != nil {
			return nil, err
		}
	}
	return padToCapacityTerm(bb, 8*sym.dataWords(), 3)
}

// writeRMQRSegment writes one segment at version v: the 3-bit mode indicator,
// the count at v's width, and QR's payload encoding. Under an ECI, byte
// segments are transcoded to its charset and counted in transcoded bytes.
func writeRMQRSegment(bb *bitBuffer, s segment, v RMQRVersion, eci ECI) error {
	bb.appendBits(rmqrModeValue(s.mode), 3)
	countBits := rmqrCountBits(s.mode, v)
	switch {
	case s.mode == ModeKanji:
		runes := []rune(s.text)
		bb.appendBits(uint32(len(runes)), countBits)
		writeKanji(bb, runes)
	case s.mode == ModeByte && eci != ECINone:
		payload, err := transcodeTo(s.text, eci)
		if err != nil {
			return err
		}
		bb.appendBits(uint32(len(payload)), countBits)
		for _, c := range payload {
			bb.appendBits(uint32(c), 8)
		}
	default:
		bb.appendBits(uint32(len(s.text)), countBits)
		writePayload(bb, s.mode, s.text)
	}
	return nil
}

// rmqrBlocks splits a symbol's data codewords into its Reed-Solomon blocks,
// the shorter blocks first and the longer ones one codeword longer, and
// computes each block's error-correction codewords. The EC codewords are
// shared equally among the blocks.
func rmqrBlocks(data []byte, sym rmqrSymbol) (dataBlocks, ecBlocks [][]byte) {
	info := sym.info()
	nb := info.blocks[sym.lvl]
	nd := info.data[sym.lvl]
	ecPer := (info.codewords - nd) / nb
	short := nd / nb
	nShort := nb - nd%nb
	offset := 0
	for b := 0; b < nb; b++ {
		n := short
		if b >= nShort {
			n++
		}
		block := data[offset : offset+n]
		offset += n
		dataBlocks = append(dataBlocks, block)
		ecBlocks = append(ecBlocks, encodeBlock(block, ecPer))
	}
	return dataBlocks, ecBlocks
}

// rmqrFinalCodewords returns the final codeword stream: the blocks of
// rmqrBlocks interleaved exactly as in QR.
func rmqrFinalCodewords(data []byte, sym rmqrSymbol) []byte {
	if len(data) != sym.dataWords() {
		panic(fmt.Sprintf("qrgen: rmqrFinalCodewords: %d data codewords, want %d for %v", len(data), sym.dataWords(), sym))
	}
	return interleaveBlocks(rmqrBlocks(data, sym))
}
