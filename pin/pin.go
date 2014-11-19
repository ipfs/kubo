// package pin implemnts structures and methods to keep track of
// which objects a user wants to keep stored locally.
package pin

import (
	"encoding/json"
	"errors"
	"sync"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	nsds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/namespace"
	"github.com/jbenet/go-ipfs/blocks/set"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("pin")
var recursePinDatastoreKey = ds.NewKey("/local/pins/recursive/keys")
var directPinDatastoreKey = ds.NewKey("/local/pins/direct/keys")
var indirectPinDatastoreKey = ds.NewKey("/local/pins/indirect/keys")

type PinMode int

const (
	Recursive PinMode = iota
	Direct
	Indirect
)

type Pinner interface {
	IsPinned(util.Key) bool
	Pin(*mdag.Node, bool) error
	Unpin(util.Key, bool) error
	Flush() error
	GetManual() ManualPinner
	DirectKeys() []util.Key
	IndirectKeys() []util.Key
	RecursiveKeys() []util.Key
}

// ManualPinner is for manually editing the pin structure
// Use with care! If used improperly, garbage collection
// may not be successful
type ManualPinner interface {
	PinWithMode(util.Key, PinMode)
	Pinner
}

// pinner implements the Pinner interface
type pinner struct {
	lock       sync.RWMutex
	recursePin set.BlockSet
	directPin  set.BlockSet
	indirPin   *indirectPin
	dserv      mdag.DAGService
	dstore     ds.Datastore
}

// NewPinner creates a new pinner using the given datastore as a backend
func NewPinner(dstore ds.Datastore, serv mdag.DAGService) Pinner {

	// Load set from given datastore...
	rcds := nsds.Wrap(dstore, recursePinDatastoreKey)
	rcset := set.NewDBWrapperSet(rcds, set.NewSimpleBlockSet())

	dirds := nsds.Wrap(dstore, directPinDatastoreKey)
	dirset := set.NewDBWrapperSet(dirds, set.NewSimpleBlockSet())

	nsdstore := nsds.Wrap(dstore, indirectPinDatastoreKey)
	return &pinner{
		recursePin: rcset,
		directPin:  dirset,
		indirPin:   NewIndirectPin(nsdstore),
		dserv:      serv,
		dstore:     dstore,
	}
}

// Pin the given node, optionally recursive
func (p *pinner) Pin(node *mdag.Node, recurse bool) error {
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

		p.recursePin.AddBlock(k)

		err := p.pinLinks(node)
		if err != nil {
			return err
		}
	} else {
		p.directPin.AddBlock(k)
	}
	return nil
}

// Unpin a given key with optional recursive unpinning
func (p *pinner) Unpin(k util.Key, recurse bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if recurse {
		p.recursePin.RemoveBlock(k)
		node, err := p.dserv.Get(k)
		if err != nil {
			return err
		}

		return p.unpinLinks(node)
	}
	p.directPin.RemoveBlock(k)
	return nil
}

func (p *pinner) unpinLinks(node *mdag.Node) error {
	for _, l := range node.Links {
		node, err := l.GetNode(p.dserv)
		if err != nil {
			return err
		}

		k, err := node.Key()
		if err != nil {
			return err
		}

		p.recursePin.RemoveBlock(k)

		err = p.unpinLinks(node)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *pinner) pinIndirectRecurse(node *mdag.Node) error {
	k, err := node.Key()
	if err != nil {
		return err
	}

	p.indirPin.Increment(k)
	return p.pinLinks(node)
}

func (p *pinner) pinLinks(node *mdag.Node) error {
	for _, l := range node.Links {
		subnode, err := l.GetNode(p.dserv)
		if err != nil {
			// TODO: Maybe just log and continue?
			return err
		}
		err = p.pinIndirectRecurse(subnode)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsPinned returns whether or not the given key is pinned
func (p *pinner) IsPinned(key util.Key) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.recursePin.HasKey(key) ||
		p.directPin.HasKey(key) ||
		p.indirPin.HasKey(key)
}

// LoadPinner loads a pinner and its keysets from the given datastore
func LoadPinner(d ds.Datastore, dserv mdag.DAGService) (Pinner, error) {
	p := new(pinner)

	{ // load recursive set
		var recurseKeys []util.Key
		if err := loadSet(d, recursePinDatastoreKey, &recurseKeys); err != nil {
			return nil, err
		}
		p.recursePin = set.SimpleSetFromKeys(recurseKeys)
	}

	{ // load direct set
		var directKeys []util.Key
		if err := loadSet(d, directPinDatastoreKey, &directKeys); err != nil {
			return nil, err
		}
		p.directPin = set.SimpleSetFromKeys(directKeys)
	}

	{ // load indirect set
		var err error
		p.indirPin, err = loadIndirPin(d, indirectPinDatastoreKey)
		if err != nil {
			return nil, err
		}
	}

	// assign services
	p.dserv = dserv
	p.dstore = d

	return p, nil
}

// DirectKeys returns a slice containing the directly pinned keys
func (p *pinner) DirectKeys() []util.Key {
	return p.directPin.GetKeys()
}

// IndirectKeys returns a slice containing the indirectly pinned keys
func (p *pinner) IndirectKeys() []util.Key {
	return p.indirPin.Set().GetKeys()
}

// RecursiveKeys returns a slice containing the recursively pinned keys
func (p *pinner) RecursiveKeys() []util.Key {
	return p.recursePin.GetKeys()
}

// Flush encodes and writes pinner keysets to the datastore
func (p *pinner) Flush() error {
	p.lock.RLock()
	defer p.lock.RUnlock()

	err := storeSet(p.dstore, directPinDatastoreKey, p.directPin.GetKeys())
	if err != nil {
		return err
	}

	err = storeSet(p.dstore, recursePinDatastoreKey, p.recursePin.GetKeys())
	if err != nil {
		return err
	}

	err = storeIndirPin(p.dstore, indirectPinDatastoreKey, p.indirPin)
	if err != nil {
		return err
	}
	return nil
}

// helpers to marshal / unmarshal a pin set
func storeSet(d ds.Datastore, k ds.Key, val interface{}) error {
	buf, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return d.Put(k, buf)
}

func loadSet(d ds.Datastore, k ds.Key, val interface{}) error {
	buf, err := d.Get(k)
	if err != nil {
		return err
	}

	bf, ok := buf.([]byte)
	if !ok {
		return errors.New("invalid pin set value in datastore")
	}
	return json.Unmarshal(bf, val)
}

// PinWithMode is a method on ManualPinners, allowing the user to have fine
// grained control over pin counts
func (p *pinner) PinWithMode(k util.Key, mode PinMode) {
	switch mode {
	case Recursive:
		p.recursePin.AddBlock(k)
	case Direct:
		p.directPin.AddBlock(k)
	case Indirect:
		p.indirPin.Increment(k)
	}
}

func (p *pinner) GetManual() ManualPinner {
	return p
}
