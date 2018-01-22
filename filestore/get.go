package filestore

import (
	"context"
	"fmt"
	"os"

	dag "github.com/ipfs/go-ipfs/merkledag"
	pi "github.com/ipfs/go-ipfs/thirdparty/posinfo"
	unixfs "github.com/ipfs/go-ipfs/unixfs"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	node "gx/ipfs/QmPN7cwmpcc4DWXb4KTB9dNAJgjuPY69h3npsMfhRrQL9c/go-ipld-format"
)

// Getter gets nodes directly into the filestore.  Call Init before
// the first use and then Get to get a node.
type Getter struct {
	Ctx       context.Context
	FullPath  string
	Nodes     node.NodeGetter
	Filestore *Filestore
	fh        *os.File
}

// Init inits the filestore getter
func (g *Getter) Init() error {
	err := g.Filestore.FileManager().CheckPath(g.FullPath)
	if err != nil {
		return err
	}

	g.fh, err = os.Create(g.FullPath)
	if err != nil {
		return err
	}

	return nil
}

// Get gets a node directly into the filestore
func (g *Getter) Get(c *cid.Cid, offset uint64) (uint64, error) {
	node, err := g.Nodes.Get(g.Ctx, c)
	if err != nil {
		return 0, err
	}
	switch n := node.(type) {
	case *dag.ProtoNode:
		pbn, err := unixfs.FromBytes(n.Data())
		if err != nil {
			return 0, err
		}
		if len(pbn.Data) != 0 {
			return 0, fmt.Errorf("%s: unsupported node type", c.String())
		}
		// still need to store the node, incase the node getter
		// bypasses the normal blockstore
		err = g.Filestore.Put(n)
		if err != nil {
			return 0, err
		}
		for _, lnk := range n.Links() {
			offset, err = g.Get(lnk.Cid, offset)
			if err != nil {
				return 0, err
			}
		}
		return offset, nil
	case *dag.RawNode:
		data := n.RawData()
		_, err := g.fh.WriteAt(data, int64(offset))
		if err != nil {
			return 0, err
		}
		fsn := &pi.FilestoreNode{node, &pi.PosInfo{
			Offset:   offset,
			FullPath: g.FullPath,
		}}
		err = g.Filestore.Put(fsn)
		if err != nil {
			return 0, err
		}
		return offset + uint64(len(data)), nil
	default:
		return 0, fmt.Errorf("%s: unsupported node type", c.String())
	}

}
