package chunk

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
)

type MaybeRabin struct {
	mask         int
	windowSize   int
	MinBlockSize int
	MaxBlockSize int
}

func NewMaybeRabin(avgBlkSize int) *MaybeRabin {
	blkbits := uint(math.Log2(float64(avgBlkSize)))
	rb := new(MaybeRabin)
	rb.mask = (1 << blkbits) - 1
	rb.windowSize = 16 // probably a good number...
	rb.MinBlockSize = avgBlkSize / 2
	rb.MaxBlockSize = (avgBlkSize / 2) * 3
	return rb
}

func (mr *MaybeRabin) Split(r io.Reader) chan []byte {
	out := make(chan []byte, 16)
	go func() {
		inbuf := bufio.NewReader(r)
		blkbuf := new(bytes.Buffer)

		// some bullshit numbers i made up
		a := 10         // honestly, no idea what this is
		MOD := 33554383 // randomly chosen (seriously)
		an := 1
		rollingHash := 0

		// Window is a circular buffer
		window := make([]byte, mr.windowSize)
		push := func(i int, val byte) (outval int) {
			outval = int(window[i%len(window)])
			window[i%len(window)] = val
			return
		}

		// Duplicate byte slice
		dup := func(b []byte) []byte {
			d := make([]byte, len(b))
			copy(d, b)
			return d
		}

		// Fill up the window
		i := 0
		for ; i < mr.windowSize; i++ {
			b, err := inbuf.ReadByte()
			if err != nil {
				fmt.Println(err)
				return
			}
			blkbuf.WriteByte(b)
			push(i, b)
			rollingHash = (rollingHash*a + int(b)) % MOD
			an = (an * a) % MOD
		}

		for ; true; i++ {
			b, err := inbuf.ReadByte()
			if err != nil {
				break
			}
			outval := push(i, b)
			blkbuf.WriteByte(b)
			rollingHash = (rollingHash*a + int(b) - an*outval) % MOD
			if (rollingHash&mr.mask == mr.mask && blkbuf.Len() > mr.MinBlockSize) ||
				blkbuf.Len() >= mr.MaxBlockSize {
				out <- dup(blkbuf.Bytes())
				blkbuf.Reset()
			}

			// Check if there are enough remaining
			peek, err := inbuf.Peek(mr.windowSize)
			if err != nil || len(peek) != mr.windowSize {
				break
			}
		}
		io.Copy(blkbuf, inbuf)
		out <- blkbuf.Bytes()
		close(out)
	}()
	return out
}
