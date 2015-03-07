package unixfs

import (
	"testing"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	pb "github.com/jbenet/go-ipfs/unixfs/pb"
)

func TestFSNode(t *testing.T) {
	fsn := new(FSNode)
	fsn.Type = TFile
	for i := 0; i < 15; i++ {
		fsn.AddBlockSize(100)
	}

	fsn.Data = make([]byte, 128)

	b, err := fsn.GetBytes()
	if err != nil {
		t.Fatal(err)
	}

	pbn := new(pb.Data)
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
