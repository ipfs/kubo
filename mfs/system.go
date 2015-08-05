// package mfs implements an in memory model of a mutable ipns filesystem,
// to be used by the fuse filesystem.
//
// It consists of four main structs:
// 1) The Filesystem
//        The filesystem serves as a container and entry point for the ipns filesystem
// 2) Root
//        Root represent the root of the keyspace controlled by a given keypair
// 3) Directories
// 4) Files
package mfs

import (
	"errors"
	"sync"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	eventlog "github.com/ipfs/go-ipfs/thirdparty/eventlog"
)

var ErrNotExist = errors.New("no such rootfs")

var log = eventlog.Logger("mfs")

var ErrIsDirectory = errors.New("error: is a directory")

// Filesystem is the writeable fuse filesystem structure
type Filesystem struct {
	ctx context.Context

	dserv dag.DAGService

	resolver *path.Resolver

	pins pin.Pinner

	roots map[string]*Root

	lk sync.Mutex
}

// NewFilesystem instantiates an ipns filesystem using the given parameters and locally owned keys
func NewFilesystem(ctx context.Context, ds dag.DAGService, pins pin.Pinner) (*Filesystem, error) {
	roots := make(map[string]*Root)
	fs := &Filesystem{
		ctx:      ctx,
		roots:    roots,
		dserv:    ds,
		pins:     pins,
		resolver: &path.Resolver{DAG: ds},
	}

	return fs, nil
}

func (fs *Filesystem) NewRoot(name string, root *dag.Node, pf PubFunc) (*Root, error) {
	fs.lk.Lock()
	defer fs.lk.Unlock()
	_, ok := fs.roots[name]
	if ok {
		return nil, errors.New("already exists")
	}

	kr, err := fs.newRoot(fs.ctx, root, pf)
	if err != nil {
		return nil, err
	}

	fs.roots[name] = kr
	return kr, nil
}

func (fs *Filesystem) Close() error {
	wg := sync.WaitGroup{}
	for _, r := range fs.roots {
		wg.Add(1)
		go func(r *Root) {
			defer wg.Done()
			r.repub.pubNow()
		}(r)
	}
	wg.Wait()
	return nil
}

// GetRoot returns the Root of the given name
func (fs *Filesystem) GetRoot(name string) (*Root, error) {
	fs.lk.Lock()
	defer fs.lk.Unlock()
	r, ok := fs.roots[name]
	if ok {
		return r, nil
	}
	panic("noooo")
	return nil, ErrNotExist
}

type RootListing struct {
	Name string
	Hash key.Key
}

func (fs *Filesystem) ListRoots() []RootListing {
	fs.lk.Lock()
	defer fs.lk.Unlock()
	var out []RootListing
	for name, r := range fs.roots {
		k := r.repub.getVal()
		out = append(out, RootListing{
			Name: name,
			Hash: k,
		})
	}
	return out
}

func (fs *Filesystem) CloseRoot(name string) (key.Key, error) {
	fs.lk.Lock()
	defer fs.lk.Unlock()
	r, ok := fs.roots[name]
	if !ok {
		return "", ErrNotExist
	}

	delete(fs.roots, name)
	return r.repub.getVal(), r.Close()
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

// Root represents the root of a filesystem tree pointed to by a given keypair
type Root struct {
	name string

	// node is the merkledag node pointed to by this keypair
	node *dag.Node

	// A pointer to the filesystem to access components
	fs *Filesystem

	// val represents the node pointed to by this key. It can either be a File or a Directory
	val FSNode

	repub *Republisher
}

type PubFunc func(context.Context, key.Key) error

// newRoot creates a new Root for the given key, and starts up a republisher routine
// for it
func (fs *Filesystem) newRoot(parent context.Context, node *dag.Node, pf PubFunc) (*Root, error) {
	name := "NO NAME (WIP)"

	ndk, err := node.Key()
	if err != nil {
		return nil, err
	}

	root := new(Root)
	root.fs = fs
	root.name = name

	root.node = node

	root.repub = NewRepublisher(parent, pf, time.Millisecond*300, time.Second*3)
	root.repub.setVal(ndk)
	go root.repub.Run()

	pbn, err := ft.FromBytes(node.Data)
	if err != nil {
		log.Error("IPNS pointer was not unixfs node")
		return nil, err
	}

	switch pbn.GetType() {
	case ft.TDirectory:
		root.val = NewDirectory(parent, ndk.String(), node, root, fs)
	case ft.TFile, ft.TMetadata, ft.TRaw:
		fi, err := NewFile(ndk.String(), node, root, fs)
		if err != nil {
			return nil, err
		}
		root.val = fi
	default:
		panic("unrecognized! (NYI)")
	}
	return root, nil
}

func (kr *Root) GetValue() FSNode {
	return kr.val
}

// closeChild implements the childCloser interface, and signals to the publisher that
// there are changes ready to be published
func (kr *Root) closeChild(name string, nd *dag.Node) error {
	k, err := kr.fs.dserv.Add(nd)
	if err != nil {
		return err
	}

	kr.repub.Update(k)
	return nil
}

func (kr *Root) Close() error {
	return kr.repub.Close()
}

// Republisher manages when to publish the ipns entry associated with a given key
type Republisher struct {
	TimeoutLong  time.Duration
	TimeoutShort time.Duration
	Publish      chan struct{}
	pubfunc      PubFunc
	pubnowch     chan struct{}

	ctx    context.Context
	cancel func()

	lk      sync.Mutex
	val     key.Key
	lastpub key.Key
}

func (rp *Republisher) getVal() key.Key {
	rp.lk.Lock()
	defer rp.lk.Unlock()
	return rp.val
}

// NewRepublisher creates a new Republisher object to republish the given keyroot
// using the given short and long time intervals
func NewRepublisher(ctx context.Context, pf PubFunc, tshort, tlong time.Duration) *Republisher {
	ctx, cancel := context.WithCancel(ctx)
	return &Republisher{
		TimeoutShort: tshort,
		TimeoutLong:  tlong,
		Publish:      make(chan struct{}, 1),
		pubfunc:      pf,
		pubnowch:     make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (p *Republisher) setVal(k key.Key) {
	p.lk.Lock()
	defer p.lk.Unlock()
	p.val = k
}

func (p *Republisher) pubNow() {
	select {
	case p.pubnowch <- struct{}{}:
	default:
	}
}

func (p *Republisher) Close() error {
	err := p.publish(p.ctx)
	p.cancel()
	return err
}

// Touch signals that an update has occurred since the last publish.
// Multiple consecutive touches may extend the time period before
// the next Publish occurs in order to more efficiently batch updates
func (np *Republisher) Update(k key.Key) {
	np.setVal(k)
	select {
	case np.Publish <- struct{}{}:
	default:
	}
}

// Run is the main republisher loop
func (np *Republisher) Run() {
	for {
		select {
		case <-np.Publish:
			quick := time.After(np.TimeoutShort)
			longer := time.After(np.TimeoutLong)

		wait:
			select {
			case <-np.ctx.Done():
				return
			case <-np.Publish:
				quick = time.After(np.TimeoutShort)
				goto wait
			case <-quick:
			case <-longer:
			case <-np.pubnowch:
			}

			err := np.publish(np.ctx)
			if err != nil {
				log.Error("republishRoot error: %s", err)
			}

		case <-np.ctx.Done():
			return
		}
	}
}

func (np *Republisher) publish(ctx context.Context) error {
	np.lk.Lock()
	topub := np.val
	np.lk.Unlock()

	log.Info("Publishing Changes!")
	err := np.pubfunc(ctx, topub)
	if err != nil {
		return err
	}
	np.lk.Lock()
	np.lastpub = topub
	np.lk.Unlock()
	return nil
}
