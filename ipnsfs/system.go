// package ipnsfs implements an in memory model of a mutable ipns filesystem,
// to be used by the fuse filesystem.
//
// It consists of four main structs:
// 1) The Filesystem
//        The filesystem serves as a container and entry point for the ipns filesystem
// 2) KeyRoots
//        KeyRoots represent the root of the keyspace controlled by a given keypair
// 3) Directories
// 4) Files
package ipnsfs

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	namesys "github.com/ipfs/go-ipfs/namesys"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("ipnsfs")

var ErrIsDirectory = errors.New("error: is a directory")

// Filesystem is the writeable fuse filesystem structure
type Filesystem struct {
	dserv dag.DAGService

	nsys namesys.NameSystem

	resolver *path.Resolver

	pins pin.Pinner

	roots map[string]*KeyRoot
}

// NewFilesystem instantiates an ipns filesystem using the given parameters and locally owned keys
func NewFilesystem(ctx context.Context, ds dag.DAGService, nsys namesys.NameSystem, pins pin.Pinner, keys ...ci.PrivKey) (*Filesystem, error) {
	roots := make(map[string]*KeyRoot)
	fs := &Filesystem{
		roots:    roots,
		nsys:     nsys,
		dserv:    ds,
		pins:     pins,
		resolver: &path.Resolver{DAG: ds},
	}
	for _, k := range keys {
		pkh, err := k.GetPublic().Hash()
		if err != nil {
			return nil, err
		}

		root, err := fs.newKeyRoot(ctx, k)
		if err != nil {
			return nil, err
		}
		roots[u.Key(pkh).Pretty()] = root
	}

	return fs, nil
}

func (fs *Filesystem) Close() error {
	wg := sync.WaitGroup{}
	for _, r := range fs.roots {
		wg.Add(1)
		go func(r *KeyRoot) {
			defer wg.Done()
			err := r.Publish(context.TODO())
			if err != nil {
				log.Error(err)
				return
			}
		}(r)
	}
	wg.Wait()
	return nil
}

// GetRoot returns the KeyRoot of the given name
func (fs *Filesystem) GetRoot(name string) (*KeyRoot, error) {
	r, ok := fs.roots[name]
	if ok {
		return r, nil
	}
	return nil, os.ErrNotExist
}

type childCloser interface {
	closeChild(string, *dag.Node) error
}

type NodeType int

const (
	TFile NodeType = iota
	TDir
)

// FSNode represents any node (directory, root, or file) in the ipns filesystem
type FSNode interface {
	GetNode() (*dag.Node, error)
	Type() NodeType
	Lock()
	Unlock()
}

// KeyRoot represents the root of a filesystem tree pointed to by a given keypair
type KeyRoot struct {
	key ci.PrivKey

	// node is the merkledag node pointed to by this keypair
	node *dag.Node

	// A pointer to the filesystem to access components
	fs *Filesystem

	// val represents the node pointed to by this key. It can either be a File or a Directory
	val FSNode

	repub *Republisher
}

// newKeyRoot creates a new KeyRoot for the given key, and starts up a republisher routine
// for it
func (fs *Filesystem) newKeyRoot(parent context.Context, k ci.PrivKey) (*KeyRoot, error) {
	hash, err := k.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	name := "/ipns/" + u.Key(hash).String()

	root := new(KeyRoot)
	root.key = k
	root.fs = fs

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	pointsTo, err := fs.nsys.Resolve(ctx, name)
	if err != nil {
		err = namesys.InitializeKeyspace(ctx, fs.dserv, fs.nsys, fs.pins, k)
		if err != nil {
			return nil, err
		}

		pointsTo, err = fs.nsys.Resolve(ctx, name)
		if err != nil {
			return nil, err
		}
	}

	mnode, err := fs.resolver.ResolvePath(ctx, pointsTo)
	if err != nil {
		log.Errorf("Failed to retrieve value '%s' for ipns entry: %s\n", pointsTo, err)
		return nil, err
	}

	root.node = mnode

	root.repub = NewRepublisher(root, time.Millisecond*300, time.Second*3)
	go root.repub.Run(parent)

	pbn, err := ft.FromBytes(mnode.Data)
	if err != nil {
		log.Error("IPNS pointer was not unixfs node")
		return nil, err
	}

	switch pbn.GetType() {
	case ft.TDirectory:
		root.val = NewDirectory(pointsTo.String(), mnode, root, fs)
	case ft.TFile, ft.TMetadata, ft.TRaw:
		fi, err := NewFile(pointsTo.String(), mnode, root, fs)
		if err != nil {
			return nil, err
		}
		root.val = fi
	default:
		panic("unrecognized! (NYI)")
	}
	return root, nil
}

func (kr *KeyRoot) GetValue() FSNode {
	return kr.val
}

// closeChild implements the childCloser interface, and signals to the publisher that
// there are changes ready to be published
func (kr *KeyRoot) closeChild(name string, nd *dag.Node) error {
	kr.repub.Touch()
	return nil
}

// Publish publishes the ipns entry associated with this key
func (kr *KeyRoot) Publish(ctx context.Context) error {
	child, ok := kr.val.(FSNode)
	if !ok {
		return errors.New("child of key root not valid type")
	}

	nd, err := child.GetNode()
	if err != nil {
		return err
	}

	// Holding this lock so our child doesnt change out from under us
	child.Lock()
	k, err := kr.fs.dserv.Add(nd)
	if err != nil {
		child.Unlock()
		return err
	}
	child.Unlock()
	// Dont want to hold the lock while we publish
	// otherwise we are holding the lock through a costly
	// network operation

	fmt.Println("Publishing!")
	return kr.fs.nsys.Publish(ctx, kr.key, path.FromKey(k))
}

// Republisher manages when to publish the ipns entry associated with a given key
type Republisher struct {
	TimeoutLong  time.Duration
	TimeoutShort time.Duration
	Publish      chan struct{}
	root         *KeyRoot
}

// NewRepublisher creates a new Republisher object to republish the given keyroot
// using the given short and long time intervals
func NewRepublisher(root *KeyRoot, tshort, tlong time.Duration) *Republisher {
	return &Republisher{
		TimeoutShort: tshort,
		TimeoutLong:  tlong,
		Publish:      make(chan struct{}, 1),
		root:         root,
	}
}

// Touch signals that an update has occurred since the last publish.
// Multiple consecutive touches may extend the time period before
// the next Publish occurs in order to more efficiently batch updates
func (np *Republisher) Touch() {
	select {
	case np.Publish <- struct{}{}:
	default:
	}
}

// Run is the main republisher loop
func (np *Republisher) Run(ctx context.Context) {
	for {
		select {
		case <-np.Publish:
			quick := time.After(np.TimeoutShort)
			longer := time.After(np.TimeoutLong)

		wait:
			select {
			case <-ctx.Done():
				return
			case <-np.Publish:
				quick = time.After(np.TimeoutShort)
				goto wait
			case <-quick:
			case <-longer:
			}

			log.Info("Publishing Changes!")
			err := np.root.Publish(ctx)
			if err != nil {
				log.Critical("republishRoot error: %s", err)
			}

		case <-ctx.Done():
			return
		}
	}
}
