package mfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	ufspb "github.com/ipfs/go-ipfs/unixfs/pb"

	node "gx/ipfs/QmRSU5EqqWVZSNdbU51yXmVoF1uNw3JgTNB6RaiL7DZM16/go-ipld-node"
)

var ErrNotYetImplemented = errors.New("not yet implemented")
var ErrInvalidChild = errors.New("invalid child node")
var ErrDirExists = errors.New("directory already has entry by that name")

type Directory struct {
	dserv  dag.DAGService
	parent childCloser

	childDirs map[string]*Directory
	files     map[string]*File

	lock sync.Mutex
	node *dag.ProtoNode
	ctx  context.Context

	modTime time.Time

	name string
}

func NewDirectory(ctx context.Context, name string, node *dag.ProtoNode, parent childCloser, dserv dag.DAGService) *Directory {
	return &Directory{
		dserv:     dserv,
		ctx:       ctx,
		name:      name,
		node:      node,
		parent:    parent,
		childDirs: make(map[string]*Directory),
		files:     make(map[string]*File),
		modTime:   time.Now(),
	}
}

// closeChild updates the child by the given name to the dag node 'nd'
// and changes its own dag node
func (d *Directory) closeChild(name string, nd *dag.ProtoNode, sync bool) error {
	mynd, err := d.closeChildUpdate(name, nd, sync)
	if err != nil {
		return err
	}

	if sync {
		return d.parent.closeChild(d.name, mynd, true)
	}
	return nil
}

// closeChildUpdate is the portion of closeChild that needs to be locked around
func (d *Directory) closeChildUpdate(name string, nd *dag.ProtoNode, sync bool) (*dag.ProtoNode, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	err := d.updateChild(name, nd)
	if err != nil {
		return nil, err
	}

	if sync {
		return d.flushCurrentNode()
	}
	return nil, nil
}

func (d *Directory) flushCurrentNode() (*dag.ProtoNode, error) {
	_, err := d.dserv.Add(d.node)
	if err != nil {
		return nil, err
	}

	return d.node.Copy().(*dag.ProtoNode), nil
}

func (d *Directory) updateChild(name string, nd node.Node) error {
	err := d.node.RemoveNodeLink(name)
	if err != nil && err != dag.ErrNotFound {
		return err
	}

	err = d.node.AddNodeLinkClean(name, nd)
	if err != nil {
		return err
	}

	d.modTime = time.Now()

	return nil
}

func (d *Directory) Type() NodeType {
	return TDir
}

// childNode returns a FSNode under this directory by the given name if it exists.
// it does *not* check the cached dirs and files
func (d *Directory) childNode(name string) (FSNode, error) {
	nd, err := d.childFromDag(name)
	if err != nil {
		return nil, err
	}

	return d.cacheNode(name, nd)
}

// cacheNode caches a node into d.childDirs or d.files and returns the FSNode.
func (d *Directory) cacheNode(name string, nd node.Node) (FSNode, error) {
	switch nd := nd.(type) {
	case *dag.ProtoNode:
		i, err := ft.FromBytes(nd.Data())
		if err != nil {
			return nil, err
		}

		switch i.GetType() {
		case ufspb.Data_Directory:
			ndir := NewDirectory(d.ctx, name, nd, d, d.dserv)
			d.childDirs[name] = ndir
			return ndir, nil
		case ufspb.Data_File, ufspb.Data_Raw, ufspb.Data_Symlink:
			nfi, err := NewFile(name, nd, d, d.dserv)
			if err != nil {
				return nil, err
			}
			d.files[name] = nfi
			return nfi, nil
		case ufspb.Data_Metadata:
			return nil, ErrNotYetImplemented
		default:
			return nil, ErrInvalidChild
		}
	case *dag.RawNode:
		nfi, err := NewFile(name, nd, d, d.dserv)
		if err != nil {
			return nil, err
		}
		d.files[name] = nfi
		return nfi, nil
	default:
		return nil, fmt.Errorf("unrecognized node type in cache node")
	}
}

// Child returns the child of this directory by the given name
func (d *Directory) Child(name string) (FSNode, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.childUnsync(name)
}

func (d *Directory) Uncache(name string) {
	d.lock.Lock()
	defer d.lock.Unlock()
	delete(d.files, name)
	delete(d.childDirs, name)
}

// childFromDag searches through this directories dag node for a child link
// with the given name
func (d *Directory) childFromDag(name string) (node.Node, error) {
	pbn, err := d.node.GetLinkedNode(d.ctx, d.dserv, name)
	switch err {
	case nil:
		return pbn, nil
	case dag.ErrLinkNotFound:
		return nil, os.ErrNotExist
	default:
		return nil, err
	}
}

// childUnsync returns the child under this directory by the given name
// without locking, useful for operations which already hold a lock
func (d *Directory) childUnsync(name string) (FSNode, error) {
	cdir, ok := d.childDirs[name]
	if ok {
		return cdir, nil
	}

	cfile, ok := d.files[name]
	if ok {
		return cfile, nil
	}

	return d.childNode(name)
}

type NodeListing struct {
	Name string
	Type int
	Size int64
	Hash string
}

func (d *Directory) ListNames() []string {
	d.lock.Lock()
	defer d.lock.Unlock()

	names := make(map[string]struct{})
	for n, _ := range d.childDirs {
		names[n] = struct{}{}
	}
	for n, _ := range d.files {
		names[n] = struct{}{}
	}

	for _, l := range d.node.Links() {
		names[l.Name] = struct{}{}
	}

	var out []string
	for n, _ := range names {
		out = append(out, n)
	}
	sort.Strings(out)

	return out
}

func (d *Directory) List() ([]NodeListing, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	var out []NodeListing
	for _, l := range d.node.Links() {
		child := NodeListing{}
		child.Name = l.Name

		c, err := d.childUnsync(l.Name)
		if err != nil {
			return nil, err
		}

		child.Type = int(c.Type())
		if c, ok := c.(*File); ok {
			size, err := c.Size()
			if err != nil {
				return nil, err
			}
			child.Size = size
		}
		nd, err := c.GetNode()
		if err != nil {
			return nil, err
		}

		child.Hash = nd.Cid().String()

		out = append(out, child)
	}

	return out, nil
}

func (d *Directory) Mkdir(name string) (*Directory, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	fsn, err := d.childUnsync(name)
	if err == nil {
		switch fsn := fsn.(type) {
		case *Directory:
			return fsn, os.ErrExist
		case *File:
			return nil, os.ErrExist
		default:
			return nil, fmt.Errorf("unrecognized type: %#v", fsn)
		}
	}

	ndir := new(dag.ProtoNode)
	ndir.SetData(ft.FolderPBData())

	_, err = d.dserv.Add(ndir)
	if err != nil {
		return nil, err
	}

	err = d.node.AddNodeLinkClean(name, ndir)
	if err != nil {
		return nil, err
	}

	dirobj := NewDirectory(d.ctx, name, ndir, d, d.dserv)
	d.childDirs[name] = dirobj
	return dirobj, nil
}

func (d *Directory) Unlink(name string) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	delete(d.childDirs, name)
	delete(d.files, name)

	err := d.node.RemoveNodeLink(name)
	if err != nil {
		return err
	}

	_, err = d.dserv.Add(d.node)
	if err != nil {
		return err
	}

	return nil
}

func (d *Directory) Flush() error {
	d.lock.Lock()
	nd, err := d.flushCurrentNode()
	if err != nil {
		d.lock.Unlock()
		return err
	}
	d.lock.Unlock()

	return d.parent.closeChild(d.name, nd, true)
}

// AddChild adds the node 'nd' under this directory giving it the name 'name'
func (d *Directory) AddChild(name string, nd node.Node) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	_, err := d.childUnsync(name)
	if err == nil {
		return ErrDirExists
	}

	_, err = d.dserv.Add(nd)
	if err != nil {
		return err
	}

	err = d.node.AddNodeLinkClean(name, nd)
	if err != nil {
		return err
	}

	d.modTime = time.Now()
	return nil
}

func (d *Directory) sync() error {
	for name, dir := range d.childDirs {
		nd, err := dir.GetNode()
		if err != nil {
			return err
		}

		err = d.updateChild(name, nd)
		if err != nil {
			return err
		}
	}

	for name, file := range d.files {
		nd, err := file.GetNode()
		if err != nil {
			return err
		}

		err = d.updateChild(name, nd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Directory) Path() string {
	cur := d
	var out string
	for cur != nil {
		out = path.Join(cur.name, out)
		cur = cur.parent.(*Directory)
	}
	return out
}

func (d *Directory) GetNode() (node.Node, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	err := d.sync()
	if err != nil {
		return nil, err
	}

	_, err = d.dserv.Add(d.node)
	if err != nil {
		return nil, err
	}

	return d.node.Copy().(*dag.ProtoNode), nil
}
