package chunk

import (
	"io"

	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("chunk")

var DefaultSplitter = &SizeSplitter{Size: 1024 * 512}

type BlockSplitter interface {
	Split(r io.Reader) chan []byte
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
				log.Errorf("Block split error: %s", err)
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
