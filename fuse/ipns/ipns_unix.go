// +build !nofuse

// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"errors"
	"os"
	"strings"

	fuse "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"

	core "github.com/ipfs/go-ipfs/core"
	nsfs "github.com/ipfs/go-ipfs/ipnsfs"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

var log = eventlog.Logger("fuse/ipns")

// FileSystem is the readwrite IPNS Fuse Filesystem.
type FileSystem struct {
	Ipfs     *core.IpfsNode
	RootNode *Root
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewFileSystem(ipfs *core.IpfsNode, sk ci.PrivKey, ipfspath, ipnspath string) (*FileSystem, error) {
	root, err := CreateRoot(ipfs, []ci.PrivKey{sk}, ipfspath, ipnspath)
	if err != nil {
		return nil, err
	}
	return &FileSystem{Ipfs: ipfs, RootNode: root}, nil
}

// Root constructs the Root of the filesystem, a Root object.
func (f *FileSystem) Root() (fs.Node, error) {
	log.Debug("Filesystem, get root")
	return f.RootNode, nil
}

func (f *FileSystem) Destroy() {
	err := f.RootNode.Close()
	if err != nil {
		log.Errorf("Error Shutting Down Filesystem: %s\n", err)
	}
}

// Root is the root object of the filesystem tree.
type Root struct {
	Ipfs *core.IpfsNode
	Keys []ci.PrivKey

	// Used for symlinking into ipfs
	IpfsRoot  string
	IpnsRoot  string
	LocalDirs map[string]fs.Node
	Roots     map[string]*nsfs.KeyRoot

	fs        *nsfs.Filesystem
	LocalLink *Link
}

func CreateRoot(ipfs *core.IpfsNode, keys []ci.PrivKey, ipfspath, ipnspath string) (*Root, error) {
	ldirs := make(map[string]fs.Node)
	roots := make(map[string]*nsfs.KeyRoot)
	for _, k := range keys {
		pkh, err := k.GetPublic().Hash()
		if err != nil {
			return nil, err
		}
		name := u.Key(pkh).B58String()
		root, err := ipfs.IpnsFs.GetRoot(name)
		if err != nil {
			return nil, err
		}

		roots[name] = root

		switch val := root.GetValue().(type) {
		case *nsfs.Directory:
			ldirs[name] = &Directory{dir: val}
		case *nsfs.File:
			ldirs[name] = &File{fi: val}
		default:
			return nil, errors.New("unrecognized type")
		}
	}

	return &Root{
		fs:        ipfs.IpnsFs,
		Ipfs:      ipfs,
		IpfsRoot:  ipfspath,
		IpnsRoot:  ipnspath,
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

	// Local symlink to the node ID keyspace
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

	// other links go through ipns resolution and are symlinked into the ipfs mountpoint
	resolved, err := s.Ipfs.Namesys.Resolve(s.Ipfs.Context(), name)
	if err != nil {
		log.Warningf("ipns: namesys resolve error: %s", err)
		return nil, fuse.ENOENT
	}

	segments := resolved.Segments()
	if segments[0] == "ipfs" {
		p := strings.Join(resolved.Segments()[1:], "/")
		return &Link{s.IpfsRoot + "/" + p}, nil
	} else if segments[0] == "ipns" {
		p := strings.Join(resolved.Segments()[1:], "/")
		return &Link{s.IpnsRoot + "/" + p}, nil
	} else {
		log.Error("Invalid path.Path: ", resolved)
		return nil, errors.New("invalid path from ipns record")
	}
}

func (r *Root) Close() error {
	for _, kr := range r.Roots {
		err := kr.Publish(r.Ipfs.Context())
		if err != nil {
			return err
		}
	}
	return nil
}

// Forget is called when the filesystem is unmounted. probably.
// see comments here: http://godoc.org/bazil.org/fuse/fs#FSDestroyer
func (r *Root) Forget() {
	err := r.Close()
	if err != nil {
		log.Error(err)
	}
}

// ReadDirAll reads a particular directory. Will show locally available keys
// as well as a symlink to the peerID key
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

// Directory is wrapper over an ipnsfs directory to satisfy the fuse fs interface
type Directory struct {
	dir *nsfs.Directory

	fs.NodeRef
}

// File is wrapper over an ipnsfs file to satisfy the fuse fs interface
type File struct {
	fi *nsfs.File

	fs.NodeRef
}

// Attr returns the attributes of a given node.
func (d *Directory) Attr() fuse.Attr {
	log.Debug("Directory Attr")
	return fuse.Attr{
		Mode: os.ModeDir | 0555,
		Uid:  uint32(os.Getuid()),
		Gid:  uint32(os.Getgid()),
	}
}

// Attr returns the attributes of a given node.
func (fi *File) Attr() fuse.Attr {
	log.Debug("File Attr")
	size, err := fi.fi.Size()
	if err != nil {
		// In this case, the dag node in question may not be unixfs
		log.Critical("Failed to get file size: %s", err)
	}
	return fuse.Attr{
		Mode: os.FileMode(0666),
		Size: uint64(size),
		Uid:  uint32(os.Getuid()),
		Gid:  uint32(os.Getgid()),
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
		return &Directory{dir: child}, nil
	case *nsfs.File:
		return &File{fi: child}, nil
	default:
		// NB: if this happens, we do not want to continue, unpredictable behaviour
		// may occur.
		panic("invalid type found under directory. programmer error.")
	}
}

// ReadDirAll reads the link structure as directory entries
func (dir *Directory) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	for _, name := range dir.dir.List() {
		dirent := fuse.Dirent{Name: name}

		// TODO: make dir.dir.List() return dirinfos
		child, err := dir.dir.Child(name)
		if err != nil {
			return nil, err
		}

		switch child.Type() {
		case nsfs.TDir:
			dirent.Type = fuse.DT_Dir
		case nsfs.TFile:
			dirent.Type = fuse.DT_File
		}

		entries = append(entries, dirent)
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	readsize := min(req.Size, int(fisize-req.Offset))
	n, err := fi.fi.CtxReadFull(ctx, resp.Data[:readsize])
	resp.Data = resp.Data[:n]
	return err
}

func (fi *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// TODO: at some point, ensure that WriteAt here respects the context
	wrote, err := fi.fi.WriteAt(req.Data, req.Offset)
	if err != nil {
		return err
	}
	resp.Size = wrote
	return nil
}

func (fi *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	errs := make(chan error, 1)
	go func() {
		errs <- fi.fi.Close()
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (fi *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	cursize, err := fi.fi.Size()
	if err != nil {
		return err
	}
	if cursize != int64(req.Size) {
		err := fi.fi.Truncate(int64(req.Size))
		if err != nil {
			return err
		}
	}
	return nil
}

// Fsync flushes the content in the file to disk, but does not
// update the dag tree internally
func (fi *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	errs := make(chan error, 1)
	go func() {
		errs <- fi.fi.Sync()
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (fi *File) Forget() {
	err := fi.fi.Sync()
	if err != nil {
		log.Debug("Forget file error: ", err)
	}
}

func (dir *Directory) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	child, err := dir.dir.Mkdir(req.Name)
	if err != nil {
		return nil, err
	}

	return &Directory{dir: child}, nil
}

func (fi *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if req.Flags&fuse.OpenTruncate != 0 {
		log.Info("Need to truncate file!")
		err := fi.fi.Truncate(0)
		if err != nil {
			return nil, err
		}
	} else if req.Flags&fuse.OpenAppend != 0 {
		log.Info("Need to append to file!")

		// seek(0) essentially resets the file object, this is required for appends to work
		// properly
		_, err := fi.fi.Seek(0, os.SEEK_SET)
		if err != nil {
			log.Error("seek reset failed: ", err)
			return nil, err
		}
	}
	return fi, nil
}

func (fi *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fi.fi.Close()
}

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

	fi, ok := child.(*nsfs.File)
	if !ok {
		return nil, nil, errors.New("child creation failed")
	}

	nodechild := &File{fi: fi}
	return nodechild, nodechild, nil
}

func (dir *Directory) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	err := dir.dir.Unlink(req.Name)
	if err != nil {
		return fuse.ENOENT
	}
	return nil
}

// Rename implements NodeRenamer
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	fs.NodeMkdirer
	fs.NodeRemover
	fs.NodeRenamer
	fs.NodeStringLookuper
}

var _ ipnsDirectory = (*Directory)(nil)

type ipnsFile interface {
	fs.HandleFlusher
	fs.HandleReader
	fs.HandleWriter
	fs.HandleReleaser
	fs.Node
	fs.NodeFsyncer
	fs.NodeOpener
}

var _ ipnsFile = (*File)(nil)
