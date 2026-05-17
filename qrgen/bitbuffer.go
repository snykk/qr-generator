// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import "fmt"

// bitBuffer accumulates a stream of bits in MSB-first order, packed into bytes.
// Bits are written most-significant-bit-first within each byte: the first bit
// written goes to byte 0, bit 7.
type bitBuffer struct {
	buf  []byte // backing bytes; len(buf) == ceil(n / 8)
	nbit int    // number of bits currently stored
}

// appendBits writes the low n bits of value (MSB of those n first) to the
// buffer. Panics if n is outside [0, 32].
func (b *bitBuffer) appendBits(value uint32, n int) {
	if n < 0 || n > 32 {
		panic(fmt.Sprintf("qrgen: bitBuffer.appendBits: invalid n=%d", n))
	}
	for i := n - 1; i >= 0; i-- {
		if (b.nbit & 7) == 0 {
			b.buf = append(b.buf, 0)
		}
		bit := byte((value >> uint(i)) & 1)
		b.buf[b.nbit>>3] |= bit << uint(7-(b.nbit&7))
		b.nbit++
	}
}

// bits returns the number of bits currently stored.
func (b *bitBuffer) bits() int { return b.nbit }

// bytes returns the underlying byte slice. The result is only meaningful when
// the bit count is a multiple of 8; otherwise the final byte's trailing bits
// are zero-padded but still part of the slice. Callers that care about byte
// boundaries should pad to one with appendBits before reading.
func (b *bitBuffer) bytes() []byte { return b.buf }
