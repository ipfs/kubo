package buffer

import (
	"bytes"
	"reflect"
	"testing"
)

func incBuf(start, size int) []byte {
	b := make([]byte, size)
	for i := 0; i < size; i++ {
		b[i] = byte((start + i) % 16)
	}
	return b
}

func testBuffer() *Circular {
	c := NewCircular(15)
	c.buf = incBuf(0, 16)
	return c
}

func TestEmptyRead(t *testing.T) {
	t.Parallel()

	var p [1]byte
	c := NewCircular(16)
	n, err := c.Read(p[:])

	if err != nil {
		t.Fatalf("Error on read operation: %v")
	}

	if n != 0 {
		t.Errorf("Read %d bytes, expected 0", n)
	}
}

// Test Read: [H+++T---]
func TestStartRead(t *testing.T) {
	t.Parallel()

	c := testBuffer()

	readSize := 8
	p := make([]byte, readSize+1)
	c.tail = readSize
	n, err := c.Read(p)

	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != readSize {
		t.Fatalf("Read expected %d bytes, got %d", readSize, n)
	}

	expected := incBuf(0, 8)
	got := p[:readSize]
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Wrong buffer values read. Expected %v, got %v", expected, got)
	}
}

func TestMiddleRead(t *testing.T) {
	t.Parallel()

	c := testBuffer()
	readSize := 8
	p := make([]byte, readSize+1)
	c.head = 4
	c.tail = 12
	n, err := c.Read(p)
	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != readSize {
		t.Fatalf("Read expected %d bytes, got %d", readSize)
	}

	expected := incBuf(4, 8)
	got := p[:readSize]
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("Wrong buffer values read. Expected %v, got %v", expected, got)
	}
}

func TestTwoReads(t *testing.T) {
	t.Parallel()

	c := testBuffer()
	readSize := 4
	p := make([]byte, readSize)
	c.head = 4
	c.tail = 12

	for i := 0; i < 2; i++ {
		n, err := c.Read(p)
		if err != nil {
			t.Fatalf("Error while reading: %v", err)
		}

		if n != readSize {
			t.Fatalf("Wrong read size. Expected %d, got %d", readSize, n)
		}

		expected := incBuf(4+(4*i), 4)
		if !reflect.DeepEqual(p, expected) {
			t.Fatalf("Wrong buffer values for read #%d. Expected %v, got %v", i+1, expected, p)
		}
	}
}

func TestReadTailZero(t *testing.T) {
	t.Parallel()

	c := testBuffer()
	readSize := 4
	p := make([]byte, readSize*2)
	c.head = 12
	c.tail = 0

	n, err := c.Read(p)
	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != readSize {
		t.Fatalf("Wrong read size. Expected %d, got %d", readSize, n)
	}

	expected := incBuf(12, 4)
	got := p[:readSize]
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Wrong buffer values for read. Expected %v, got %v", expected, got)
	}
}

func TestReadWrap(t *testing.T) {
	t.Parallel()

	c := testBuffer()
	readSize := 14
	p := make([]byte, readSize*2)
	c.head = 12
	c.tail = 10

	n, err := c.Read(p)
	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != readSize {
		t.Fatalf("Wrong read size. Expected %d, got %d", readSize, n)
	}

	expected := incBuf(12, readSize)
	got := p[:readSize]
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Wrong buffer values for read. Expected %v, got %v", expected, got)
	}
}

func TestEmptyReadAfterExhaustion(t *testing.T) {
	t.Parallel()

	c := testBuffer()
	readSize := 14
	p := make([]byte, readSize*2)
	c.head = 12
	c.tail = 10

	n, err := c.Read(p)
	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != readSize {
		t.Fatalf("Wrong read size. Expected %d, got %d", readSize, n)
	}

	expected := incBuf(12, readSize)
	got := p[:readSize]
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Wrong buffer values for read. Expected %v, got %v", expected, p)
	}

	n, err = c.Read(p)
	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != 0 {
		t.Fatalf("Wrong read size. Expected 0, got %d", n)
	}
}

func TestWriteTooBig(t *testing.T) {
	t.Parallel()

	size := 16
	p := bytes.NewBuffer(make([]byte, size+1))
	c := NewCircular(size)

	_, err := c.ReadFrom(p)
	if err != FullError {
		t.Errorf("Expected FULL error but got %v", err)
	}
}

func TestWriteReadFullFromZero(t *testing.T) {
	toWrite := incBuf(0, 16)
	c := NewCircular(16)

	n, err := c.ReadFrom(bytes.NewBuffer(toWrite))
	if err != nil {
		t.Fatalf("Error while writing: %v", err)
	}

	if n != 16 {
		t.Fatalf("Wrong number of bytes written. Expceted 16, got %d", n)
	}

	p := make([]byte, 16)
	n, err = c.Read(p)
	if err != nil {
		t.Fatalf("Error while reading: %v", err)
	}

	if n != 16 {
		t.Fatalf("")
	}
}
