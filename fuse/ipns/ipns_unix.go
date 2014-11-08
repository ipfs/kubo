// package fuse/ipns implements a fuse filesystem that interfaces
// with ipns, the naming system for ipfs.
package ipns

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	fuse "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	fs "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	core "github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("ipns")

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
func NewIpns(ipfs *core.IpfsNode, ipfspath string) (*FileSystem, error) {
	root, err := CreateRoot(ipfs, []ci.PrivKey{ipfs.Identity.PrivKey()}, ipfspath)
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
			log.Errorf("Read Root Error: %s", err)
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
		nd.repub = NewRepublisher(nd, shortRepublishTimeout, longRepublishTimeout)

		go nd.repub.Run()

		pointsTo, err := n.Namesys.Resolve(name)
		if err != nil {
			log.Warning("Could not resolve value for local ipns entry, providing empty dir")
			nd.Nd = &mdag.Node{Data: ft.FolderPBData()}
			root.LocalDirs[name] = nd
			continue
		}

		if !u.IsValidHash(pointsTo) {
			log.Criticalf("Got back bad data from namesys resolve! [%s]", pointsTo)
			return nil, nil
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
	log.Debugf("ipns: Root Lookup: '%s'", name)
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

	log.Debugf("ipns: Falling back to resolution for [%s].", name)
	resolved, err := s.Ipfs.Namesys.Resolve(name)
	if err != nil {
		log.Warningf("ipns: namesys resolve error: %s", err)
		return nil, fuse.ENOENT
	}

	return &Link{s.IpfsRoot + "/" + resolved}, nil
}

// ReadDir reads a particular directory. Disallowed for root.
func (r *Root) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	log.Debug("Read Root.")
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
			log.Errorf("Read Root Error: %s", err)
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
	dagMod *uio.DagModifier
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
			log.Errorf("Error loading PBData for file: '%s'", s.name)
		}
	}
	switch s.cached.GetType() {
	case ftpb.Data_Directory:
		return fuse.Attr{Mode: os.ModeDir | 0555}
	case ftpb.Data_File, ftpb.Data_Raw:
		size, err := ft.DataSize(s.Nd.Data)
		if err != nil {
			log.Errorf("Error getting size of file: %s", err)
			size = 0
		}
		if size == 0 {
			size = s.dagMod.Size()
		}
		return fuse.Attr{
			Mode:   0666,
			Size:   size,
			Blocks: uint64(len(s.Nd.Links)),
		}
	default:
		log.Error("Invalid data type.")
		return fuse.Attr{}
	}
}

// Lookup performs a lookup under this node.
func (s *Node) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debugf("ipns: node[%s] Lookup '%s'", s.name, name)
	nd, err := s.Ipfs.Resolver.ResolveLinks(s.Nd, []string{name})
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return s.makeChild(name, nd), nil
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
	log.Debugf("ipns: ReadAll [%s]", s.name)
	r, err := uio.NewDagReader(s.Nd, s.Ipfs.DAG)
	if err != nil {
		return nil, err
	}
	// this is a terrible function... 'ReadAll'?
	// what if i have a 6TB file? GG RAM.
	b, err := ioutil.ReadAll(r)
	if err != nil {
		log.Errorf("[%s] Readall error: %s", s.name, err)
		return nil, err
	}
	return b, nil
}

func (n *Node) Write(req *fuse.WriteRequest, resp *fuse.WriteResponse, intr fs.Intr) fuse.Error {
	log.Debugf("ipns: Node Write [%s]: flags = %s, offset = %d, size = %d", n.name, req.Flags.String(), req.Offset, len(req.Data))

	if n.dagMod == nil {
		// Create a DagModifier to allow us to change the existing dag node
		dmod, err := uio.NewDagModifier(n.Nd, n.Ipfs.DAG, chunk.DefaultSplitter)
		if err != nil {
			log.Errorf("Error creating dag modifier: %s", err)
			return err
		}
		n.dagMod = dmod
	}
	wrote, err := n.dagMod.WriteAt(req.Data, uint64(req.Offset))
	if err != nil {
		return err
	}
	resp.Size = wrote
	return nil
}

func (n *Node) Flush(req *fuse.FlushRequest, intr fs.Intr) fuse.Error {
	log.Debugf("Got flush request [%s]!", n.name)

	// If a write has happened
	if n.dagMod != nil {
		newNode, err := n.dagMod.GetNode()
		if err != nil {
			log.Errorf("Error getting dag node from dagMod: %s", err)
			return err
		}

		if n.parent != nil {
			log.Debug("updating self in parent!")
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
	root := n.nsRoot
	if root == nil {
		root = n
	}

	root.repub.Publish <- struct{}{}
}

func (n *Node) republishRoot() error {
	log.Debug("Republish root")

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
		log.Errorf("getKey error: %s", err)
		return err
	}
	log.Debug("Publishing changes!")

	err = n.Ipfs.Namesys.Publish(root.key, ndkey.Pretty())
	if err != nil {
		log.Errorf("ipns: Publish Failed: %s", err)
		return err
	}
	return nil
}

func (n *Node) Fsync(req *fuse.FsyncRequest, intr fs.Intr) fuse.Error {
	log.Debug("Got fsync request!")
	return nil
}

func (n *Node) Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debug("Got mkdir request!")
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

func (n *Node) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
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

func (n *Node) Mknod(req *fuse.MknodRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	log.Debug("Got mknod request!")
	return nil, nil
}

func (n *Node) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	log.Debugf("Got create request: %s", req.Name)

	// New 'empty' file
	nd := &mdag.Node{Data: ft.FilePBData(nil, 0)}
	child := n.makeChild(req.Name, nd)

	nnode := n.Nd.Copy()

	err := nnode.AddNodeLink(req.Name, nd)
	if err != nil {
		log.Errorf("Error adding child to node: %s", err)
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

func (n *Node) Remove(req *fuse.RemoveRequest, intr fs.Intr) fuse.Error {
	log.Debugf("[%s] Got Remove request: %s", n.name, req.Name)
	nnode := n.Nd.Copy()
	err := nnode.RemoveNodeLink(req.Name)
	if err != nil {
		log.Error("Remove: No such file.")
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

func (n *Node) Rename(req *fuse.RenameRequest, newDir fs.Node, intr fs.Intr) fuse.Error {
	log.Debugf("Got Rename request '%s' -> '%s'", req.OldName, req.NewName)
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
			log.Errorf("Error adding node to new dir on rename: %s", err)
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
	log.Debugf("update '%s' in '%s'", name, n.name)
	nnode := n.Nd.Copy()
	err := nnode.RemoveNodeLink(name)
	if err != nil {
		return err
	}
	nnode.AddNodeLink(name, newnode)

	if n.parent != nil {
		err := n.parent.update(n.name, nnode)
		if err != nil {
			return err
		}
	}
	n.Nd = nnode
	return nil
}
