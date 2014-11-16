// +build linux darwin freebsd

// package fuse/readonly implements a fuse filesystem to access files
// stored inside of ipfs.
package readonly

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"time"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	core "github.com/jbenet/go-ipfs/core"
	mount "github.com/jbenet/go-ipfs/fuse/mount"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("ipfs")

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

	nd, err := s.Ipfs.Resolver.ResolvePath(name)
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
		log.Error("Invalid data type.")
		return fuse.Attr{}
	}
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debugf("Lookup '%s'", name)
	nd, err := s.Ipfs.Resolver.ResolveLinks(s.Nd, []string{name})
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return &Node{Ipfs: s.Ipfs, Nd: nd}, nil
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

// ReadAll reads the object data as file data
func (s *Node) ReadAll(intr fs.Intr) ([]byte, fuse.Error) {
	log.Debug("Read node.")
	r, err := uio.NewDagReader(s.Nd, s.Ipfs.DAG)
	if err != nil {
		return nil, err
	}
	// this is a terrible function... 'ReadAll'?
	// what if i have a 6TB file? GG RAM.
	return ioutil.ReadAll(r)
}

// Mount mounts an IpfsNode instance at a particular path. It
// serves until the process receives exit signals (to Unmount).
func Mount(ipfs *core.IpfsNode, fpath string) (mount.Mount, error) {
	log.Infof("Mounting ipfs at %s...", fpath)

	// setup the Mount abstraction.
	m := mount.New(ipfs.Context(), fpath)

	// go serve the mount
	m.Mount(func(m mount.Mount) error {
		return internalMount(ipfs, m)
	}, internalUnmount)

	select {
	case <-m.Closed():
		return nil, fmt.Errorf("failed to mount")
	case <-time.After(time.Second):
		// assume it worked...
	}

	// bind the mount (ContextCloser) to the node, so that when the node exits
	// the fsclosers are automatically closed.
	ipfs.AddCloserChild(m)
	return m, nil
}

// mount attempts to mount the provided FUSE mount point
func internalMount(ipfs *core.IpfsNode, m mount.Mount) error {
	c, err := fuse.Mount(m.MountPoint())
	if err != nil {
		return err
	}
	defer c.Close()

	fsys := FileSystem{Ipfs: ipfs}

	log.Infof("Mounted ipfs at %s.", m.MountPoint())
	if err := fs.Serve(c, fsys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		m.Unmount()
		return err
	}
	return nil
}

// unmount attempts to unmount the provided FUSE mount point, forcibly
// if necessary.
func internalUnmount(m mount.Mount) error {
	point := m.MountPoint()
	log.Infof("Unmounting ipfs at %s...", point)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("diskutil", "umount", "force", point)
	case "linux":
		cmd = exec.Command("fusermount", "-u", point)
	default:
		return fmt.Errorf("unmount: unimplemented")
	}

	errc := make(chan error, 1)
	go func() {
		if err := exec.Command("umount", point).Run(); err == nil {
			errc <- err
		}
		// retry to unmount with the fallback cmd
		errc <- cmd.Run()
	}()

	select {
	case <-time.After(1 * time.Second):
		return fmt.Errorf("umount timeout")
	case err := <-errc:
		return err
	}
}
