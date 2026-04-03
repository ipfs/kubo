//go:build (linux || darwin || freebsd || netbsd || openbsd) && !nofuse

package mfs

import (
	"context"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/kubo/core"
)

const (
	ipfsCIDXattr = "ipfs_cid"
	mfsDirMode   = os.ModeDir | 0755
	mfsFileMode  = 0644
	blockSize    = 512
	dirSize      = 8
)

// FUSE filesystem mounted at /mfs.
type FileSystem struct {
	root     Dir
	repoPath string
}

// Get filesystem root.
func (fs *FileSystem) Root() (fs.Node, error) {
	return &fs.root, nil
}

// Filesystem statistics (statfs)
func (fs *FileSystem) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(fs.repoPath, &stat); err != nil {
		return err
	}
	resp.Blocks = stat.Blocks
	resp.Bfree = stat.Bfree
	resp.Bavail = uint64(stat.Bavail)
	resp.Files = stat.Files
	resp.Ffree = uint64(stat.Ffree)
	resp.Bsize = uint32(stat.Bsize)
	resp.Namelen = 255
	return nil
}

// FUSE Adapter for MFS directories.
type Dir struct {
	mfsDir *mfs.Directory
}

// Directory attributes (stat).
func (dir *Dir) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Valid = 0
	// TODO: use Mode from UnixFS record if present
	attr.Mode = mfsDirMode
	attr.Size = dirSize * blockSize
	attr.Blocks = dirSize
	attr.Uid = uint32(os.Getuid())
	attr.Gid = uint32(os.Getgid())
	return nil
}

// Access files in a directory.
func (dir *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	mfsNode, err := dir.mfsDir.Child(req.Name)
	switch err {
	case os.ErrNotExist:
		return nil, syscall.Errno(syscall.ENOENT)
	case nil:
	default:
		return nil, err
	}

	resp.EntryValid = 0

	switch mfsNode.Type() {
	case mfs.TDir:
		result := Dir{
			mfsDir: mfsNode.(*mfs.Directory),
		}
		return &result, nil
	case mfs.TFile:
		result := File{
			mfsFile: mfsNode.(*mfs.File),
		}
		return &result, nil
	}

	return nil, syscall.Errno(syscall.ENOENT)
}

// List (ls) MFS directory.
func (dir *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent
	nodes, err := dir.mfsDir.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		nodeType := fuse.DT_File
		if node.Type == 1 {
			nodeType = fuse.DT_Dir
		}
		res = append(res, fuse.Dirent{
			Type: nodeType,
			Name: node.Name,
		})
	}
	return res, nil
}

// Mkdir (mkdir) in MFS.
func (dir *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	mfsDir, err := dir.mfsDir.Mkdir(req.Name)
	if err != nil {
		return nil, err
	}
	return &Dir{
		mfsDir: mfsDir,
	}, nil
}

// Remove (rm/rmdir) an MFS file.
func (dir *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	// Check for empty directory.
	if req.Dir {
		targetNode, err := dir.mfsDir.Child(req.Name)
		if err != nil {
			return err
		}
		target := targetNode.(*mfs.Directory)

		children, err := target.ListNames(ctx)
		if err != nil {
			return err
		}
		if len(children) > 0 {
			return os.ErrExist
		}
	}
	err := dir.mfsDir.Unlink(req.Name)
	if err != nil {
		return err
	}
	return dir.mfsDir.Flush()
}

// Move (mv) an MFS file.
func (dir *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	child, err := dir.mfsDir.Child(req.OldName)
	if err != nil {
		return err
	}

	nd, err := child.GetNode()
	if err != nil {
		return err
	}

	// Unlink the source first. For same-directory renames, this clears
	// the old name from the directory's entry cache before AddChild
	// repopulates it with the new name. Without this ordering, Flush
	// would sync the stale cache entry back into the DAG.
	if err := dir.mfsDir.Unlink(req.OldName); err != nil {
		return err
	}

	targetDir := newDir.(*Dir)
	if err := targetDir.mfsDir.Unlink(req.NewName); err != nil && err != os.ErrNotExist {
		return err
	}
	if err := targetDir.mfsDir.AddChild(req.NewName, nd); err != nil {
		return err
	}

	return dir.mfsDir.Flush()
}

// Create (touch) an MFS file.
func (dir *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	node := dag.NodeWithData(ft.FilePBData(nil, 0))
	if err := node.SetCidBuilder(dir.mfsDir.GetCidBuilder()); err != nil {
		return nil, nil, err
	}

	if err := dir.mfsDir.AddChild(req.Name, node); err != nil {
		return nil, nil, err
	}

	if err := dir.mfsDir.Flush(); err != nil {
		return nil, nil, err
	}

	mfsNode, err := dir.mfsDir.Child(req.Name)
	if err != nil {
		return nil, nil, err
	}
	if err := mfsNode.SetModTime(time.Now()); err != nil {
		return nil, nil, err
	}

	mfsFile := mfsNode.(*mfs.File)

	file := File{
		mfsFile: mfsFile,
	}

	// Read access flags and create a handler.
	accessMode := req.Flags & fuse.OpenAccessModeMask
	flags := mfs.Flags{
		Read:  accessMode == fuse.OpenReadOnly || accessMode == fuse.OpenReadWrite,
		Write: accessMode == fuse.OpenWriteOnly || accessMode == fuse.OpenReadWrite,
		Sync:  true, // FUSE writes must propagate to the MFS root on close
	}

	fd, err := mfsFile.Open(flags)
	if err != nil {
		return nil, nil, err
	}
	handler := FileHandler{
		mfsFD: fd,
	}

	return &file, &handler, nil
}

// List dir xattr.
func (dir *Dir) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	resp.Append(ipfsCIDXattr)
	return nil
}

// Get dir xattr.
func (dir *Dir) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	switch req.Name {
	case ipfsCIDXattr:
		node, err := dir.mfsDir.GetNode()
		if err != nil {
			return err
		}
		resp.Xattr = []byte(node.Cid().String())
		return nil
	default:
		return fuse.ErrNoXattr
	}
}

// FUSE adapter for MFS files.
type File struct {
	mfsFile *mfs.File
}

// File attributes.
func (file *File) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Valid = 0

	size, _ := file.mfsFile.Size()

	attr.Size = uint64(size)
	if size%blockSize == 0 {
		attr.Blocks = uint64(size / blockSize)
	} else {
		attr.Blocks = uint64(size/blockSize + 1)
	}

	mtime, _ := file.mfsFile.ModTime()
	attr.Mtime = mtime

	// TODO: use Mode from UnixFS record if present
	attr.Mode = mfsFileMode
	attr.Uid = uint32(os.Getuid())
	attr.Gid = uint32(os.Getgid())
	return nil
}

// Open an MFS file.
func (file *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	accessMode := req.Flags & fuse.OpenAccessModeMask
	flags := mfs.Flags{
		Read:  accessMode == fuse.OpenReadOnly || accessMode == fuse.OpenReadWrite,
		Write: accessMode == fuse.OpenWriteOnly || accessMode == fuse.OpenReadWrite,
		Sync:  true, // FUSE writes must propagate to the MFS root on close
	}
	fd, err := file.mfsFile.Open(flags)
	if err != nil {
		return nil, err
	}

	if flags.Write {
		if err := file.mfsFile.SetModTime(time.Now()); err != nil {
			return nil, err
		}
	}

	return &FileHandler{
		mfsFD: fd,
	}, nil
}

// Fsync is a no-op. We can't flush here because mfs.File.Flush opens a new
// write descriptor, which needs an exclusive lock (desclock) that the caller
// already holds from Open. Attempting it deadlocks until the FUSE timeout.
// Data is flushed when the file is closed instead.
// TODO: a proper fix needs changes in boxo/mfs to allow flushing from an
// existing descriptor. Ideas welcome, but for now this is the best we can do.
func (file *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

// List file xattr.
func (file *File) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	resp.Append(ipfsCIDXattr)
	return nil
}

// Get file xattr.
func (file *File) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	switch req.Name {
	case ipfsCIDXattr:
		node, err := file.mfsFile.GetNode()
		if err != nil {
			return err
		}
		resp.Xattr = []byte(node.Cid().String())
		return nil
	default:
		return fuse.ErrNoXattr
	}
}

// Wrapper for MFS's file descriptor that conforms to the FUSE fs.Handler
// interface.
type FileHandler struct {
	mfsFD mfs.FileDescriptor
	mu    sync.Mutex
}

// Read a opened MFS file.
func (fh *FileHandler) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	_, err := fh.mfsFD.Seek(req.Offset, io.SeekStart)
	if err != nil {
		return err
	}

	buf := make([]byte, req.Size)
	l, err := fh.mfsFD.Read(buf)

	resp.Data = buf[:l]

	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		return nil
	default:
		return err
	}
}

// Write writes to an opened MFS file.
func (fh *FileHandler) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	l, err := fh.mfsFD.WriteAt(req.Data, req.Offset)
	if err != nil {
		return err
	}
	resp.Size = l

	return nil
}

// Flushes the file's buffer.
func (fh *FileHandler) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fh.mfsFD.Flush()
}

// Closes the file.
func (fh *FileHandler) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	return fh.mfsFD.Close()
}

// Create new filesystem.
func NewFileSystem(ipfs *core.IpfsNode) fs.FS {
	return &FileSystem{
		root: Dir{
			mfsDir: ipfs.FilesRoot.GetDirectory(),
		},
		repoPath: ipfs.Repo.Path(),
	}
}

var _ fs.FSStatfser = (*FileSystem)(nil)

// Check that our structs implement all the interfaces we want.
type mfsDir interface {
	fs.Node
	fs.NodeGetxattrer
	fs.NodeListxattrer
	fs.HandleReadDirAller
	fs.NodeRequestLookuper
	fs.NodeMkdirer
	fs.NodeRenamer
	fs.NodeRemover
	fs.NodeCreater
}

var _ mfsDir = (*Dir)(nil)

type mfsFile interface {
	fs.Node
	fs.NodeGetxattrer
	fs.NodeListxattrer
	fs.NodeOpener
	fs.NodeFsyncer
}

var _ mfsFile = (*File)(nil)

type mfsHandler interface {
	fs.Handle
	fs.HandleReader
	fs.HandleWriter
	fs.HandleFlusher
	fs.HandleReleaser
}

var _ mfsHandler = (*FileHandler)(nil)
