// Package writable implements FUSE filesystem types shared by the
// mutable /mfs and /ipns mounts. Both mounts expose MFS directories
// as writable POSIX filesystems; the only differences are how the
// root is created and how xattr names are published.
//
//go:build (linux || darwin || freebsd) && !nofuse

package writable

import (
	"context"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/ipfs/boxo/files"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	uio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/boxo/mfs"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
)

var log = logging.Logger("fuse/writable")

// Config controls write-side behavior for writable mounts.
type Config struct {
	StoreMtime bool            // persist mtime on create and open-for-write
	StoreMode  bool            // persist mode on chmod
	DAG        ipld.DAGService // for read-only opens that bypass MFS locking
}

// NewDir creates a Dir node backed by the given MFS directory.
func NewDir(d *mfs.Directory, cfg *Config) *Dir {
	return &Dir{MFSDir: d, Cfg: cfg}
}

// Dir is the FUSE adapter for MFS directories.
type Dir struct {
	fs.Inode
	MFSDir *mfs.Directory
	Cfg    *Config
}

func (d *Dir) fillAttr(a *fuse.Attr) {
	a.Mode = uint32(fusemnt.DefaultDirModeRW.Perm())
	if m, err := d.MFSDir.Mode(); err == nil && m != 0 {
		a.Mode = files.ModePermsToUnixPerms(m)
	}
	if t, err := d.MFSDir.ModTime(); err == nil && !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (d *Dir) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	d.fillAttr(&out.Attr)
	return 0
}

// Setattr handles chmod and mtime changes on directories.
// Tools like tar and rsync set directory timestamps after extraction.
//
// Mode and mtime are stored as UnixFS optional metadata.
// The UnixFS spec supports all 12 permission bits, but boxo's MFS
// layer exposes only the lower 9 (ugo-rwx); setuid/setgid/sticky
// are silently dropped. FUSE mounts are always nosuid so these
// bits would have no execution effect anyway.
// See https://specs.ipfs.tech/unixfs/#dag-pb-optional-metadata
func (d *Dir) Setattr(_ context.Context, _ fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if mode, ok := in.GetMode(); ok && d.Cfg.StoreMode {
		if err := d.MFSDir.SetMode(files.UnixPermsToModePerms(mode)); err != nil {
			return fs.ToErrno(err)
		}
	}
	if mtime, ok := in.GetMTime(); ok && d.Cfg.StoreMtime {
		if err := d.MFSDir.SetModTime(mtime); err != nil {
			return fs.ToErrno(err)
		}
	}
	d.fillAttr(&out.Attr)
	return 0
}

func (d *Dir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	mfsNode, err := d.MFSDir.Child(name)
	if err != nil {
		return nil, syscall.ENOENT
	}

	switch mfsNode.Type() {
	case mfs.TDir:
		child := &Dir{MFSDir: mfsNode.(*mfs.Directory), Cfg: d.Cfg}
		child.fillAttr(&out.Attr)
		return d.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
	case mfs.TFile:
		mfsFile := mfsNode.(*mfs.File)
		if target := SymlinkTarget(mfsFile); target != "" {
			child := &Symlink{Target: target, MFSFile: mfsFile, Cfg: d.Cfg}
			child.fillAttr(&out.Attr)
			return d.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFLNK}), 0
		}
		child := &FileInode{MFSFile: mfsFile, Cfg: d.Cfg}
		child.fillAttr(&out.Attr)
		return d.NewInode(ctx, child, fs.StableAttr{}), 0
	default:
		log.Errorf("unexpected MFS node type %d under directory", mfsNode.Type())
		return nil, syscall.EIO
	}
}

func (d *Dir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	nodes, err := d.MFSDir.List(ctx)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	entries := make([]fuse.DirEntry, len(nodes))
	for i, node := range nodes {
		var mode uint32
		switch {
		case node.Type == int(mfs.TDir):
			mode = syscall.S_IFDIR
		case node.Type == int(mfs.TFile):
			// MFS represents symlinks as TFile; check the DAG node.
			if child, err := d.MFSDir.Child(node.Name); err == nil {
				if f, ok := child.(*mfs.File); ok && SymlinkTarget(f) != "" {
					mode = syscall.S_IFLNK
				}
			}
		}
		entries[i] = fuse.DirEntry{Name: node.Name, Mode: mode}
	}
	return fs.NewListDirStream(entries), 0
}

func (d *Dir) Mkdir(ctx context.Context, name string, _ uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	mfsDir, err := d.MFSDir.Mkdir(name)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	out.Attr.Mode = uint32(fusemnt.DefaultDirModeRW.Perm())
	child := &Dir{MFSDir: mfsDir, Cfg: d.Cfg}
	return d.NewInode(ctx, child, fs.StableAttr{Mode: syscall.S_IFDIR}), 0
}

func (d *Dir) Unlink(_ context.Context, name string) syscall.Errno {
	if err := d.MFSDir.Unlink(name); err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(d.MFSDir.Flush())
}

func (d *Dir) Rmdir(ctx context.Context, name string) syscall.Errno {
	child, err := d.MFSDir.Child(name)
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

	if err := d.MFSDir.Unlink(name); err != nil {
		return fs.ToErrno(err)
	}
	return fs.ToErrno(d.MFSDir.Flush())
}

func (d *Dir) Rename(_ context.Context, oldName string, newParent fs.InodeEmbedder, newName string, _ uint32) syscall.Errno {
	child, err := d.MFSDir.Child(oldName)
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
	if err := d.MFSDir.Unlink(oldName); err != nil {
		return fs.ToErrno(err)
	}

	targetDir, ok := newParent.EmbeddedInode().Operations().(*Dir)
	if !ok {
		return syscall.EINVAL
	}
	if err := targetDir.MFSDir.Unlink(newName); err != nil && err != os.ErrNotExist {
		return fs.ToErrno(err)
	}
	if err := targetDir.MFSDir.AddChild(newName, nd); err != nil {
		return fs.ToErrno(err)
	}

	return fs.ToErrno(d.MFSDir.Flush())
}

func (d *Dir) Create(ctx context.Context, name string, flags uint32, _ uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	node := dag.NodeWithData(ft.FilePBData(nil, 0))
	if err := node.SetCidBuilder(d.MFSDir.GetCidBuilder()); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	if err := d.MFSDir.AddChild(name, node); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	if err := d.MFSDir.Flush(); err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	mfsNode, err := d.MFSDir.Child(name)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}
	if d.Cfg.StoreMtime {
		if err := mfsNode.SetModTime(time.Now()); err != nil {
			return nil, nil, 0, fs.ToErrno(err)
		}
	}

	mfsFile, ok := mfsNode.(*mfs.File)
	if !ok {
		return nil, nil, 0, syscall.EIO
	}
	fileInode := &FileInode{MFSFile: mfsFile, Cfg: d.Cfg}

	accessMode := flags & syscall.O_ACCMODE
	fd, err := mfsFile.Open(mfs.Flags{
		Read:  accessMode == syscall.O_RDONLY || accessMode == syscall.O_RDWR,
		Write: accessMode == syscall.O_WRONLY || accessMode == syscall.O_RDWR,
		Sync:  true,
	})
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	inode := d.NewInode(ctx, fileInode, fs.StableAttr{})
	return inode, &FileHandle{inode: inode, fd: fd}, 0, 0
}

func (d *Dir) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data := []byte(fusemnt.XattrCID + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (d *Dir) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if attr == fusemnt.XattrCIDDeprecated {
		log.Errorf("xattr %q is deprecated, use %q instead", fusemnt.XattrCIDDeprecated, fusemnt.XattrCID)
		attr = fusemnt.XattrCID
	}
	if attr != fusemnt.XattrCID {
		return 0, fs.ENOATTR
	}
	nd, err := d.MFSDir.GetNode()
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

// Symlink creates a new symlink in this directory.
func (d *Dir) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	data, err := ft.SymlinkData(target)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	nd := dag.NodeWithData(data)
	if err := nd.SetCidBuilder(d.MFSDir.GetCidBuilder()); err != nil {
		return nil, fs.ToErrno(err)
	}
	if err := d.MFSDir.AddChild(name, nd); err != nil {
		return nil, fs.ToErrno(err)
	}
	if err := d.MFSDir.Flush(); err != nil {
		return nil, fs.ToErrno(err)
	}

	// Retrieve the mfs.File so Setattr can persist mtime.
	mfsNode, err := d.MFSDir.Child(name)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	mfsFile, _ := mfsNode.(*mfs.File)

	sym := &Symlink{Target: target, MFSFile: mfsFile, Cfg: d.Cfg}
	sym.fillAttr(&out.Attr)
	return d.NewInode(ctx, sym, fs.StableAttr{Mode: syscall.S_IFLNK}), 0
}

// FileInode is the FUSE adapter for MFS file inodes.
type FileInode struct {
	fs.Inode
	MFSFile *mfs.File
	Cfg     *Config
}

func (fi *FileInode) fillAttr(a *fuse.Attr) {
	size, _ := fi.MFSFile.Size()
	a.Size = uint64(size)
	a.Mode = uint32(fusemnt.DefaultFileModeRW.Perm())
	if m, err := fi.MFSFile.Mode(); err == nil && m != 0 {
		a.Mode = files.ModePermsToUnixPerms(m)
	}
	if t, _ := fi.MFSFile.ModTime(); !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (fi *FileInode) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	fi.fillAttr(&out.Attr)
	return 0
}

func (fi *FileInode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	accessMode := flags & syscall.O_ACCMODE

	// Read-only opens bypass MFS's desclock by creating a DagReader
	// directly from the current DAG node. MFS holds desclock.RLock
	// for the lifetime of a read descriptor, which blocks any
	// concurrent write open on the same file (desclock.Lock). Tools
	// like rsync --inplace open the destination for reading and
	// writing simultaneously, deadlocking on MFS's lock. Creating
	// a DagReader here avoids the lock entirely: the reader gets a
	// snapshot of the file at open time, and writers proceed through
	// MFS independently.
	if accessMode == syscall.O_RDONLY && fi.Cfg.DAG != nil {
		nd, err := fi.MFSFile.GetNode()
		if err != nil {
			return nil, 0, fs.ToErrno(err)
		}
		r, err := uio.NewDagReader(ctx, nd, fi.Cfg.DAG)
		if err != nil {
			return nil, 0, fs.ToErrno(err)
		}
		return &roFileHandle{r: r}, fuse.FOPEN_KEEP_CACHE, 0
	}

	mfsFlags := mfs.Flags{
		Read:  accessMode == syscall.O_RDONLY || accessMode == syscall.O_RDWR,
		Write: accessMode == syscall.O_WRONLY || accessMode == syscall.O_RDWR,
		Sync:  true,
	}
	fd, err := fi.MFSFile.Open(mfsFlags)
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

	if mfsFlags.Write && fi.Cfg.StoreMtime {
		if err := fi.MFSFile.SetModTime(time.Now()); err != nil {
			fd.Close()
			return nil, 0, fs.ToErrno(err)
		}
	}

	return &FileHandle{inode: fi.EmbeddedInode(), fd: fd, appendMode: flags&syscall.O_APPEND != 0}, 0, 0
}

// Setattr handles chmod, mtime changes (touch), and ftruncate.
//
// Mode and mtime are stored as UnixFS optional metadata.
// The UnixFS spec supports all 12 permission bits, but boxo's MFS
// layer exposes only the lower 9 (ugo-rwx); setuid/setgid/sticky
// are silently dropped. FUSE mounts are always nosuid so these
// bits would have no execution effect anyway.
// See https://specs.ipfs.tech/unixfs/#dag-pb-optional-metadata
//
// With hanwen/go-fuse, the kernel passes the open file handle (fh) when
// the caller uses ftruncate(fd, size). This lets us truncate through
// the existing write descriptor without opening a second one. For
// truncate(path, size) without a handle, a temporary descriptor is
// opened; this may block if another writer holds MFS's desclock.
func (fi *FileInode) Setattr(_ context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, ok := in.GetSize(); ok {
		if f, ok := fh.(*FileHandle); ok {
			// ftruncate(fd, size): use the existing write descriptor.
			f.mu.Lock()
			err := f.fd.Truncate(int64(sz))
			f.mu.Unlock()
			if err != nil {
				return fs.ToErrno(err)
			}
		} else {
			// truncate(path, size) without an open file descriptor.
			// Open a temporary write descriptor, truncate, flush, and
			// close. This may block if another writer holds MFS's
			// desclock; the FUSE kernel timeout (30s) bounds the wait.
			fd, err := fi.MFSFile.Open(mfs.Flags{Write: true, Sync: true})
			if err != nil {
				return fs.ToErrno(err)
			}
			if err := fd.Truncate(int64(sz)); err != nil {
				fd.Close()
				return fs.ToErrno(err)
			}
			if err := fd.Flush(); err != nil {
				fd.Close()
				return fs.ToErrno(err)
			}
			if err := fd.Close(); err != nil {
				return fs.ToErrno(err)
			}
		}
	}
	if mode, ok := in.GetMode(); ok && fi.Cfg.StoreMode {
		if err := fi.MFSFile.SetMode(files.UnixPermsToModePerms(mode)); err != nil {
			return fs.ToErrno(err)
		}
	}
	if mtime, ok := in.GetMTime(); ok && fi.Cfg.StoreMtime {
		if err := fi.MFSFile.SetModTime(mtime); err != nil {
			return fs.ToErrno(err)
		}
	}
	return 0
}

func (fi *FileInode) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	data := []byte(fusemnt.XattrCID + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (fi *FileInode) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if attr == fusemnt.XattrCIDDeprecated {
		log.Errorf("xattr %q is deprecated, use %q instead", fusemnt.XattrCIDDeprecated, fusemnt.XattrCID)
		attr = fusemnt.XattrCID
	}
	if attr != fusemnt.XattrCID {
		return 0, fs.ENOATTR
	}
	nd, err := fi.MFSFile.GetNode()
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

// FileHandle wraps an MFS file descriptor for FUSE operations.
// All methods are serialized by mu because the FUSE server dispatches
// each request in its own goroutine and the underlying DagModifier
// is not safe for concurrent use.
type FileHandle struct {
	inode      *fs.Inode // back-pointer for kernel cache invalidation
	fd         mfs.FileDescriptor
	mu         sync.Mutex
	appendMode bool // O_APPEND: writes always go to end of file
}

func (fh *FileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if _, err := fh.fd.Seek(off, io.SeekStart); err != nil {
		return nil, fs.ToErrno(err)
	}

	size, err := fh.fd.Size()
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	n := min(len(dest), int(size-off))
	if n <= 0 {
		return fuse.ReadResultData(nil), 0
	}
	got, err := fh.fd.CtxReadFull(ctx, dest[:n])
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return fuse.ReadResultData(dest[:got]), 0
}

func (fh *FileHandle) Write(_ context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if fh.appendMode {
		// O_APPEND: the kernel may send offset 0, but POSIX says
		// writes must go to the end of the file.
		if _, err := fh.fd.Seek(0, io.SeekEnd); err != nil {
			return 0, fs.ToErrno(err)
		}
		n, err := fh.fd.Write(data)
		if err != nil {
			return 0, fs.ToErrno(err)
		}
		return uint32(n), 0
	}

	n, err := fh.fd.WriteAt(data, off)
	if err != nil {
		return 0, fs.ToErrno(err)
	}
	return uint32(n), 0
}

// Flush persists buffered writes to the DAG and invalidates the
// kernel's cached attrs so the next stat sees the updated size.
//
// We intentionally ignore ctx: the underlying MFS flush cannot be
// safely canceled mid-operation, and abandoning it would leak a
// background goroutine that races with the subsequent Release.
//
// Cache invalidation happens here (in addition to Release) because
// the kernel calls Flush synchronously inside close() but sends
// Release asynchronously after close() returns. Without this, a
// stat() immediately after close() could see stale cached attrs.
func (fh *FileHandle) Flush(_ context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	err := fh.fd.Flush()
	if fh.inode != nil {
		_ = fh.inode.NotifyContent(0, 0)
	}
	return fs.ToErrno(err)
}

// Release closes the descriptor and invalidates the kernel's cached
// content and attrs so readers opening the same path see the new data.
// Invalidation happens here (not in Flush) because fd.Close commits
// the final DAG node; Flush alone may not have the final size yet.
func (fh *FileHandle) Release(_ context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	err := fh.fd.Close()
	if fh.inode != nil {
		_ = fh.inode.NotifyContent(0, 0)
	}
	return fs.ToErrno(err)
}

// Fsync flushes the write buffer through the open file descriptor.
// Editors (vim, emacs) and databases call fsync after writing to
// ensure data reaches persistent storage.
func (fh *FileHandle) Fsync(_ context.Context, _ uint32) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fs.ToErrno(fh.fd.Flush())
}

// Symlink is the FUSE adapter for UnixFS TSymlink nodes on writable mounts.
// Target is resolved once at Lookup/Create time and never changes
// (POSIX symlinks are immutable; changing the target requires unlink + symlink).
type Symlink struct {
	fs.Inode
	Target  string
	MFSFile *mfs.File // backing MFS node for mtime persistence
	Cfg     *Config
}

func (s *Symlink) Readlink(_ context.Context) ([]byte, syscall.Errno) {
	return []byte(s.Target), 0
}

func (s *Symlink) fillAttr(a *fuse.Attr) {
	a.Mode = uint32(fusemnt.SymlinkMode.Perm())
	a.Size = uint64(len(s.Target))
	if s.MFSFile != nil {
		if t, err := s.MFSFile.ModTime(); err == nil && !t.IsZero() {
			a.SetTimes(nil, &t, nil)
		}
	}
}

func (s *Symlink) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	s.fillAttr(&out.Attr)
	return 0
}

// Setattr handles mtime changes on symlinks.
// Tools like rsync call lutimes on symlinks after creating them and
// treat ENOTSUP as an error. Every major FUSE filesystem (gocryptfs,
// rclone, sshfs, s3fs) implements Setattr on symlinks for this reason.
//
// Mode is always 0777 per POSIX convention (access control uses the
// target's mode), so chmod requests are silently accepted but not stored.
func (s *Symlink) Setattr(_ context.Context, _ fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if s.MFSFile != nil {
		if mtime, ok := in.GetMTime(); ok && s.Cfg.StoreMtime {
			if err := s.MFSFile.SetModTime(mtime); err != nil {
				return fs.ToErrno(err)
			}
		}
	}
	s.fillAttr(&out.Attr)
	return 0
}

// roFileHandle is a read-only file handle backed by a DagReader.
// Used for O_RDONLY opens to bypass MFS's desclock (see FileInode.Open).
type roFileHandle struct {
	r  uio.DagReader
	mu sync.Mutex
}

func (fh *roFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	if _, err := fh.r.Seek(off, io.SeekStart); err != nil {
		return nil, fs.ToErrno(err)
	}
	n, err := fh.r.CtxReadFull(ctx, dest)
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
	default:
		return nil, fs.ToErrno(err)
	}
	return fuse.ReadResultData(dest[:n]), 0
}

func (fh *roFileHandle) Release(_ context.Context) syscall.Errno {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fs.ToErrno(fh.r.Close())
}

// SymlinkTarget extracts the symlink target from an MFS file, or
// returns "" if the file is not a TSymlink node. MFS represents
// symlinks as *mfs.File, so the DAG node's UnixFS type must be checked.
func SymlinkTarget(f *mfs.File) string {
	nd, err := f.GetNode()
	if err != nil {
		return ""
	}
	fsn, err := ft.ExtractFSNode(nd)
	if err != nil {
		return ""
	}
	if fsn.Type() != ft.TSymlink {
		return ""
	}
	return string(fsn.Data())
}

// Interface compliance checks.
var (
	_ fs.NodeGetattrer   = (*Dir)(nil)
	_ fs.NodeSetattrer   = (*Dir)(nil)
	_ fs.NodeLookuper    = (*Dir)(nil)
	_ fs.NodeReaddirer   = (*Dir)(nil)
	_ fs.NodeMkdirer     = (*Dir)(nil)
	_ fs.NodeUnlinker    = (*Dir)(nil)
	_ fs.NodeRmdirer     = (*Dir)(nil)
	_ fs.NodeRenamer     = (*Dir)(nil)
	_ fs.NodeCreater     = (*Dir)(nil)
	_ fs.NodeSymlinker   = (*Dir)(nil)
	_ fs.NodeGetxattrer  = (*Dir)(nil)
	_ fs.NodeListxattrer = (*Dir)(nil)

	_ fs.NodeGetattrer   = (*FileInode)(nil)
	_ fs.NodeOpener      = (*FileInode)(nil)
	_ fs.NodeSetattrer   = (*FileInode)(nil)
	_ fs.NodeGetxattrer  = (*FileInode)(nil)
	_ fs.NodeListxattrer = (*FileInode)(nil)

	_ fs.NodeGetattrer  = (*Symlink)(nil)
	_ fs.NodeSetattrer  = (*Symlink)(nil)
	_ fs.NodeReadlinker = (*Symlink)(nil)

	_ fs.FileReader   = (*FileHandle)(nil)
	_ fs.FileWriter   = (*FileHandle)(nil)
	_ fs.FileFlusher  = (*FileHandle)(nil)
	_ fs.FileReleaser = (*FileHandle)(nil)
	_ fs.FileFsyncer  = (*FileHandle)(nil)

	_ fs.FileReader   = (*roFileHandle)(nil)
	_ fs.FileReleaser = (*roFileHandle)(nil)
)
