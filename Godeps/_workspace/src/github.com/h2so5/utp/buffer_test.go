package utp

import (
	"bytes"
	"math"
	"testing"
	"time"
)

func TestPacketBuffer(t *testing.T) {
	size := 12
	b := newPacketBuffer(12, 1)

	if b.space() != size {
		t.Errorf("expected space == %d; got %d", size, b.space())
	}

	for i := 1; i <= size; i++ {
		b.push(&packet{header: header{seq: uint16(i)}})
	}

	if b.space() != 0 {
		t.Errorf("expected space == 0; got %d", b.space())
	}

	a := []byte{255, 7}
	ack := b.generateSelectiveACK()
	if !bytes.Equal(a, ack) {
		t.Errorf("expected ack == %v; got %v", a, ack)
	}

	err := b.push(&packet{header: header{seq: 15}})
	if err == nil {
		t.Fatal("push should fail")
	}

	all := b.all()
	if len(all) != size {
		t.Errorf("expected %d packets sequence; got %d", size, len(all))
	}

	f := b.fetch(6)
	if f == nil {
		t.Fatal("fetch should not fail")
	}

	b.compact()

	err = b.push(&packet{header: header{seq: 15}})
	if err != nil {
		t.Fatal(err)
	}

	err = b.push(&packet{header: header{seq: 17}})
	if err != nil {
		t.Fatal(err)
	}

	for i := 7; i <= size; i++ {
		f := b.fetch(uint16(i))
		if f == nil {
			t.Fatal("fetch should not fail")
		}
	}

	a = []byte{128, 2}
	ack = b.generateSelectiveACK()
	if !bytes.Equal(a, ack) {
		t.Errorf("expected ack == %v; got %v", a, ack)
	}

	all = b.all()
	if len(all) != 2 {
		t.Errorf("expected 2 packets sequence; got %d", len(all))
	}

	b.compact()
	if b.space() != 9 {
		t.Errorf("expected space == 9; got %d", b.space())
	}

	ack = b.generateSelectiveACK()
	b.processSelectiveACK(ack)

	all = b.all()
	if len(all) != 1 {
		t.Errorf("expected size == 1; got %d", len(all))
	}
}

func TestPacketBufferBoundary(t *testing.T) {
	begin := math.MaxUint16 - 3
	b := newPacketBuffer(12, begin)
	for i := begin; i != 5; i = (i + 1) % (math.MaxUint16 + 1) {
		err := b.push(&packet{header: header{seq: uint16(i)}})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPacketRingBuffer(t *testing.T) {
	b := newPacketRingBuffer(5)
	for i := 0; i < 7; i++ {
		b.push(&packet{header: header{seq: uint16(i)}})
	}

	if b.size() != 5 {
		t.Errorf("expected size == 5; got %d", b.size())
	}

	p := b.pop()
	if p.header.seq != 2 {
		t.Errorf("expected header.seq == 2; got %d", p.header.seq)
	}

	if b.size() != 4 {
		t.Errorf("expected size == 4; got %d", b.size())
	}

	for b.pop() != nil {
	}

	if !b.empty() {
		t.Errorf("buffer must be empty")
	}

	go func() {
		for i := 0; i < 5; i++ {
			b.push(&packet{header: header{seq: uint16(i)}})
		}
	}()

	p, err := b.popOne(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if p.header.seq != 0 {
		t.Errorf("expected header.seq == 0; got %d", p.header.seq)
	}
}

func TestByteRingBuffer(t *testing.T) {

	b := newByteRingBuffer(5)
	for i := 0; i < 100; i++ {
		b.Write([]byte{byte(i)})
	}

	var buf [10]byte
	l, err := b.ReadTimeout(buf[:], 0)
	if err != nil {
		t.Fatal(err)
	}

	e := []byte{95, 96, 97, 98, 99}
	if !bytes.Equal(buf[:l], e) {
		t.Errorf("expected payload of %v; got %v", e, buf[:l])
	}

	e2 := []byte("abcdefghijklmnopqrstuvwxyz")
	go func() {
		_, err := b.Write(e2)
		if err != nil {
			t.Fatal(err)
		}
	}()

	l, err = b.ReadTimeout(buf[:], 0)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf[:l], e2[len(e2)-5:]) {
		t.Errorf("expected payload of %v; got %v", e2[len(e2)-5:], buf[:l])
	}
}
