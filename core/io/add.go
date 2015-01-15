package core_io

// TODO rename package to something that doesn't conflict with io/ioutil.
// Pretty names are hard to find.
//
// Candidates:
//
// go-ipfs/core/unix
// go-ipfs/core/io
// go-ipfs/core/ioutil
// go-ipfs/core/coreio
// go-ipfs/core/coreunix

import (
	"io"

	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	u "github.com/jbenet/go-ipfs/util"
)

func Add(n *core.IpfsNode, r io.Reader) (u.Key, error) {
	// TODO more attractive function signature importer.BuildDagFromReader
	dagNode, err := importer.BuildDagFromReader(
		r,
		n.DAG,
		n.Pinning.GetManual(), // Fix this interface
		chunk.DefaultSplitter,
	)
	if err != nil {
		return "", err
	}
	return dagNode.Key()
}
