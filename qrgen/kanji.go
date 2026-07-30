// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "sort"

//go:generate go run gen_kanji_table.go

// Kanji mode (ISO/IEC 18004, mode indicator 1000) encodes JIS X 0208 characters
// as 13-bit values. The Unicode-to-Shift-JIS mapping lives in the generated,
// committed kanjiByValue table (kanji_table.go), sorted by 13-bit value; the
// generator is the only code that touches golang.org/x/text. See
// docs/theory/22-kanji-mode.md.

// kanjiEntry maps a Kanji-mode 13-bit value to its Unicode rune.
type kanjiEntry struct {
	v uint16
	r rune
}

// kanjiEncode is the rune -> 13-bit-value map, derived once from kanjiByValue.
// Because kanjiByValue is sorted by value ascending, the first entry seen for a
// rune has the smallest value, giving a canonical encoding for the rare runes
// with more than one Shift-JIS code.
var kanjiEncode = func() map[rune]uint16 {
	m := make(map[rune]uint16, len(kanjiByValue))
	for _, e := range kanjiByValue {
		if _, ok := m[e.r]; !ok {
			m[e.r] = e.v
		}
	}
	return m
}()

// kanjiEligible reports whether r has a JIS X 0208 code, i.e. whether it can be
// encoded in Kanji mode.
func kanjiEligible(r rune) bool {
	_, ok := kanjiEncode[r]
	return ok
}

// kanjiValue returns the 13-bit Kanji-mode value for r, or ok=false if r is not
// a JIS X 0208 character.
func kanjiValue(r rune) (uint16, bool) {
	v, ok := kanjiEncode[r]
	return v, ok
}

// kanjiRune returns the rune for a 13-bit Kanji-mode value, or ok=false if the
// value is not assigned in JIS X 0208. It binary-searches the value-sorted
// table, so no map is built for the decode direction.
func kanjiRune(v uint16) (rune, bool) {
	i := sort.Search(len(kanjiByValue), func(i int) bool { return kanjiByValue[i].v >= v })
	if i < len(kanjiByValue) && kanjiByValue[i].v == v {
		return kanjiByValue[i].r, true
	}
	return 0, false
}
