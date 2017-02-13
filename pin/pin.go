// package pin implements structures and methods to keep track of
// which objects a user wants to keep stored locally.
package pin

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	mdag "github.com/ipfs/go-ipfs/merkledag"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
	node "gx/ipfs/QmYDscK7dmdo2GZ9aumS8s5auUUAH5mR1jvj5pYhWusfK7/go-ipld-node"
)

var log = logging.Logger("pin")

var pinDatastoreKey = ds.NewKey("/local/pins")

var emptyKey *cid.Cid

func init() {
	e, err := cid.Decode("QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n")
	if err != nil {
		log.Error("failed to decode empty key constant")
		os.Exit(1)
	}
	emptyKey = e
}

const (
	linkRecursive = "recursive"
	linkDirect    = "direct"
	linkIndirect  = "indirect"
	linkInternal  = "internal"
	linkNotPinned = "not pinned"
	linkAny       = "any"
	linkAll       = "all"
)

type PinMode int

const (
	Recursive PinMode = iota
	Direct
	Indirect
	Internal
	NotPinned
	Any
)

func PinModeToString(mode PinMode) (string, bool) {
	m := map[PinMode]string{
		Recursive: linkRecursive,
		Direct:    linkDirect,
		Indirect:  linkIndirect,
		Internal:  linkInternal,
		NotPinned: linkNotPinned,
		Any:       linkAny,
	}
	s, ok := m[mode]
	return s, ok
}

func StringToPinMode(s string) (PinMode, bool) {
	m := map[string]PinMode{
		linkRecursive: Recursive,
		linkDirect:    Direct,
		linkIndirect:  Indirect,
		linkInternal:  Internal,
		linkNotPinned: NotPinned,
		linkAny:       Any,
		linkAll:       Any, // "all" and "any" means the same thing
	}
	mode, ok := m[s]
	return mode, ok
}

type Pinner interface {
	IsPinned(*cid.Cid) (string, bool, error)
	IsPinnedWithType(*cid.Cid, PinMode) (string, bool, error)
	Pin(context.Context, node.Node, bool) error
	Unpin(context.Context, *cid.Cid, bool) error

	// Check if a set of keys are pinned, more efficient than
	// calling IsPinned for each key
	CheckIfPinned(cids ...*cid.Cid) ([]Pinned, error)

	// PinWithMode is for manually editing the pin structure. Use with
	// care! If used improperly, garbage collection may not be
	// successful.
	PinWithMode(*cid.Cid, PinMode)
	// RemovePinWithMode is for manually editing the pin structure.
	// Use with care! If used improperly, garbage collection may not
	// be successful.
	RemovePinWithMode(*cid.Cid, PinMode)

	Flush() error
	DirectKeys() []*cid.Cid
	RecursiveKeys() []*cid.Cid
	InternalPins() []*cid.Cid
}

type Pinned struct {
	Key  *cid.Cid
	Mode PinMode
	Via  *cid.Cid
}

func (p Pinned) Pinned() bool {
	if p.Mode == NotPinned {
		return false
	} else {
		return true
	}
}

func (p Pinned) String() string {
	switch p.Mode {
	case NotPinned:
		return "not pinned"
	case Indirect:
		return fmt.Sprintf("pinned via %s", p.Via)
	default:
		modeStr, _ := PinModeToString(p.Mode)
		return fmt.Sprintf("pinned: %s", modeStr)
	}
}

// pinner implements the Pinner interface
type pinner struct {
	lock       sync.RWMutex
	recursePin *cid.Set
	directPin  *cid.Set

	// Track the keys used for storing the pinning state, so gc does
	// not delete them.
	internalPin *cid.Set
	dserv       mdag.DAGService
	internal    mdag.DAGService // dagservice used to store internal objects
	dstore      ds.Datastore
}

// NewPinner creates a new pinner using the given datastore as a backend
func NewPinner(dstore ds.Datastore, serv, internal mdag.DAGService) Pinner {

	rcset := cid.NewSet()
	dirset := cid.NewSet()

	return &pinner{
		recursePin:  rcset,
		directPin:   dirset,
		dserv:       serv,
		dstore:      dstore,
		internal:    internal,
		internalPin: cid.NewSet(),
	}
}

// Pin the given node, optionally recursive
func (p *pinner) Pin(ctx context.Context, node node.Node, recurse bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	c := node.Cid()

	if recurse {
		if p.recursePin.Has(c) {
			return nil
		}

		if p.directPin.Has(c) {
			p.directPin.Remove(c)
		}

		// fetch entire graph
		err := mdag.FetchGraph(ctx, c, p.dserv)
		if err != nil {
			return err
		}

		p.recursePin.Add(c)
	} else {
		if _, err := p.dserv.Get(ctx, c); err != nil {
			return err
		}

		if p.recursePin.Has(c) {
			return fmt.Errorf("%s already pinned recursively", c.String())
		}

		p.directPin.Add(c)
	}
	return nil
}

var ErrNotPinned = fmt.Errorf("not pinned")

// Unpin a given key
func (p *pinner) Unpin(ctx context.Context, c *cid.Cid, recursive bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	reason, pinned, err := p.isPinnedWithType(c, Any)
	if err != nil {
		return err
	}
	if !pinned {
		return ErrNotPinned
	}
	switch reason {
	case "recursive":
		if recursive {
			p.recursePin.Remove(c)
			return nil
		} else {
			return fmt.Errorf("%s is pinned recursively", c)
		}
	case "direct":
		p.directPin.Remove(c)
		return nil
	default:
		return fmt.Errorf("%s is pinned indirectly under %s", c, reason)
	}
}

func (p *pinner) isInternalPin(c *cid.Cid) bool {
	return p.internalPin.Has(c)
}

// IsPinned returns whether or not the given key is pinned
// and an explanation of why its pinned
func (p *pinner) IsPinned(c *cid.Cid) (string, bool, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.isPinnedWithType(c, Any)
}

func (p *pinner) IsPinnedWithType(c *cid.Cid, mode PinMode) (string, bool, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.isPinnedWithType(c, mode)
}

// isPinnedWithType is the implementation of IsPinnedWithType that does not lock.
// intended for use by other pinned methods that already take locks
func (p *pinner) isPinnedWithType(c *cid.Cid, mode PinMode) (string, bool, error) {
	switch mode {
	case Any, Direct, Indirect, Recursive, Internal:
	default:
		err := fmt.Errorf("Invalid Pin Mode '%d', must be one of {%d, %d, %d, %d, %d}",
			mode, Direct, Indirect, Recursive, Internal, Any)
		return "", false, err
	}
	if (mode == Recursive || mode == Any) && p.recursePin.Has(c) {
		return linkRecursive, true, nil
	}
	if mode == Recursive {
		return "", false, nil
	}

	if (mode == Direct || mode == Any) && p.directPin.Has(c) {
		return linkDirect, true, nil
	}
	if mode == Direct {
		return "", false, nil
	}

	if (mode == Internal || mode == Any) && p.isInternalPin(c) {
		return linkInternal, true, nil
	}
	if mode == Internal {
		return "", false, nil
	}

	// Default is Indirect
	for _, rc := range p.recursePin.Keys() {
		has, err := hasChild(p.dserv, rc, c)
		if err != nil {
			return "", false, err
		}
		if has {
			return rc.String(), true, nil
		}
	}
	return "", false, nil
}

func (p *pinner) CheckIfPinned(cids ...*cid.Cid) ([]Pinned, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	pinned := make([]Pinned, 0, len(cids))
	toCheck := cid.NewSet()

	// First check for non-Indirect pins directly
	for _, c := range cids {
		if p.recursePin.Has(c) {
			pinned = append(pinned, Pinned{Key: c, Mode: Recursive})
		} else if p.directPin.Has(c) {
			pinned = append(pinned, Pinned{Key: c, Mode: Direct})
		} else if p.isInternalPin(c) {
			pinned = append(pinned, Pinned{Key: c, Mode: Internal})
		} else {
			toCheck.Add(c)
		}
	}

	// Now walk all recursive pins to check for indirect pins
	var checkChildren func(*cid.Cid, *cid.Cid) error
	checkChildren = func(rk, parentKey *cid.Cid) error {
		links, err := p.dserv.GetLinks(context.Background(), parentKey)
		if err != nil {
			return err
		}
		for _, lnk := range links {
			c := lnk.Cid

			if toCheck.Has(c) {
				pinned = append(pinned,
					Pinned{Key: c, Mode: Indirect, Via: rk})
				toCheck.Remove(c)
			}

			err := checkChildren(rk, c)
			if err != nil {
				return err
			}

			if toCheck.Len() == 0 {
				return nil
			}
		}
		return nil
	}

	for _, rk := range p.recursePin.Keys() {
		err := checkChildren(rk, rk)
		if err != nil {
			return nil, err
		}
		if toCheck.Len() == 0 {
			break
		}
	}

	// Anything left in toCheck is not pinned
	for _, k := range toCheck.Keys() {
		pinned = append(pinned, Pinned{Key: k, Mode: NotPinned})
	}

	return pinned, nil
}

func (p *pinner) RemovePinWithMode(c *cid.Cid, mode PinMode) {
	p.lock.Lock()
	defer p.lock.Unlock()
	switch mode {
	case Direct:
		p.directPin.Remove(c)
	case Recursive:
		p.recursePin.Remove(c)
	default:
		// programmer error, panic OK
		panic("unrecognized pin type")
	}
}

func cidSetWithValues(cids []*cid.Cid) *cid.Set {
	out := cid.NewSet()
	for _, c := range cids {
		out.Add(c)
	}
	return out
}

// LoadPinner loads a pinner and its keysets from the given datastore
func LoadPinner(d ds.Datastore, dserv, internal mdag.DAGService) (Pinner, error) {
	p := new(pinner)

	rootKeyI, err := d.Get(pinDatastoreKey)
	if err != nil {
		return nil, fmt.Errorf("cannot load pin state: %v", err)
	}
	rootKeyBytes, ok := rootKeyI.([]byte)
	if !ok {
		return nil, fmt.Errorf("cannot load pin state: %s was not bytes", pinDatastoreKey)
	}

	rootCid, err := cid.Cast(rootKeyBytes)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel()

	root, err := internal.Get(ctx, rootCid)
	if err != nil {
		return nil, fmt.Errorf("cannot find pinning root object: %v", err)
	}

	rootpb, ok := root.(*mdag.ProtoNode)
	if !ok {
		return nil, mdag.ErrNotProtobuf
	}

	internalset := cid.NewSet()
	internalset.Add(rootCid)
	recordInternal := internalset.Add

	{ // load recursive set
		recurseKeys, err := loadSet(ctx, internal, rootpb, linkRecursive, recordInternal)
		if err != nil {
			return nil, fmt.Errorf("cannot load recursive pins: %v", err)
		}
		p.recursePin = cidSetWithValues(recurseKeys)
	}

	{ // load direct set
		directKeys, err := loadSet(ctx, internal, rootpb, linkDirect, recordInternal)
		if err != nil {
			return nil, fmt.Errorf("cannot load direct pins: %v", err)
		}
		p.directPin = cidSetWithValues(directKeys)
	}

	p.internalPin = internalset

	// assign services
	p.dserv = dserv
	p.dstore = d
	p.internal = internal

	return p, nil
}

// DirectKeys returns a slice containing the directly pinned keys
func (p *pinner) DirectKeys() []*cid.Cid {
	return p.directPin.Keys()
}

// RecursiveKeys returns a slice containing the recursively pinned keys
func (p *pinner) RecursiveKeys() []*cid.Cid {
	return p.recursePin.Keys()
}

// Flush encodes and writes pinner keysets to the datastore
func (p *pinner) Flush() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	ctx := context.TODO()

	internalset := cid.NewSet()
	recordInternal := internalset.Add

	root := &mdag.ProtoNode{}
	{
		n, err := storeSet(ctx, p.internal, p.directPin.Keys(), recordInternal)
		if err != nil {
			return err
		}
		if err := root.AddNodeLink(linkDirect, n); err != nil {
			return err
		}
	}

	{
		n, err := storeSet(ctx, p.internal, p.recursePin.Keys(), recordInternal)
		if err != nil {
			return err
		}
		if err := root.AddNodeLink(linkRecursive, n); err != nil {
			return err
		}
	}

	// add the empty node, its referenced by the pin sets but never created
	_, err := p.internal.Add(new(mdag.ProtoNode))
	if err != nil {
		return err
	}

	k, err := p.internal.Add(root)
	if err != nil {
		return err
	}

	internalset.Add(k)
	if err := p.dstore.Put(pinDatastoreKey, k.Bytes()); err != nil {
		return fmt.Errorf("cannot store pin state: %v", err)
	}
	p.internalPin = internalset
	return nil
}

func (p *pinner) InternalPins() []*cid.Cid {
	p.lock.Lock()
	defer p.lock.Unlock()
	var out []*cid.Cid
	for _, c := range p.internalPin.Keys() {
		out = append(out, c)
	}
	return out
}

// PinWithMode allows the user to have fine grained control over pin
// counts
func (p *pinner) PinWithMode(c *cid.Cid, mode PinMode) {
	p.lock.Lock()
	defer p.lock.Unlock()
	switch mode {
	case Recursive:
		p.recursePin.Add(c)
	case Direct:
		p.directPin.Add(c)
	}
}

func hasChild(ds mdag.LinkService, root *cid.Cid, child *cid.Cid) (bool, error) {
	links, err := ds.GetLinks(context.Background(), root)
	if err != nil {
		return false, err
	}
	for _, lnk := range links {
		c := lnk.Cid
		if lnk.Cid.Equals(child) {
			return true, nil
		}

		has, err := hasChild(ds, c, child)
		if err != nil {
			return false, err
		}

		if has {
			return has, nil
		}
	}
	return false, nil
}
