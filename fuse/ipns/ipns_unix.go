//go:build !nofuse && !openbsd && !netbsd && !plan9
// +build !nofuse,!openbsd,!netbsd,!plan9

// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/path"

	fuse "bazil.org/fuse"
	fs "bazil.org/fuse/fs"
	mfs "github.com/ipfs/boxo/mfs"
	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	iface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

func init() {
	if os.Getenv("IPFS_FUSE_DEBUG") != "" {
		fuse.Debug = func(msg interface{}) {
			fmt.Println(msg)
		}
	}
}

var log = logging.Logger("fuse/ipns")

// FileSystem is the readwrite IPNS Fuse Filesystem.
type FileSystem struct {
	Ipfs     iface.CoreAPI
	RootNode *Root
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewFileSystem(ctx context.Context, ipfs iface.CoreAPI, ipfspath, ipnspath string) (*FileSystem, error) {
	key, err := ipfs.Key().Self(ctx)
	if err != nil {
		return nil, err
	}
	root, err := CreateRoot(ctx, ipfs, map[string]iface.Key{"local": key}, ipfspath, ipnspath)
	if err != nil {
		return nil, err
	}

	return &FileSystem{Ipfs: ipfs, RootNode: root}, nil
}

// Root constructs the Root of the filesystem, a Root object.
func (f *FileSystem) Root() (fs.Node, error) {
	log.Debug("filesystem, get root")
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
	Ipfs iface.CoreAPI
	Keys map[string]iface.Key

	// Used for symlinking into ipfs
	IpfsRoot  string
	IpnsRoot  string
	LocalDirs map[string]fs.Node
	Roots     map[string]*mfs.Root

	LocalLinks map[string]*Link
}

func ipnsPubFunc(ipfs iface.CoreAPI, key iface.Key) mfs.PubFunc {
	return func(ctx context.Context, c cid.Cid) error {
		_, err := ipfs.Name().Publish(ctx, path.FromCid(c), options.Name.Key(key.Name()))
		return err
	}
}

func loadRoot(ctx context.Context, ipfs iface.CoreAPI, key iface.Key) (*mfs.Root, fs.Node, error) {
	node, err := ipfs.ResolveNode(ctx, key.Path())
	switch err {
	case nil:
	case iface.ErrResolveFailed:
		node = ft.EmptyDirNode()
	default:
		log.Errorf("looking up %s: %s", key.Path(), err)
		return nil, nil, err
	}

	pbnode, ok := node.(*dag.ProtoNode)
	if !ok {
		return nil, nil, dag.ErrNotProtobuf
	}

	root, err := mfs.NewRoot(ctx, ipfs.Dag(), pbnode, ipnsPubFunc(ipfs, key))
	if err != nil {
		return nil, nil, err
	}

	return root, &Directory{dir: root.GetDirectory()}, nil
}

func CreateRoot(ctx context.Context, ipfs iface.CoreAPI, keys map[string]iface.Key, ipfspath, ipnspath string) (*Root, error) {
	ldirs := make(map[string]fs.Node)
	roots := make(map[string]*mfs.Root)
	links := make(map[string]*Link)
	for alias, k := range keys {
		root, fsn, err := loadRoot(ctx, ipfs, k)
		if err != nil {
			return nil, err
		}

		name := k.ID().String()

		roots[name] = root
		ldirs[name] = fsn

		// set up alias symlink
		links[alias] = &Link{
			Target: name,
		}
	}

	return &Root{
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
func (r *Root) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Root Attr")
	a.Mode = os.ModeDir | 0o111 // -rw+x
	return nil
}

// Lookup performs a lookup under this node.
func (r *Root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, syscall.Errno(syscall.ENOENT)
	}

	if lnk, ok := r.LocalLinks[name]; ok {
		return lnk, nil
	}

	nd, ok := r.LocalDirs[name]
	if ok {
		switch nd := nd.(type) {
		case *Directory:
			return nd, nil
		case *FileNode:
			return nd, nil
		default:
			return nil, syscall.Errno(syscall.EIO)
		}
	}

	// other links go through ipns resolution and are symlinked into the ipfs mountpoint
	ipnsName := "/ipns/" + name
	resolved, err := r.Ipfs.Name().Resolve(ctx, ipnsName)
	if err != nil {
		log.Warnf("ipns: namesys resolve error: %s", err)
		return nil, syscall.Errno(syscall.ENOENT)
	}

	if resolved.Namespace() != path.IPFSNamespace {
		return nil, errors.New("invalid path from ipns record")
	}

	return &Link{r.IpfsRoot + "/" + strings.TrimPrefix(resolved.String(), "/ipfs/")}, nil
}

func (r *Root) Close() error {
	for _, mr := range r.Roots {
		err := mr.Close()
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
// as well as a symlink to the peerID key.
func (r *Root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("Root ReadDirAll")

	listing := make([]fuse.Dirent, 0, len(r.Keys)*2)
	for alias, k := range r.Keys {
		ent := fuse.Dirent{
			Name: k.ID().String(),
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

// Directory is wrapper over an mfs directory to satisfy the fuse fs interface.
type Directory struct {
	dir *mfs.Directory
}

type FileNode struct {
	fi *mfs.File
}

// File is wrapper over an mfs file to satisfy the fuse fs interface.
type File struct {
	fi mfs.FileDescriptor
}

// Attr returns the attributes of a given node.
func (d *Directory) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Directory Attr")
	a.Mode = os.ModeDir | 0o555
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getgid())
	return nil
}

// Attr returns the attributes of a given node.
func (fi *FileNode) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("File Attr")
	size, err := fi.fi.Size()
	if err != nil {
		// In this case, the dag node in question may not be unixfs
		return fmt.Errorf("fuse/ipns: failed to get file.Size(): %s", err)
	}
	a.Mode = os.FileMode(0o666)
	a.Size = uint64(size)
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getgid())
	return nil
}

// Lookup performs a lookup under this node.
func (d *Directory) Lookup(ctx context.Context, name string) (fs.Node, error) {
	child, err := d.dir.Child(name)
	if err != nil {
		// todo: make this error more versatile.
		return nil, syscall.Errno(syscall.ENOENT)
	}

	switch child := child.(type) {
	case *mfs.Directory:
		return &Directory{dir: child}, nil
	case *mfs.File:
		return &FileNode{fi: child}, nil
	default:
		// NB: if this happens, we do not want to continue, unpredictable behaviour
		// may occur.
		panic("invalid type found under directory. programmer error.")
	}
}

// ReadDirAll reads the link structure as directory entries.
func (d *Directory) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	listing, err := d.dir.List(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]fuse.Dirent, len(listing))
	for i, entry := range listing {
		dirent := fuse.Dirent{Name: entry.Name}

		switch mfs.NodeType(entry.Type) {
		case mfs.TDir:
			dirent.Type = fuse.DT_Dir
		case mfs.TFile:
			dirent.Type = fuse.DT_File
		}

		entries[i] = dirent
	}

	if len(entries) > 0 {
		return entries, nil
	}
	return nil, syscall.Errno(syscall.ENOENT)
}

func (fi *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	_, err := fi.fi.Seek(req.Offset, io.SeekStart)
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
		errs <- fi.fi.Flush()
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (fi *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if req.Valid.Size() {
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
	}
	return nil
}

// Fsync flushes the content in the file to disk.
func (fi *FileNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	// This needs to perform a *full* flush because, in MFS, a write isn't
	// persisted until the root is updated.
	errs := make(chan error, 1)
	go func() {
		errs <- fi.fi.Flush()
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (fi *File) Forget() {
	// TODO(steb): this seems like a place where we should be *uncaching*, not flushing.
	err := fi.fi.Flush()
	if err != nil {
		log.Debug("forget file error: ", err)
	}
}

func (d *Directory) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	child, err := d.dir.Mkdir(req.Name)
	if err != nil {
		return nil, err
	}

	return &Directory{dir: child}, nil
}

func (fi *FileNode) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	fd, err := fi.fi.Open(mfs.Flags{
		Read:  req.Flags.IsReadOnly() || req.Flags.IsReadWrite(),
		Write: req.Flags.IsWriteOnly() || req.Flags.IsReadWrite(),
		Sync:  true,
	})
	if err != nil {
		return nil, err
	}

	if req.Flags&fuse.OpenTruncate != 0 {
		if req.Flags.IsReadOnly() {
			log.Error("tried to open a readonly file with truncate")
			return nil, syscall.Errno(syscall.ENOTSUP)
		}
		log.Info("Need to truncate file!")
		err := fd.Truncate(0)
		if err != nil {
			return nil, err
		}
	} else if req.Flags&fuse.OpenAppend != 0 {
		log.Info("Need to append to file!")
		if req.Flags.IsReadOnly() {
			log.Error("tried to open a readonly file with append")
			return nil, syscall.Errno(syscall.ENOTSUP)
		}

		_, err := fd.Seek(0, io.SeekEnd)
		if err != nil {
			log.Error("seek reset failed: ", err)
			return nil, err
		}
	}

	return &File{fi: fd}, nil
}

func (fi *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fi.fi.Close()
}

func (d *Directory) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	// New 'empty' file
	nd := dag.NodeWithData(ft.FilePBData(nil, 0))
	err := d.dir.AddChild(req.Name, nd)
	if err != nil {
		return nil, nil, err
	}

	child, err := d.dir.Child(req.Name)
	if err != nil {
		return nil, nil, err
	}

	fi, ok := child.(*mfs.File)
	if !ok {
		return nil, nil, errors.New("child creation failed")
	}

	nodechild := &FileNode{fi: fi}

	fd, err := fi.Open(mfs.Flags{
		Read:  req.Flags.IsReadOnly() || req.Flags.IsReadWrite(),
		Write: req.Flags.IsWriteOnly() || req.Flags.IsReadWrite(),
		Sync:  true,
	})
	if err != nil {
		return nil, nil, err
	}

	return nodechild, &File{fi: fd}, nil
}

func (d *Directory) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	err := d.dir.Unlink(req.Name)
	if err != nil {
		return syscall.Errno(syscall.ENOENT)
	}
	return nil
}

// Rename implements NodeRenamer.
func (d *Directory) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	cur, err := d.dir.Child(req.OldName)
	if err != nil {
		return err
	}

	err = d.dir.Unlink(req.OldName)
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
	case *FileNode:
		log.Error("Cannot move node into a file!")
		return syscall.Errno(syscall.EPERM)
	default:
		log.Error("Unknown node type for rename target dir!")
		return errors.New("unknown fs node type")
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// to check that out Node implements all the interfaces we want.
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
}

type ipnsFileNode interface {
	fs.Node
	fs.NodeFsyncer
	fs.NodeOpener
}

var (
	_ ipnsFileNode = (*FileNode)(nil)
	_ ipnsFile     = (*File)(nil)
)
