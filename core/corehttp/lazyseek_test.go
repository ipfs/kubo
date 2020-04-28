package corehttp

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

type badSeeker struct {
	io.ReadSeeker
}

var badSeekErr = fmt.Errorf("I'm a bad seeker")

func (bs badSeeker) Seek(offset int64, whence int) (int64, error) {
	off, err := bs.ReadSeeker.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}
	return off, badSeekErr
}

func TestLazySeekerError(t *testing.T) {
	underlyingBuffer := strings.NewReader("fubar")
	s := &lazySeeker{
		reader: badSeeker{underlyingBuffer},
		size:   underlyingBuffer.Size(),
	}
	off, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatal(err)
	}
	if off != s.size {
		t.Fatal("expected to seek to the end")
	}

	// shouldn't have actually seeked.
	b, err := ioutil.ReadAll(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatal("expected to read nothing")
	}

	// shouldn't need to actually seek.
	off, err = s.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if off != 0 {
		t.Fatal("expected to seek to the start")
	}
	b, err = ioutil.ReadAll(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "fubar" {
		t.Fatal("expected to read string")
	}

	// should fail the second time.
	off, err = s.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if off != 0 {
		t.Fatal("expected to seek to the start")
	}
	// right here...
	b, err = ioutil.ReadAll(s)
	if err == nil {
		t.Fatalf("expected an error, got output %s", string(b))
	}
	if err != badSeekErr {
		t.Fatalf("expected a bad seek error, got %s", err)
	}
	if len(b) != 0 {
		t.Fatalf("expected to read nothing")
	}
}

func TestLazySeeker(t *testing.T) {
	underlyingBuffer := strings.NewReader("fubar")
	s := &lazySeeker{
		reader: underlyingBuffer,
		size:   underlyingBuffer.Size(),
	}
	expectByte := func(b byte) {
		t.Helper()
		var buf [1]byte
		n, err := io.ReadFull(s, buf[:])
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected to read one byte, read %d", n)
		}
		if buf[0] != b {
			t.Fatalf("expected %b, got %b", b, buf[0])
		}
	}
	expectSeek := func(whence int, off, expOff int64, expErr string) {
		t.Helper()
		n, err := s.Seek(off, whence)
		if expErr == "" {
			if err != nil {
				t.Fatal("unexpected seek error: ", err)
			}
		} else {
			if err == nil || err.Error() != expErr {
				t.Fatalf("expected %s, got %s", err, expErr)
			}
		}
		if n != expOff {
			t.Fatalf("expected offset %d, got, %d", expOff, n)
		}
	}

	expectSeek(io.SeekEnd, 0, s.size, "")
	b, err := ioutil.ReadAll(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 0 {
		t.Fatal("expected to read nothing")
	}
	expectSeek(io.SeekEnd, -1, s.size-1, "")
	expectByte('r')
	expectSeek(io.SeekStart, 0, 0, "")
	expectByte('f')
	expectSeek(io.SeekCurrent, 1, 2, "")
	expectByte('b')
	expectSeek(io.SeekCurrent, -100, 3, "invalid seek offset")
}
