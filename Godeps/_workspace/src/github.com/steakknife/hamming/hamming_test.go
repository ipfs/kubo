package hamming

import (
  "testing"
)

type testCountBitsUint64Case struct {
  x uint64
  n int
}

type testCountBitsByteCase struct {
  x byte
  n int
}

var testCountBitsByteCases = []testCountBitsByteCase{
  {0x00, 0},
  {0x01, 1},
  {0x02, 1},
  {0x03, 2},
  {0xaa, 4},
  {0x55, 4},
  {0x7f, 7},
  {0xff, 8},
}

var testCountBitsUint64Cases = []testCountBitsUint64Case{
  {0x00, 0},
  {0x01, 1},
  {0x02, 1},
  {0x03, 2},
  {0xaa, 4},
  {0x55, 4},
  {0x7f, 7},
  {0xff, 8},
  {0xffff, 16},
  {0xffffffff, 32},
  {0x1ffffffff, 33},
  {0x3ffffffff, 34},
  {0x7ffffffff, 35},
  {0xfffffffff, 36},
  {0x3fffffffffffffff, 62},
  {0x7fffffffffffffff, 63},
  {0xffffffffffffffff, 64},
}

func TestCountBitByte(t *testing.T) {
  for _, c := range testCountBitsByteCases {
    if actualN := CountBitsByte(c.x); actualN != c.n {
      t.Fatal("CountBitsByte(", c.x, ") = ", actualN, "  != ", c.n)
    } else {
      t.Log("CountBitsByte(", c.x, ") == ", c.n)
    }
  }
}

func TestCountBitUint64(t *testing.T) {
  for _, c := range testCountBitsUint64Cases {
    if actualN := CountBitsUint64(c.x); actualN != c.n {
      t.Fatal("CountBitsUint64(", c.x, ") = ", actualN, "  != ", c.n)
    } else {
      t.Log("CountBitsUint64(", c.x, ") == ", c.n)
    }
  }
}

func BenchmarkCountBitsUint64(b *testing.B) {
  j := 0
  for i := 0; i < b.N; i++ {
    CountBitsUint64(testCountBitsUint64Cases[j].x)
    j++
    if j == len(testCountBitsUint64Cases) {
      j = 0
    }
  }
}

func BenchmarkCountBitsByte(b *testing.B) {
  j := 0
  for i := 0; i < b.N; i++ {
    CountBitsByte(testCountBitsByteCases[j].x)
    j++
    if j == len(testCountBitsByteCases) {
      j = 0
    }
  }
}
