// +build !nofuse

// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"errors"
	"io"
	"os"
	"time"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"

	core "github.com/jbenet/go-ipfs/core"
	nsfs "github.com/jbenet/go-ipfs/ipnsfs"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

var DAGSERV dag.DAGService

var log = eventlog.Logger("fuse/ipns")

var (
	shortRepublishTimeout = time.Millisecond * 5
	longRepublishTimeout  = time.Millisecond * 500
)

// FileSystem is the readwrite IPNS Fuse Filesystem.
type FileSystem struct {
	Ipfs     *core.IpfsNode
	RootNode *Root
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewFileSystem(ipfs *core.IpfsNode, sk ci.PrivKey, ipfspath string) (*FileSystem, error) {
	root, err := CreateRoot(ipfs, []ci.PrivKey{sk}, ipfspath)
	DAGSERV = ipfs.DAG
	if err != nil {
		return nil, err
	}
	return &FileSystem{Ipfs: ipfs, RootNode: root}, nil
}

// Root constructs the Root of the filesystem, a Root object.
func (f FileSystem) Root() (fs.Node, error) {
	log.Debug("Filesystem, get root")
	return f.RootNode, nil
}

// Root is the root object of the filesystem tree.
type Root struct {
	Ipfs *core.IpfsNode
	Keys []ci.PrivKey

	// Used for symlinking into ipfs
	IpfsRoot  string
	LocalDirs map[string]fs.Node
	Roots     map[string]*nsfs.KeyRoot

	fs        *nsfs.Filesystem
	LocalLink *Link
}

func CreateRoot(ipfs *core.IpfsNode, keys []ci.PrivKey, ipfspath string) (*Root, error) {
	fi, err := nsfs.NewFilesystem(ipfs, keys...)
	if err != nil {
		return nil, err
	}
	ldirs := make(map[string]fs.Node)
	roots := make(map[string]*nsfs.KeyRoot)
	for _, k := range keys {
		pkh, err := k.GetPublic().Hash()
		if err != nil {
			return nil, err
		}
		name := u.Key(pkh).B58String()
		root, err := fi.GetRoot(name)
		if err != nil {
			return nil, err
		}

		roots[name] = root

		switch val := root.GetValue().(type) {
		case *nsfs.Directory:
			ldirs[name] = &Directory{val}
		case nsfs.File:
			ldirs[name] = &File{val}
		default:
			return nil, errors.New("unrecognized type")
		}
	}

	return &Root{
		fs:        fi,
		Ipfs:      ipfs,
		IpfsRoot:  ipfspath,
		Keys:      keys,
		LocalDirs: ldirs,
		LocalLink: &Link{ipfs.Identity.Pretty()},
		Roots:     roots,
	}, nil
}

// Attr returns file attributes.
func (*Root) Attr() fuse.Attr {
	log.Debug("Root Attr")
	return fuse.Attr{Mode: os.ModeDir | 0111} // -rw+x
}

// Lookup performs a lookup under this node.
func (s *Root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, fuse.ENOENT
	}

	if name == "local" {
		if s.LocalLink == nil {
			return nil, fuse.ENOENT
		}
		return s.LocalLink, nil
	}

	nd, ok := s.LocalDirs[name]
	if ok {
		switch nd := nd.(type) {
		case *Directory:
			return nd, nil
		case *File:
			return nd, nil
		default:
			return nil, fuse.EIO
		}
	}

	resolved, err := s.Ipfs.Namesys.Resolve(s.Ipfs.Context(), name)
	if err != nil {
		log.Warningf("ipns: namesys resolve error: %s", err)
		return nil, fuse.ENOENT
	}

	return &Link{s.IpfsRoot + "/" + resolved.B58String()}, nil
}

// ReadDirAll reads a particular directory. Will show locally available keys
func (r *Root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("Root ReadDirAll")
	listing := []fuse.Dirent{
		fuse.Dirent{
			Name: "local",
			Type: fuse.DT_Link,
		},
	}
	for _, k := range r.Keys {
		pub := k.GetPublic()
		hash, err := pub.Hash()
		if err != nil {
			continue
		}
		ent := fuse.Dirent{
			Name: u.Key(hash).Pretty(),
			Type: fuse.DT_Dir,
		}
		listing = append(listing, ent)
	}
	return listing, nil
}

/*
// Node is the core object representing a filesystem tree node.
type Node struct {
	root   *Root
	nsRoot *Node
	parent *Node

	repub *Republisher

	// This nodes name in its parent dir.
	// NOTE: this strategy wont work well if we allow hard links
	// (im all for murdering the thought of hard links)
	name string

	// Private keys held by nodes at the root of a keyspace
	// WARNING(security): the PrivKey interface is currently insecure
	// (holds the raw key). It will be secured later.
	key ci.PrivKey

	Ipfs   *core.IpfsNode
	Nd     *mdag.Node
	dagMod *mod.DagModifier
	cached *ftpb.Data
}
*/

type Directory struct {
	dir *nsfs.Directory
}

type File struct {
	fi nsfs.File
}

/*
func (s *Node) loadData() error {
	s.cached = new(ftpb.Data)
	return proto.Unmarshal(s.Nd.Data, s.cached)
}
*/

// Attr returns the attributes of a given node.
func (d *Directory) Attr() fuse.Attr {
	log.Debug("Directory Attr")
	return fuse.Attr{Mode: os.ModeDir | 0555}
}

// Attr returns the attributes of a given node.
func (fi *File) Attr() fuse.Attr {
	log.Debug("File Attr")
	size, err := fi.fi.Size()
	if err != nil {
		log.Critical("Failed to get file size: %s", err)
	}
	return fuse.Attr{
		Mode: os.FileMode(0666),
		Size: uint64(size),
	}
}

// Lookup performs a lookup under this node.
func (s *Directory) Lookup(ctx context.Context, name string) (fs.Node, error) {
	child, err := s.dir.Child(name)
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	switch child := child.(type) {
	case *nsfs.Directory:
		return &Directory{child}, nil
	case nsfs.File:
		return &File{child}, nil
	default:
		panic("system has proven to be insane")
	}
}

// ReadDirAll reads the link structure as directory entries
func (dir *Directory) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	for _, name := range dir.dir.List() {
		entries = append(entries, fuse.Dirent{Name: name, Type: fuse.DT_File})
	}

	if len(entries) > 0 {
		return entries, nil
	}
	return nil, fuse.ENOENT
}

func (fi *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	_, err := fi.fi.Seek(req.Offset, os.SEEK_SET)
	if err != nil {
		return err
	}

	fisize, err := fi.fi.Size()
	if err != nil {
		return err
	}

	readsize := min(req.Size, int(fisize-req.Offset))
	n, err := io.ReadFull(fi.fi, resp.Data[:readsize])
	resp.Data = resp.Data[:n]
	return err // may be non-nil / not succeeded
}

func (fi *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	wrote, err := fi.fi.WriteAt(req.Data, req.Offset)
	if err != nil {
		return err
	}
	resp.Size = wrote

	return nil
}

func (fi *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	return fi.fi.Flush()
}

func (n *Directory) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	panic("NYI")
}

func (fi *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return fi.fi.Flush()
}

func (dir *Directory) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	child, err := dir.dir.Mkdir(req.Name)
	if err != nil {
		return nil, err
	}

	return &Directory{child}, nil
}

func (fi *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	//log.Debug("[%s] Received open request! flags = %s", n.name, req.Flags.String())
	//TODO: check open flags and truncate if necessary
	if req.Flags&fuse.OpenTruncate != 0 {
		log.Warning("Need to truncate file!")
		err := fi.fi.Truncate(0)
		if err != nil {
			return nil, err
		}
	} else if req.Flags&fuse.OpenAppend != 0 {
		log.Warning("Need to append to file!")
	}
	return fi, nil
}

func (fi *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fi.fi.Close()
}

/*
func (n *Node) Mknod(ctx context.Context, req *fuse.MknodRequest) (fs.Node, error) {
	return nil, nil
}
*/

func (dir *Directory) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	// New 'empty' file
	nd := &dag.Node{Data: ft.FilePBData(nil, 0)}
	err := dir.dir.AddChild(req.Name, nd)
	if err != nil {
		return nil, nil, err
	}

	child, err := dir.dir.Child(req.Name)
	if err != nil {
		return nil, nil, err
	}

	fi, ok := child.(nsfs.File)
	if !ok {
		return nil, nil, errors.New("child creation failed")
	}

	nodechild := &File{fi}
	return nodechild, nodechild, nil
}

func (dir *Directory) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	err := dir.dir.Unlink(req.Name)
	if err != nil {
		return fuse.ENOENT
	}
	return nil
}

func (dir *Directory) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	cur, err := dir.dir.Child(req.OldName)
	if err != nil {
		return err
	}

	err = dir.dir.Unlink(req.OldName)
	if err != nil {
		return err
	}

	switch newDir := newDir.(type) {
	case *Directory:
		nd, err := cur.GetNode()
		if err != nil {
			return err
		}

		err = newDir.dir.AddChild(req.NewName, nd)
		if err != nil {
			return err
		}
	case *File:
		log.Critical("Cannot move node into a file!")
		return fuse.EPERM
	default:
		log.Critical("Unknown node type for rename target dir!")
		return errors.New("Unknown fs node type!")
	}
	return nil
}

// to check that out Node implements all the interfaces we want
type ipnsRoot interface {
	fs.Node
	fs.HandleReadDirAller
	fs.NodeStringLookuper
}

var _ ipnsRoot = (*Root)(nil)

type ipnsDirectory interface {
	fs.HandleReadDirAller
	fs.Node
	fs.NodeCreater
	fs.NodeFsyncer
	fs.NodeMkdirer
	fs.NodeRemover
	fs.NodeRenamer
	fs.NodeStringLookuper
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type ipnsFile interface {
	fs.HandleFlusher
	fs.HandleReader
	fs.HandleWriter
	fs.HandleReleaser
	fs.Node
	fs.NodeFsyncer
	fs.NodeOpener
}

var _ ipnsDirectory = (*Directory)(nil)
var _ ipnsFile = (*File)(nil)
