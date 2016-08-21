// package pin implements structures and methods to keep track of
// which objects a user wants to keep stored locally.
package pin

import (
	"fmt"
	"sync"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/blocks/set"
	mdag "github.com/ipfs/go-ipfs/merkledag"
	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var log = logging.Logger("pin")

var pinDatastoreKey = ds.NewKey("/local/pins")

var emptyKey = key.B58KeyDecode("QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n")

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
	IsPinned(key.Key) (string, bool, error)
	IsPinnedWithType(key.Key, PinMode) (string, bool, error)
	Pin(context.Context, *mdag.Node, bool) error
	Unpin(context.Context, key.Key, bool) error

	// Check if a set of keys are pinned, more efficient than
	// calling IsPinned for each key
	CheckIfPinned(keys ...key.Key) ([]Pinned, error)

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

type Pinned struct {
	Key  key.Key
	Mode PinMode
	Via  key.Key
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
		if _, err := p.dserv.Get(ctx, k); err != nil {
			return err
		}

		if p.recursePin.HasKey(k) {
			return fmt.Errorf("%s already pinned recursively", k.B58String())
		}

		p.directPin.AddBlock(k)
	}
	return nil
}

var ErrNotPinned = fmt.Errorf("not pinned")

// Unpin a given key
func (p *pinner) Unpin(ctx context.Context, k key.Key, recursive bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	reason, pinned, err := p.isPinnedWithType(k, Any)
	if err != nil {
		return err
	}
	if !pinned {
		return ErrNotPinned
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
	return p.isPinnedWithType(k, Any)
}

func (p *pinner) IsPinnedWithType(k key.Key, mode PinMode) (string, bool, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.isPinnedWithType(k, mode)
}

// isPinnedWithType is the implementation of IsPinnedWithType that does not lock.
// intended for use by other pinned methods that already take locks
func (p *pinner) isPinnedWithType(k key.Key, mode PinMode) (string, bool, error) {
	switch mode {
	case Any, Direct, Indirect, Recursive, Internal:
	default:
		err := fmt.Errorf("Invalid Pin Mode '%d', must be one of {%d, %d, %d, %d, %d}",
			mode, Direct, Indirect, Recursive, Internal, Any)
		return "", false, err
	}
	if (mode == Recursive || mode == Any) && p.recursePin.HasKey(k) {
		return linkRecursive, true, nil
	}
	if mode == Recursive {
		return "", false, nil
	}

	if (mode == Direct || mode == Any) && p.directPin.HasKey(k) {
		return linkDirect, true, nil
	}
	if mode == Direct {
		return "", false, nil
	}

	if (mode == Internal || mode == Any) && p.isInternalPin(k) {
		return linkInternal, true, nil
	}
	if mode == Internal {
		return "", false, nil
	}

	// Default is Indirect
	for _, rk := range p.recursePin.GetKeys() {
		links, err := p.dserv.GetLinks(context.Background(), rk)
		if err != nil {
			return "", false, err
		}

		has, err := hasChild(p.dserv, links, k)
		if err != nil {
			return "", false, err
		}
		if has {
			return rk.B58String(), true, nil
		}
	}
	return "", false, nil
}

func (p *pinner) CheckIfPinned(keys ...key.Key) ([]Pinned, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	pinned := make([]Pinned, 0, len(keys))
	toCheck := make(map[key.Key]struct{})

	// First check for non-Indirect pins directly
	for _, k := range keys {
		if p.recursePin.HasKey(k) {
			pinned = append(pinned, Pinned{Key: k, Mode: Recursive})
		} else if p.directPin.HasKey(k) {
			pinned = append(pinned, Pinned{Key: k, Mode: Direct})
		} else if p.isInternalPin(k) {
			pinned = append(pinned, Pinned{Key: k, Mode: Internal})
		} else {
			toCheck[k] = struct{}{}
		}
	}

	// Now walk all recursive pins to check for indirect pins
	var checkChildren func(key.Key, key.Key) error
	checkChildren = func(rk key.Key, parentKey key.Key) error {
		parent, err := p.dserv.Get(context.Background(), parentKey)
		if err != nil {
			return err
		}
		for _, lnk := range parent.Links {
			k := key.Key(lnk.Hash)

			if _, found := toCheck[k]; found {
				pinned = append(pinned,
					Pinned{Key: k, Mode: Indirect, Via: rk})
				delete(toCheck, k)
			}

			err := checkChildren(rk, k)
			if err != nil {
				return err
			}

			if len(toCheck) == 0 {
				return nil
			}
		}
		return nil
	}
	for _, rk := range p.recursePin.GetKeys() {
		err := checkChildren(rk, rk)
		if err != nil {
			return nil, err
		}
		if len(toCheck) == 0 {
			break
		}
	}

	// Anything left in toCheck is not pinned
	for k, _ := range toCheck {
		pinned = append(pinned, Pinned{Key: k, Mode: NotPinned})
	}

	return pinned, nil
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

func hasChild(ds mdag.DAGService, links []*mdag.Link, child key.Key) (bool, error) {
	for _, lnk := range links {
		k := key.Key(lnk.Hash)
		if k == child {
			return true, nil
		}

		children, err := ds.GetLinks(context.Background(), k)
		if err != nil {
			return false, err
		}

		has, err := hasChild(ds, children, child)
		if err != nil {
			return false, err
		}

		if has {
			return has, nil
		}
	}
	return false, nil
}
