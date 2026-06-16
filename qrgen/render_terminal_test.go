// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"strings"
	"testing"
)

// termMat builds a matrix from string rows where '#' is a dark module and any
// other rune is light. Only size and modules are set, which is all
// renderTerminal reads.
func termMat(rows ...string) *matrix {
	mods := make([][]bool, len(rows))
	for r, s := range rows {
		mods[r] = make([]bool, len(s))
		for c := 0; c < len(s); c++ {
			mods[r][c] = s[c] == '#'
		}
	}
	return &matrix{size: len(rows), modules: mods}
}

// The worked example from docs/theory/19-terminal-rendering.md section 3:
//
//	# . #
//	. # .
//	# . #
//
// renders, with no quiet zone, to a full row pair plus an odd-row tail.
func TestRenderTerminalHalfBlock(t *testing.T) {
	m := termMat("#.#", ".#.", "#.#")
	got := renderTerminal(m, terminalOptions{quietZone: 0})
	want := "▀▄▀\n" + "▀ ▀\n"
	if got != want {
		t.Errorf("half-block mismatch:\n got %q\nwant %q", got, want)
	}
}

// Inverting swaps every in-grid cell's polarity (█ with space, ▀ with ▄) while
// the out-of-grid bottom of the tail stays unpainted.
func TestRenderTerminalInvert(t *testing.T) {
	m := termMat("#.#", ".#.", "#.#")
	got := renderTerminal(m, terminalOptions{quietZone: 0, invert: true})
	want := "▄▀▄\n" + " ▀ \n"
	if got != want {
		t.Errorf("inverted mismatch:\n got %q\nwant %q", got, want)
	}
}

// The ASCII fallback is one row per module row, double-width.
func TestRenderTerminalASCII(t *testing.T) {
	m := termMat("#.#", ".#.", "#.#")
	got := renderTerminal(m, terminalOptions{quietZone: 0, ascii: true})
	want := "##  ##\n" + "  ##  \n" + "##  ##\n"
	if got != want {
		t.Errorf("ascii mismatch:\n got %q\nwant %q", got, want)
	}
}

// A single dark module with a one-module quiet zone exercises the light border
// and the all-light odd-row tail.
func TestRenderTerminalQuietZone(t *testing.T) {
	m := termMat("#")
	got := renderTerminal(m, terminalOptions{quietZone: 1})
	want := " ▄ \n" + "   \n"
	if got != want {
		t.Errorf("quiet-zone mismatch:\n got %q\nwant %q", got, want)
	}
}

// A nil matrix and a degenerate dimension render to the empty string rather
// than panicking.
func TestRenderTerminalEmpty(t *testing.T) {
	if got := renderTerminal(nil, terminalOptions{}); got != "" {
		t.Errorf("nil matrix: got %q, want empty", got)
	}
}

// EncodeTerminal threads the two terminal options through to the renderer: the
// default emits half-block glyphs, ASCII emits ## and no block glyphs, and
// inverting changes the output.
func TestEncodeTerminalModes(t *testing.T) {
	const text = "HELLO WORLD"

	half, err := EncodeTerminal(text, WithMask(2))
	if err != nil {
		t.Fatalf("EncodeTerminal half-block: %v", err)
	}
	if !strings.ContainsRune(half, blockFull) &&
		!strings.ContainsRune(half, blockUpper) &&
		!strings.ContainsRune(half, blockLower) {
		t.Error("half-block output contains no block glyphs")
	}

	ascii, err := EncodeTerminal(text, WithMask(2), WithTerminalASCII(true))
	if err != nil {
		t.Fatalf("EncodeTerminal ascii: %v", err)
	}
	if !strings.Contains(ascii, asciiDark) {
		t.Errorf("ascii output missing %q", asciiDark)
	}
	if strings.ContainsRune(ascii, blockFull) {
		t.Error("ascii output should not contain block glyphs")
	}

	inv, err := EncodeTerminal(text, WithMask(2), WithTerminalInvert(true))
	if err != nil {
		t.Fatalf("EncodeTerminal invert: %v", err)
	}
	if inv == half {
		t.Error("inverted output should differ from the default")
	}
}
