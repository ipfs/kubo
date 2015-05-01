// +build linux darwin freebsd
// +build !nofuse

package readonly

import (
	"io"
	"os"

	fuse "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/ipfs/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
	uio "github.com/ipfs/go-ipfs/unixfs/io"
	ftpb "github.com/ipfs/go-ipfs/unixfs/pb"
	lgbl "github.com/ipfs/go-ipfs/util/eventlog/loggables"
)

var log = eventlog.Logger("fuse/ipfs")

// FileSystem is the readonly Ipfs Fuse Filesystem.
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
func (*Root) Attr() fuse.Attr {
	return fuse.Attr{Mode: os.ModeDir | 0111} // -rw+x
}

// Lookup performs a lookup under this node.
func (s *Root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	log.Debugf("Root Lookup: '%s'", name)
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, fuse.ENOENT
	}

	nd, err := s.Ipfs.Resolver.ResolvePath(ctx, path.Path(name))
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return &Node{Ipfs: s.Ipfs, Nd: nd}, nil
}

// ReadDirAll reads a particular directory. Disallowed for root.
func (*Root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("Read Root.")
	return nil, fuse.EPERM
}

// Node is the core object representing a filesystem tree node.
type Node struct {
	Ipfs   *core.IpfsNode
	Nd     *mdag.Node
	fd     *uio.DagReader
	cached *ftpb.Data
}

func (s *Node) loadData() error {
	s.cached = new(ftpb.Data)
	return proto.Unmarshal(s.Nd.Data, s.cached)
}

// Attr returns the attributes of a given node.
func (s *Node) Attr() fuse.Attr {
	log.Debug("Node attr.")
	if s.cached == nil {
		s.loadData()
	}
	switch s.cached.GetType() {
	case ftpb.Data_Directory:
		return fuse.Attr{
			Mode: os.ModeDir | 0555,
			Uid:  uint32(os.Getuid()),
			Gid:  uint32(os.Getgid()),
		}
	case ftpb.Data_File:
		size := s.cached.GetFilesize()
		return fuse.Attr{
			Mode:   0444,
			Size:   uint64(size),
			Blocks: uint64(len(s.Nd.Links)),
			Uid:    uint32(os.Getuid()),
			Gid:    uint32(os.Getgid()),
		}
	case ftpb.Data_Raw:
		return fuse.Attr{
			Mode:   0444,
			Size:   uint64(len(s.cached.GetData())),
			Blocks: uint64(len(s.Nd.Links)),
			Uid:    uint32(os.Getuid()),
			Gid:    uint32(os.Getgid()),
		}

	default:
		log.Debug("Invalid data type.")
		return fuse.Attr{}
	}
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	log.Debugf("Lookup '%s'", name)
	nodes, err := s.Ipfs.Resolver.ResolveLinks(ctx, s.Nd, []string{name})
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return &Node{Ipfs: s.Ipfs, Nd: nodes[len(nodes)-1]}, nil
}

// ReadDirAll reads the link structure as directory entries
func (s *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debug("Node ReadDir")
	entries := make([]fuse.Dirent, len(s.Nd.Links))
	for i, link := range s.Nd.Links {
		n := link.Name
		if len(n) == 0 {
			n = link.Hash.B58String()
		}
		entries[i] = fuse.Dirent{Name: n, Type: fuse.DT_File}
	}

	if len(entries) > 0 {
		return entries, nil
	}
	return nil, fuse.ENOENT
}

func (s *Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {

	k, err := s.Nd.Key()
	if err != nil {
		return err
	}

	// setup our logging event
	lm := make(lgbl.DeferredMap)
	lm["fs"] = "ipfs"
	lm["key"] = func() interface{} { return k.Pretty() }
	lm["req_offset"] = req.Offset
	lm["req_size"] = req.Size
	defer log.EventBegin(ctx, "fuseRead", lm).Done()

	r, err := uio.NewDagReader(ctx, s.Nd, s.Ipfs.DAG)
	if err != nil {
		return err
	}
	o, err := r.Seek(req.Offset, os.SEEK_SET)
	lm["res_offset"] = o
	if err != nil {
		return err
	}

	buf := resp.Data[:min(req.Size, int(r.Size()-req.Offset))]
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF {
		return err
	}
	resp.Data = resp.Data[:n]
	lm["res_size"] = n
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
}

var _ roNode = (*Node)(nil)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
