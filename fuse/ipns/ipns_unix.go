// +build !nofuse

// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"

	core "github.com/jbenet/go-ipfs/core"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	path "github.com/jbenet/go-ipfs/path"
	ft "github.com/jbenet/go-ipfs/unixfs"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	mod "github.com/jbenet/go-ipfs/unixfs/mod"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	u "github.com/jbenet/go-ipfs/util"
	lgbl "github.com/jbenet/go-ipfs/util/eventlog/loggables"
)

const IpnsReadonly = true

var log = eventlog.Logger("fuse/ipns")

var (
	shortRepublishTimeout = time.Millisecond * 5
	longRepublishTimeout  = time.Millisecond * 500
)

// FileSystem is the readwrite IPNS Fuse Filesystem.
type FileSystem struct {
	Ipfs     *core.IpfsNode
	RootNode *Root
}

// NewFileSystem constructs new fs using given core.IpfsNode instance.
func NewFileSystem(ipfs *core.IpfsNode, sk ci.PrivKey, ipfspath string) (*FileSystem, error) {
	root, err := CreateRoot(ipfs, []ci.PrivKey{sk}, ipfspath)
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
			return nil, err
		}
		root.LocalLink = &Link{u.Key(hash).Pretty()}
	}

	for _, k := range keys {
		hash, err := k.GetPublic().Hash()
		if err != nil {
			log.Debug("failed to hash public key.")
			continue
		}
		name := u.Key(hash).Pretty()
		nd := new(Node)
		nd.Ipfs = n
		nd.key = k
		nd.repub = NewRepublisher(nd, shortRepublishTimeout, longRepublishTimeout)

		go nd.repub.Run()

		pointsTo, err := n.Namesys.Resolve(n.Context(), name)
		if err != nil {
			log.Warning("Could not resolve value for local ipns entry, providing empty dir")
			nd.Nd = &mdag.Node{Data: ft.FolderPBData()}
			root.LocalDirs[name] = nd
			continue
		}

		if !u.IsValidHash(pointsTo.B58String()) {
			log.Criticalf("Got back bad data from namesys resolve! [%s]", pointsTo)
			return nil, nil
		}

		node, err := n.Resolver.ResolvePath(path.Path(pointsTo.B58String()))
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
func (f FileSystem) Root() (fs.Node, error) {
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
func (s *Root) Lookup(ctx context.Context, name string) (fs.Node, error) {
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

	resolved, err := s.Ipfs.Namesys.Resolve(s.Ipfs.Context(), name)
	if err != nil {
		log.Warningf("ipns: namesys resolve error: %s", err)
		return nil, fuse.ENOENT
	}

	return &Link{s.IpfsRoot + "/" + resolved.B58String()}, nil
}

// ReadDirAll reads a particular directory. Disallowed for root.
func (r *Root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
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
	root   *Root
	nsRoot *Node
	parent *Node

	repub *Republisher

	// This nodes name in its parent dir.
	// NOTE: this strategy wont work well if we allow hard links
	// (im all for murdering the thought of hard links)
	name string

	// Private keys held by nodes at the root of a keyspace
	// WARNING(security): the PrivKey interface is currently insecure
	// (holds the raw key). It will be secured later.
	key ci.PrivKey

	Ipfs   *core.IpfsNode
	Nd     *mdag.Node
	dagMod *mod.DagModifier
	cached *ftpb.Data
}

func (s *Node) loadData() error {
	s.cached = new(ftpb.Data)
	return proto.Unmarshal(s.Nd.Data, s.cached)
}

// Attr returns the attributes of a given node.
func (s *Node) Attr() fuse.Attr {
	if s.cached == nil {
		err := s.loadData()
		if err != nil {
			log.Debugf("Error loading PBData for file: '%s'", s.name)
		}
	}
	switch s.cached.GetType() {
	case ftpb.Data_Directory:
		return fuse.Attr{Mode: os.ModeDir | 0555}
	case ftpb.Data_File, ftpb.Data_Raw:
		size, err := ft.DataSize(s.Nd.Data)
		if err != nil {
			log.Debugf("Error getting size of file: %s", err)
			size = 0
		}
		if size == 0 {
			dmsize, err := s.dagMod.Size()
			if err != nil {
				log.Error(err)
			}
			size = uint64(dmsize)
		}

		mode := os.FileMode(0666)
		if IpnsReadonly {
			mode = 0444
		}

		return fuse.Attr{
			Mode:   mode,
			Size:   size,
			Blocks: uint64(len(s.Nd.Links)),
		}
	default:
		log.Debug("Invalid data type.")
		return fuse.Attr{}
	}
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	nodes, err := s.Ipfs.Resolver.ResolveLinks(s.Nd, []string{name})
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return s.makeChild(name, nodes[len(nodes)-1]), nil
}

func (n *Node) makeChild(name string, node *mdag.Node) *Node {
	child := &Node{
		Ipfs:   n.Ipfs,
		Nd:     node,
		name:   name,
		nsRoot: n.nsRoot,
		parent: n,
	}

	// Always ensure that each child knows where the root is
	if n.nsRoot == nil {
		child.nsRoot = n
	} else {
		child.nsRoot = n.nsRoot
	}

	return child
}

// ReadDirAll reads the link structure as directory entries
func (s *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
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
	lm["fs"] = "ipns"
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

	buf := resp.Data[:min(req.Size, int(r.Size()))]
	n, err := io.ReadFull(r, buf)
	resp.Data = resp.Data[:n]
	lm["res_size"] = n
	return err // may be non-nil / not succeeded
}

func (n *Node) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// log.Debugf("ipns: Node Write [%s]: flags = %s, offset = %d, size = %d", n.name, req.Flags.String(), req.Offset, len(req.Data))
	if IpnsReadonly {
		log.Debug("Attempted to write on readonly ipns filesystem.")
		return fuse.EPERM
	}

	if n.dagMod == nil {
		// Create a DagModifier to allow us to change the existing dag node
		dmod, err := mod.NewDagModifier(ctx, n.Nd, n.Ipfs.DAG, n.Ipfs.Pinning.GetManual(), chunk.DefaultSplitter)
		if err != nil {
			return err
		}
		n.dagMod = dmod
	}
	wrote, err := n.dagMod.WriteAt(req.Data, int64(req.Offset))
	if err != nil {
		return err
	}
	resp.Size = wrote
	return nil
}

func (n *Node) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	if IpnsReadonly {
		return nil
	}

	// If a write has happened
	if n.dagMod != nil {
		newNode, err := n.dagMod.GetNode()
		if err != nil {
			return err
		}

		if n.parent != nil {
			log.Error("updating self in parent!")
			err := n.parent.update(n.name, newNode)
			if err != nil {
				log.Criticalf("error in updating ipns dag tree: %s", err)
				// return fuse.ETHISISPRETTYBAD
				return err
			}
		}
		n.Nd = newNode

		/*/TEMP
		dr, err := mdag.NewDagReader(n.Nd, n.Ipfs.DAG)
		if err != nil {
			log.Critical("Verification read failed.")
		}
		b, err := ioutil.ReadAll(dr)
		if err != nil {
			log.Critical("Verification read failed.")
		}
		fmt.Println("VERIFICATION READ")
		fmt.Printf("READ %d BYTES\n", len(b))
		fmt.Println(string(b))
		fmt.Println(b)
		//*/

		n.dagMod = nil

		n.wasChanged()
	}
	return nil
}

// Signal that a node in this tree was changed so the root can republish
func (n *Node) wasChanged() {
	if IpnsReadonly {
		return
	}
	root := n.nsRoot
	if root == nil {
		root = n
	}

	root.repub.Publish <- struct{}{}
}

func (n *Node) republishRoot() error {

	// We should already be the root, this is just a sanity check
	var root *Node
	if n.nsRoot != nil {
		root = n.nsRoot
	} else {
		root = n
	}

	// Add any nodes that may be new to the DAG service
	err := n.Ipfs.DAG.AddRecursive(root.Nd)
	if err != nil {
		log.Criticalf("ipns: Dag Add Error: %s", err)
		return err
	}

	ndkey, err := root.Nd.Key()
	if err != nil {
		return err
	}

	err = n.Ipfs.Namesys.Publish(n.Ipfs.Context(), root.key, ndkey)
	if err != nil {
		return err
	}
	return nil
}

func (n *Node) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

func (n *Node) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	if IpnsReadonly {
		return nil, fuse.EPERM
	}
	dagnd := &mdag.Node{Data: ft.FolderPBData()}
	nnode := n.Nd.Copy()
	nnode.AddNodeLink(req.Name, dagnd)

	child := &Node{
		Ipfs: n.Ipfs,
		Nd:   dagnd,
		name: req.Name,
	}

	if n.nsRoot == nil {
		child.nsRoot = n
	} else {
		child.nsRoot = n.nsRoot
	}

	if n.parent != nil {
		err := n.parent.update(n.name, nnode)
		if err != nil {
			log.Criticalf("Error updating node: %s", err)
			return nil, err
		}
	}
	n.Nd = nnode

	n.wasChanged()

	return child, nil
}

func (n *Node) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	//log.Debug("[%s] Received open request! flags = %s", n.name, req.Flags.String())
	//TODO: check open flags and truncate if necessary
	if req.Flags&fuse.OpenTruncate != 0 {
		log.Warning("Need to truncate file!")
		n.cached = nil
		n.Nd = &mdag.Node{Data: ft.FilePBData(nil, 0)}
	} else if req.Flags&fuse.OpenAppend != 0 {
		log.Warning("Need to append to file!")
	}
	return n, nil
}

func (n *Node) Mknod(ctx context.Context, req *fuse.MknodRequest) (fs.Node, error) {
	return nil, nil
}

func (n *Node) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if IpnsReadonly {
		log.Debug("Attempted to call Create on a readonly filesystem.")
		return nil, nil, fuse.EPERM
	}

	// New 'empty' file
	nd := &mdag.Node{Data: ft.FilePBData(nil, 0)}
	child := n.makeChild(req.Name, nd)

	nnode := n.Nd.Copy()

	err := nnode.AddNodeLink(req.Name, nd)
	if err != nil {
		return nil, nil, err
	}
	if n.parent != nil {
		err := n.parent.update(n.name, nnode)
		if err != nil {
			log.Criticalf("Error updating node: %s", err)
			// Can we panic, please?
			return nil, nil, err
		}
	}
	n.Nd = nnode
	n.wasChanged()

	return child, child, nil
}

func (n *Node) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	if IpnsReadonly {
		return fuse.EPERM
	}

	nnode := n.Nd.Copy()
	err := nnode.RemoveNodeLink(req.Name)
	if err != nil {
		return fuse.ENOENT
	}

	if n.parent != nil {
		err := n.parent.update(n.name, nnode)
		if err != nil {
			log.Criticalf("Error updating node: %s", err)
			return err
		}
	}
	n.Nd = nnode
	n.wasChanged()
	return nil
}

func (n *Node) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	if IpnsReadonly {
		log.Debug("Attempted to call Rename on a readonly filesystem.")
		return fuse.EPERM
	}

	var mdn *mdag.Node
	for _, l := range n.Nd.Links {
		if l.Name == req.OldName {
			mdn = l.Node
		}
	}
	if mdn == nil {
		log.Critical("nil Link found on rename!")
		return fuse.ENOENT
	}
	n.Nd.RemoveNodeLink(req.OldName)

	switch newDir := newDir.(type) {
	case *Node:
		err := newDir.Nd.AddNodeLink(req.NewName, mdn)
		if err != nil {
			return err
		}
	default:
		log.Critical("Unknown node type for rename target dir!")
		return errors.New("Unknown fs node type!")
	}
	return nil
}

// Updates the child of this node, specified by name to the given newnode
func (n *Node) update(name string, newnode *mdag.Node) error {
	nnode, err := n.Nd.UpdateNodeLink(name, newnode)
	if err != nil {
		return err
	}

	if n.parent != nil {
		err := n.parent.update(n.name, nnode)
		if err != nil {
			return err
		}
	}
	n.Nd = nnode
	return nil
}

// to check that out Node implements all the interfaces we want
type ipnsRoot interface {
	fs.Node
	fs.HandleReadDirAller
	fs.NodeStringLookuper
}

var _ ipnsRoot = (*Root)(nil)

type ipnsNode interface {
	fs.HandleFlusher
	fs.HandleReadDirAller
	fs.HandleReader
	fs.HandleWriter
	fs.Node
	fs.NodeCreater
	fs.NodeFsyncer
	fs.NodeMkdirer
	fs.NodeMknoder
	fs.NodeOpener
	fs.NodeRemover
	fs.NodeRenamer
	fs.NodeStringLookuper
}

var _ ipnsNode = (*Node)(nil)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
