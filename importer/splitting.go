package importer

import (
	"io"

	u "github.com/jbenet/go-ipfs/util"
)

// OLD
type BlockSplitter interface {
	Split(io.Reader) chan []byte
}

// NEW
type StreamSplitter interface {
	Split(chan []byte) chan []byte
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

type SizeSplitter2 struct {
	Size int
}

func (ss *SizeSplitter2) Split(in chan []byte) chan []byte {
	out := make(chan []byte)
	go func() {
		defer close(out)
		var buf []byte
		for b := range in {
			buf = append(buf, b...)
			for len(buf) > ss.Size {
				out <- buf[:ss.Size]
				buf = buf[ss.Size:]
			}
		}
		if len(buf) > 0 {
			out <- buf
		}
	}()
	return out
}
