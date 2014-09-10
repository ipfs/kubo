// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sha3

// This file implements the core Keccak permutation function necessary for computing SHA3.
// This is implemented in a separate file to allow for replacement by an optimized implementation.
// Nothing in this package is exported.
// For the detailed specification, refer to the Keccak web site (http://keccak.noekeon.org/).

// rc stores the round constants for use in the ι step.
var rc = [...]uint64{
	0x0000000000000001,
	0x0000000000008082,
	0x800000000000808A,
	0x8000000080008000,
	0x000000000000808B,
	0x0000000080000001,
	0x8000000080008081,
	0x8000000000008009,
	0x000000000000008A,
	0x0000000000000088,
	0x0000000080008009,
	0x000000008000000A,
	0x000000008000808B,
	0x800000000000008B,
	0x8000000000008089,
	0x8000000000008003,
	0x8000000000008002,
	0x8000000000000080,
	0x000000000000800A,
	0x800000008000000A,
	0x8000000080008081,
	0x8000000000008080,
	0x0000000080000001,
	0x8000000080008008,
}

// keccakF computes the complete Keccak-f function consisting of 24 rounds with a different
// constant (rc) in each round. This implementation fully unrolls the round function to avoid
// inner loops, as well as pre-calculating shift offsets.
func keccakF(a *[numLanes]uint64) {
	var t, bc0, bc1, bc2, bc3, bc4 uint64
	for _, roundConstant := range rc {
		// θ step
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		t = bc4 ^ (bc1<<1 ^ bc1>>63)
		a[0] ^= t
		a[5] ^= t
		a[10] ^= t
		a[15] ^= t
		a[20] ^= t
		t = bc0 ^ (bc2<<1 ^ bc2>>63)
		a[1] ^= t
		a[6] ^= t
		a[11] ^= t
		a[16] ^= t
		a[21] ^= t
		t = bc1 ^ (bc3<<1 ^ bc3>>63)
		a[2] ^= t
		a[7] ^= t
		a[12] ^= t
		a[17] ^= t
		a[22] ^= t
		t = bc2 ^ (bc4<<1 ^ bc4>>63)
		a[3] ^= t
		a[8] ^= t
		a[13] ^= t
		a[18] ^= t
		a[23] ^= t
		t = bc3 ^ (bc0<<1 ^ bc0>>63)
		a[4] ^= t
		a[9] ^= t
		a[14] ^= t
		a[19] ^= t
		a[24] ^= t

		// ρ and π steps
		t = a[1]
		t, a[10] = a[10], t<<1^t>>(64-1)
		t, a[7] = a[7], t<<3^t>>(64-3)
		t, a[11] = a[11], t<<6^t>>(64-6)
		t, a[17] = a[17], t<<10^t>>(64-10)
		t, a[18] = a[18], t<<15^t>>(64-15)
		t, a[3] = a[3], t<<21^t>>(64-21)
		t, a[5] = a[5], t<<28^t>>(64-28)
		t, a[16] = a[16], t<<36^t>>(64-36)
		t, a[8] = a[8], t<<45^t>>(64-45)
		t, a[21] = a[21], t<<55^t>>(64-55)
		t, a[24] = a[24], t<<2^t>>(64-2)
		t, a[4] = a[4], t<<14^t>>(64-14)
		t, a[15] = a[15], t<<27^t>>(64-27)
		t, a[23] = a[23], t<<41^t>>(64-41)
		t, a[19] = a[19], t<<56^t>>(64-56)
		t, a[13] = a[13], t<<8^t>>(64-8)
		t, a[12] = a[12], t<<25^t>>(64-25)
		t, a[2] = a[2], t<<43^t>>(64-43)
		t, a[20] = a[20], t<<62^t>>(64-62)
		t, a[14] = a[14], t<<18^t>>(64-18)
		t, a[22] = a[22], t<<39^t>>(64-39)
		t, a[9] = a[9], t<<61^t>>(64-61)
		t, a[6] = a[6], t<<20^t>>(64-20)
		a[1] = t<<44 ^ t>>(64-44)

		// χ step
		bc0 = a[0]
		bc1 = a[1]
		bc2 = a[2]
		bc3 = a[3]
		bc4 = a[4]
		a[0] ^= bc2 &^ bc1
		a[1] ^= bc3 &^ bc2
		a[2] ^= bc4 &^ bc3
		a[3] ^= bc0 &^ bc4
		a[4] ^= bc1 &^ bc0
		bc0 = a[5]
		bc1 = a[6]
		bc2 = a[7]
		bc3 = a[8]
		bc4 = a[9]
		a[5] ^= bc2 &^ bc1
		a[6] ^= bc3 &^ bc2
		a[7] ^= bc4 &^ bc3
		a[8] ^= bc0 &^ bc4
		a[9] ^= bc1 &^ bc0
		bc0 = a[10]
		bc1 = a[11]
		bc2 = a[12]
		bc3 = a[13]
		bc4 = a[14]
		a[10] ^= bc2 &^ bc1
		a[11] ^= bc3 &^ bc2
		a[12] ^= bc4 &^ bc3
		a[13] ^= bc0 &^ bc4
		a[14] ^= bc1 &^ bc0
		bc0 = a[15]
		bc1 = a[16]
		bc2 = a[17]
		bc3 = a[18]
		bc4 = a[19]
		a[15] ^= bc2 &^ bc1
		a[16] ^= bc3 &^ bc2
		a[17] ^= bc4 &^ bc3
		a[18] ^= bc0 &^ bc4
		a[19] ^= bc1 &^ bc0
		bc0 = a[20]
		bc1 = a[21]
		bc2 = a[22]
		bc3 = a[23]
		bc4 = a[24]
		a[20] ^= bc2 &^ bc1
		a[21] ^= bc3 &^ bc2
		a[22] ^= bc4 &^ bc3
		a[23] ^= bc0 &^ bc4
		a[24] ^= bc1 &^ bc0

		// ι step
		a[0] ^= roundConstant
	}
}
