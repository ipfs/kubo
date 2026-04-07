//go:build (linux || darwin || freebsd) && !nofuse

package mfs

import (
	"context"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/mfs"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

var log = logging.Logger("fuse/mfs")

// fileSystemConfig holds write-side config (reads are always on).
type fileSystemConfig struct {
	storeMtime bool // persist mtime on create and open-for-write
	storeMode  bool // persist mode on chmod
}

// Dir is the FUSE adapter for MFS directories.
type Dir struct {
	fs.Inode
	mfsDir *mfs.Directory
	cfg    *fileSystemConfig
}

func (d *Dir) fillAttr(a *fuse.Attr) {
	a.Mode = uint32(fusemnt.DefaultDirModeRW.Perm())
	if m, err := d.mfsDir.Mode(); err == nil && m != 0 {
		a.Mode = uint32(m) & 07777
	}
	if t, err := d.mfsDir.ModTime(); err == nil && !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (d *Dir) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	d.fillAttr(&out.Attr)
	return 0
}

func (d *Dir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	mfsNode, err := d.mfsDir.Child(name)
	if err != nil {
		return nil, syscall.ENOENT
	}

	switch mfsNode.Type() {
	case mfs.TDir:
		child := &Dir{mfsDir: mfsNode.(*mfs.Directory), cfg: d.cfg}
		child.fillAttr(&out.Attr)
		return d.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
	case mfs.TFile:
		child := &FileInode{mfsFile: mfsNode.(*mfs.File), cfg: d.cfg}
		child.fillAttr(&out.Attr)
		return d.NewInode(ctx, child, fs.StableAttr{}), 0
	}

	return nil, syscall.ENOENT
}

func (d *Dir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	nodes, err := d.mfsDir.List(ctx)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	entries := make([]fuse.DirEntry, len(nodes))
	for i, node := range nodes {
		var mode uint32
		if node.Type == int(mfs.TDir) {
			mode = syscall.S_IFDIR
		}
		entries[i] = fuse.DirEntry{Name: node.Name, Mode: mode}
	}
	return fs.NewListDirStream(entries), 0
}

func (d *Dir) Mkdir(ctx context.Context, name string, _ uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	mfsDir, err := d.mfsDir.Mkdir(name)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	out.Attr.Mode = uint32(fusemnt.DefaultDirModeRW.Perm())
	child := &Dir{mfsDir: mfsDir, cfg: d.cfg}
	return d.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
}

func (d *Dir) Unlink(_ context.Context, name string) syscall.Errno {
	if err := d.mfsDir.Unlink(name); err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(d.mfsDir.Flush())
}

func (d *Dir) Rmdir(ctx context.Context, name string) syscall.Errno {
	child, err := d.mfsDir.Child(name)
	if err != nil {
		return fs.ToErrno(err)
	}
	target, ok := child.(*mfs.Directory)
	if !ok {
		return syscall.ENOTDIR
	}

	children, err := target.ListNames(ctx)
	if err != nil {
		return fs.ToErrno(err)
	}
	if len(children) > 0 {
		return syscall.ENOTEMPTY
	}

	if err := d.mfsDir.Unlink(name); err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(d.mfsDir.Flush())
}

func (d *Dir) Rename(_ context.Context, oldName string, newParent fs.InodeEmbedder, newName string, _ uint32) syscall.Errno {
	child, err := d.mfsDir.Child(oldName)
	if err != nil {
		return fs.ToErrno(err)
	}

	nd, err := child.GetNode()
	if err != nil {
		return fs.ToErrno(err)
	}

	// Unlink the source first. For same-directory renames, this clears
	// the old name from the directory's entry cache before AddChild
	// repopulates it with the new name. Without this ordering, Flush
	// would sync the stale cache entry back into the DAG.
	if err := d.mfsDir.Unlink(oldName); err != nil {
		return fs.ToErrno(err)
	}

	targetDir := newParent.EmbeddedInode().Operations().(*Dir)
	if err := targetDir.mfsDir.Unlink(newName); err != nil && err != os.ErrNotExist {
		return fs.ToErrno(err)
	}
	if err := targetDir.mfsDir.AddChild(newName, nd); err != nil {
		return fs.ToErrno(err)
	}

	return fs.ToErrno(d.mfsDir.Flush())
}

func (d *Dir) Create(ctx context.Context, name string, flags uint32, _ uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	node := dag.NodeWithData(ft.FilePBData(nil, 0))
	if err := node.SetCidBuilder(d.mfsDir.GetCidBuilder()); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	if err := d.mfsDir.AddChild(name, node); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	if err := d.mfsDir.Flush(); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	mfsNode, err := d.mfsDir.Child(name)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}
	if d.cfg.storeMtime {
		if err := mfsNode.SetModTime(time.Now()); err != nil {
			return nil, nil, 0, fs.ToErrno(err)
		}
	}

	mfsFile := mfsNode.(*mfs.File)
	fileInode := &FileInode{mfsFile: mfsFile, cfg: d.cfg}

	accessMode := flags & syscall.O_ACCMODE
	mfsFlags := mfs.Flags{
		Read:  accessMode == syscall.O_RDONLY || accessMode == syscall.O_RDWR,
		Write: accessMode == syscall.O_WRONLY || accessMode == syscall.O_RDWR,
		Sync:  true,
	}

	fd, err := mfsFile.Open(mfsFlags)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	inode := d.NewInode(ctx, fileInode, fs.StableAttr{})
	return inode, &FileHandle{inode: inode, mfsFD: fd}, 0, 0
}

func (d *Dir) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data := []byte(fusemnt.XattrCID + "\x00" + fusemnt.XattrCIDDeprecated + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (d *Dir) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	switch attr {
	case fusemnt.XattrCID, fusemnt.XattrCIDDeprecated:
		node, err := d.mfsDir.GetNode()
		if err != nil {
			return 0, fs.ToErrno(err)
		}
		data := []byte(node.Cid().String())
		if len(dest) == 0 {
			return uint32(len(data)), 0
		}
		if len(dest) < len(data) {
			return 0, syscall.ERANGE
		}
		return uint32(copy(dest, data)), 0
	default:
		return 0, fs.ENOATTR
	}
}

// FileInode is the FUSE adapter for MFS file inodes.
type FileInode struct {
	fs.Inode
	mfsFile *mfs.File
	cfg     *fileSystemConfig
}

func (fi *FileInode) fillAttr(a *fuse.Attr) {
	size, _ := fi.mfsFile.Size()
	a.Size = uint64(size)
	a.Mode = uint32(fusemnt.DefaultFileModeRW.Perm())
	if m, err := fi.mfsFile.Mode(); err == nil && m != 0 {
		a.Mode = uint32(m) & 07777
	}
	if t, _ := fi.mfsFile.ModTime(); !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (fi *FileInode) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	fi.fillAttr(&out.Attr)
	return 0
}

func (fi *FileInode) Open(_ context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	accessMode := flags & syscall.O_ACCMODE
	mfsFlags := mfs.Flags{
		Read:  accessMode == syscall.O_RDONLY || accessMode == syscall.O_RDWR,
		Write: accessMode == syscall.O_WRONLY || accessMode == syscall.O_RDWR,
		Sync:  true,
	}
	fd, err := fi.mfsFile.Open(mfsFlags)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}

	if flags&syscall.O_TRUNC != 0 {
		if !mfsFlags.Write {
			fd.Close()
			log.Error("tried to open a readonly file with truncate")
			return nil, 0, syscall.ENOTSUP
		}
		if err := fd.Truncate(0); err != nil {
			fd.Close()
			return nil, 0, fs.ToErrno(err)
		}
	}
	// O_APPEND is handled in FileHandle.Write by seeking to end.

	if mfsFlags.Write && fi.cfg.storeMtime {
		if err := fi.mfsFile.SetModTime(time.Now()); err != nil {
			fd.Close()
			return nil, 0, fs.ToErrno(err)
		}
	}

	return &FileHandle{inode: fi.EmbeddedInode(), mfsFD: fd, append: flags&syscall.O_APPEND != 0}, 0, 0
}

// Setattr handles chmod, mtime changes (touch), and ftruncate.
//
// With hanwen/go-fuse, the kernel passes the open file handle (fh) when
// the caller uses ftruncate(fd, size). This lets us truncate through
// the existing write descriptor without opening a second one.
func (fi *FileInode) Setattr(_ context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok {
		if f, ok := fh.(*FileHandle); ok {
			// ftruncate(fd, size): use the existing write descriptor.
			f.mu.Lock()
			err := f.mfsFD.Truncate(int64(sz))
			f.mu.Unlock()
			if err != nil {
				return fs.ToErrno(err)
			}
		} else {
			// truncate(path, size) without an open handle.
			cursize, err := fi.mfsFile.Size()
			if err != nil {
				return fs.ToErrno(err)
			}
			if cursize != int64(sz) {
				return syscall.ENOTSUP
			}
		}
	}
	if mode, ok := in.GetMode(); ok && fi.cfg.storeMode {
		if err := fi.mfsFile.SetMode(os.FileMode(mode) & os.ModePerm); err != nil {
			return fs.ToErrno(err)
		}
	}
	if mtime, ok := in.GetMTime(); ok && fi.cfg.storeMtime {
		if err := fi.mfsFile.SetModTime(mtime); err != nil {
			return fs.ToErrno(err)
		}
	}
	return 0
}

func (fi *FileInode) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data := []byte(fusemnt.XattrCID + "\x00" + fusemnt.XattrCIDDeprecated + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (fi *FileInode) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	switch attr {
	case fusemnt.XattrCID, fusemnt.XattrCIDDeprecated:
		node, err := fi.mfsFile.GetNode()
		if err != nil {
			return 0, fs.ToErrno(err)
		}
		data := []byte(node.Cid().String())
		if len(dest) == 0 {
			return uint32(len(data)), 0
		}
		if len(dest) < len(data) {
			return 0, syscall.ERANGE
		}
		return uint32(copy(dest, data)), 0
	default:
		return 0, fs.ENOATTR
	}
}

// FileHandle wraps an MFS file descriptor for FUSE operations.
// All methods are serialized by mu because the FUSE server dispatches
// each request in its own goroutine and the underlying DagModifier
// is not safe for concurrent use.
type FileHandle struct {
	inode  *fs.Inode // back-pointer for kernel cache invalidation
	mfsFD  mfs.FileDescriptor
	mu     sync.Mutex
	append bool // O_APPEND: writes always go to end of file
}

func (fh *FileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if _, err := fh.mfsFD.Seek(off, io.SeekStart); err != nil {
		return nil, fs.ToErrno(err)
	}

	n, err := fh.mfsFD.CtxReadFull(ctx, dest)
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		return fuse.ReadResultData(dest[:n]), 0
	default:
		return nil, fs.ToErrno(err)
	}
}

func (fh *FileHandle) Write(_ context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.append {
		// O_APPEND: the kernel may send offset 0, but POSIX says
		// writes must go to the end of the file.
		if _, err := fh.mfsFD.Seek(0, io.SeekEnd); err != nil {
			return 0, fs.ToErrno(err)
		}
		n, err := fh.mfsFD.Write(data)
		if err != nil {
			return 0, fs.ToErrno(err)
		}
		return uint32(n), 0
	}

	n, err := fh.mfsFD.WriteAt(data, off)
	if err != nil {
		return 0, fs.ToErrno(err)
	}
	return uint32(n), 0
}

// Flush persists buffered writes to the DAG and tells the kernel
// to drop cached attrs so the next stat sees the updated size.
func (fh *FileHandle) Flush(_ context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if err := fh.mfsFD.Flush(); err != nil {
		return fs.ToErrno(err)
	}
	// Invalidate the kernel's cached content and attrs for this inode
	// so readers opening the same path see the new data and size.
	if fh.inode != nil {
		_ = fh.inode.NotifyContent(0, 0)
	}
	return 0
}

func (fh *FileHandle) Release(_ context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fs.ToErrno(fh.mfsFD.Close())
}

// Fsync flushes the write buffer through the open file descriptor.
// This was previously a no-op with bazil.org/fuse because Fsync was
// dispatched to the inode, which couldn't reach the open descriptor.
// hanwen/go-fuse dispatches FileFsyncer to the handle directly.
func (fh *FileHandle) Fsync(_ context.Context, _ uint32) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fs.ToErrno(fh.mfsFD.Flush())
}

// NewFileSystem creates a new MFS FUSE root node.
func NewFileSystem(ipfs *core.IpfsNode, cfg config.Mounts) *Dir {
	c := &fileSystemConfig{
		storeMtime: cfg.StoreMtime.WithDefault(config.DefaultStoreMtime),
		storeMode:  cfg.StoreMode.WithDefault(config.DefaultStoreMode),
	}
	return &Dir{mfsDir: ipfs.FilesRoot.GetDirectory(), cfg: c}
}

// Interface checks.
var (
	_ fs.NodeGetattrer   = (*Dir)(nil)
	_ fs.NodeLookuper    = (*Dir)(nil)
	_ fs.NodeReaddirer   = (*Dir)(nil)
	_ fs.NodeMkdirer     = (*Dir)(nil)
	_ fs.NodeUnlinker    = (*Dir)(nil)
	_ fs.NodeRmdirer     = (*Dir)(nil)
	_ fs.NodeRenamer     = (*Dir)(nil)
	_ fs.NodeCreater     = (*Dir)(nil)
	_ fs.NodeGetxattrer  = (*Dir)(nil)
	_ fs.NodeListxattrer = (*Dir)(nil)

	_ fs.NodeGetattrer   = (*FileInode)(nil)
	_ fs.NodeOpener      = (*FileInode)(nil)
	_ fs.NodeSetattrer   = (*FileInode)(nil)
	_ fs.NodeGetxattrer  = (*FileInode)(nil)
	_ fs.NodeListxattrer = (*FileInode)(nil)

	_ fs.FileReader   = (*FileHandle)(nil)
	_ fs.FileWriter   = (*FileHandle)(nil)
	_ fs.FileFlusher  = (*FileHandle)(nil)
	_ fs.FileReleaser = (*FileHandle)(nil)
	_ fs.FileFsyncer  = (*FileHandle)(nil)
)
