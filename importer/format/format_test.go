package format

import (
	"testing"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

func TestMultiBlock(t *testing.T) {
	mbf := new(MultiBlock)
	for i := 0; i < 15; i++ {
		mbf.AddBlockSize(100)
	}

	mbf.Data = make([]byte, 128)

	b, err := mbf.GetBytes()
	if err != nil {
		t.Fatal(err)
	}

	pbn := new(PBData)
	err = proto.Unmarshal(b, pbn)
	if err != nil {
		t.Fatal(err)
	}

	ds, err := DataSize(b)
	if err != nil {
		t.Fatal(err)
	}

	if ds != (100*15)+128 {
		t.Fatal("Datasize calculations incorrect!")
	}
}
