package chunk

import (
	"hash/fnv"
	"io"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/chunker"
)

// IpfsRabinPoly is the irreducible polynomial of degree 53 used by for Rabin.
var IpfsRabinPoly = chunker.Pol(17437180132763653)

// Rabin implements the Splitter interface and splits content with Rabin
// fingerprints.
type Rabin struct {
	r      *chunker.Chunker
	reader io.Reader
}

// NewRabin creates a new Rabin splitter with the given
// average block size.
func NewRabin(r io.Reader, avgBlkSize uint64) *Rabin {
	min := avgBlkSize / 3
	max := avgBlkSize + (avgBlkSize / 2)

	return NewRabinMinMax(r, min, avgBlkSize, max)
}

// NewRabinMinMax returns a new Rabin splitter which uses
// the given min, average and max block sizes.
func NewRabinMinMax(r io.Reader, min, avg, max uint64) *Rabin {
	h := fnv.New32a()
	ch := chunker.New(r, IpfsRabinPoly, h, avg, min, max)

	return &Rabin{
		r:      ch,
		reader: r,
	}
}

// NextBytes reads the next bytes from the reader and returns a slice.
func (r *Rabin) NextBytes() ([]byte, error) {
	ch, err := r.r.Next()
	if err != nil {
		return nil, err
	}

	return ch.Data, nil
}

// Reader returns the io.Reader associated to this Splitter.
func (r *Rabin) Reader() io.Reader {
	return r.reader
}
