package testutil

import (
	"bytes"
	"errors"

	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	unixfs "github.com/ipfs/go-ipfs/shell/unixfs"
)

func AddCat(adder *core.IpfsNode, catter *core.IpfsNode, data []byte) error {
	dagNode, err := unixfs.AddFromReader(adder, bytes.NewBuffer(data))
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
