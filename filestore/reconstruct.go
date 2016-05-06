package filestore

import (
	//"fmt"
	//"errors"
	//"io"
	//"os"

	dag "github.com/ipfs/go-ipfs/merkledag/pb"
	fs "github.com/ipfs/go-ipfs/unixfs/pb"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
)

func reconstruct(data []byte, blockData []byte) (interface{}, error) {
	// Decode data to merkledag protobuffer
	var pbn dag.PBNode
	err := pbn.Unmarshal(data)
	if err != nil {
		panic(err)
	}

	// Decode node's data to unixfs protobuffer
	fs_pbn := new(fs.Data)
	err = proto.Unmarshal(pbn.Data, fs_pbn)
	if err != nil {
		panic(err)
	}

	// replace data
	fs_pbn.Data = blockData

	// Reencode unixfs protobuffer
	pbn.Data, err = proto.Marshal(fs_pbn)
	if err != nil {
		panic(err)
	}

	// Reencode merkledag protobuffer
	return pbn.Marshal()
}
