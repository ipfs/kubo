package importer

import (
	"io"

	u "github.com/jbenet/go-ipfs/util"
)

type BlockSplitter func(io.Reader) chan []byte

func SplitterBySize(n int) BlockSplitter {
	return func(r io.Reader) chan []byte {
		out := make(chan []byte)
		go func(n int) {
			defer close(out)
			for {
				chunk := make([]byte, n)
				nread, err := r.Read(chunk)
				if err != nil {
					if err == io.EOF {
						return
					}
					u.PErr("block split error: %v\n", err)
					return
				}
				if nread < n {
					chunk = chunk[:nread]
				}
				out <- chunk
			}
		}(n)
		return out
	}
}

// TODO: this should take a reader, not a byte array. what if we're splitting a 3TB file?
//Rabin Fingerprinting for file chunking
func Rabin(b []byte) [][]byte {
	var out [][]byte
	windowsize := uint64(48)
	chunkMax := 1024 * 16
	minBlkSize := 2048
	blkBegI := 0
	prime := uint64(61)

	var poly uint64
	var curchecksum uint64

	// Smaller than a window?  Get outa here!
	if len(b) <= int(windowsize) {
		return [][]byte{b}
	}

	i := 0
	for n := i; i < n+int(windowsize); i++ {
		cur := uint64(b[i])
		curchecksum = (curchecksum * prime) + cur
		poly = (poly * prime) + cur
	}

	for ; i < len(b); i++ {
		cur := uint64(b[i])
		curchecksum = (curchecksum * prime) + cur
		poly = (poly * prime) + cur
		curchecksum -= (uint64(b[i-1]) * prime)

		if i-blkgBegI >= chunkMax {
			// push block
			out = append(out, b[blkgBegI:i])
			blkgBegI = i
		}

		// first 13 bits of polynomial are 0
		if poly%8192 == 0 && i-blkgBegI >= minBlkSize {
			// push block
			out = append(out, b[blkgBegI:i])
			blkgBegI = i
		}
	}
	if i > blkgBegI {
		out = append(out, b[blkgBegI:])
	}
	return out
}
