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
