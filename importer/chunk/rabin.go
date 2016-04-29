package chunk

import (
	"hash/fnv"
	"io"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/chunker"
)

var IpfsRabinPoly = chunker.Pol(17437180132763653)

type Rabin struct {
	r *chunker.Chunker
}

func NewRabin(r io.Reader, avgBlkSize uint64) *Rabin {
	min := avgBlkSize / 3
	max := avgBlkSize + (avgBlkSize / 2)

	return NewRabinMinMax(r, min, avgBlkSize, max)
}

func NewRabinMinMax(r io.Reader, min, avg, max uint64) *Rabin {
	h := fnv.New32a()
	ch := chunker.New(r, IpfsRabinPoly, h, avg, min, max)

	return &Rabin{
		r: ch,
	}
}

func (r *Rabin) NextBytes() (Bytes, error) {
	ch, err := r.r.Next()
	if err != nil {
		return Bytes{}, err
	}

	return Bytes{nil, ch.Data}, nil
}
