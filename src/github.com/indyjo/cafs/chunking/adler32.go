// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package adler32 implements the Adler-32 checksum.
//
// It is defined in RFC 1950:
//	Adler-32 is composed of two sums accumulated per byte: s1 is
//	the sum of all bytes, s2 is the sum of all s1 values. Both sums
//	are done modulo 65521. s1 is initialized to 1, s2 to zero.  The
//	Adler-32 checksum is stored as s2*65536 + s1 in most-
//	significant-byte first (network) order.
package chunking

const (
	// mod is the largest prime that is less than 65536.
	mod = 65521
	// nmax_push is the largest n such that
	// 255 * n * (n+1) / 2 + (n+1) * (mod-1) <= 2^32-1.
	// It is mentioned in RFC 1950 (search for "5552").
	nmax_push = 5552

	// In popFront(), we must multiply with the size parameter,
	// which can be much higher (up to 65520) -- Jonas
	nmax_pop = 256

	WINDOW_SIZE = 48
	MIN_CHUNK   = 128
	MAX_CHUNK   = 131072
)

type adlerChunker struct {
	a      uint32
	n, p   int
	window [WINDOW_SIZE]byte
}

// New returns a new hash.Hash32 computing the Adler-32 checksum.
func New() Chunker {
	var c = adlerChunker{a: 1}
	return &c
}

func (c *adlerChunker) Scan(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	prefixLen := 0
	// Initially, fill window
	if c.n < WINDOW_SIZE {
		prefixLen = WINDOW_SIZE - c.n
		if len(data) < prefixLen {
			prefixLen = len(data)
		}
		c.a = pushBack(c.a, data[:prefixLen])
		c.n += prefixLen
		copy(c.window[c.p:c.p+prefixLen], data[:prefixLen])
		c.p += prefixLen
		if c.p == WINDOW_SIZE {
			c.p = 0
		}
		data = data[prefixLen:]
	}

	for i, _ := range data {
		c.a = popFront(c.a, c.window[c.p:c.p+1], WINDOW_SIZE)
		c.window[c.p] = data[i]
		c.a = pushBack(c.a, data[i:i+1])
		c.n++

		// Chunk boundary at MAX_CHUNK or if hash is 4159 modulo 8191 (both are prime)
		if c.n > MIN_CHUNK && 4159 == (c.a % 8191) || c.n > MAX_CHUNK {
			// Reset chunker and return position in data
			*c = adlerChunker{a: 1}
			return i+prefixLen // Byte will become beginning of next segment
		}

		c.p++
		if c.p == WINDOW_SIZE {
			c.p = 0
		}
	}

	return len(data) + prefixLen
}

// Add p to the running checksum d.
func pushBack(d uint32, p []byte) uint32 {
	s1, s2 := uint32(d&0xffff), uint32(d>>16)
	for len(p) > 0 {
		var q []byte
		if len(p) > nmax_push {
			p, q = p[:nmax_push], p[nmax_push:]
		}
		for _, x := range p {
			s1 += uint32(x)
			s2 += s1
		}
		s1 %= mod
		s2 %= mod
		p = q
	}
	return uint32(s2<<16 | s1)
}

// Remove p from the front of the running checksum d.
// size is the number of elements in the hash before popFront is executed.
func popFront(d uint32, p []byte, size int) uint32 {
	s1, s2 := uint32(d&0xffff), uint32(d>>16)
	if size >= mod {
		size %= mod
	}
	for len(p) > 0 {
		var q []byte
		var run = nmax_pop
		if size < run {
			run = size
		}
		if len(p) > run {
			p, q = p[:run], p[run:]
		}
		s1 += 65550 * mod // Maximum x = 0 (mod 65521) so x+65520 is still uint32
		s2 += 65550 * mod
		for _, x := range p {
			s1 -= uint32(x)
			s2 -= uint32(size)*uint32(x) + 1
			size--
		}
		s1 %= mod
		s2 %= mod
		size %= mod
		p = q
	}
	return uint32(s2<<16 | s1)
}

// Checksum returns the Adler-32 checksum of data.
func Checksum(data []byte) uint32 { return pushBack(1, data) }
