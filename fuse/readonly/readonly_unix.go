// +build linux darwin freebsd

// package fuse/readonly implements a fuse filesystem to access files
// stored inside of ipfs.
package readonly

import (
	"io"
	"os"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	core "github.com/jbenet/go-ipfs/core"
	path "github.com/jbenet/go-ipfs/path"
	mdag "github.com/jbenet/go-ipfs/struct/merkledag"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
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
func (f FileSystem) Root() (fs.Node, fuse.Error) {
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
func (s *Root) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debugf("Root Lookup: '%s'", name)
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, fuse.ENOENT
	}

	nd, err := s.Ipfs.Resolver.ResolvePath(path.Path(name))
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return &Node{Ipfs: s.Ipfs, Nd: nd}, nil
}

// ReadDir reads a particular directory. Disallowed for root.
func (*Root) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
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
		return fuse.Attr{Mode: os.ModeDir | 0555}
	case ftpb.Data_File:
		size := s.cached.GetFilesize()
		return fuse.Attr{
			Mode:   0444,
			Size:   uint64(size),
			Blocks: uint64(len(s.Nd.Links)),
		}
	case ftpb.Data_Raw:
		return fuse.Attr{
			Mode:   0444,
			Size:   uint64(len(s.cached.GetData())),
			Blocks: uint64(len(s.Nd.Links)),
		}

	default:
		log.Debug("Invalid data type.")
		return fuse.Attr{}
	}
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debugf("Lookup '%s'", name)
	nodes, err := s.Ipfs.Resolver.ResolveLinks(s.Nd, []string{name})
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return &Node{Ipfs: s.Ipfs, Nd: nodes[len(nodes)-1]}, nil
}

// ReadDir reads the link structure as directory entries
func (s *Node) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
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

func (s *Node) Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error {
	// intr will be closed by fuse if the request is cancelled. turn this into a context.
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel() // make sure all operations we started close.

	// we wait on intr and cancel our context if it closes.
	go func() {
		select {
		case <-intr: // closed by fuse
			cancel() // cancel our context
		case <-ctx.Done():
		}
	}()

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
	n, err := io.ReadFull(r, resp.Data[:req.Size])
	resp.Data = resp.Data[:n]
	lm["res_size"] = n
	return err // may be non-nil / not succeeded
}
