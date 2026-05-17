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
	"testing"
)

func TestBitBufferAppendSingleBit(t *testing.T) {
	b := &bitBuffer{}
	b.appendBits(1, 1)
	if b.bits() != 1 {
		t.Fatalf("bits() = %d, want 1", b.bits())
	}
	if !bytes.Equal(b.bytes(), []byte{0x80}) {
		t.Errorf("bytes() = %X, want 80", b.bytes())
	}
}

func TestBitBufferAppendAcrossBoundary(t *testing.T) {
	b := &bitBuffer{}
	// 4 bits of 0b0010 (mode indicator for alphanumeric)
	b.appendBits(0b0010, 4)
	// 9 bits of count = 11 = 0b000001011
	b.appendBits(11, 9)
	// Should now have 13 bits total.
	if b.bits() != 13 {
		t.Fatalf("bits() = %d, want 13", b.bits())
	}
	// Layout: 0010 0000 0101 1xxx => 0x20 0x58
	want := []byte{0x20, 0x58}
	if !bytes.Equal(b.bytes(), want) {
		t.Errorf("bytes() = %X, want %X", b.bytes(), want)
	}
}

func TestBitBufferAppendFullByte(t *testing.T) {
	b := &bitBuffer{}
	b.appendBits(0xEC, 8)
	b.appendBits(0x11, 8)
	if b.bits() != 16 {
		t.Fatalf("bits() = %d, want 16", b.bits())
	}
	if !bytes.Equal(b.bytes(), []byte{0xEC, 0x11}) {
		t.Errorf("bytes() = %X, want EC 11", b.bytes())
	}
}

func TestBitBufferAppendZeroBitsNoOp(t *testing.T) {
	b := &bitBuffer{}
	b.appendBits(0xFFFF, 4)
	before := b.bits()
	b.appendBits(0, 0)
	if b.bits() != before {
		t.Errorf("appendBits(_, 0) changed bits from %d to %d", before, b.bits())
	}
}

func TestBitBufferAppendOversizedPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("appendBits with n=33 did not panic")
		}
	}()
	b := &bitBuffer{}
	b.appendBits(0, 33)
}
