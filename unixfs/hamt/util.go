package hamt

import (
	"math/big"
	"math/bits"
)

// hashBits is a helper that allows the reading of the 'next n bits' as an integer.
type hashBits struct {
	b        []byte
	consumed int
}

func mkmask(n int) byte {
	return (1 << uint(n)) - 1
}

// Next returns the next 'i' bits of the hashBits value as an integer
func (hb *hashBits) Next(i int) int {
	curbi := hb.consumed / 8
	leftb := 8 - (hb.consumed % 8)

	curb := hb.b[curbi]
	if i == leftb {
		out := int(mkmask(i) & curb)
		hb.consumed += i
		return out
	} else if i < leftb {
		a := curb & mkmask(leftb) // mask out the high bits we don't want
		b := a & ^mkmask(leftb-i) // mask out the low bits we don't want
		c := b >> uint(leftb-i)   // shift whats left down
		hb.consumed += i
		return int(c)
	} else {
		out := int(mkmask(leftb) & curb)
		out <<= uint(i - leftb)
		hb.consumed += leftb
		out += hb.Next(i - leftb)
		return out
	}
}

func popCount(i *big.Int) int {
	var n int
	for _, v := range i.Bits() {
		n += bits.OnesCount64(uint64(v))
	}
	return n
}
