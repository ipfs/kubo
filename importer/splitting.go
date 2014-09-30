package importer

import (
	"io"

	u "github.com/jbenet/go-ipfs/util"
)

type BlockSplitter interface {
	Split(io.Reader) chan []byte
}

type SizeSplitter struct {
	Size int
}

func (ss *SizeSplitter) Split(r io.Reader) chan []byte {
	out := make(chan []byte)
	go func() {
		defer close(out)
		for {
			chunk := make([]byte, ss.Size)
			nread, err := r.Read(chunk)
			if err != nil {
				if err == io.EOF {
					if nread > 0 {
						out <- chunk[:nread]
					}
					return
				}
				u.PErr("block split error: %v\n", err)
				return
			}
			if nread < ss.Size {
				chunk = chunk[:nread]
			}
			out <- chunk
		}
	}()
	return out
}
