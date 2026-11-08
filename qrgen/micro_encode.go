// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// microSegmentCost is the segmentation cost model at Micro version v: only the
// modes v supports, in QR's tie-breaking order, each paying v's mode-indicator
// width plus its own character-count width.
func microSegmentCost(v MicroVersion) segmentCost {
	var modes []Mode
	for _, m := range qrModes {
		if microCountBits(m, v) > 0 {
			modes = append(modes, m)
		}
	}
	return segmentCost{
		modes:      modes,
		headerBits: func(m Mode) int { return v.modeIndicatorBits() + microCountBits(m, v) },
	}
}

// microCapacityError reports that a text fits no Micro symbol the options
// allow. It matches ErrCapacityExceeded under errors.Is, but carries a Micro
// message instead of that sentinel's QR wording.
type microCapacityError struct{ detail string }

func (e *microCapacityError) Error() string { return "qrgen: " + e.detail }
func (e *microCapacityError) Unwrap() error { return ErrCapacityExceeded }

// encodeMicroText chooses the Micro symbol type for text and returns it with
// the final message bit stream: the data codewords (M1 and M3's final 4-bit
// codeword included as 4 bits), then the EC codewords. force restricts the
// version (0 means the smallest that fits); ec and ecSet carry the caller's
// error-correction level, if any, under the policy of microCandidates.
// See docs/theory/26-micro-qr.md sections 3 to 5.
func encodeMicroText(text string, force MicroVersion, ec ECLevel, ecSet bool) (microSymbol, *bitBuffer, error) {
	if force != 0 && !force.IsValid() {
		return microSymbol{}, nil, fmt.Errorf("qrgen: invalid Micro QR version %v", force)
	}
	if ecSet && ec == ECLevelH {
		return microSymbol{}, nil, fmt.Errorf("qrgen: Micro QR has no error-correction level H")
	}
	cands := microCandidates(force, ec, ecSet)
	if len(cands) == 0 {
		// Only a forced version can rule out every type: M1 with any explicit
		// level, or M2/M3 with Q.
		if force == M1 {
			return microSymbol{}, nil, fmt.Errorf("qrgen: Micro QR M1 has no error-correction level (it only detects errors); leave the level unset")
		}
		return microSymbol{}, nil, fmt.Errorf("qrgen: Micro QR %v has no error-correction level %v", force, ec)
	}

	for _, sym := range cands {
		segs, bits, ok := segmentWith(text, microSegmentCost(sym.version))
		if !ok || bits > sym.dataBits {
			continue
		}
		data := microData(segs, sym)
		return sym, microFinalStream(data, sym), nil
	}

	last := cands[len(cands)-1]
	if _, _, ok := segmentWith(text, microSegmentCost(last.version)); !ok {
		modes := "numeric digits only"
		if last.version == M2 {
			modes = "numeric and alphanumeric characters only"
		}
		return microSymbol{}, nil, &microCapacityError{fmt.Sprintf("Micro QR %v cannot encode this text: it supports %s", last.version, modes)}
	}
	return microSymbol{}, nil, &microCapacityError{fmt.Sprintf("payload exceeds the capacity of Micro QR %v", last)}
}

// microData writes segs with sym's field widths and pads them to sym's data
// capacity, returning the data bit stream.
func microData(segs []segment, sym microSymbol) *bitBuffer {
	data := &bitBuffer{}
	for _, s := range segs {
		writeMicroSegment(data, s, sym.version)
	}
	microPadToCapacity(data, sym)
	return data
}

// writeMicroSegment writes one segment with Micro's mode-indicator and
// character-count widths at version v. The payload encodings are QR's.
func writeMicroSegment(bb *bitBuffer, s segment, v MicroVersion) {
	bb.appendBits(microModeValue(s.mode), v.modeIndicatorBits())
	if s.mode == ModeKanji {
		runes := []rune(s.text)
		bb.appendBits(uint32(len(runes)), microCountBits(s.mode, v))
		writeKanji(bb, runes)
		return
	}
	bb.appendBits(uint32(len(s.text)), microCountBits(s.mode, v))
	writePayload(bb, s.mode, s.text)
}

// microPadToCapacity extends the data bit stream to the symbol's capacity:
// the version's terminator (cut short if the capacity runs out first), zero
// bits to the next codeword boundary (none if already on one), then pad
// codewords 11101100 and 00010001 alternately through the full data codewords.
// In M1 and M3 the final data codeword is 4 bits long; any part of it the data
// leaves unused is zero, which covers both its padding bits and its 0000 pad
// value.
func microPadToCapacity(bb *bitBuffer, sym microSymbol) {
	remaining := sym.dataBits - bb.bits()
	if remaining < 0 {
		panic(fmt.Sprintf("qrgen: microPadToCapacity: %d data bits exceed %v's capacity of %d", bb.bits(), sym, sym.dataBits))
	}
	bb.appendBits(0, min(sym.version.terminatorBits(), remaining))

	full := sym.dataBits - sym.dataBits%8 // bits in the full 8-bit data codewords
	if bb.bits() < full {
		if pad := (8 - bb.bits()%8) % 8; pad > 0 {
			bb.appendBits(0, pad)
		}
		padBytes := [2]uint32{0xEC, 0x11}
		for i := 0; bb.bits() < full; i++ {
			bb.appendBits(padBytes[i%2], 8)
		}
	}
	if rest := sym.dataBits - bb.bits(); rest > 0 {
		bb.appendBits(0, rest)
	}
}

// microFinalStream computes the EC codewords over the padded data and returns
// the final message bit stream. Reed-Solomon sees M1/M3's 4-bit final codeword
// as a byte with those bits in the high nibble and zeros below, which is how
// bitBuffer stores a trailing partial byte; only the 4 bits are emitted.
func microFinalStream(data *bitBuffer, sym microSymbol) *bitBuffer {
	words := data.bytes()
	if data.bits() != sym.dataBits || len(words) != sym.dataWords {
		panic(fmt.Sprintf("qrgen: microFinalStream: %d bits / %d codewords, want %d / %d for %v",
			data.bits(), len(words), sym.dataBits, sym.dataWords, sym))
	}
	ecWords := encodeBlock(words, sym.ecWords)

	out := &bitBuffer{}
	full := sym.dataWords
	if sym.hasHalfCodeword() {
		full--
	}
	for _, w := range words[:full] {
		out.appendBits(uint32(w), 8)
	}
	if sym.hasHalfCodeword() {
		out.appendBits(uint32(words[full]>>4), 4)
	}
	for _, w := range ecWords {
		out.appendBits(uint32(w), 8)
	}
	return out
}
