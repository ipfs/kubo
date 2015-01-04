// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sha3 implements the SHA-3 fixed-output-length hash functions and
// the SHAKE variable-output-length hash functions defined by FIPS-202.
//
// Both types of hash function use the "sponge" construction and the Keccak
// permutation. For a detailed specification see http://keccak.noekeon.org/
//
//
// Guidance
//
// If you aren't sure what function you need, use SHAKE256 with at least 64
// bytes of output.
//
// If you need a secret-key MAC (message authentication code), prepend the
// secret key to the input, hash with SHAKE256 and read at least 32 bytes of
// output.
//
//
// Security strengths
//
// The SHA3-x functions have a security strength against preimage attacks of x
// bits. Since they only produce x bits of output, their collision-resistance
// is only x/2 bits.
//
// The SHAKE-x functions have a generic security strength of x bits against
// all attacks, provided that at least 2x bits of their output is used.
// Requesting more than 2x bits of output does not increase the collision-
// resistance of the SHAKE functions.
//
//
// The sponge construction
//
// A sponge builds a pseudo-random function from a pseudo-random permutation,
// by applying the permutation to a state of "rate + capacity" bytes, but
// hiding "capacity" of the bytes.
//
// A sponge starts out with a zero state. To hash an input using a sponge, up
// to "rate" bytes of the input are XORed into the sponge's state. The sponge
// has thus been "filled up" and the permutation is applied. This process is
// repeated until all the input has been "absorbed". The input is then padded.
// The digest is "squeezed" from the sponge by the same method, except that
// output is copied out.
//
// A sponge is parameterized by its generic security strength, which is equal
// to half its capacity; capacity + rate is equal to the permutation's width.
//
// Since the KeccakF-1600 permutation is 1600 bits (200 bytes) wide, this means
// that security_strength == (1600 - bitrate) / 2.
//
//
// Recommendations, detailed
//
// The SHAKE functions are recommended for most new uses. They can produce
// output of arbitrary length. SHAKE256, with an output length of at least
// 64 bytes, provides 256-bit security against all attacks.
//
// The Keccak team recommends SHAKE256 for most applications upgrading from
// SHA2-512. (NIST chose a much stronger, but much slower, sponge instance
// for SHA3-512.)
//
// The SHA-3 functions are "drop-in" replacements for the SHA-2 functions.
// They produce output of the same length, with the same security strengths
// against all attacks. This means, in particular, that SHA3-256 only has
// 128-bit collision resistance, because its output length is 32 bytes.
package sha3
