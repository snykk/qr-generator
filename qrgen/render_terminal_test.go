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

// parseTerminalToSymbol inverts renderTerminal: it reads the rendered glyphs
// back into the bare symbol grid (quiet zone stripped, polarity un-inverted) so
// a DecodeMatrix round-trip can confirm the rendering is loss-free. It mirrors
// the mapping in render_terminal.go for the given mode.
func parseTerminalToSymbol(s string, quietZone int, invert, ascii bool) [][]bool {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	var painted [][]bool
	if ascii {
		for _, line := range lines {
			rs := []rune(line)
			row := make([]bool, len(rs)/2)
			for c := range row {
				row[c] = rs[2*c] == '#'
			}
			painted = append(painted, row)
		}
	} else {
		for _, line := range lines {
			var top, bottom []bool
			for _, ru := range line {
				switch ru {
				case blockFull:
					top, bottom = append(top, true), append(bottom, true)
				case blockUpper:
					top, bottom = append(top, true), append(bottom, false)
				case blockLower:
					top, bottom = append(top, false), append(bottom, true)
				default:
					top, bottom = append(top, false), append(bottom, false)
				}
			}
			painted = append(painted, top, bottom)
		}
	}
	dim := len(painted[0])  // padded side in modules
	painted = painted[:dim] // drop the out-of-grid tail half-row if present
	size := dim - 2*quietZone
	out := make([][]bool, size)
	for r := 0; r < size; r++ {
		out[r] = make([]bool, size)
		for c := 0; c < size; c++ {
			// paint = dark XOR invert, so dark = paint XOR invert.
			out[r][c] = painted[r+quietZone][c+quietZone] != invert
		}
	}
	return out
}

// TestTerminalRoundTrip renders a range of payloads in every mode, parses the
// glyphs back into the symbol grid, and confirms DecodeMatrix recovers the
// exact input — proving the rendering is loss-free in half-block, inverted, and
// ASCII forms.
func TestTerminalRoundTrip(t *testing.T) {
	payloads := []string{
		"HELLO WORLD",
		"https://example.com",
		"12345678901234567890",
		"Order #1234567890",
	}
	modes := []struct {
		name          string
		invert, ascii bool
	}{
		{"halfblock", false, false},
		{"invert", true, false},
		{"ascii", false, true},
	}
	for _, text := range payloads {
		for _, mode := range modes {
			s, err := EncodeTerminal(text,
				WithTerminalInvert(mode.invert), WithTerminalASCII(mode.ascii))
			if err != nil {
				t.Fatalf("EncodeTerminal(%q, %s): %v", text, mode.name, err)
			}
			grid := parseTerminalToSymbol(s, defaultQuietZone, mode.invert, mode.ascii)
			got, err := DecodeMatrix(grid)
			if err != nil {
				t.Fatalf("DecodeMatrix(%q, %s): %v", text, mode.name, err)
			}
			if got != text {
				t.Errorf("round-trip %q in %s mode: got %q", text, mode.name, got)
			}
		}
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
