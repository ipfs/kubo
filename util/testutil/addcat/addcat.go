package testutil

import (
	"bytes"
	"errors"

	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
)

func AddCat(adder *core.IpfsNode, catter *core.IpfsNode, data []byte) error {
	dagNode, err := importer.BuildDagFromReader(
		bytes.NewBuffer(data),
		adder.DAG,
		adder.Pinning.GetManual(),
		chunk.DefaultSplitter)
	if err != nil {
		return err
	}
	key, err := dagNode.Key()
	if err != nil {
		return err
	}
	added := key.String()

	reader, err := coreunix.Cat(catter, added)
	if err != nil {
		return err
	}

	// verify
	var buf bytes.Buffer
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return err
	}
	if !bytes.Equal(buf.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	return nil
}
