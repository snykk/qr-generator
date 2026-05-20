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
	"math/rand/v2"
	"testing"
)

// helloWorldFullBlock is the V1-M HELLO WORLD codeword stream from
// docs/theory/10-worked-example.md: 16 data + 10 EC = 26 bytes. With n=10
// the decoder can correct up to floor(10/2) = 5 errors per block.
var helloWorldFullBlock = []byte{
	// 16 data codewords
	0x20, 0x5B, 0x0B, 0x78, 0xD1, 0x72, 0xDC, 0x4D,
	0x43, 0x40, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11,
	// 10 EC codewords
	0xC4, 0x23, 0x27, 0x77, 0xEB, 0xD7, 0xE7, 0xE2, 0x5D, 0x17,
}

var helloWorldDataPart = helloWorldFullBlock[:16]

func TestRSDecodeNoErrors(t *testing.T) {
	got, err := rsDecode(helloWorldFullBlock, 10)
	if err != nil {
		t.Fatalf("rsDecode: %v", err)
	}
	if !bytes.Equal(got, helloWorldDataPart) {
		t.Errorf("got % X\nwant % X", got, helloWorldDataPart)
	}
}

// TestRSDecodeSingleByteCorruption flips one byte at every position in turn
// and asserts the decoder recovers the original data.
func TestRSDecodeSingleByteCorruption(t *testing.T) {
	for pos := 0; pos < len(helloWorldFullBlock); pos++ {
		corrupted := append([]byte(nil), helloWorldFullBlock...)
		corrupted[pos] ^= 0xFF
		got, err := rsDecode(corrupted, 10)
		if err != nil {
			t.Errorf("pos=%d: %v", pos, err)
			continue
		}
		if !bytes.Equal(got, helloWorldDataPart) {
			t.Errorf("pos=%d: recovered % X, want % X", pos, got, helloWorldDataPart)
		}
	}
}

// TestRSDecodeMultiByteCorruption corrupts 2..5 bytes (within RS capacity for
// n=10) at varied positions and asserts exact recovery each time.
func TestRSDecodeMultiByteCorruption(t *testing.T) {
	cases := []struct {
		name      string
		positions []int
		xors      []byte
	}{
		{"2 byte adjacent", []int{0, 1}, []byte{0xAA, 0xBB}},
		{"2 byte spread", []int{2, 22}, []byte{0x80, 0x40}},
		{"3 bytes", []int{0, 12, 25}, []byte{0xFF, 0x55, 0xCC}},
		{"4 bytes data+ec mix", []int{1, 8, 17, 24}, []byte{0x11, 0x22, 0x33, 0x44}},
		{"5 bytes (capacity)", []int{0, 5, 10, 15, 20}, []byte{0xAA, 0xAA, 0xAA, 0xAA, 0xAA}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			corrupted := append([]byte(nil), helloWorldFullBlock...)
			for i, p := range c.positions {
				corrupted[p] ^= c.xors[i]
			}
			got, err := rsDecode(corrupted, 10)
			if err != nil {
				t.Fatalf("rsDecode: %v", err)
			}
			if !bytes.Equal(got, helloWorldDataPart) {
				t.Errorf("recovered % X\nwant     % X", got, helloWorldDataPart)
			}
		})
	}
}

// TestRSDecodeExceedsCapacity corrupts 6+ bytes — more than t=5 — and
// asserts the decoder either returns ErrTooManyErrors or, if it does return
// data, the data is NOT the original (i.e. the decoder did not silently
// "succeed" with wrong content). Both outcomes are spec-compliant.
func TestRSDecodeExceedsCapacity(t *testing.T) {
	cases := []struct {
		name      string
		positions []int
	}{
		{"6 byte burst", []int{0, 1, 2, 3, 4, 5}},
		{"7 spread", []int{0, 3, 8, 12, 17, 21, 24}},
		{"all 26 bytes", func() []int {
			out := make([]int, 26)
			for i := range out {
				out[i] = i
			}
			return out
		}()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			corrupted := append([]byte(nil), helloWorldFullBlock...)
			for _, p := range c.positions {
				corrupted[p] ^= 0xA5
			}
			got, err := rsDecode(corrupted, 10)
			if err != nil {
				if !errors.Is(err, ErrTooManyErrors) {
					t.Errorf("expected ErrTooManyErrors or success, got %v", err)
				}
				return
			}
			// If no error, the decoder must NOT have silently returned the
			// original data (that would be a coincidence; we still want to
			// confirm no false-positive recovery beyond capacity).
			if bytes.Equal(got, helloWorldDataPart) {
				t.Errorf("decoder claimed recovery beyond capacity: returned original data despite %d corruptions", len(c.positions))
			}
		})
	}
}

// TestRSDecodeRandomBlocks fuzzes the decoder by Reed-Solomon-encoding random
// data blocks, corrupting up to floor(n/2) random bytes, and asserting exact
// recovery. Catches systemic bugs that the HELLO WORLD fixture wouldn't.
func TestRSDecodeRandomBlocks(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))

	cases := []struct {
		dataLen int
		n       int
	}{
		{16, 10}, // V1-M shape
		{19, 7},  // V1-L shape
		{9, 17},  // V1-H shape
		{43, 24}, // V5-M (one block of group 1)
		{116, 30},
	}

	for _, c := range cases {
		blockLen := c.dataLen + c.n
		t.Run("", func(t *testing.T) {
			for trial := 0; trial < 50; trial++ {
				// Random data.
				data := make([]byte, c.dataLen)
				for i := range data {
					data[i] = byte(r.Uint32() & 0xFF)
				}
				// Encode.
				ec := encodeBlock(data, c.n)
				block := append(append([]byte(nil), data...), ec...)

				// Corrupt up to floor(n/2) bytes at random positions.
				maxErrors := c.n / 2
				errorCount := r.IntN(maxErrors + 1) // 0..maxErrors inclusive
				positions := r.Perm(blockLen)[:errorCount]
				corrupted := append([]byte(nil), block...)
				for _, p := range positions {
					flip := byte(1 + r.Uint32()&0xFE)
					corrupted[p] ^= flip
				}

				got, err := rsDecode(corrupted, c.n)
				if err != nil {
					t.Errorf("trial %d (dataLen=%d, n=%d, errors=%d): %v", trial, c.dataLen, c.n, errorCount, err)
					continue
				}
				if !bytes.Equal(got, data) {
					t.Errorf("trial %d (dataLen=%d, n=%d, errors=%d): recovery mismatch", trial, c.dataLen, c.n, errorCount)
				}
			}
		})
	}
}
