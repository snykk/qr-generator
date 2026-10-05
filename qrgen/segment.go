// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "unicode/utf8"

// segment is a run of text to be encoded in a single QR mode. A segmentation
// is a slice of these covering the whole input in order; concatenating their
// text fields reproduces the original input exactly.
type segment struct {
	mode Mode
	text string
}

// numericPayloadBits returns the payload bit count for n numeric characters,
// excluding the mode indicator and character-count indicator. Matches the
// numeric arm of payloadBitLength but takes a count so the DP avoids slicing.
func numericPayloadBits(n int) int {
	bits := (n / 3) * 10
	switch n % 3 {
	case 1:
		bits += 4
	case 2:
		bits += 7
	}
	return bits
}

// alphanumericPayloadBits returns the payload bit count for n alphanumeric
// characters, excluding the headers.
func alphanumericPayloadBits(n int) int {
	bits := (n / 2) * 11
	if n%2 == 1 {
		bits += 6
	}
	return bits
}

// segmentsBitLength returns the total encoded length in bits of segs at version
// v: each segment contributes its 4-bit mode indicator, its version-dependent
// character-count indicator, and its payload. Excludes the shared terminator
// and padding, which do not depend on the segmentation.
func segmentsBitLength(segs []segment, v Version) int {
	total := 0
	for _, s := range segs {
		total += 4 + s.mode.CharCountBits(v) + payloadBitLength(s.mode, s.text)
	}
	return total
}

// segmentCost is the bit-cost model the segmentation DP minimises: the modes a
// symbol allows, in tie-breaking order (on equal cost the earlier mode wins),
// and the header each segment pays in a given mode, i.e. its mode indicator
// plus character-count field. QR allows every mode with a 4-bit indicator;
// Micro QR narrows the modes by version and shortens both fields.
type segmentCost struct {
	modes      []Mode
	headerBits func(Mode) int
}

// qrModes is QR's mode list in the DP's historical tie-breaking order.
var qrModes = []Mode{ModeNumeric, ModeAlphanumeric, ModeByte, ModeKanji}

// qrSegmentCost is QR's cost model at version v.
func qrSegmentCost(v Version) segmentCost {
	return segmentCost{
		modes:      qrModes,
		headerBits: func(m Mode) int { return 4 + m.CharCountBits(v) },
	}
}

// segmentText computes a minimum-bit-length QR segmentation of text at version
// v. It is segmentWith under QR's cost model, where byte mode is always
// available, so a segmentation always exists. For homogeneous input the DP
// returns a single segment identical to the greedy analyzer's choice, so
// pure-mode payloads encode byte-for-byte as before.
// See docs/theory/17-optimal-segmentation.md.
func segmentText(text string, v Version) []segment {
	segs, _, _ := segmentWith(text, qrSegmentCost(v))
	return segs
}

// segmentWith computes a minimum-bit-length segmentation of text under the
// cost model sc via dynamic programming, and returns it with its length in
// bits (headers plus payloads, excluding terminator and padding). dp[i] is the
// least number of bits to encode the first i runes; each transition closes one
// segment runes[j:i] in an allowed, eligible mode. Numeric, alphanumeric, and
// Kanji segments may only cover contiguous runs of eligible runes (the inner
// loop breaks at the first ineligible rune); byte mode accepts any rune.
// Multi-byte runes are counted in bytes in byte mode, so the DP never splits
// a rune.
//
// ok is false when no segmentation exists, which happens only when the model
// leaves out byte mode (Micro M1 and M2) and the text contains a rune the
// remaining modes cannot encode.
func segmentWith(text string, sc segmentCost) (segs []segment, bits int, ok bool) {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		// Match the pre-segmentation empty-input behaviour: a single
		// zero-length numeric segment (numeric keeps size math consistent).
		return []segment{{mode: ModeNumeric, text: ""}}, sc.headerBits(ModeNumeric), true
	}

	// Prefix arrays for O(1) eligibility checks and byte counting.
	// nonNumeric[i]/nonAlpha[i] count runes in runes[0:i] that are NOT
	// encodable in numeric/alphanumeric mode; byteLen[i] is the UTF-8 byte
	// length of runes[0:i].
	nonNumeric := make([]int, n+1)
	nonAlpha := make([]int, n+1)
	nonKanji := make([]int, n+1)
	byteLen := make([]int, n+1)
	for i, r := range runes {
		nn, na, nk := 0, 0, 0
		if r < '0' || r > '9' {
			nn = 1
		}
		if alphanumericValue(r) < 0 {
			na = 1
		}
		if !kanjiEligible(r) {
			nk = 1
		}
		nonNumeric[i+1] = nonNumeric[i] + nn
		nonAlpha[i+1] = nonAlpha[i] + na
		nonKanji[i+1] = nonKanji[i] + nk
		byteLen[i+1] = byteLen[i] + utf8.RuneLen(r)
	}

	const inf = 1 << 30
	dp := make([]int, n+1)
	backStart := make([]int, n+1)
	backMode := make([]Mode, n+1)
	for i := 1; i <= n; i++ {
		dp[i] = inf
	}
	dp[0] = 0

	// Header cost per allowed mode, computed once outside the O(n^2) loops.
	header := make(map[Mode]int, len(sc.modes))
	for _, m := range sc.modes {
		header[m] = sc.headerBits(m)
	}
	for i := 1; i <= n; i++ {
		for _, m := range sc.modes {
			for j := i - 1; j >= 0; j-- {
				// Eligibility: a numeric/alphanumeric segment can only cover a
				// contiguous run of eligible runes. The first ineligible rune
				// (scanning back from i) bounds the run, so break.
				switch m {
				case ModeNumeric:
					if nonNumeric[i]-nonNumeric[j] != 0 {
						goto doneMode
					}
				case ModeAlphanumeric:
					if nonAlpha[i]-nonAlpha[j] != 0 {
						goto doneMode
					}
				case ModeKanji:
					if nonKanji[i]-nonKanji[j] != 0 {
						goto doneMode
					}
				}
				if dp[j] == inf {
					continue
				}
				var payload int
				switch m {
				case ModeNumeric:
					payload = numericPayloadBits(i - j)
				case ModeAlphanumeric:
					payload = alphanumericPayloadBits(i - j)
				case ModeKanji:
					payload = (i - j) * 13
				default: // ModeByte
					payload = (byteLen[i] - byteLen[j]) * 8
				}
				cost := dp[j] + header[m] + payload
				if cost < dp[i] {
					dp[i] = cost
					backStart[i] = j
					backMode[i] = m
				}
			}
		doneMode:
		}
	}

	if dp[n] == inf {
		return nil, 0, false
	}

	// Reconstruct segments back to front.
	for i := n; i > 0; {
		j := backStart[i]
		segs = append(segs, segment{mode: backMode[i], text: string(runes[j:i])})
		i = j
	}
	// Reverse into reading order.
	for l, r := 0, len(segs)-1; l < r; l, r = l+1, r-1 {
		segs[l], segs[r] = segs[r], segs[l]
	}
	return segs, dp[n], true
}
