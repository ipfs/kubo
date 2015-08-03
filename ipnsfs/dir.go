package ipnsfs

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
	ufspb "github.com/ipfs/go-ipfs/unixfs/pb"
)

var ErrNotYetImplemented = errors.New("not yet implemented")
var ErrInvalidChild = errors.New("invalid child node")

type Directory struct {
	fs     *Filesystem
	parent childCloser

	childDirs map[string]*Directory
	files     map[string]*File

	lock sync.Mutex
	node *dag.Node
	ctx  context.Context

	name string
}

func NewDirectory(ctx context.Context, name string, node *dag.Node, parent childCloser, fs *Filesystem) *Directory {
	return &Directory{
		ctx:       ctx,
		fs:        fs,
		name:      name,
		node:      node,
		parent:    parent,
		childDirs: make(map[string]*Directory),
		files:     make(map[string]*File),
	}
}

// closeChild updates the child by the given name to the dag node 'nd'
// and changes its own dag node, then propogates the changes upward
func (d *Directory) closeChild(name string, nd *dag.Node) error {
	_, err := d.fs.dserv.Add(nd)
	if err != nil {
		return err
	}

	d.lock.Lock()
	defer d.lock.Unlock()
	err = d.node.RemoveNodeLink(name)
	if err != nil && err != dag.ErrNotFound {
		return err
	}

	err = d.node.AddNodeLinkClean(name, nd)
	if err != nil {
		return err
	}

	return d.parent.closeChild(d.name, d.node)
}

func (d *Directory) Type() NodeType {
	return TDir
}

// childFile returns a file under this directory by the given name if it exists
func (d *Directory) childFile(name string) (*File, error) {
	fi, ok := d.files[name]
	if ok {
		return fi, nil
	}

	nd, err := d.childFromDag(name)
	if err != nil {
		return nil, err
	}
	i, err := ft.FromBytes(nd.Data)
	if err != nil {
		return nil, err
	}

	switch i.GetType() {
	case ufspb.Data_Directory:
		return nil, ErrIsDirectory
	case ufspb.Data_File:
		nfi, err := NewFile(name, nd, d, d.fs)
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
}

// childDir returns a directory under this directory by the given name if it
// exists.
func (d *Directory) childDir(name string) (*Directory, error) {
	dir, ok := d.childDirs[name]
	if ok {
		return dir, nil
	}

	nd, err := d.childFromDag(name)
	if err != nil {
		return nil, err
	}

	i, err := ft.FromBytes(nd.Data)
	if err != nil {
		return nil, err
	}

	switch i.GetType() {
	case ufspb.Data_Directory:
		ndir := NewDirectory(d.ctx, name, nd, d, d.fs)
		d.childDirs[name] = ndir
		return ndir, nil
	case ufspb.Data_File:
		return nil, fmt.Errorf("%s is not a directory", name)
	case ufspb.Data_Metadata:
		return nil, ErrNotYetImplemented
	default:
		return nil, ErrInvalidChild
	}
}

// childFromDag searches through this directories dag node for a child link
// with the given name
func (d *Directory) childFromDag(name string) (*dag.Node, error) {
	for _, lnk := range d.node.Links {
		if lnk.Name == name {
			return lnk.GetNode(d.ctx, d.fs.dserv)
		}
	}

	return nil, os.ErrNotExist
}

// Child returns the child of this directory by the given name
func (d *Directory) Child(name string) (FSNode, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.childUnsync(name)
}

// childUnsync returns the child under this directory by the given name
// without locking, useful for operations which already hold a lock
func (d *Directory) childUnsync(name string) (FSNode, error) {
	dir, err := d.childDir(name)
	if err == nil {
		return dir, nil
	}
	fi, err := d.childFile(name)
	if err == nil {
		return fi, nil
	}

	return nil, os.ErrNotExist
}

type NodeListing struct {
	Name string
	Type int
	Size int64
}

func (d *Directory) List() ([]NodeListing, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	var out []NodeListing
	for _, l := range d.node.Links {
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

		out = append(out, child)
	}

	return out, nil
}

func (d *Directory) Mkdir(name string) (*Directory, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	_, err := d.childDir(name)
	if err == nil {
		return nil, os.ErrExist
	}
	_, err = d.childFile(name)
	if err == nil {
		return nil, os.ErrExist
	}

	ndir := &dag.Node{Data: ft.FolderPBData()}

	_, err = d.fs.dserv.Add(ndir)
	if err != nil {
		return nil, err
	}

	err = d.node.AddNodeLinkClean(name, ndir)
	if err != nil {
		return nil, err
	}

	err = d.parent.closeChild(d.name, d.node)
	if err != nil {
		return nil, err
	}

	return d.childDir(name)
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

	return d.parent.closeChild(d.name, d.node)
}

// AddChild adds the node 'nd' under this directory giving it the name 'name'
func (d *Directory) AddChild(name string, nd *dag.Node) error {
	d.Lock()
	defer d.Unlock()
	pbn, err := ft.FromBytes(nd.Data)
	if err != nil {
		return err
	}

	_, err = d.childUnsync(name)
	if err == nil {
		return errors.New("directory already has entry by that name")
	}

	err = d.node.AddNodeLinkClean(name, nd)
	if err != nil {
		return err
	}

	switch pbn.GetType() {
	case ft.TDirectory:
		d.childDirs[name] = NewDirectory(d.ctx, name, nd, d, d.fs)
	case ft.TFile, ft.TMetadata, ft.TRaw:
		nfi, err := NewFile(name, nd, d, d.fs)
		if err != nil {
			return err
		}
		d.files[name] = nfi
	default:
		return ErrInvalidChild
	}
	return d.parent.closeChild(d.name, d.node)
}

func (d *Directory) GetNode() (*dag.Node, error) {
	return d.node, nil
}

func (d *Directory) Lock() {
	d.lock.Lock()
}

func (d *Directory) Unlock() {
	d.lock.Unlock()
}

func DirLookup(d *Directory, path string) (FSNode, error) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && parts[0] == "" {
		return d, nil
	}

	var cur FSNode
	cur = d
	for i, p := range parts {
		chdir, ok := cur.(*Directory)
		if !ok {
			return nil, fmt.Errorf("cannot access %s: Not a directory", strings.Join(parts[:i+1], "/"))
		}

		child, err := chdir.Child(p)
		if err != nil {
			return nil, err
		}

		cur = child
	}
	return cur, nil
}
