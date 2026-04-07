//go:build (linux || darwin || freebsd) && !nofuse

// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/boxo/path"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	mfs "github.com/ipfs/boxo/mfs"
	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	iface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/internal/fusemount"
)

var log = logging.Logger("fuse/ipns")

// Root is the root object of the /ipns filesystem tree.
type Root struct {
	fs.Inode
	Ipfs iface.CoreAPI
	Keys map[string]iface.Key

	// Used for symlinking into ipfs
	IpfsRoot  string
	IpnsRoot  string
	LocalDirs map[string]*Directory
	Roots     map[string]*mfs.Root

	LocalLinks map[string]*Link

	// Write-side config (reads are always on).
	storeMtime bool
	storeMode  bool
}

func ipnsPubFunc(ipfs iface.CoreAPI, key iface.Key) mfs.PubFunc {
	return func(ctx context.Context, c cid.Cid) error {
		// Bypass the "cannot publish while IPNS is mounted" guard.
		// Without this the mount's own publishes are blocked,
		// causing silent data loss on daemon restart (issue #2168).
		ctx = fusemount.ContextWithPublish(ctx)
		_, err := ipfs.Name().Publish(ctx, path.FromCid(c), options.Name.Key(key.Name()), options.Name.AllowOffline(true))
		return err
	}
}

func loadRoot(ctx context.Context, ipfs iface.CoreAPI, key iface.Key) (*mfs.Root, *Directory, error) {
	node, err := ipfs.ResolveNode(ctx, key.Path())
	switch err {
	case nil:
	case namesys.ErrResolveFailed:
		node = ft.EmptyDirNode()
	default:
		log.Errorf("looking up %s: %s", key.Path(), err)
		return nil, nil, err
	}

	pbnode, ok := node.(*dag.ProtoNode)
	if !ok {
		return nil, nil, dag.ErrNotProtobuf
	}

	root, err := mfs.NewRoot(ctx, ipfs.Dag(), pbnode, ipnsPubFunc(ipfs, key), nil)
	if err != nil {
		return nil, nil, err
	}

	return root, &Directory{dir: root.GetDirectory()}, nil
}

func CreateRoot(ctx context.Context, ipfs iface.CoreAPI, keys map[string]iface.Key, ipfspath, ipnspath string, cfg config.Mounts) (*Root, error) {
	ldirs := make(map[string]*Directory)
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

		links[alias] = &Link{
			Target: name,
		}
	}

	r := &Root{
		Ipfs:       ipfs,
		IpfsRoot:   ipfspath,
		IpnsRoot:   ipnspath,
		Keys:       keys,
		LocalDirs:  ldirs,
		LocalLinks: links,
		Roots:      roots,
		storeMtime: cfg.StoreMtime.WithDefault(config.DefaultStoreMtime),
		storeMode:  cfg.StoreMode.WithDefault(config.DefaultStoreMode),
	}

	// Wire back-pointer so all directories can access config.
	for _, d := range r.LocalDirs {
		d.root = r
	}

	return r, nil
}

// Getattr returns the root directory attributes.
// Timeout is 0 (no kernel caching) because IPNS records are mutable.
// TODO: set out.SetTimeout() from the IPNS record TTL (capped at
// Ipns.MaxCacheTTL) so the kernel can cache attrs for the record's
// remaining validity period instead of re-asking on every stat.
func (r *Root) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Root Attr")
	out.Attr.Mode = uint32(fusemnt.NamespaceRootMode.Perm())
	return 0
}

func (r *Root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch name {
	case "mach_kernel", ".hidden", "._.":
		return nil, syscall.ENOENT
	}

	if lnk, ok := r.LocalLinks[name]; ok {
		return r.NewInode(ctx, lnk, fs.StableAttr{Mode: syscall.S_IFLNK}), 0
	}

	if dir, ok := r.LocalDirs[name]; ok {
		return r.NewInode(ctx, dir, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
	}

	// Other links go through IPNS resolution and are symlinked into the /ipfs mount.
	ipnsName := "/ipns/" + name
	resolved, err := r.Ipfs.Name().Resolve(ctx, ipnsName)
	if err != nil {
		log.Warnf("ipns: namesys resolve error: %s", err)
		return nil, syscall.ENOENT
	}

	if resolved.Namespace() != path.IPFSNamespace {
		return nil, syscall.ENOENT
	}

	lnk := &Link{Target: r.IpfsRoot + "/" + strings.TrimPrefix(resolved.String(), "/ipfs/")}
	return r.NewInode(ctx, lnk, fs.StableAttr{Mode: syscall.S_IFLNK}), 0
}

func (r *Root) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Root ReadDirAll")

	entries := make([]fuse.DirEntry, 0, len(r.Keys)*2)
	for alias, k := range r.Keys {
		entries = append(entries,
			fuse.DirEntry{Name: k.ID().String(), Mode: syscall.S_IFDIR},
			fuse.DirEntry{Name: alias, Mode: syscall.S_IFLNK},
		)
	}
	return fs.NewListDirStream(entries), 0
}

func (r *Root) Close() error {
	for _, mr := range r.Roots {
		if err := mr.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Directory is wrapper over an mfs directory to satisfy the fuse fs interface.
type Directory struct {
	fs.Inode
	dir  *mfs.Directory
	root *Root
}

type FileNode struct {
	fs.Inode
	fi   *mfs.File
	root *Root
}

// File is wrapper over an mfs file descriptor.
// All methods are serialized by mu because the FUSE server dispatches
// each request in its own goroutine and the underlying DagModifier
// is not safe for concurrent use.
type File struct {
	inode  *fs.Inode // back-pointer for kernel cache invalidation
	fi     mfs.FileDescriptor
	mu     sync.Mutex
	append bool // O_APPEND: writes always go to end of file
}

func (d *Directory) fillAttr(a *fuse.Attr) {
	a.Mode = uint32(fusemnt.DefaultDirModeRW.Perm())
	if m, err := d.dir.Mode(); err == nil && m != 0 {
		a.Mode = uint32(m) & 07777
	}
	if t, err := d.dir.ModTime(); err == nil && !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (d *Directory) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Directory Attr")
	d.fillAttr(&out.Attr)
	return 0
}

func (d *Directory) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data := []byte(fusemnt.XattrCID + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (d *Directory) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if attr != fusemnt.XattrCID {
		return 0, fs.ENOATTR
	}
	nd, err := d.dir.GetNode()
	if err != nil {
		return 0, fs.ToErrno(err)
	}
	data := []byte(nd.Cid().String())
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (fi *FileNode) fillAttr(a *fuse.Attr) {
	a.Mode = uint32(fusemnt.DefaultFileModeRW.Perm())
	if sz, err := fi.fi.Size(); err == nil {
		a.Size = uint64(sz)
	}
	if m, err := fi.fi.Mode(); err == nil && m != 0 {
		a.Mode = uint32(m) & 07777
	}
	if t, err := fi.fi.ModTime(); err == nil && !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (fi *FileNode) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("File Attr")
	fi.fillAttr(&out.Attr)
	return 0
}

func (fi *FileNode) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data := []byte(fusemnt.XattrCID + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (fi *FileNode) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if attr != fusemnt.XattrCID {
		return 0, fs.ENOATTR
	}
	nd, err := fi.fi.GetNode()
	if err != nil {
		return 0, fs.ToErrno(err)
	}
	data := []byte(nd.Cid().String())
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (d *Directory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, err := d.dir.Child(name)
	if err != nil {
		return nil, syscall.ENOENT
	}

	switch child := child.(type) {
	case *mfs.Directory:
		dirNode := &Directory{dir: child, root: d.root}
		dirNode.fillAttr(&out.Attr)
		return d.NewInode(ctx, dirNode, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
	case *mfs.File:
		fileNode := &FileNode{fi: child, root: d.root}
		fileNode.fillAttr(&out.Attr)
		return d.NewInode(ctx, fileNode, fs.StableAttr{}), 0
	default:
		panic("invalid type found under directory. programmer error.")
	}
}

func (d *Directory) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	listing, err := d.dir.List(ctx)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	entries := make([]fuse.DirEntry, len(listing))
	for i, entry := range listing {
		var mode uint32
		if mfs.NodeType(entry.Type) == mfs.TDir {
			mode = syscall.S_IFDIR
		}
		entries[i] = fuse.DirEntry{Name: entry.Name, Mode: mode}
	}
	return fs.NewListDirStream(entries), 0
}

func (f *File) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, err := f.fi.Seek(off, io.SeekStart); err != nil {
		return nil, fs.ToErrno(err)
	}

	fisize, err := f.fi.Size()
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	readsize := min(len(dest), int(fisize-off))
	if readsize <= 0 {
		return fuse.ReadResultData(nil), 0
	}
	n, err := f.fi.CtxReadFull(ctx, dest[:readsize])
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return fuse.ReadResultData(dest[:n]), 0
}

func (f *File) Write(_ context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.append {
		// O_APPEND: the kernel may send offset 0, but POSIX says
		// writes must go to the end of the file.
		if _, err := f.fi.Seek(0, io.SeekEnd); err != nil {
			return 0, fs.ToErrno(err)
		}
		n, err := f.fi.Write(data)
		if err != nil {
			return 0, fs.ToErrno(err)
		}
		return uint32(n), 0
	}

	wrote, err := f.fi.WriteAt(data, off)
	if err != nil {
		return 0, fs.ToErrno(err)
	}
	return uint32(wrote), 0
}

// Flush persists buffered writes to the DAG. We intentionally ignore ctx
// here: the underlying MFS flush cannot be safely canceled mid-operation,
// and abandoning it would leak a background goroutine that races with the
// subsequent Release call on the same file descriptor.
// Flush persists buffered writes to the DAG and tells the kernel
// to drop cached attrs so the next stat sees the updated size.
func (f *File) Flush(_ context.Context) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fi.Flush(); err != nil {
		return fs.ToErrno(err)
	}
	if f.inode != nil {
		_ = f.inode.NotifyContent(0, 0)
	}
	return 0
}

// Setattr handles chmod, mtime changes (touch), and ftruncate.
//
// With hanwen/go-fuse, the kernel passes the open file handle (fh) when
// the caller uses ftruncate(fd, size). This lets us truncate through
// the existing write descriptor without opening a second one.
func (fi *FileNode) Setattr(_ context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok {
		if f, ok := fh.(*File); ok {
			// ftruncate(fd, size): use the existing write descriptor.
			f.mu.Lock()
			err := f.fi.Truncate(int64(sz))
			f.mu.Unlock()
			if err != nil {
				return fs.ToErrno(err)
			}
		} else {
			// truncate(path, size) without an open handle.
			cursize, err := fi.fi.Size()
			if err != nil {
				return fs.ToErrno(err)
			}
			if cursize != int64(sz) {
				return syscall.ENOTSUP
			}
		}
	}
	if mode, ok := in.GetMode(); ok && fi.root.storeMode {
		if err := fi.fi.SetMode(os.FileMode(mode) & os.ModePerm); err != nil {
			return fs.ToErrno(err)
		}
	}
	if mtime, ok := in.GetMTime(); ok && fi.root.storeMtime {
		if err := fi.fi.SetModTime(mtime); err != nil {
			return fs.ToErrno(err)
		}
	}
	return 0
}

// Fsync flushes the write buffer through the open file descriptor.
// This was previously a no-op with bazil.org/fuse because Fsync was
// dispatched to the inode, which couldn't reach the open descriptor.
// hanwen/go-fuse dispatches FileFsyncer to the handle directly.
func (f *File) Fsync(_ context.Context, _ uint32) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	return fs.ToErrno(f.fi.Flush())
}

func (d *Directory) Mkdir(ctx context.Context, name string, _ uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	child, err := d.dir.Mkdir(name)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	out.Attr.Mode = uint32(fusemnt.DefaultDirModeRW.Perm())
	return d.NewInode(ctx, &Directory{dir: child, root: d.root}, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
}

func (fi *FileNode) Open(_ context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	accessMode := flags & syscall.O_ACCMODE
	mfsFlags := mfs.Flags{
		Read:  accessMode == syscall.O_RDONLY || accessMode == syscall.O_RDWR,
		Write: accessMode == syscall.O_WRONLY || accessMode == syscall.O_RDWR,
		Sync:  true,
	}
	fd, err := fi.fi.Open(mfsFlags)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	if flags&syscall.O_TRUNC != 0 {
		if !mfsFlags.Write {
			fd.Close()
			log.Error("tried to open a readonly file with truncate")
			return nil, 0, syscall.EINVAL
		}
		if err := fd.Truncate(0); err != nil {
			fd.Close()
			return nil, 0, fs.ToErrno(err)
		}
	}
	// O_APPEND is handled in File.Write by seeking to end.

	if mfsFlags.Write && fi.root.storeMtime {
		if err := fi.fi.SetModTime(time.Now()); err != nil {
			fd.Close()
			return nil, 0, fs.ToErrno(err)
		}
	}

	return &File{inode: fi.EmbeddedInode(), fi: fd, append: flags&syscall.O_APPEND != 0}, 0, 0
}

func (f *File) Release(_ context.Context) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	return fs.ToErrno(f.fi.Close())
}

func (d *Directory) Create(ctx context.Context, name string, flags uint32, _ uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	nd := dag.NodeWithData(ft.FilePBData(nil, 0))
	if err := nd.SetCidBuilder(d.dir.GetCidBuilder()); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	if err := d.dir.AddChild(name, nd); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	if err := d.dir.Flush(); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	child, err := d.dir.Child(name)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	fi, ok := child.(*mfs.File)
	if !ok {
		return nil, nil, 0, syscall.EIO
	}

	if d.root.storeMtime {
		if err := fi.SetModTime(time.Now()); err != nil {
			return nil, nil, 0, fs.ToErrno(err)
		}
	}

	fileNode := &FileNode{fi: fi, root: d.root}

	accessMode := flags & syscall.O_ACCMODE
	fd, err := fi.Open(mfs.Flags{
		Read:  accessMode == syscall.O_RDONLY || accessMode == syscall.O_RDWR,
		Write: accessMode == syscall.O_WRONLY || accessMode == syscall.O_RDWR,
		Sync:  true,
	})
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	inode := d.NewInode(ctx, fileNode, fs.StableAttr{})
	return inode, &File{inode: inode, fi: fd}, 0, 0
}

func (d *Directory) Unlink(_ context.Context, name string) syscall.Errno {
	if err := d.dir.Unlink(name); err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(d.dir.Flush())
}

func (d *Directory) Rmdir(ctx context.Context, name string) syscall.Errno {
	child, err := d.dir.Child(name)
	if err != nil {
		return syscall.ENOENT
	}
	dir, ok := child.(*mfs.Directory)
	if !ok {
		return syscall.ENOTDIR
	}
	entries, err := dir.ListNames(ctx)
	if err != nil {
		return fs.ToErrno(err)
	}
	if len(entries) > 0 {
		return syscall.ENOTEMPTY
	}

	if err := d.dir.Unlink(name); err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(d.dir.Flush())
}

func (d *Directory) Rename(_ context.Context, oldName string, newParent fs.InodeEmbedder, newName string, _ uint32) syscall.Errno {
	cur, err := d.dir.Child(oldName)
	if err != nil {
		return fs.ToErrno(err)
	}

	nd, err := cur.GetNode()
	if err != nil {
		return fs.ToErrno(err)
	}

	// Unlink the source before adding to the destination. For
	// same-directory renames, this clears the old name from the
	// directory's entry cache before AddChild repopulates it.
	if err := d.dir.Unlink(oldName); err != nil {
		return fs.ToErrno(err)
	}

	switch target := newParent.EmbeddedInode().Operations().(type) {
	case *Directory:
		if err := target.dir.AddChild(newName, nd); err != nil {
			return fs.ToErrno(err)
		}
	case *FileNode:
		log.Error("Cannot move node into a file!")
		return syscall.EPERM
	default:
		log.Error("Unknown node type for rename target dir!")
		return syscall.EIO
	}
	return fs.ToErrno(d.dir.Flush())
}

// Interface checks.
var (
	_ fs.NodeGetattrer = (*Root)(nil)
	_ fs.NodeLookuper  = (*Root)(nil)
	_ fs.NodeReaddirer = (*Root)(nil)

	_ fs.NodeGetattrer   = (*Directory)(nil)
	_ fs.NodeLookuper    = (*Directory)(nil)
	_ fs.NodeReaddirer   = (*Directory)(nil)
	_ fs.NodeCreater     = (*Directory)(nil)
	_ fs.NodeMkdirer     = (*Directory)(nil)
	_ fs.NodeUnlinker    = (*Directory)(nil)
	_ fs.NodeRmdirer     = (*Directory)(nil)
	_ fs.NodeRenamer     = (*Directory)(nil)
	_ fs.NodeGetxattrer  = (*Directory)(nil)
	_ fs.NodeListxattrer = (*Directory)(nil)

	_ fs.NodeGetattrer   = (*FileNode)(nil)
	_ fs.NodeOpener      = (*FileNode)(nil)
	_ fs.NodeSetattrer   = (*FileNode)(nil)
	_ fs.NodeGetxattrer  = (*FileNode)(nil)
	_ fs.NodeListxattrer = (*FileNode)(nil)

	_ fs.FileReader   = (*File)(nil)
	_ fs.FileWriter   = (*File)(nil)
	_ fs.FileFlusher  = (*File)(nil)
	_ fs.FileReleaser = (*File)(nil)
	_ fs.FileFsyncer  = (*File)(nil)
)
