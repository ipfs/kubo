// package pin implements structures and methods to keep track of
// which objects a user wants to keep stored locally.
package pin

import (
	"fmt"
	"sync"
	"time"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/blocks/set"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/util"
)

var log = util.Logger("pin")

var pinDatastoreKey = ds.NewKey("/local/pins")

var emptyKey = key.B58KeyDecode("QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n")

const (
	linkDirect    = "direct"
	linkRecursive = "recursive"
)

type PinMode int

const (
	Recursive PinMode = iota
	Direct
	NotPinned
)

type Pinner interface {
	IsPinned(key.Key) (string, bool, error)
	Pin(context.Context, *mdag.Node, bool) error
	Unpin(context.Context, key.Key, bool) error

	// PinWithMode is for manually editing the pin structure. Use with
	// care! If used improperly, garbage collection may not be
	// successful.
	PinWithMode(key.Key, PinMode)
	// RemovePinWithMode is for manually editing the pin structure.
	// Use with care! If used improperly, garbage collection may not
	// be successful.
	RemovePinWithMode(key.Key, PinMode)

	Flush() error
	DirectKeys() []key.Key
	RecursiveKeys() []key.Key
	InternalPins() []key.Key
}

// pinner implements the Pinner interface
type pinner struct {
	lock       sync.RWMutex
	recursePin set.BlockSet
	directPin  set.BlockSet

	// Track the keys used for storing the pinning state, so gc does
	// not delete them.
	internalPin map[key.Key]struct{}
	dserv       mdag.DAGService
	dstore      ds.Datastore
}

// NewPinner creates a new pinner using the given datastore as a backend
func NewPinner(dstore ds.Datastore, serv mdag.DAGService) Pinner {

	// Load set from given datastore...
	rcset := set.NewSimpleBlockSet()

	dirset := set.NewSimpleBlockSet()

	return &pinner{
		recursePin: rcset,
		directPin:  dirset,
		dserv:      serv,
		dstore:     dstore,
	}
}

// Pin the given node, optionally recursive
func (p *pinner) Pin(ctx context.Context, node *mdag.Node, recurse bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	k, err := node.Key()
	if err != nil {
		return err
	}

	if recurse {
		if p.recursePin.HasKey(k) {
			return nil
		}

		if p.directPin.HasKey(k) {
			p.directPin.RemoveBlock(k)
		}

		// fetch entire graph
		err := mdag.FetchGraph(ctx, node, p.dserv)
		if err != nil {
			return err
		}

		p.recursePin.AddBlock(k)
	} else {
		_, err := p.dserv.Get(ctx, k)
		if err != nil {
			return err
		}

		if p.recursePin.HasKey(k) {
			return fmt.Errorf("%s already pinned recursively", k.B58String())
		}

		p.directPin.AddBlock(k)
	}
	return nil
}

// Unpin a given key
func (p *pinner) Unpin(ctx context.Context, k key.Key, recursive bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	reason, pinned, err := p.isPinned(k)
	if err != nil {
		return err
	}
	if !pinned {
		return fmt.Errorf("%s is not pinned", k)
	}
	switch reason {
	case "recursive":
		if recursive {
			p.recursePin.RemoveBlock(k)
			return nil
		} else {
			return fmt.Errorf("%s is pinned recursively", k)
		}
	case "direct":
		p.directPin.RemoveBlock(k)
		return nil
	default:
		return fmt.Errorf("%s is pinned indirectly under %s", k, reason)
	}
}

func (p *pinner) isInternalPin(key key.Key) bool {
	_, ok := p.internalPin[key]
	return ok
}

// IsPinned returns whether or not the given key is pinned
// and an explanation of why its pinned
func (p *pinner) IsPinned(k key.Key) (string, bool, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.isPinned(k)
}

// isPinned is the implementation of IsPinned that does not lock.
// intended for use by other pinned methods that already take locks
func (p *pinner) isPinned(k key.Key) (string, bool, error) {
	if p.recursePin.HasKey(k) {
		return "recursive", true, nil
	}
	if p.directPin.HasKey(k) {
		return "direct", true, nil
	}
	if p.isInternalPin(k) {
		return "internal", true, nil
	}

	for _, rk := range p.recursePin.GetKeys() {
		rnd, err := p.dserv.Get(context.Background(), rk)
		if err != nil {
			return "", false, err
		}

		has, err := hasChild(p.dserv, rnd, k)
		if err != nil {
			return "", false, err
		}
		if has {
			return rk.B58String(), true, nil
		}
	}
	return "", false, nil
}

func (p *pinner) RemovePinWithMode(key key.Key, mode PinMode) {
	p.lock.Lock()
	defer p.lock.Unlock()
	switch mode {
	case Direct:
		p.directPin.RemoveBlock(key)
	case Recursive:
		p.recursePin.RemoveBlock(key)
	default:
		// programmer error, panic OK
		panic("unrecognized pin type")
	}
}

// LoadPinner loads a pinner and its keysets from the given datastore
func LoadPinner(d ds.Datastore, dserv mdag.DAGService) (Pinner, error) {
	p := new(pinner)

	rootKeyI, err := d.Get(pinDatastoreKey)
	if err != nil {
		return nil, fmt.Errorf("cannot load pin state: %v", err)
	}
	rootKeyBytes, ok := rootKeyI.([]byte)
	if !ok {
		return nil, fmt.Errorf("cannot load pin state: %s was not bytes", pinDatastoreKey)
	}

	rootKey := key.Key(rootKeyBytes)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel()

	root, err := dserv.Get(ctx, rootKey)
	if err != nil {
		return nil, fmt.Errorf("cannot find pinning root object: %v", err)
	}

	internalPin := map[key.Key]struct{}{
		rootKey: struct{}{},
	}
	recordInternal := func(k key.Key) {
		internalPin[k] = struct{}{}
	}

	{ // load recursive set
		recurseKeys, err := loadSet(ctx, dserv, root, linkRecursive, recordInternal)
		if err != nil {
			return nil, fmt.Errorf("cannot load recursive pins: %v", err)
		}
		p.recursePin = set.SimpleSetFromKeys(recurseKeys)
	}

	{ // load direct set
		directKeys, err := loadSet(ctx, dserv, root, linkDirect, recordInternal)
		if err != nil {
			return nil, fmt.Errorf("cannot load direct pins: %v", err)
		}
		p.directPin = set.SimpleSetFromKeys(directKeys)
	}

	p.internalPin = internalPin

	// assign services
	p.dserv = dserv
	p.dstore = d

	return p, nil
}

// DirectKeys returns a slice containing the directly pinned keys
func (p *pinner) DirectKeys() []key.Key {
	return p.directPin.GetKeys()
}

// RecursiveKeys returns a slice containing the recursively pinned keys
func (p *pinner) RecursiveKeys() []key.Key {
	return p.recursePin.GetKeys()
}

// Flush encodes and writes pinner keysets to the datastore
func (p *pinner) Flush() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	ctx := context.TODO()

	internalPin := make(map[key.Key]struct{})
	recordInternal := func(k key.Key) {
		internalPin[k] = struct{}{}
	}

	root := &mdag.Node{}
	{
		n, err := storeSet(ctx, p.dserv, p.directPin.GetKeys(), recordInternal)
		if err != nil {
			return err
		}
		if err := root.AddNodeLink(linkDirect, n); err != nil {
			return err
		}
	}

	{
		n, err := storeSet(ctx, p.dserv, p.recursePin.GetKeys(), recordInternal)
		if err != nil {
			return err
		}
		if err := root.AddNodeLink(linkRecursive, n); err != nil {
			return err
		}
	}

	// add the empty node, its referenced by the pin sets but never created
	_, err := p.dserv.Add(new(mdag.Node))
	if err != nil {
		return err
	}

	k, err := p.dserv.Add(root)
	if err != nil {
		return err
	}

	internalPin[k] = struct{}{}
	if err := p.dstore.Put(pinDatastoreKey, []byte(k)); err != nil {
		return fmt.Errorf("cannot store pin state: %v", err)
	}
	p.internalPin = internalPin
	return nil
}

func (p *pinner) InternalPins() []key.Key {
	p.lock.Lock()
	defer p.lock.Unlock()
	var out []key.Key
	for k, _ := range p.internalPin {
		out = append(out, k)
	}
	return out
}

// PinWithMode allows the user to have fine grained control over pin
// counts
func (p *pinner) PinWithMode(k key.Key, mode PinMode) {
	p.lock.Lock()
	defer p.lock.Unlock()
	switch mode {
	case Recursive:
		p.recursePin.AddBlock(k)
	case Direct:
		p.directPin.AddBlock(k)
	}
}

func hasChild(ds mdag.DAGService, root *mdag.Node, child key.Key) (bool, error) {
	for _, lnk := range root.Links {
		k := key.Key(lnk.Hash)
		if k == child {
			return true, nil
		}

		nd, err := ds.Get(context.Background(), k)
		if err != nil {
			return false, err
		}

		has, err := hasChild(ds, nd, child)
		if err != nil {
			return false, err
		}

		if has {
			return has, nil
		}
	}
	return false, nil
}
