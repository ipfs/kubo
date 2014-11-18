// package chunk implements streaming block splitters
package chunk

import (
	"io"

	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("chunk")

var DefaultSplitter = &SizeSplitter{Size: 1024 * 256}

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

		// all-chunks loop (keep creating chunks)
		for {
			// log.Infof("making chunk with size: %d", ss.Size)
			chunk := make([]byte, ss.Size)
			sofar := 0

			// this-chunk loop (keep reading until this chunk full)
			for {
				nread, err := r.Read(chunk[sofar:])
				sofar += nread
				if err == io.EOF {
					if sofar > 0 {
						// log.Infof("sending out chunk with size: %d", sofar)
						out <- chunk[:sofar]
					}
					return
				}
				if err != nil {
					log.Errorf("Block split error: %s", err)
					return
				}
				if sofar == ss.Size {
					// log.Infof("sending out chunk with size: %d", sofar)
					out <- chunk[:sofar]
					break // break out of this-chunk loop
				}
			}
		}
	}()
	return out
}
