// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sha3

// Tests include all the ShortMsgKATs provided by the Keccak team at
// https://github.com/gvanas/KeccakCodePackage
//
// They only include the zero-bit case of the utterly useless bitwise
// testvectors published by NIST in the draft of FIPS-202.

import (
	"bytes"
	"compress/flate"
	"encoding/hex"
	"encoding/json"
	"hash"
	"os"
	"strings"
	"testing"
)

const (
	testString  = "brekeccakkeccak koax koax"
	katFilename = "keccakKats.json.deflate"
)

// Internal-use instances of SHAKE used to test against KATs.
func newHashShake128() hash.Hash {
	return &state{rate: 168, dsbyte: 0x1f, outputLen: 512}
}
func newHashShake256() hash.Hash {
	return &state{rate: 136, dsbyte: 0x1f, outputLen: 512}
}

// testDigests contains functions returning hash.Hash instances
// with output-length equal to the KAT length for both SHA-3 and
// SHAKE instances.
var testDigests = map[string]func() hash.Hash{
	"SHA3-224": New224,
	"SHA3-256": New256,
	"SHA3-384": New384,
	"SHA3-512": New512,
	"SHAKE128": newHashShake128,
	"SHAKE256": newHashShake256,
}

// testShakes contains functions returning ShakeHash instances for
// testing the ShakeHash-specific interface.
var testShakes = map[string]func() ShakeHash{
	"SHAKE128": NewShake128,
	"SHAKE256": NewShake256,
}

// decodeHex converts an hex-encoded string into a raw byte string.
func decodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// structs used to marshal JSON test-cases.
type KeccakKats struct {
	Kats map[string][]struct {
		Digest  string `json:"digest"`
		Length  int64  `json:"length"`
		Message string `json:"message"`
	}
}

// TestKeccakKats tests the SHA-3 and Shake implementations against all the
// ShortMsgKATs from https://github.com/gvanas/KeccakCodePackage
// (The testvectors are stored in keccakKats.json.deflate due to their length.)
func TestKeccakKats(t *testing.T) {
	// Read the KATs.
	deflated, err := os.Open(katFilename)
	if err != nil {
		t.Errorf("Error opening %s: %s", katFilename, err)
	}
	file := flate.NewReader(deflated)
	dec := json.NewDecoder(file)
	var katSet KeccakKats
	err = dec.Decode(&katSet)
	if err != nil {
		t.Errorf("%s", err)
	}

	// Do the KATs.
	for functionName, kats := range katSet.Kats {
		d := testDigests[functionName]()
		t.Logf("%s", functionName)
		for _, kat := range kats {
			d.Reset()
			in, err := hex.DecodeString(kat.Message)
			if err != nil {
				t.Errorf("%s", err)
			}
			d.Write(in[:kat.Length/8])
			got := strings.ToUpper(hex.EncodeToString(d.Sum(nil)))
			want := kat.Digest
			if got != want {
				t.Errorf("function=%s, length=%d\nmessage:\n  %s\ngot:\n  %s\nwanted:\n %s",
					functionName, kat.Length, kat.Message, got, want)
				t.Logf("wanted %+v", kat)
				t.FailNow()
			}
		}
	}
}

// TestUnalignedWrite tests that writing data in an arbitrary pattern with
// small input buffers.
func TestUnalignedWrite(t *testing.T) {
	buf := sequentialBytes(0x10000)
	for alg, df := range testDigests {
		d := df()
		d.Reset()
		d.Write(buf)
		want := d.Sum(nil)
		d.Reset()
		for i := 0; i < len(buf); {
			// Cycle through offsets which make a 137 byte sequence.
			// Because 137 is prime this sequence should exercise all corner cases.
			offsets := [17]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 1}
			for _, j := range offsets {
				if v := len(buf) - i; v < j {
					j = v
				}
				d.Write(buf[i : i+j])
				i += j
			}
		}
		got := d.Sum(nil)
		if !bytes.Equal(got, want) {
			t.Errorf("Unaligned writes, alg=%s\ngot %q, want %q", alg, got, want)
		}
	}
}

// Test that appending works when reallocation is necessary.
func TestAppend(t *testing.T) {
	d := New224()

	for capacity := 2; capacity < 64; capacity += 64 {
		// The first time around the loop, Sum will have to reallocate.
		// The second time, it will not.
		buf := make([]byte, 2, capacity)
		d.Reset()
		d.Write([]byte{0xcc})
		buf = d.Sum(buf)
		expected := "0000DF70ADC49B2E76EEE3A6931B93FA41841C3AF2CDF5B32A18B5478C39"
		if got := strings.ToUpper(hex.EncodeToString(buf)); got != expected {
			t.Errorf("got %s, want %s", got, expected)
		}
	}
}

// Test that appending works when no reallocation is necessary.
func TestAppendNoRealloc(t *testing.T) {
	buf := make([]byte, 1, 200)
	d := New224()
	d.Write([]byte{0xcc})
	buf = d.Sum(buf)
	expected := "00DF70ADC49B2E76EEE3A6931B93FA41841C3AF2CDF5B32A18B5478C39"
	if got := strings.ToUpper(hex.EncodeToString(buf)); got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

// TestSqueezing checks that squeezing the full output a single time produces
// the same output as repeatedly squeezing the instance.
func TestSqueezing(t *testing.T) {
	for functionName, newShakeHash := range testShakes {
		t.Logf("%s", functionName)
		d0 := newShakeHash()
		d0.Write([]byte(testString))
		ref := make([]byte, 32)
		d0.Read(ref)

		d1 := newShakeHash()
		d1.Write([]byte(testString))
		var multiple []byte
		for _ = range ref {
			one := make([]byte, 1)
			d1.Read(one)
			multiple = append(multiple, one...)
		}
		if !bytes.Equal(ref, multiple) {
			t.Errorf("squeezing %d bytes one at a time failed", len(ref))
		}
	}
}

func TestReadSimulation(t *testing.T) {
	d := NewShake256()
	d.Write(nil)
	dwr := make([]byte, 32)
	d.Read(dwr)

}

// sequentialBytes produces a buffer of size consecutive bytes 0x00, 0x01, ..., used for testing.
func sequentialBytes(size int) []byte {
	result := make([]byte, size)
	for i := range result {
		result[i] = byte(i)
	}
	return result
}

// BenchmarkPermutationFunction measures the speed of the permutation function
// with no input data.
func BenchmarkPermutationFunction(b *testing.B) {
	b.SetBytes(int64(200))
	var lanes [25]uint64
	for i := 0; i < b.N; i++ {
		keccakF1600(&lanes)
	}
}

// benchmarkBulkHash tests the speed to hash a buffer of buflen.
func benchmarkBulkHash(b *testing.B, h hash.Hash, size int) {
	b.StopTimer()
	h.Reset()
	data := sequentialBytes(size)
	b.SetBytes(int64(size))
	b.StartTimer()

	var state []byte
	for i := 0; i < b.N; i++ {
		h.Write(data)
		state = h.Sum(state[:0])
	}
	b.StopTimer()
	h.Reset()
}

func BenchmarkSha3_512_MTU(b *testing.B) { benchmarkBulkHash(b, New512(), 1350) }
func BenchmarkSha3_384_MTU(b *testing.B) { benchmarkBulkHash(b, New384(), 1350) }
func BenchmarkSha3_256_MTU(b *testing.B) { benchmarkBulkHash(b, New256(), 1350) }
func BenchmarkSha3_224_MTU(b *testing.B) { benchmarkBulkHash(b, New224(), 1350) }
func BenchmarkShake256_MTU(b *testing.B) { benchmarkBulkHash(b, newHashShake256(), 1350) }
func BenchmarkShake128_MTU(b *testing.B) { benchmarkBulkHash(b, newHashShake128(), 1350) }

func BenchmarkSha3_512_1MiB(b *testing.B) { benchmarkBulkHash(b, New512(), 1<<20) }
func BenchmarkShake256_1MiB(b *testing.B) { benchmarkBulkHash(b, newHashShake256(), 1<<20) }
