package utp

import (
	"errors"
	"io"
	"math"
	"sync"
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

func (b *packetBuffer) front() *packet {
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

	if len(ack) == 0 {
		return nil
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

type packetRingBuffer struct {
	b     []*packet
	begin int
	s     int
	mutex sync.RWMutex
	rch   chan int
}

func newPacketRingBuffer(s int) *packetRingBuffer {
	return &packetRingBuffer{
		b:   make([]*packet, s),
		rch: make(chan int),
	}
}

func (b *packetRingBuffer) size() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.s
}

func (b *packetRingBuffer) empty() bool {
	return b.size() == 0
}

func (b *packetRingBuffer) push(p *packet) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.b[(b.begin+b.s)%len(b.b)] = p
	if b.s < len(b.b) {
		b.s++
	} else {
		b.begin = (b.begin + 1) % len(b.b)
	}
	select {
	case b.rch <- 0:
	default:
	}
}

func (b *packetRingBuffer) pop() *packet {
	if b.empty() {
		return nil
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	p := b.b[b.begin]
	b.begin = (b.begin + 1) % len(b.b)
	b.s--
	return p
}

func (b *packetRingBuffer) popOne(timeout time.Duration) (*packet, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	if timeout == 0 {
		t.Stop()
	}
	if b.empty() {
		select {
		case <-b.rch:
		case <-t.C:
			return nil, errTimeout
		}
	}
	return b.pop(), nil
}

type byteRingBuffer struct {
	b            []byte
	begin        int
	s            int
	mutex        sync.RWMutex
	rch          chan int
	closech      chan int
	closechMutex sync.Mutex
}

func newByteRingBuffer(s int) *byteRingBuffer {
	return &byteRingBuffer{
		b:       make([]byte, s),
		rch:     make(chan int),
		closech: make(chan int),
	}
}

func (r *byteRingBuffer) size() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.s
}

func (r *byteRingBuffer) space() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.b) - r.s
}

func (r *byteRingBuffer) empty() bool {
	return r.size() == 0
}

func (r *byteRingBuffer) Write(b []byte) (int, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for len(b) > 0 {
		end := (r.begin + r.s) % len(r.b)
		n := copy(r.b[end:], b)
		b = b[n:]

		s := r.s + n
		if s > len(r.b) {
			r.begin = (r.begin + s - len(r.b)) % len(r.b)
			r.s = len(r.b)
		} else {
			r.s += n
		}
	}
	select {
	case r.rch <- 0:
	case <-r.closech:
		return 0, io.EOF
	default:
	}
	return len(b), nil
}

func (r *byteRingBuffer) ReadTimeout(b []byte, timeout time.Duration) (int, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	if timeout == 0 {
		t.Stop()
	}
	if r.empty() {
		select {
		case <-r.rch:
		case <-t.C:
			return 0, errTimeout
		case <-r.closech:
			return 0, io.EOF
		}
	}
	l := r.size()
	if l > len(b) {
		l = len(b)
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.begin+l > len(r.b) {
		n := copy(b, r.b[r.begin:])
		n = copy(b[n:], r.b[:])
		r.begin = n
	} else {
		copy(b, r.b[r.begin:r.begin+l])
		r.begin = (r.begin + l) % len(r.b)
	}
	r.s -= l
	return l, nil
}

func (r *byteRingBuffer) Close() error {
	r.closechMutex.Lock()
	defer r.closechMutex.Unlock()
	select {
	case <-r.closech:
		return errClosing
	default:
		close(r.closech)
	}
	return nil
}

type rateLimitedBuffer struct {
	wch          chan<- []byte
	closech      chan int
	closechMutex sync.Mutex
	size         uint32
	sizech       chan uint32
	sizeMutex    sync.Mutex
}

func newRateLimitedBuffer(ch chan<- []byte, size uint32) *rateLimitedBuffer {
	return &rateLimitedBuffer{
		wch:     ch,
		closech: make(chan int),
		size:    size,
		sizech:  make(chan uint32),
	}
}

func (r *rateLimitedBuffer) WriteTimeout(b []byte, timeout time.Duration) (int, error) {
	t := time.NewTimer(timeout)
	defer t.Stop()
	if timeout == 0 {
		t.Stop()
	}

	for wrote := uint32(0); wrote < uint32(len(b)); {
		r.sizeMutex.Lock()
		s := r.size
		r.sizeMutex.Unlock()
		if s == 0 {
			select {
			case ns := <-r.sizech:
				s = ns
			case <-r.closech:
				return 0, errClosing
			}
		}
		if s > uint32(len(b))-wrote {
			s = uint32(len(b)) - wrote
		}
		select {
		case r.wch <- append([]byte{}, b[wrote:wrote+s]...):
			wrote += s
			r.sizeMutex.Lock()
			r.size -= uint32(s)
			r.sizeMutex.Unlock()
		case <-r.closech:
			return 0, errClosing
		case <-t.C:
			return 0, errTimeout
		}
	}

	return len(b), nil
}

func (r *rateLimitedBuffer) Reset(size uint32) {
	r.sizeMutex.Lock()
	defer r.sizeMutex.Unlock()
	r.size = size
	select {
	case r.sizech <- size:
	default:
	}
}

func (r *rateLimitedBuffer) Close() error {
	r.closechMutex.Lock()
	defer r.closechMutex.Unlock()
	select {
	case <-r.closech:
		return errClosing
	default:
		close(r.closech)
	}
	return nil
}

type baseDelayBuffer struct {
	b    [6]uint32
	last int
	min  uint32
}

func (b *baseDelayBuffer) Push(val uint32) {
	t := time.Now()
	i := t.Second()/20 + (t.Minute()%2)*3
	if b.last == i {
		if b.b[i] > val {
			b.b[i] = val
		}
	} else {
		b.b[i] = val
		b.last = i
	}
	min := val
	for _, v := range b.b {
		if v > 0 && min > v {
			min = v
		}
	}
	b.min = min
}

func (b *baseDelayBuffer) Min() uint32 {
	return b.min
}
