// package mfs implements an in memory model of a mutable IPFS filesystem.
//
// It consists of four main structs:
// 1) The Filesystem
//        The filesystem serves as a container and entry point for various mfs filesystems
// 2) Root
//        Root represents an individual filesystem mounted within the mfs system as a whole
// 3) Directories
// 4) Files
package mfs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"

	logging "gx/ipfs/QmTG23dvpBCBjqQwyDxV8CQT6jmS4PSftNr1VqHhE3MLy7/go-log"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

var ErrNotExist = errors.New("no such rootfs")

var log = logging.Logger("mfs")

var ErrIsDirectory = errors.New("error: is a directory")

type childCloser interface {
	closeChild(string, ipld.Node, bool) error
}

type NodeType int

const (
	TFile NodeType = iota
	TDir
)

// FSNode represents any node (directory, root, or file) in the mfs filesystem.
type FSNode interface {
	GetNode() (ipld.Node, error)
	Flush() error
	Type() NodeType
}

// Root represents the root of a filesystem tree.
type Root struct {
	// node is the merkledag root.
	node *dag.ProtoNode

	// val represents the node. It can either be a File or a Directory.
	val FSNode

	repub *Republisher

	dserv ipld.DAGService

	Type string
}

// PubFunc is the function used by the `publish()` method.
type PubFunc func(context.Context, *cid.Cid) error

// NewRoot creates a new Root and starts up a republisher routine for it.
func NewRoot(parent context.Context, ds ipld.DAGService, node *dag.ProtoNode, pf PubFunc) (*Root, error) {

	var repub *Republisher
	if pf != nil {
		repub = NewRepublisher(parent, pf, time.Millisecond*300, time.Second*3)
		repub.setVal(node.Cid())
		go repub.Run()
	}

	root := &Root{
		node:  node,
		repub: repub,
		dserv: ds,
	}

	pbn, err := ft.FromBytes(node.Data())
	if err != nil {
		log.Error("IPNS pointer was not unixfs node")
		return nil, err
	}

	switch pbn.GetType() {
	case ft.TDirectory, ft.THAMTShard:
		rval, err := NewDirectory(parent, node.String(), node, root, ds)
		if err != nil {
			return nil, err
		}

		root.val = rval
	case ft.TFile, ft.TMetadata, ft.TRaw:
		fi, err := NewFile(node.String(), node, root, ds)
		if err != nil {
			return nil, err
		}
		root.val = fi
	default:
		return nil, fmt.Errorf("unrecognized unixfs type: %s", pbn.GetType())
	}
	return root, nil
}

// GetValue returns the value of Root.
func (kr *Root) GetValue() FSNode {
	return kr.val
}

// Flush signals that an update has occurred since the last publish,
// and updates the Root republisher.
func (kr *Root) Flush() error {
	nd, err := kr.GetValue().GetNode()
	if err != nil {
		return err
	}

	if kr.repub != nil {
		kr.repub.Update(nd.Cid())
	}
	return nil
}

// FlushMemFree flushes the root directory and then uncaches all of its links.
// This has the effect of clearing out potentially stale references and allows
// them to be garbage collected.
// CAUTION: Take care not to ever call this while holding a reference to any
// child directories. Those directories will be bad references and using them
// may have unintended racy side effects.
// A better implemented mfs system (one that does smarter internal caching and
// refcounting) shouldnt need this method.
func (kr *Root) FlushMemFree(ctx context.Context) error {
	dir, ok := kr.GetValue().(*Directory)
	if !ok {
		return fmt.Errorf("invalid mfs structure, root should be a directory")
	}

	if err := dir.Flush(); err != nil {
		return err
	}

	dir.lock.Lock()
	defer dir.lock.Unlock()
	for name := range dir.files {
		delete(dir.files, name)
	}
	for name := range dir.childDirs {
		delete(dir.childDirs, name)
	}

	return nil
}

// closeChild implements the childCloser interface, and signals to the publisher that
// there are changes ready to be published.
func (kr *Root) closeChild(name string, nd ipld.Node, sync bool) error {
	err := kr.dserv.Add(context.TODO(), nd)
	if err != nil {
		return err
	}

	if kr.repub != nil {
		kr.repub.Update(nd.Cid())
	}
	return nil
}

func (kr *Root) Close() error {
	nd, err := kr.GetValue().GetNode()
	if err != nil {
		return err
	}

	if kr.repub != nil {
		kr.repub.Update(nd.Cid())
		return kr.repub.Close()
	}

	return nil
}

// Republisher manages when to publish a given entry.
type Republisher struct {
	TimeoutLong  time.Duration
	TimeoutShort time.Duration
	Publish      chan struct{}
	pubfunc      PubFunc
	pubnowch     chan chan struct{}

	ctx    context.Context
	cancel func()

	lk      sync.Mutex
	val     *cid.Cid
	lastpub *cid.Cid
}

// NewRepublisher creates a new Republisher object to republish the given root
// using the given short and long time intervals.
func NewRepublisher(ctx context.Context, pf PubFunc, tshort, tlong time.Duration) *Republisher {
	ctx, cancel := context.WithCancel(ctx)
	return &Republisher{
		TimeoutShort: tshort,
		TimeoutLong:  tlong,
		Publish:      make(chan struct{}, 1),
		pubfunc:      pf,
		pubnowch:     make(chan chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (p *Republisher) setVal(c *cid.Cid) {
	p.lk.Lock()
	defer p.lk.Unlock()
	p.val = c
}

// WaitPub Returns immediately if `lastpub` value is consistent with the
// current value `val`, else will block until `val` has been published.
func (p *Republisher) WaitPub() {
	p.lk.Lock()
	consistent := p.lastpub == p.val
	p.lk.Unlock()
	if consistent {
		return
	}

	wait := make(chan struct{})
	p.pubnowch <- wait
	<-wait
}

func (p *Republisher) Close() error {
	err := p.publish(p.ctx)
	p.cancel()
	return err
}

// Touch signals that an update has occurred since the last publish.
// Multiple consecutive touches may extend the time period before
// the next Publish occurs in order to more efficiently batch updates.
func (np *Republisher) Update(c *cid.Cid) {
	np.setVal(c)
	select {
	case np.Publish <- struct{}{}:
	default:
	}
}

// Run is the main republisher loop.
func (np *Republisher) Run() {
	for {
		select {
		case <-np.Publish:
			quick := time.After(np.TimeoutShort)
			longer := time.After(np.TimeoutLong)

		wait:
			var pubnowresp chan struct{}

			select {
			case <-np.ctx.Done():
				return
			case <-np.Publish:
				quick = time.After(np.TimeoutShort)
				goto wait
			case <-quick:
			case <-longer:
			case pubnowresp = <-np.pubnowch:
			}

			err := np.publish(np.ctx)
			if pubnowresp != nil {
				pubnowresp <- struct{}{}
			}
			if err != nil {
				log.Errorf("republishRoot error: %s", err)
			}

		case <-np.ctx.Done():
			return
		}
	}
}

// publish calls the `PubFunc`.
func (np *Republisher) publish(ctx context.Context) error {
	np.lk.Lock()
	topub := np.val
	np.lk.Unlock()

	err := np.pubfunc(ctx, topub)
	if err != nil {
		return err
	}
	np.lk.Lock()
	np.lastpub = topub
	np.lk.Unlock()
	return nil
}
