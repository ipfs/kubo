package bigfft

import (
	"fmt"
	. "math/big"
	"math/rand"
	"testing"
)

func parseHex(s string) fermat {
	z := new(Int)
	z, ok := z.SetString(s, 0)
	if !ok {
		panic(s)
	}
	return append(fermat(z.Bits()), 0)
}

func compare(t *testing.T, a, b fermat) error {
	var x, y Int
	x.SetBits(a)
	y.SetBits(b)
	if x.Cmp(&y) != 0 {
		return fmt.Errorf("%x != %x (%x)", &x, &y, new(Int).Xor(&x, &y))
	}
	return nil
}

func TestFermatShift(t *testing.T) {
	const n = 4
	f := make(fermat, n+1)
	for i := 0; i < n; i++ {
		f[i] = Word(rand.Int63())
	}
	b := NewInt(1)
	b = b.Lsh(b, uint(n*_W))
	b = b.Add(b, NewInt(1))
	z := make(fermat, len(f)) // Test with uninitialized z.
	for shift := -2048; shift < 2048; shift++ {
		z.Shift(f, shift)

		z2 := new(Int)
		z2.SetBits(f)
		if shift < 0 {
			s2 := (-shift) % (2 * n * _W)
			z2 = z2.Lsh(z2, uint(2*n*_W-s2))
		} else {
			z2 = z2.Lsh(z2, uint(shift))
		}
		z2 = z2.Mod(z2, b)
		if err := compare(t, z, z2.Bits()); err != nil {
			t.Errorf("error in shift by %d: %s", shift, err)
		}
	}
}

type test struct{ a, b, c fermat }

// addTests is a series of mod 2^256+1 tests.
var addTests = []test{
	{
		parseHex("0x5555555555555555555555555555555555555555555555555555555555555555"),
		parseHex("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab"),
		parseHex("0x10000000000000000000000000000000000000000000000000000000000000000"),
	},
	{
		parseHex("0x5555555555555555555555555555555555555555555555555555555555555555"),
		parseHex("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		parseHex("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
	},
	{
		parseHex("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		parseHex("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		parseHex("0x5555555555555555555555555555555555555555555555555555555555555553"),
	},
}

func TestFermatAdd(t *testing.T) {
	for _, item := range addTests {
		z := make(fermat, len(item.a))
		z = z.Add(item.a, item.b)
		compare(t, z, item.c)
	}
}

var mulTests = []test{
	{ // 3^400 = 3^200 * 3^200
		parseHex("0xc21a937a76f3432ffd73d97e447606b683ecf6f6e4a7ae223c2578e26c486a03"),
		parseHex("0xc21a937a76f3432ffd73d97e447606b683ecf6f6e4a7ae223c2578e26c486a03"),
		parseHex("0x0e65f4d3508036eaca8faa2b8194ace009c863e44bdc040c459a7127bf8bcc62"),
	},
}

func TestFermatMul(t *testing.T) {
	for _, item := range mulTests {
		z := make(fermat, 3*len(item.a))
		z = z.Mul(item.a, item.b)
		compare(t, z, item.c)
	}
}
