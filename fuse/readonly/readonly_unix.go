//go:build (linux || darwin || freebsd) && !nofuse

package readonly

import (
	"context"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	mdag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	uio "github.com/ipfs/boxo/ipld/unixfs/io"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	core "github.com/ipfs/kubo/core"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

var log = logging.Logger("fuse/ipfs")

// /ipfs paths are immutable (content-addressed by CID), so the kernel
// can cache attributes and directory entries for as long as it wants.
var immutableAttrCacheTime = 365 * 24 * time.Hour

// Root is the root object of the /ipfs filesystem tree.
type Root struct {
	fs.Inode
	ipfs *core.IpfsNode
}

// NewRoot constructs a new readonly root node.
func NewRoot(ipfs *core.IpfsNode) *Root {
	return &Root{ipfs: ipfs}
}

func (*Root) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Attr.Mode = uint32(fusemnt.NamespaceRootMode.Perm())
	out.SetTimeout(immutableAttrCacheTime)
	return 0
}

func (r *Root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debugf("Root Lookup: '%s'", name)
	switch name {
	case "mach_kernel", ".hidden", "._.":
		return nil, syscall.ENOENT
	}

	p, err := path.NewPath("/ipfs/" + name)
	if err != nil {
		log.Debugf("fuse failed to parse path: %q: %s", name, err)
		return nil, syscall.ENOENT
	}

	imPath, err := path.NewImmutablePath(p)
	if err != nil {
		log.Debugf("fuse failed to convert path: %q: %s", name, err)
		return nil, syscall.ENOENT
	}

	nd, ndLnk, err := r.ipfs.UnixFSPathResolver.ResolvePath(ctx, imPath)
	if err != nil {
		return nil, syscall.ENOENT
	}

	cidLnk, ok := ndLnk.(cidlink.Link)
	if !ok {
		log.Debugf("non-cidlink returned from ResolvePath: %v", ndLnk)
		return nil, syscall.ENOENT
	}

	blk, err := r.ipfs.Blockstore.Get(ctx, cidLnk.Cid)
	if err != nil {
		log.Debugf("fuse failed to retrieve block: %v: %s", cidLnk, err)
		return nil, syscall.ENOENT
	}

	var fnd ipld.Node
	switch cidLnk.Cid.Prefix().Codec {
	case cid.DagProtobuf:
		fnd, err = mdag.DecodeProtobuf(blk.RawData())
	case cid.Raw:
		fnd, err = mdag.RawNodeConverter(blk, nd)
	default:
		log.Error("fuse node was not a supported type")
		return nil, syscall.ENOTSUP
	}
	if err != nil {
		log.Errorf("could not decode block as protobuf or raw node: %s", err)
		return nil, syscall.ENOENT
	}

	child := &Node{ipfs: r.ipfs, nd: fnd}
	stable := stableAttrFor(child)

	// Fill attrs in the lookup response so the kernel doesn't cache zeros.
	child.fillAttr(&out.Attr)
	out.SetEntryTimeout(immutableAttrCacheTime)
	out.SetAttrTimeout(immutableAttrCacheTime)
	return r.NewInode(ctx, child, stable), 0
}

// Readdir on the namespace root is not allowed (execute-only).
func (*Root) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	return nil, syscall.EPERM
}

// Node is the core object representing a filesystem tree node.
type Node struct {
	fs.Inode
	ipfs   *core.IpfsNode
	nd     ipld.Node
	cached *ft.FSNode
}

func (n *Node) loadData() error {
	if pbnd, ok := n.nd.(*mdag.ProtoNode); ok {
		fsn, err := ft.FSNodeFromBytes(pbnd.Data())
		if err != nil {
			return err
		}
		n.cached = fsn
	}
	return nil
}

func (n *Node) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Node attr")
	out.SetTimeout(immutableAttrCacheTime)
	n.fillAttr(&out.Attr)
	return 0
}

// Open is required by the kernel before Read can be called.
// Returns nil handle since reads are served by NodeReader directly.
func (n *Node) Open(_ context.Context, _ uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, fuse.FOPEN_KEEP_CACHE, 0
}

// fillAttr populates a fuse.Attr from this node's UnixFS metadata.
// Used by both Getattr and Lookup (to fill EntryOut.Attr so the kernel
// doesn't cache zero values for the entry timeout duration).
func (n *Node) fillAttr(a *fuse.Attr) {
	if rawnd, ok := n.nd.(*mdag.RawNode); ok {
		a.Mode = uint32(fusemnt.DefaultFileModeRO.Perm())
		a.Size = uint64(len(rawnd.RawData()))
		a.Blocks = 1
		return
	}

	if n.cached == nil {
		if err := n.loadData(); err != nil {
			log.Errorf("readonly: loadData() failed: %s", err)
			return
		}
	}

	switch n.cached.Type() {
	case ft.TDirectory, ft.THAMTShard:
		a.Mode = uint32(fusemnt.DefaultDirModeRO.Perm())
	case ft.TFile:
		a.Mode = uint32(fusemnt.DefaultFileModeRO.Perm())
		a.Size = n.cached.FileSize()
		a.Blocks = uint64(len(n.nd.Links()))
	case ft.TRaw:
		a.Mode = uint32(fusemnt.DefaultFileModeRO.Perm())
		a.Size = uint64(len(n.cached.Data()))
		a.Blocks = uint64(len(n.nd.Links()))
	case ft.TSymlink:
		a.Mode = 0o777
		a.Size = uint64(len(n.cached.Data()))
	default:
		log.Errorf("invalid data type: %s", n.cached.Type())
		return
	}

	// Use mode and mtime from UnixFS metadata when present.
	if m := n.cached.Mode(); m != 0 {
		a.Mode = uint32(m) & 07777
	}
	if t := n.cached.ModTime(); !t.IsZero() {
		a.SetTimes(nil, &t, nil)
	}
}

func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debugf("Lookup '%s'", name)
	link, _, err := uio.ResolveUnixfsOnce(ctx, n.ipfs.DAG, n.nd, []string{name})
	switch err {
	case os.ErrNotExist, mdag.ErrLinkNotFound:
		return nil, syscall.ENOENT
	case nil:
	default:
		log.Errorf("fuse lookup %q: %s", name, err)
		return nil, syscall.EIO
	}

	nd, err := n.ipfs.DAG.Get(ctx, link.Cid)
	if err != nil && !ipld.IsNotFound(err) {
		log.Errorf("fuse lookup %q: %s", name, err)
		return nil, syscall.EIO
	}

	child := &Node{ipfs: n.ipfs, nd: nd}
	stable := stableAttrFor(child)

	child.fillAttr(&out.Attr)
	out.SetEntryTimeout(immutableAttrCacheTime)
	out.SetAttrTimeout(immutableAttrCacheTime)
	return n.NewInode(ctx, child, stable), 0
}

func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Node ReadDir")
	dir, err := uio.NewDirectoryFromNode(n.ipfs.DAG, n.nd)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	var entries []fuse.DirEntry
	err = dir.ForEachLink(ctx, func(lnk *ipld.Link) error {
		name := lnk.Name
		if len(name) == 0 {
			name = lnk.Cid.String()
		}
		nd, err := n.ipfs.DAG.Get(ctx, lnk.Cid)
		if err != nil {
			log.Warn("error fetching directory child node: ", err)
		}

		var mode uint32
		switch nd := nd.(type) {
		case *mdag.RawNode:
			// regular file (mode 0 = S_IFREG)
		case *mdag.ProtoNode:
			if fsn, err := ft.FSNodeFromBytes(nd.Data()); err != nil {
				log.Warn("failed to unmarshal protonode data field:", err)
			} else {
				switch fsn.Type() {
				case ft.TDirectory, ft.THAMTShard:
					mode = syscall.S_IFDIR
				case ft.TFile, ft.TRaw:
					// regular file
				case ft.TSymlink:
					mode = syscall.S_IFLNK
				case ft.TMetadata:
					log.Error("metadata object in fuse should contain its wrapped type")
				default:
					log.Error("unrecognized protonode data type: ", fsn.Type())
				}
			}
		}
		entries = append(entries, fuse.DirEntry{Name: name, Mode: mode})
		return nil
	})
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	return fs.NewListDirStream(entries), 0
}

func (n *Node) Listxattr(_ context.Context, dest []byte) (uint32, syscall.Errno) {
	// Null-terminated list of attribute names.
	data := []byte(fusemnt.XattrCID + "\x00")
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (n *Node) Getxattr(_ context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	if attr != fusemnt.XattrCID {
		return 0, fs.ENOATTR
	}
	data := []byte(n.nd.Cid().String())
	if len(dest) == 0 {
		return uint32(len(data)), 0
	}
	if len(dest) < len(data) {
		return 0, syscall.ERANGE
	}
	return uint32(copy(dest, data)), 0
}

func (n *Node) Readlink(_ context.Context) ([]byte, syscall.Errno) {
	if n.cached == nil || n.cached.Type() != ft.TSymlink {
		return nil, syscall.EINVAL
	}
	return n.cached.Data(), 0
}

func (n *Node) Read(ctx context.Context, _ fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	r, err := uio.NewDagReader(ctx, n.nd, n.ipfs.DAG)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	_, err = r.Seek(off, io.SeekStart)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	nread, err := io.ReadFull(r, dest)
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
	default:
		return nil, fs.ToErrno(err)
	}
	return fuse.ReadResultData(dest[:nread]), 0
}

// stableAttrFor returns the StableAttr (file type bits) for a Node.
func stableAttrFor(n *Node) fs.StableAttr {
	if _, ok := n.nd.(*mdag.RawNode); ok {
		return fs.StableAttr{} // S_IFREG
	}
	if n.cached == nil {
		_ = n.loadData()
	}
	if n.cached != nil {
		switch n.cached.Type() {
		case ft.TDirectory, ft.THAMTShard:
			return fs.StableAttr{Mode: syscall.S_IFDIR}
		case ft.TSymlink:
			return fs.StableAttr{Mode: syscall.S_IFLNK}
		}
	}
	return fs.StableAttr{} // S_IFREG
}

// Interface checks.
var (
	_ fs.NodeGetattrer   = (*Root)(nil)
	_ fs.NodeLookuper    = (*Root)(nil)
	_ fs.NodeReaddirer   = (*Root)(nil)
	_ fs.NodeGetattrer   = (*Node)(nil)
	_ fs.NodeLookuper    = (*Node)(nil)
	_ fs.NodeOpener      = (*Node)(nil)
	_ fs.NodeReaddirer   = (*Node)(nil)
	_ fs.NodeReader      = (*Node)(nil)
	_ fs.NodeReadlinker  = (*Node)(nil)
	_ fs.NodeGetxattrer  = (*Node)(nil)
	_ fs.NodeListxattrer = (*Node)(nil)
)
