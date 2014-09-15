package importer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

//pseudocode stolen from the internet
func rollhash(S []byte) {
	a := 10
	mask := 0xfff
	MOD := 33554383 //randomly chosen
	windowSize := 16
	an := 1
	rollingHash := 0
	for i := 0; i < windowSize; i++ {
		rollingHash = (rollingHash*a + int(S[i])) % MOD
		an = (an * a) % MOD
	}
	if rollingHash&mask == mask {
		// "match"
		fmt.Println("match")
	}
	for i := 1; i < len(S)-windowSize; i++ {
		rollingHash = (rollingHash*a + int(S[i+windowSize-1]) - an*int(S[i-1])) % MOD
		if rollingHash&mask == mask {
			//print "match"
			fmt.Println("match")
		}
	}
}

func ThisMightBeRabin(r io.Reader) chan []byte {
	out := make(chan []byte)
	go func() {
		inbuf := bufio.NewReader(r)
		blkbuf := new(bytes.Buffer)

		// some bullshit numbers
		a := 10
		mask := 0xfff   //make this smaller for smaller blocks
		MOD := 33554383 //randomly chosen
		windowSize := 16
		an := 1
		rollingHash := 0

		window := make([]byte, windowSize)
		get := func(i int) int { return int(window[i%len(window)]) }
		set := func(i int, val byte) { window[i%len(window)] = val }
		dup := func(b []byte) []byte {
			d := make([]byte, len(b))
			copy(d, b)
			return d
		}

		i := 0
		for ; i < windowSize; i++ {
			b, err := inbuf.ReadByte()
			if err != nil {
				fmt.Println(err)
				return
			}
			blkbuf.WriteByte(b)
			window[i] = b
			rollingHash = (rollingHash*a + int(b)) % MOD
			an = (an * a) % MOD
		}
		/* This is too short for a block
		if rollingHash&mask == mask {
			// "match"
			fmt.Println("match")
		}
		*/
		for ; true; i++ {
			b, err := inbuf.ReadByte()
			if err != nil {
				break
			}
			outval := get(i)
			set(i, b)
			blkbuf.WriteByte(b)
			rollingHash = (rollingHash*a + get(i) - an*outval) % MOD
			if rollingHash&mask == mask {
				//print "match"
				out <- dup(blkbuf.Bytes())
				blkbuf.Reset()
			}
			peek, err := inbuf.Peek(windowSize)
			if err != nil {
				break
			}
			if len(peek) != windowSize {
				break
			}
		}
		io.Copy(blkbuf, inbuf)
		out <- blkbuf.Bytes()
		close(out)
	}()
	return out
}

/*
func WhyrusleepingCantImplementRabin(r io.Reader) chan []byte {
	out := make(chan []byte, 4)
	go func() {
		buf := bufio.NewReader(r)
		blkbuf := new(bytes.Buffer)
		window := make([]byte, 16)
		var val uint64
		prime := uint64(61)

		get := func(i int) uint64 {
			return uint64(window[i%len(window)])
		}

		set := func(i int, val byte) {
			window[i%len(window)] = val
		}

		for i := 0; ; i++ {
			curb, err := buf.ReadByte()
			if err != nil {
				break
			}
			set(i, curb)
			blkbuf.WriteByte(curb)

			hash := md5.Sum(window)
			if hash[0] == 0 && hash[1] == 0 {
				out <- blkbuf.Bytes()
				blkbuf.Reset()
			}
		}
		out <- blkbuf.Bytes()
		close(out)
	}()

	return out
}
*/
