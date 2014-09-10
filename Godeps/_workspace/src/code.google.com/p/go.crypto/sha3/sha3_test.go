// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sha3

// These tests are a subset of those provided by the Keccak web site(http://keccak.noekeon.org/).

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
	"testing"
)

// testDigests maintains a digest state of each standard type.
var testDigests = map[string]*digest{
	"Keccak224": {outputSize: 224 / 8, capacity: 2 * 224 / 8},
	"Keccak256": {outputSize: 256 / 8, capacity: 2 * 256 / 8},
	"Keccak384": {outputSize: 384 / 8, capacity: 2 * 384 / 8},
	"Keccak512": {outputSize: 512 / 8, capacity: 2 * 512 / 8},
}

// testVector represents a test input and expected outputs from multiple algorithm variants.
type testVector struct {
	desc   string
	input  []byte
	repeat int // input will be concatenated the input this many times.
	want   map[string]string
}

// decodeHex converts an hex-encoded string into a raw byte string.
func decodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// shortTestVectors stores a series of short testVectors.
// Inputs of 8, 248, and 264 bits from http://keccak.noekeon.org/ are included below.
// The standard defines additional test inputs of all sizes between 0 and 2047 bits.
// Because the current implementation can only handle an integral number of bytes,
// most of the standard test inputs can't be used.
var shortKeccakTestVectors = []testVector{
	{
		desc:   "short-8b",
		input:  decodeHex("CC"),
		repeat: 1,
		want: map[string]string{
			"Keccak224": "A9CAB59EB40A10B246290F2D6086E32E3689FAF1D26B470C899F2802",
			"Keccak256": "EEAD6DBFC7340A56CAEDC044696A168870549A6A7F6F56961E84A54BD9970B8A",
			"Keccak384": "1B84E62A46E5A201861754AF5DC95C4A1A69CAF4A796AE405680161E29572641F5FA1E8641D7958336EE7B11C58F73E9",
			"Keccak512": "8630C13CBD066EA74BBE7FE468FEC1DEE10EDC1254FB4C1B7C5FD69B646E44160B8CE01D05A0908CA790DFB080F4B513BC3B6225ECE7A810371441A5AC666EB9",
		},
	},
	{
		desc:   "short-248b",
		input:  decodeHex("84FB51B517DF6C5ACCB5D022F8F28DA09B10232D42320FFC32DBECC3835B29"),
		repeat: 1,
		want: map[string]string{
			"Keccak224": "81AF3A7A5BD4C1F948D6AF4B96F93C3B0CF9C0E7A6DA6FCD71EEC7F6",
			"Keccak256": "D477FB02CAAA95B3280EC8EE882C29D9E8A654B21EF178E0F97571BF9D4D3C1C",
			"Keccak384": "503DCAA4ADDA5A9420B2E436DD62D9AB2E0254295C2982EF67FCE40F117A2400AB492F7BD5D133C6EC2232268BC27B42",
			"Keccak512": "9D8098D8D6EDBBAA2BCFC6FB2F89C3EAC67FEC25CDFE75AA7BD570A648E8C8945FF2EC280F6DCF73386109155C5BBC444C707BB42EAB873F5F7476657B1BC1A8",
		},
	},
	{
		desc:   "short-264b",
		input:  decodeHex("DE8F1B3FAA4B7040ED4563C3B8E598253178E87E4D0DF75E4FF2F2DEDD5A0BE046"),
		repeat: 1,
		want: map[string]string{
			"Keccak224": "F217812E362EC64D4DC5EACFABC165184BFA456E5C32C2C7900253D0",
			"Keccak256": "E78C421E6213AFF8DE1F025759A4F2C943DB62BBDE359C8737E19B3776ED2DD2",
			"Keccak384": "CF38764973F1EC1C34B5433AE75A3AAD1AAEF6AB197850C56C8617BCD6A882F6666883AC17B2DCCDBAA647075D0972B5",
			"Keccak512": "9A7688E31AAF40C15575FC58C6B39267AAD3722E696E518A9945CF7F7C0FEA84CB3CB2E9F0384A6B5DC671ADE7FB4D2B27011173F3EEEAF17CB451CF26542031",
		},
	},
}

// longTestVectors stores longer testVectors (currently only one).
// The computed test vector is 64 MiB long and is a truncated version of the
// ExtremelyLongMsgKAT taken from http://keccak.noekeon.org/.
var longKeccakTestVectors = []testVector{
	{
		desc:   "long-64MiB",
		input:  []byte("abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmno"),
		repeat: 1024 * 1024,
		want: map[string]string{
			"Keccak224": "50E35E40980FEEFF1EA490957B0E970257F75EA0D410EE0F0B8A7A58",
			"Keccak256": "5015A4935F0B51E091C6550A94DCD262C08998232CCAA22E7F0756DEAC0DC0D0",
			"Keccak384": "7907A8D0FAA7BC6A90FE14C6C958C956A0877E751455D8F13ACDB96F144B5896E716C06EC0CB56557A94EF5C3355F6F3",
			"Keccak512": "3EC327D6759F769DEB74E80CA70C831BC29CAB048A4BF4190E4A1DD5C6507CF2B4B58937FDE81D36014E7DFE1B1DD8B0F27CB7614F9A645FEC114F1DAAEFC056",
		},
	},
}

// TestKeccakVectors checks that correct output is produced for a set of known testVectors.
func TestKeccakVectors(t *testing.T) {
	testCases := append([]testVector{}, shortKeccakTestVectors...)
	if !testing.Short() {
		testCases = append(testCases, longKeccakTestVectors...)
	}
	for _, tc := range testCases {
		for alg, want := range tc.want {
			d := testDigests[alg]
			d.Reset()
			for i := 0; i < tc.repeat; i++ {
				d.Write(tc.input)
			}
			got := strings.ToUpper(hex.EncodeToString(d.Sum(nil)))
			if got != want {
				t.Errorf("%s, alg=%s\ngot %q, want %q", tc.desc, alg, got, want)
			}
		}
	}
}

// dumpState is a debugging function to pretty-print the internal state of the hash.
func (d *digest) dumpState() {
	fmt.Printf("SHA3 hash, %d B output, %d B capacity (%d B rate)\n", d.outputSize, d.capacity, d.rate())
	fmt.Printf("Internal state after absorbing %d B:\n", d.absorbed)

	for x := 0; x < sliceSize; x++ {
		for y := 0; y < sliceSize; y++ {
			fmt.Printf("%v, ", d.a[x*sliceSize+y])
		}
		fmt.Println("")
	}
}

// TestUnalignedWrite tests that writing data in an arbitrary pattern with small input buffers.
func TestUnalignedWrite(t *testing.T) {
	buf := sequentialBytes(0x10000)
	for alg, d := range testDigests {
		d.Reset()
		d.Write(buf)
		want := d.Sum(nil)
		d.Reset()
		for i := 0; i < len(buf); {
			// Cycle through offsets which make a 137 byte sequence.
			// Because 137 is prime this sequence should exercise all corner cases.
			offsets := [17]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 1}
			for _, j := range offsets {
				j = minInt(j, len(buf)-i)
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

func TestAppend(t *testing.T) {
	d := NewKeccak224()

	for capacity := 2; capacity < 64; capacity += 64 {
		// The first time around the loop, Sum will have to reallocate.
		// The second time, it will not.
		buf := make([]byte, 2, capacity)
		d.Reset()
		d.Write([]byte{0xcc})
		buf = d.Sum(buf)
		expected := "0000A9CAB59EB40A10B246290F2D6086E32E3689FAF1D26B470C899F2802"
		if got := strings.ToUpper(hex.EncodeToString(buf)); got != expected {
			t.Errorf("got %s, want %s", got, expected)
		}
	}
}

func TestAppendNoRealloc(t *testing.T) {
	buf := make([]byte, 1, 200)
	d := NewKeccak224()
	d.Write([]byte{0xcc})
	buf = d.Sum(buf)
	expected := "00A9CAB59EB40A10B246290F2D6086E32E3689FAF1D26B470C899F2802"
	if got := strings.ToUpper(hex.EncodeToString(buf)); got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

// sequentialBytes produces a buffer of size consecutive bytes 0x00, 0x01, ..., used for testing.
func sequentialBytes(size int) []byte {
	result := make([]byte, size)
	for i := range result {
		result[i] = byte(i)
	}
	return result
}

// benchmarkBlockWrite tests the speed of writing data and never calling the permutation function.
func benchmarkBlockWrite(b *testing.B, d *digest) {
	b.StopTimer()
	d.Reset()
	// Write all but the last byte of a block, to ensure that the permutation is not called.
	data := sequentialBytes(d.rate() - 1)
	b.SetBytes(int64(len(data)))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		d.absorbed = 0 // Reset absorbed to avoid ever calling the permutation function
		d.Write(data)
	}
	b.StopTimer()
	d.Reset()
}

// BenchmarkPermutationFunction measures the speed of the permutation function with no input data.
func BenchmarkPermutationFunction(b *testing.B) {
	b.SetBytes(int64(stateSize))
	var lanes [numLanes]uint64
	for i := 0; i < b.N; i++ {
		keccakF(&lanes)
	}
}

// BenchmarkSingleByteWrite tests the latency from writing a single byte
func BenchmarkSingleByteWrite(b *testing.B) {
	b.StopTimer()
	d := testDigests["Keccak512"]
	d.Reset()
	data := sequentialBytes(1) //1 byte buffer
	b.SetBytes(int64(d.rate()) - 1)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		d.absorbed = 0 // Reset absorbed to avoid ever calling the permutation function

		// Write all but the last byte of a block, one byte at a time.
		for j := 0; j < d.rate()-1; j++ {
			d.Write(data)
		}
	}
	b.StopTimer()
	d.Reset()
}

// BenchmarkSingleByteX measures the block write speed for each size of the digest.
func BenchmarkBlockWrite512(b *testing.B) { benchmarkBlockWrite(b, testDigests["Keccak512"]) }
func BenchmarkBlockWrite384(b *testing.B) { benchmarkBlockWrite(b, testDigests["Keccak384"]) }
func BenchmarkBlockWrite256(b *testing.B) { benchmarkBlockWrite(b, testDigests["Keccak256"]) }
func BenchmarkBlockWrite224(b *testing.B) { benchmarkBlockWrite(b, testDigests["Keccak224"]) }

// benchmarkBulkHash tests the speed to hash a 16 KiB buffer.
func benchmarkBulkHash(b *testing.B, h hash.Hash) {
	b.StopTimer()
	h.Reset()
	size := 1 << 14
	data := sequentialBytes(size)
	b.SetBytes(int64(size))
	b.StartTimer()

	var digest []byte
	for i := 0; i < b.N; i++ {
		h.Write(data)
		digest = h.Sum(digest[:0])
	}
	b.StopTimer()
	h.Reset()
}

// benchmarkBulkKeccakX test the speed to hash a 16 KiB buffer by calling benchmarkBulkHash.
func BenchmarkBulkKeccak512(b *testing.B) { benchmarkBulkHash(b, NewKeccak512()) }
func BenchmarkBulkKeccak384(b *testing.B) { benchmarkBulkHash(b, NewKeccak384()) }
func BenchmarkBulkKeccak256(b *testing.B) { benchmarkBulkHash(b, NewKeccak256()) }
func BenchmarkBulkKeccak224(b *testing.B) { benchmarkBulkHash(b, NewKeccak224()) }
