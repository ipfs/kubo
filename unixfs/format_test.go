package unixfs

import (
	"testing"

	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"

	pb "github.com/ipfs/go-ipfs/unixfs/pb"
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
