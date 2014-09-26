package ipns

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"bytes"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"code.google.com/p/goprotobuf/proto"
	"github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	imp "github.com/jbenet/go-ipfs/importer"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("ipns")

// FileSystem is the readonly Ipfs Fuse Filesystem.
type FileSystem struct {
	Ipfs     *core.IpfsNode
	RootNode *Root
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewIpns(ipfs *core.IpfsNode, ipfspath string) (*FileSystem, error) {
	root, err := CreateRoot(ipfs, []ci.PrivKey{ipfs.Identity.PrivKey}, ipfspath)
	if err != nil {
		return nil, err
	}
	return &FileSystem{Ipfs: ipfs, RootNode: root}, nil
}

func CreateRoot(n *core.IpfsNode, keys []ci.PrivKey, ipfsroot string) (*Root, error) {
	root := new(Root)
	root.LocalDirs = make(map[string]*Node)
	root.Ipfs = n
	abspath, err := filepath.Abs(ipfsroot)
	if err != nil {
		return nil, err
	}
	root.IpfsRoot = abspath

	root.Keys = keys

	if len(keys) == 0 {
		log.Warning("No keys given for ipns root creation")
	} else {
		k := keys[0]
		pub := k.GetPublic()
		hash, err := pub.Hash()
		if err != nil {
			log.Error("Read Root Error: %s", err)
			return nil, err
		}
		root.LocalLink = &Link{u.Key(hash).Pretty()}
	}

	for _, k := range keys {
		hash, err := k.GetPublic().Hash()
		if err != nil {
			log.Error("failed to hash public key.")
			continue
		}
		name := u.Key(hash).Pretty()
		nd := new(Node)
		nd.Ipfs = n
		nd.key = k

		pointsTo, err := n.Namesys.Resolve(name)
		if err != nil {
			log.Warning("Could not resolve value for local ipns entry")
			continue
		}

		node, err := n.Resolver.ResolvePath(pointsTo)
		if err != nil {
			log.Warning("Failed to resolve value from ipns entry in ipfs")
			continue
		}

		nd.Nd = node
		root.LocalDirs[name] = nd
	}

	return root, nil
}

// Root constructs the Root of the filesystem, a Root object.
func (f FileSystem) Root() (fs.Node, fuse.Error) {
	return f.RootNode, nil
}

// Root is the root object of the filesystem tree.
type Root struct {
	Ipfs *core.IpfsNode
	Keys []ci.PrivKey

	// Used for symlinking into ipfs
	IpfsRoot  string
	LocalDirs map[string]*Node

	LocalLink *Link
}

// Attr returns file attributes.
func (*Root) Attr() fuse.Attr {
	return fuse.Attr{Mode: os.ModeDir | 0111} // -rw+x
}

// Lookup performs a lookup under this node.
func (s *Root) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debug("ipns: Root Lookup: '%s'", name)
	switch name {
	case "mach_kernel", ".hidden", "._.":
		// Just quiet some log noise on OS X.
		return nil, fuse.ENOENT
	}

	if name == "local" {
		if s.LocalLink == nil {
			return nil, fuse.ENOENT
		}
		return s.LocalLink, nil
	}

	nd, ok := s.LocalDirs[name]
	if ok {
		return nd, nil
	}

	log.Debug("ipns: Falling back to resolution.")
	resolved, err := s.Ipfs.Namesys.Resolve(name)
	if err != nil {
		log.Error("ipns: namesys resolve error: %s", err)
		return nil, fuse.ENOENT
	}

	return &Link{s.IpfsRoot + "/" + resolved}, nil
}

// ReadDir reads a particular directory. Disallowed for root.
func (r *Root) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	u.DOut("Read Root.\n")
	listing := []fuse.Dirent{
		fuse.Dirent{
			Name: "local",
			Type: fuse.DT_Link,
		},
	}
	for _, k := range r.Keys {
		pub := k.GetPublic()
		hash, err := pub.Hash()
		if err != nil {
			log.Error("Read Root Error: %s", err)
			continue
		}
		ent := fuse.Dirent{
			Name: u.Key(hash).Pretty(),
			Type: fuse.DT_Dir,
		}
		listing = append(listing, ent)
	}
	return listing, nil
}

// Node is the core object representing a filesystem tree node.
type Node struct {
	nsRoot *Node

	// Private keys held by nodes at the root of a keyspace
	key ci.PrivKey

	Ipfs   *core.IpfsNode
	Nd     *mdag.Node
	fd     *mdag.DagReader
	cached *mdag.PBData

	dataBuf *bytes.Buffer
	changed bool
}

func (s *Node) loadData() error {
	s.cached = new(mdag.PBData)
	return proto.Unmarshal(s.Nd.Data, s.cached)
}

// Attr returns the attributes of a given node.
func (s *Node) Attr() fuse.Attr {
	u.DOut("Node attr.\n")
	if s.cached == nil {
		s.loadData()
	}
	switch s.cached.GetType() {
	case mdag.PBData_Directory:
		u.DOut("this is a directory.\n")
		return fuse.Attr{Mode: os.ModeDir | 0555}
	case mdag.PBData_File, mdag.PBData_Raw:
		u.DOut("this is a file.\n")
		size, _ := s.Nd.Size()
		return fuse.Attr{
			Mode:   0666,
			Size:   uint64(size),
			Blocks: uint64(len(s.Nd.Links)),
		}
	default:
		u.PErr("Invalid data type.")
		return fuse.Attr{}
	}
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debug("ipns node Lookup '%s'", name)
	nd, err := s.Ipfs.Resolver.ResolveLinks(s.Nd, []string{name})
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	child := &Node{
		Ipfs: s.Ipfs,
		Nd:   nd,
	}

	if s.nsRoot == nil {
		child.nsRoot = s
	} else {
		child.nsRoot = s.nsRoot
	}

	return child, nil
}

// ReadDir reads the link structure as directory entries
func (s *Node) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	u.DOut("Node ReadDir\n")
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
	log.Debug("ipns: ReadAll Node")
	r, err := mdag.NewDagReader(s.Nd, s.Ipfs.DAG)
	if err != nil {
		return nil, err
	}
	// this is a terrible function... 'ReadAll'?
	// what if i have a 6TB file? GG RAM.
	return ioutil.ReadAll(r)
}

func (n *Node) Write(req *fuse.WriteRequest, resp *fuse.WriteResponse, intr fs.Intr) fuse.Error {
	if n.dataBuf == nil {
		n.dataBuf = new(bytes.Buffer)
	}
	log.Debug("ipns Node Write: flags = %s, offset = %d, size = %d", req.Flags.String(), req.Offset, len(req.Data))
	if req.Offset == 0 {
		n.dataBuf.Reset()
		n.dataBuf.Write(req.Data)
		n.changed = true
		resp.Size = len(req.Data)
	} else {
		log.Error("Unhandled write to offset!")
	}
	return nil
}

func (n *Node) Flush(req *fuse.FlushRequest, intr fs.Intr) fuse.Error {
	log.Debug("Got flush request!")

	if n.changed {
		//TODO:
		// This operation holds everything in memory,
		// should be changed to stream the block creation/storage
		// but for now, since the buf is all in memory anyways...
		nnode, err := imp.NewDagFromReader(n.dataBuf)
		if err != nil {
			log.Error("ipns Flush error: %s", err)
			// return fuse.EVERYBAD
			return fuse.ENODATA
		}

		err = n.Ipfs.DAG.AddRecursive(nnode)
		if err != nil {
			log.Critical("ipns Dag Add Error: %s", err)
		}

		n.Nd = nnode
		n.changed = false
		n.dataBuf = nil

		ndkey, err := nnode.Key()
		if err != nil {
			log.Error("getKey error: %s", err)
			// return fuse.ETHISREALLYSUCKS
			return fuse.ENODATA
		}
		log.Debug("Publishing changes!")

		err = n.Ipfs.Publisher.Publish(n.key, ndkey)
		if err != nil {
			log.Error("ipns Publish Failed: %s", err)
		}
	}
	return nil
}

func (n *Node) Fsync(req *fuse.FsyncRequest, intr fs.Intr) fuse.Error {
	log.Debug("Got fsync request!")
	return nil
}

// Mount mounts an IpfsNode instance at a particular path. It
// serves until the process receives exit signals (to Unmount).
func Mount(ipfs *core.IpfsNode, fpath string, ipfspath string) error {

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sigc
		for {
			err := Unmount(fpath)
			if err == nil {
				return
			}
			time.Sleep(time.Millisecond * 10)
		}
		ipfs.Network.Close()
	}()

	c, err := fuse.Mount(fpath)
	if err != nil {
		return err
	}
	defer c.Close()

	fsys, err := NewIpns(ipfs, ipfspath)
	if err != nil {
		return err
	}

	err = fs.Serve(c, fsys)
	if err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}
	return nil
}

// Unmount attempts to unmount the provided FUSE mount point, forcibly
// if necessary.
func Unmount(point string) error {
	fmt.Printf("Unmounting %s...\n", point)

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

type Link struct {
	Target string
}

func (l *Link) Attr() fuse.Attr {
	log.Debug("Link attr.")
	return fuse.Attr{
		Mode: os.ModeSymlink | 0555,
	}
}

func (l *Link) Readlink(req *fuse.ReadlinkRequest, intr fs.Intr) (string, fuse.Error) {
	return l.Target, nil
}
