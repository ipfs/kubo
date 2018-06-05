package io

import (
	"context"
	"fmt"
	"os"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	format "github.com/ipfs/go-ipfs/unixfs"
	hamt "github.com/ipfs/go-ipfs/unixfs/hamt"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// ShardSplitThreshold specifies how large of an unsharded directory
// the Directory code will generate. Adding entries over this value will
// result in the node being restructured into a sharded object.
var ShardSplitThreshold = 1000

// UseHAMTSharding is a global flag that signifies whether or not to use the
// HAMT sharding scheme for directory creation
var UseHAMTSharding = false

// DefaultShardWidth is the default value used for hamt sharding width.
var DefaultShardWidth = 256

// Directory allows to work with UnixFS directory nodes, adding and removing
// children. It allows to work with different directory schemes,
// like the classic or the HAMT one.
type Directory struct {
	dserv   ipld.DAGService
	dirnode *mdag.ProtoNode

	shard *hamt.Shard
}

// NewDirectory returns a Directory. It needs a DAGService to add the Children
func NewDirectory(dserv ipld.DAGService) *Directory {
	db := new(Directory)
	db.dserv = dserv
	if UseHAMTSharding {
		s, err := hamt.NewShard(dserv, DefaultShardWidth)
		if err != nil {
			panic(err) // will only panic if DefaultShardWidth is a bad value
		}
		db.shard = s
	} else {
		db.dirnode = format.EmptyDirNode()
	}
	return db
}

// ErrNotADir implies that the given node was not a unixfs directory
var ErrNotADir = fmt.Errorf("merkledag node was not a directory or shard")

// NewDirectoryFromNode loads a unixfs directory from the given IPLD node and
// DAGService.
func NewDirectoryFromNode(dserv ipld.DAGService, nd ipld.Node) (*Directory, error) {
	pbnd, ok := nd.(*mdag.ProtoNode)
	if !ok {
		return nil, ErrNotADir
	}

	pbd, err := format.FromBytes(pbnd.Data())
	if err != nil {
		return nil, err
	}

	switch pbd.GetType() {
	case format.TDirectory:
		return &Directory{
			dserv:   dserv,
			dirnode: pbnd.Copy().(*mdag.ProtoNode),
		}, nil
	case format.THAMTShard:
		shard, err := hamt.NewHamtFromDag(dserv, nd)
		if err != nil {
			return nil, err
		}

		return &Directory{
			dserv: dserv,
			shard: shard,
		}, nil
	default:
		return nil, ErrNotADir
	}
}

// SetPrefix sets the prefix of the root node
func (d *Directory) SetPrefix(prefix *cid.Prefix) {
	if d.dirnode != nil {
		d.dirnode.SetPrefix(prefix)
	}
	if d.shard != nil {
		d.shard.SetPrefix(prefix)
	}
}

// AddChild adds a (name, key)-pair to the root node.
func (d *Directory) AddChild(ctx context.Context, name string, nd ipld.Node) error {
	if d.shard == nil {
		if !UseHAMTSharding {
			_ = d.dirnode.RemoveNodeLink(name)
			return d.dirnode.AddNodeLink(name, nd)
		}

		err := d.switchToSharding(ctx)
		if err != nil {
			return err
		}
	}

	return d.shard.Set(ctx, name, nd)
}

func (d *Directory) switchToSharding(ctx context.Context) error {
	s, err := hamt.NewShard(d.dserv, DefaultShardWidth)
	if err != nil {
		return err
	}
	s.SetPrefix(&d.dirnode.Prefix)

	d.shard = s
	for _, lnk := range d.dirnode.Links() {
		cnd, err := d.dserv.Get(ctx, lnk.Cid)
		if err != nil {
			return err
		}

		err = d.shard.Set(ctx, lnk.Name, cnd)
		if err != nil {
			return err
		}
	}

	d.dirnode = nil
	return nil
}

// ForEachLink applies the given function to Links in the directory.
func (d *Directory) ForEachLink(ctx context.Context, f func(*ipld.Link) error) error {
	if d.shard == nil {
		for _, l := range d.dirnode.Links() {
			if err := f(l); err != nil {
				return err
			}
		}
		return nil
	}

	return d.shard.ForEachLink(ctx, f)
}

// Links returns the all the links in the directory node.
func (d *Directory) Links(ctx context.Context) ([]*ipld.Link, error) {
	if d.shard == nil {
		return d.dirnode.Links(), nil
	}

	return d.shard.EnumLinks(ctx)
}

// Find returns the root node of the file named 'name' within this directory.
// In the case of HAMT-directories, it will traverse the tree.
func (d *Directory) Find(ctx context.Context, name string) (ipld.Node, error) {
	if d.shard == nil {
		lnk, err := d.dirnode.GetNodeLink(name)
		switch err {
		case mdag.ErrLinkNotFound:
			return nil, os.ErrNotExist
		default:
			return nil, err
		case nil:
		}

		return d.dserv.Get(ctx, lnk.Cid)
	}

	lnk, err := d.shard.Find(ctx, name)
	if err != nil {
		return nil, err
	}

	return lnk.GetNode(ctx, d.dserv)
}

// RemoveChild removes the child with the given name.
func (d *Directory) RemoveChild(ctx context.Context, name string) error {
	if d.shard == nil {
		return d.dirnode.RemoveNodeLink(name)
	}

	return d.shard.Remove(ctx, name)
}

// GetNode returns the root of this Directory
func (d *Directory) GetNode() (ipld.Node, error) {
	if d.shard == nil {
		return d.dirnode, nil
	}

	return d.shard.Node()
}

// GetPrefix returns the CID Prefix used
func (d *Directory) GetPrefix() *cid.Prefix {
	if d.shard == nil {
		return &d.dirnode.Prefix
	}

	return d.shard.Prefix()
}
