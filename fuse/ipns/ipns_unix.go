// +build !nofuse

// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"errors"
	"fmt"
	"os"
	"strings"

	fuse "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"

	key "github.com/ipfs/go-ipfs/blocks/key"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	mfs "github.com/ipfs/go-ipfs/mfs"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
)

func init() {
	fuse.Debug = func(msg interface{}) { fmt.Println(msg) }
}

var log = eventlog.Logger("fuse/ipns")

// FileSystem is the readwrite IPNS Fuse Filesystem.
type FileSystem struct {
	Ipfs     *core.IpfsNode
	RootNode *Root
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewFileSystem(ipfs *core.IpfsNode, sk ci.PrivKey, ipfspath, ipnspath string) (*FileSystem, error) {

	kmap := map[string]ci.PrivKey{
		"local": sk,
	}
	root, err := CreateRoot(ipfs, kmap, ipfspath, ipnspath)
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
	Keys map[string]ci.PrivKey

	// Used for symlinking into ipfs
	IpfsRoot  string
	IpnsRoot  string
	LocalDirs map[string]fs.Node
	Roots     map[string]*keyRoot

	fs         *mfs.Filesystem
	LocalLinks map[string]*Link
}

func ipnsPubFunc(ipfs *core.IpfsNode, k ci.PrivKey) mfs.PubFunc {
	return func(ctx context.Context, key key.Key) error {
		return ipfs.Namesys.Publish(ctx, k, path.FromKey(key))
	}
}

func (r *Root) loadRoot(ctx context.Context, name string) (*mfs.Root, error) {

	rt, ok := r.Roots[name]
	if !ok {
		return nil, fmt.Errorf("no key by name '%s'", name)
	}

	// already loaded
	if rt.root != nil {
		return rt.root, nil
	}

	p, err := path.ParsePath("/ipns/" + name)
	if err != nil {
		log.Errorf("mkpath %s: %s", name, err)
		return nil, err
	}

	node, err := core.Resolve(ctx, r.Ipfs, p)
	if err != nil {
		log.Errorf("looking up %s: %s", p, err)
		return nil, err
	}

	// load it lazily
	root, err := r.Ipfs.Mfs.NewRoot(rt.alias, node, ipnsPubFunc(r.Ipfs, rt.k))
	if err != nil {
		return nil, err
	}

	rt.root = root

	switch val := root.GetValue().(type) {
	case *mfs.Directory:
		r.LocalDirs[name] = &Directory{dir: val}
	case *mfs.File:
		r.LocalDirs[name] = &File{fi: val}
	default:
		return nil, errors.New("unrecognized type")
	}

	return root, nil
}

type keyRoot struct {
	k     ci.PrivKey
	alias string
	root  *mfs.Root
}

func CreateRoot(ipfs *core.IpfsNode, keys map[string]ci.PrivKey, ipfspath, ipnspath string) (*Root, error) {
	ldirs := make(map[string]fs.Node)
	roots := make(map[string]*keyRoot)
	links := make(map[string]*Link)
	for alias, k := range keys {
		pkh, err := k.GetPublic().Hash()
		if err != nil {
			return nil, err
		}
		name := key.Key(pkh).B58String()

		roots[name] = &keyRoot{
			k:     k,
			alias: alias,
		}

		links[alias] = &Link{
			Target: name,
		}

	}

	return &Root{
		fs:         ipfs.Mfs,
		Ipfs:       ipfs,
		IpfsRoot:   ipfspath,
		IpnsRoot:   ipnspath,
		Keys:       keys,
		LocalDirs:  ldirs,
		LocalLinks: links,
		Roots:      roots,
	}, nil
}

// Attr returns file attributes.
func (*Root) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Root Attr")
	*a = fuse.Attr{Mode: os.ModeDir | 0111} // -rw+x
	return nil
}

// Lookup performs a lookup under this node.
func (s *Root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, fuse.ENOENT
	}

	if lnk, ok := s.LocalLinks[name]; ok {
		return lnk, nil
	}

	nd, ok := s.LocalDirs[name]
	if !ok {
		// see if we have an unloaded root by this name
		_, ok = s.Roots[name]
		if ok {
			_, err := s.loadRoot(ctx, name)
			if err != nil {
				log.Error("error loading key root: ", err)
				return nil, err
			}

			nd = s.LocalDirs[name]
		}
		// fallthrough to logic after 'if ok { switch'
	}
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
	}

	log.Error("Invalid path.Path: ", resolved)
	return nil, errors.New("invalid path from ipns record")
}

func (r *Root) Close() error {
	return r.fs.Close()
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

	var listing []fuse.Dirent
	for alias, k := range r.Keys {
		pub := k.GetPublic()
		hash, err := pub.Hash()
		if err != nil {
			continue
		}
		ent := fuse.Dirent{
			Name: key.Key(hash).Pretty(),
			Type: fuse.DT_Dir,
		}
		link := fuse.Dirent{
			Name: alias,
			Type: fuse.DT_Link,
		}
		listing = append(listing, ent, link)
	}
	return listing, nil
}

// Directory is wrapper over an mfs directory to satisfy the fuse fs interface
type Directory struct {
	dir *mfs.Directory

	fs.NodeRef
}

// File is wrapper over an mfs file to satisfy the fuse fs interface
type File struct {
	fi *mfs.File

	fs.NodeRef
}

// Attr returns the attributes of a given node.
func (d *Directory) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Directory Attr")
	*a = fuse.Attr{
		Mode: os.ModeDir | 0555,
		Uid:  uint32(os.Getuid()),
		Gid:  uint32(os.Getgid()),
	}
	return nil
}

// Attr returns the attributes of a given node.
func (fi *File) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("File Attr")
	size, err := fi.fi.Size()
	if err != nil {
		// In this case, the dag node in question may not be unixfs
		return fmt.Errorf("fuse/ipns: failed to get file.Size(): %s", err)
	}
	*a = fuse.Attr{
		Mode: os.FileMode(0666),
		Size: uint64(size),
		Uid:  uint32(os.Getuid()),
		Gid:  uint32(os.Getgid()),
	}
	return nil
}

// Lookup performs a lookup under this node.
func (s *Directory) Lookup(ctx context.Context, name string) (fs.Node, error) {
	child, err := s.dir.Child(name)
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	switch child := child.(type) {
	case *mfs.Directory:
		return &Directory{dir: child}, nil
	case *mfs.File:
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
	listing, err := dir.dir.List()
	if err != nil {
		return nil, err
	}
	for _, entry := range listing {
		dirent := fuse.Dirent{Name: entry.Name}

		switch mfs.NodeType(entry.Type) {
		case mfs.TDir:
			dirent.Type = fuse.DT_Dir
		case mfs.TFile:
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

	fi, ok := child.(*mfs.File)
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
		log.Error("Cannot move node into a file!")
		return fuse.EPERM
	default:
		log.Error("Unknown node type for rename target dir!")
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
