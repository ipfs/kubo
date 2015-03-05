package ipnsfs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	core "github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	ft "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("ipnsfs")

var ErrIsDirectory = errors.New("error: is a directory")

var ErrNoSuch = errors.New("no such file or directory")

// Filesystem is the writeable fuse filesystem structure
type Filesystem struct {
	nd    *core.IpfsNode
	roots map[string]*KeyRoot

	// A journal (TO BE IMPLEMENTED)
	journal interface{}
}

func NewFilesystem(nd *core.IpfsNode, keys ...ci.PrivKey) (*Filesystem, error) {
	roots := make(map[string]*KeyRoot)
	for _, k := range keys {
		pkh, err := k.GetPublic().Hash()
		if err != nil {
			return nil, err
		}

		root, err := NewKeyRoot(nd, k)
		if err != nil {
			return nil, err
		}
		roots[u.Key(pkh).Pretty()] = root
	}
	return &Filesystem{
		nd:    nd,
		roots: roots,
	}, nil
}

func (fs *Filesystem) Open(tpath string, mode int) (File, error) {
	pathelem := strings.Split(tpath, "/")
	r, ok := fs.roots[pathelem[0]]
	if !ok {
		return nil, ErrNoSuch
	}

	return r.Open(pathelem[1:], mode)
}

func (fs *Filesystem) GetRoot(name string) (*KeyRoot, error) {
	r, ok := fs.roots[name]
	if ok {
		return r, nil
	}
	return nil, ErrNoSuch
}

type FSNode interface {
	GetNode() (*dag.Node, error)
}

// KeyRoot represents the root of a filesystem tree pointed to by a given keypair
type KeyRoot struct {
	key ci.PrivKey

	// node is the merkledag node pointed to by this keypair
	node *dag.Node

	//
	corenode *core.IpfsNode

	// val represents the node pointed to by this key. It can either be a File or a Directory
	val FSNode

	repub *Republisher
}

func NewKeyRoot(nd *core.IpfsNode, k ci.PrivKey) (*KeyRoot, error) {
	hash, err := k.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	name := u.Key(hash).Pretty()

	root := new(KeyRoot)
	root.key = k
	root.corenode = nd

	ctx, cancel := context.WithCancel(nd.Context())
	defer cancel()

	pointsTo, err := nd.Namesys.Resolve(ctx, name)
	if err != nil {
		err = InitializeKeyspace(nd, k)
		if err != nil {
			return nil, err
		}

		pointsTo, err = nd.Namesys.Resolve(ctx, name)
		if err != nil {
			return nil, err
		}
	}

	mnode, err := nd.DAG.Get(pointsTo)
	if err != nil {
		return nil, err
	}

	root.node = mnode

	root.repub = NewRepublisher(root, time.Millisecond*300, time.Second*3)
	go root.repub.Run()

	pbn, err := ft.FromBytes(mnode.Data)
	if err != nil {
		log.Error("IPNS pointer was not unixfs node")
		return nil, err
	}

	switch pbn.GetType() {
	case ft.TDirectory:
		root.val = NewDirectory(pointsTo.B58String(), mnode, root, nd.DAG)
	case ft.TFile, ft.TMetadata, ft.TRaw:
		fi, err := NewFile(pointsTo.B58String(), mnode, root, nd.DAG)
		if err != nil {
			return nil, err
		}
		root.val = fi
	default:
		panic("unrecognized! (NYI)")
	}
	return root, nil
}

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	emptyDir := &dag.Node{Data: ft.FolderPBData()}
	nodek, err := n.DAG.Add(emptyDir)
	if err != nil {
		return err
	}

	err = n.Pinning.Pin(emptyDir, false)
	if err != nil {
		return err
	}

	err = n.Pinning.Flush()
	if err != nil {
		return err
	}

	err = n.Namesys.Publish(n.Context(), key, nodek)
	if err != nil {
		return err
	}

	return nil
}

func (kr *KeyRoot) GetValue() FSNode {
	return kr.val
}

func (kr *KeyRoot) Open(tpath []string, mode int) (File, error) {
	if kr.val == nil {
		// No entry here... what should we do?
		panic("nyi")
	}
	if len(tpath) > 0 {
		// Make sure our root is a directory
		dir, ok := kr.val.(*Directory)
		if !ok {
			return nil, fmt.Errorf("no such file or directory: %s", tpath[0])
		}

		return dir.Open(tpath, mode)
	}

	switch t := kr.val.(type) {
	case *Directory:
		return nil, ErrIsDirectory
	case File:
		return t, nil
	default:
		panic("unrecognized type, should not happen")
	}
}

// closeChild implements the childCloser interface, and signals to the publisher that
// there are changes ready to be published
func (kr *KeyRoot) closeChild(name string) error {
	kr.repub.Touch()
	return nil
}

// Publish publishes the ipns entry associated with this key
func (kr *KeyRoot) Publish() error {
	child, ok := kr.val.(FSNode)
	if !ok {
		return errors.New("child of key root not valid type")
	}

	nd, err := child.GetNode()
	if err != nil {
		return err
	}

	k, err := kr.corenode.DAG.Add(nd)
	if err != nil {
		return err
	}

	fmt.Println("Publishing!")
	return kr.corenode.Namesys.Publish(kr.corenode.Context(), kr.key, k)
}

// Republisher manages when to publish the ipns entry associated with a given key
type Republisher struct {
	TimeoutLong  time.Duration
	TimeoutShort time.Duration
	Publish      chan struct{}
	root         *KeyRoot
}

func NewRepublisher(root *KeyRoot, tshort, tlong time.Duration) *Republisher {
	return &Republisher{
		TimeoutShort: tshort,
		TimeoutLong:  tlong,
		Publish:      make(chan struct{}, 1),
		root:         root,
	}
}

func (np *Republisher) Touch() {
	select {
	case np.Publish <- struct{}{}:
	default:
	}
}

func (np *Republisher) Run() {
	for _ = range np.Publish {
		quick := time.After(np.TimeoutShort)
		longer := time.After(np.TimeoutLong)

	wait:
		select {
		case <-quick:
		case <-longer:
		case <-np.Publish:
			quick = time.After(np.TimeoutShort)
			goto wait
		}

		log.Info("Publishing Changes!")
		err := np.root.Publish()
		if err != nil {
			log.Critical("republishRoot error: %s", err)
		}

	}
}
