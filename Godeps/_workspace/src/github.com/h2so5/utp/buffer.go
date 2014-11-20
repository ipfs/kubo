package utp

import (
	"errors"
	"math"
	"time"
)

type packetBuffer struct {
	root  *packetBufferNode
	size  int
	begin int
}

type packetBufferNode struct {
	p      *packet
	next   *packetBufferNode
	pushed time.Time
}

func newPacketBuffer(size, begin int) *packetBuffer {
	return &packetBuffer{
		size:  size,
		begin: begin,
	}
}

func (b *packetBuffer) push(p *packet) error {
	if int(p.header.seq) > b.begin+b.size-1 {
		return errors.New("out of bounds")
	} else if int(p.header.seq) < b.begin {
		if int(p.header.seq)+math.MaxUint16 > b.begin+b.size-1 {
			return errors.New("out of bounds")
		}
	}
	if b.root == nil {
		b.root = &packetBufferNode{}
	}
	n := b.root
	i := b.begin
	for {
		if i == int(p.header.seq) {
			n.p = p
			n.pushed = time.Now()
			return nil
		} else if n.next == nil {
			n.next = &packetBufferNode{}
		}
		n = n.next
		i = (i + 1) % (math.MaxUint16 + 1)
	}
	return nil
}

func (b *packetBuffer) fetch(id uint16) *packet {
	for p := b.root; p != nil; p = p.next {
		if p.p != nil {
			if p.p.header.seq < id {
				p.p = nil
			} else if p.p.header.seq == id {
				r := p.p
				p.p = nil
				return r
			}
		}
	}
	return nil
}

func (b *packetBuffer) compact() {
	for b.root != nil && b.root.p == nil {
		b.root = b.root.next
		b.begin = (b.begin + 1) % (math.MaxUint16 + 1)
	}
}

func (b *packetBuffer) first() *packet {
	if b.root == nil || b.root.p == nil {
		return nil
	}
	return b.root.p
}

func (b *packetBuffer) frontPushedTime() (time.Time, error) {
	if b.root == nil || b.root.p == nil {
		return time.Time{}, errors.New("no first packet")
	}
	return b.root.pushed, nil
}

func (b *packetBuffer) fetchSequence() []*packet {
	var a []*packet
	for ; b.root != nil && b.root.p != nil; b.root = b.root.next {
		a = append(a, b.root.p)
		b.begin = (b.begin + 1) % (math.MaxUint16 + 1)
	}
	return a
}

func (b *packetBuffer) sequence() []*packet {
	var a []*packet
	n := b.root
	for ; n != nil && n.p != nil; n = n.next {
		a = append(a, n.p)
	}
	return a
}

func (b *packetBuffer) space() int {
	s := b.size
	for p := b.root; p != nil; p = p.next {
		s--
	}
	return s
}

func (b *packetBuffer) empty() bool {
	return b.root == nil
}

// test use only
func (b *packetBuffer) all() []*packet {
	var a []*packet
	for p := b.root; p != nil; p = p.next {
		if p.p != nil {
			a = append(a, p.p)
		}
	}
	return a
}

func (b *packetBuffer) generateSelectiveACK() []byte {
	if b.empty() {
		return nil
	}

	var ack []byte
	var bit uint
	var octet byte
	for p := b.root.next; p != nil; p = p.next {
		if p.p != nil {
			octet |= (1 << bit)
		}
		bit++
		if bit == 8 {
			ack = append(ack, octet)
			bit = 0
			octet = 0
		}
	}

	if bit != 0 {
		ack = append(ack, octet)
	}

	for len(ack) > 0 && ack[len(ack)-1] == 0 {
		ack = ack[:len(ack)-1]
	}

	return ack
}

func (b *packetBuffer) processSelectiveACK(ack []byte) {
	if b.empty() {
		return
	}

	p := b.root.next
	if p == nil {
		return
	}

	for _, a := range ack {
		for i := 0; i < 8; i++ {
			acked := (a & 1) != 0
			a >>= 1
			if acked {
				p.p = nil
			}
			p = p.next
			if p == nil {
				return
			}
		}
	}
}

type timedBuffer struct {
	d    time.Duration
	root *timedBufferNode
}

type timedBufferNode struct {
	val    float64
	next   *timedBufferNode
	pushed time.Time
}

func (b *timedBuffer) push(val float64) {
	var before *timedBufferNode
	for n := b.root; n != nil; n = n.next {
		if time.Now().Sub(n.pushed) >= b.d {
			if before != nil {
				before.next = nil
			} else {
				b.root = nil
			}
			break
		}
		before = n
	}
	b.root = &timedBufferNode{
		val:    val,
		next:   b.root,
		pushed: time.Now(),
	}
}

func (b *timedBuffer) min() float64 {
	if b.root == nil {
		return 0
	}
	min := b.root.val
	for n := b.root; n != nil; n = n.next {
		if min > n.val {
			min = n.val
		}
	}
	return min
}
