package chunk

import (
	"bufio"
	"bytes"
	"io"
	"math"
)

type MaybeRabin struct {
	mask         int
	windowSize   int
	MinBlockSize int
	MaxBlockSize int
	inbuf        *bufio.Reader
	buf          bytes.Buffer
	window       []byte // Window is a circular buffer
	wi           int    // window index
	rollingHash  int
	an           int
	readers      []io.Reader
}

func NewMaybeRabin(avgBlkSize int) *MaybeRabin {
	blkbits := uint(math.Log2(float64(avgBlkSize)))
	rb := new(MaybeRabin)
	rb.mask = (1 << blkbits) - 1
	rb.windowSize = 16 // probably a good number...
	rb.MinBlockSize = avgBlkSize / 2
	rb.MaxBlockSize = (avgBlkSize / 2) * 3
	rb.window = make([]byte, rb.windowSize)
	rb.an = 1
	return rb
}

func (mr *MaybeRabin) push(val byte) (outval int) {
	outval = int(mr.window[mr.wi%len(mr.window)])
	mr.window[mr.wi%len(mr.window)] = val
	return
}

// Duplicate byte slice
func dup(b []byte) []byte {
	d := make([]byte, len(b))
	copy(d, b)
	return d
}

func (mr *MaybeRabin) nextReader() ([]byte, error) {
	if len(mr.readers) == 0 {
		mr.inbuf = nil
		return mr.buf.Bytes(), nil
	}
	ri := len(mr.readers) - 1
	mr.inbuf = bufio.NewReader(mr.readers[ri])
	mr.readers = mr.readers[:ri]
	return mr.Next()
}

func (mr *MaybeRabin) Next() ([]byte, error) {
	if mr.inbuf == nil {
		return nil, io.EOF
	}

	// some bullshit numbers i made up
	a := 10         // honestly, no idea what this is
	MOD := 33554383 // randomly chosen (seriously)

	var b byte
	var err error
	// Fill up the window
	for ; mr.wi < mr.windowSize; mr.wi++ {
		b, err = mr.inbuf.ReadByte()
		if err != nil {
			if err == io.EOF {
				return mr.nextReader()
			}
			return nil, err
		}
		mr.buf.WriteByte(b)
		mr.push(b)
		mr.rollingHash = (mr.rollingHash*a + int(b)) % MOD
		mr.an = (mr.an * a) % MOD
	}

	for ; true; mr.wi++ {
		b, err = mr.inbuf.ReadByte()
		if err != nil {
			break
		}
		outval := mr.push(b)
		mr.buf.WriteByte(b)
		mr.rollingHash = (mr.rollingHash*a + int(b) - mr.an*outval) % MOD
		if (mr.rollingHash&mr.mask == mr.mask && mr.buf.Len() > mr.MinBlockSize) || mr.buf.Len() >= mr.MaxBlockSize {
			block := dup(mr.buf.Bytes())
			mr.buf.Reset()
			return block, nil
		}
	}
	if err == io.EOF {
		return mr.nextReader()
	}
	return nil, err
}

func (mr *MaybeRabin) Size() int { return mr.MaxBlockSize }

func (mr *MaybeRabin) Push(r io.Reader) {
	if mr.inbuf == nil {
		mr.inbuf = bufio.NewReader(r)
	} else {
		mr.readers = append(mr.readers, r)
	}
}
