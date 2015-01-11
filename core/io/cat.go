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
	path "github.com/jbenet/go-ipfs/path"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

func Cat(n *core.IpfsNode, k u.Key) (io.Reader, error) {
	dag := n.DAG
	dagNode, err := (&path.Resolver{dag}).ResolvePath(k.String())
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(dagNode, dag)
}
