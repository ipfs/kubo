// +build linux darwin freebsd
// +build !nofuse

package readonly

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"

	core "github.com/ipfs/go-ipfs/core"
	mdag "github.com/ipfs/go-merkledag"
	path "github.com/ipfs/go-path"
	ft "github.com/ipfs/go-unixfs"
	uio "github.com/ipfs/go-unixfs/io"

	fuse "bazil.org/fuse"
	fs "bazil.org/fuse/fs"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("fuse/ipfs")

// FileSystem is the readonly IPFS Fuse Filesystem.
type FileSystem struct {
	Ipfs *core.IpfsNode
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewFileSystem(ipfs *core.IpfsNode) *FileSystem {
	return &FileSystem{Ipfs: ipfs}
}

// Root constructs the Root of the filesystem, a Root object.
func (f FileSystem) Root() (fs.Node, error) {
	return &Root{Ipfs: f.Ipfs}, nil
}

// Root is the root object of the filesystem tree.
type Root struct {
	Ipfs *core.IpfsNode
}

// Attr returns file attributes.
func (*Root) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0111 // -rw+x
	return nil
}

// Lookup performs a lookup under this node.
func (s *Root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	log.Debugf("Root Lookup: '%s'", name)
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, fuse.ENOENT
	}

	p, err := path.ParsePath(name)
	if err != nil {
		log.Debugf("fuse failed to parse path: %q: %s", name, err)
		return nil, fuse.ENOENT
	}

	nd, err := s.Ipfs.Resolver.ResolvePath(ctx, p)
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	switch nd := nd.(type) {
	case *mdag.ProtoNode, *mdag.RawNode:
		return &Node{Ipfs: s.Ipfs, Nd: nd}, nil
	default:
		log.Error("fuse node was not a protobuf node")
		return nil, fuse.ENOTSUP
	}

}

// ReadDirAll reads a particular directory. Disallowed for root.
func (*Root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("read Root")
	return nil, fuse.EPERM
}

// Node is the core object representing a filesystem tree node.
type Node struct {
	Ipfs   *core.IpfsNode
	Nd     ipld.Node
	cached *ft.FSNode
}

func (s *Node) loadData() error {
	if pbnd, ok := s.Nd.(*mdag.ProtoNode); ok {
		fsn, err := ft.FSNodeFromBytes(pbnd.Data())
		if err != nil {
			return err
		}
		s.cached = fsn
	}
	return nil
}

// Attr returns the attributes of a given node.
func (s *Node) Attr(ctx context.Context, a *fuse.Attr) error {
	log.Debug("Node attr")
	if rawnd, ok := s.Nd.(*mdag.RawNode); ok {
		a.Mode = 0444
		a.Size = uint64(len(rawnd.RawData()))
		a.Blocks = 1
		return nil
	}

	if s.cached == nil {
		if err := s.loadData(); err != nil {
			return fmt.Errorf("readonly: loadData() failed: %s", err)
		}
	}
	switch s.cached.Type() {
	case ft.TDirectory, ft.THAMTShard:
		a.Mode = os.ModeDir | 0555
	case ft.TFile:
		size := s.cached.FileSize()
		a.Mode = 0444
		a.Size = uint64(size)
		a.Blocks = uint64(len(s.Nd.Links()))
	case ft.TRaw:
		a.Mode = 0444
		a.Size = uint64(len(s.cached.Data()))
		a.Blocks = uint64(len(s.Nd.Links()))
	case ft.TSymlink:
		a.Mode = 0777 | os.ModeSymlink
		a.Size = uint64(len(s.cached.Data()))
	default:
		return fmt.Errorf("invalid data type - %s", s.cached.Type())
	}
	return nil
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	log.Debugf("Lookup '%s'", name)
	link, _, err := uio.ResolveUnixfsOnce(ctx, s.Ipfs.DAG, s.Nd, []string{name})
	switch err {
	case os.ErrNotExist, mdag.ErrLinkNotFound:
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	default:
		log.Errorf("fuse lookup %q: %s", name, err)
		return nil, fuse.EIO
	case nil:
		// noop
	}

	nd, err := s.Ipfs.DAG.Get(ctx, link.Cid)
	switch err {
	case ipld.ErrNotFound:
	default:
		log.Errorf("fuse lookup %q: %s", name, err)
		return nil, err
	case nil:
		// noop
	}

	return &Node{Ipfs: s.Ipfs, Nd: nd}, nil
}

// ReadDirAll reads the link structure as directory entries
func (s *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("Node ReadDir")
	dir, err := uio.NewDirectoryFromNode(s.Ipfs.DAG, s.Nd)
	if err != nil {
		return nil, err
	}

	var entries []fuse.Dirent
	err = dir.ForEachLink(ctx, func(lnk *ipld.Link) error {
		n := lnk.Name
		if len(n) == 0 {
			n = lnk.Cid.String()
		}
		nd, err := s.Ipfs.DAG.Get(ctx, lnk.Cid)
		if err != nil {
			log.Warn("error fetching directory child node: ", err)
		}

		t := fuse.DT_Unknown
		switch nd := nd.(type) {
		case *mdag.RawNode:
			t = fuse.DT_File
		case *mdag.ProtoNode:
			if fsn, err := ft.FSNodeFromBytes(nd.Data()); err != nil {
				log.Warn("failed to unmarshal protonode data field:", err)
			} else {
				switch fsn.Type() {
				case ft.TDirectory, ft.THAMTShard:
					t = fuse.DT_Dir
				case ft.TFile, ft.TRaw:
					t = fuse.DT_File
				case ft.TSymlink:
					t = fuse.DT_Link
				case ft.TMetadata:
					log.Error("metadata object in fuse should contain its wrapped type")
				default:
					log.Error("unrecognized protonode data type: ", fsn.Type())
				}
			}
		}
		entries = append(entries, fuse.Dirent{Name: n, Type: t})
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(entries) > 0 {
		return entries, nil
	}
	return nil, fuse.ENOENT
}

func (s *Node) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	// TODO: is nil the right response for 'bug off, we ain't got none' ?
	resp.Xattr = nil
	return nil
}

func (s *Node) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	if s.cached == nil || s.cached.Type() != ft.TSymlink {
		return "", fuse.Errno(syscall.EINVAL)
	}
	return string(s.cached.Data()), nil
}

func (s *Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	r, err := uio.NewDagReader(ctx, s.Nd, s.Ipfs.DAG)
	if err != nil {
		return err
	}
	_, err = r.Seek(req.Offset, io.SeekStart)
	if err != nil {
		return err
	}
	// Data has a capacity of Size
	buf := resp.Data[:int(req.Size)]
	n, err := io.ReadFull(r, buf)
	resp.Data = buf[:n]
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
	default:
		return err
	}
	resp.Data = resp.Data[:n]
	return nil // may be non-nil / not succeeded
}

// to check that out Node implements all the interfaces we want
type roRoot interface {
	fs.Node
	fs.HandleReadDirAller
	fs.NodeStringLookuper
}

var _ roRoot = (*Root)(nil)

type roNode interface {
	fs.HandleReadDirAller
	fs.HandleReader
	fs.Node
	fs.NodeStringLookuper
	fs.NodeReadlinker
	fs.NodeGetxattrer
}

var _ roNode = (*Node)(nil)
