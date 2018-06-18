// Package pin implements structures and methods to keep track of
// which objects a user wants to keep stored locally.
package pin

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	mdag "github.com/ipfs/go-ipfs/merkledag"
	dutils "github.com/ipfs/go-ipfs/merkledag/utils"
	"github.com/ipfs/go-ipfs/thirdparty/recpinset"

	ipld "gx/ipfs/QmWi2BYBL5gJ3CiAiQchg6rn1A8iBsrWy51EYxvHVjFvLb/go-ipld-format"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
	logging "gx/ipfs/Qmbi1CTJsbnBZjCEgc2otwu8cUFPsGpzWXG7edVCLZ7Gvk/go-log"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
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

// Mode allows to specify different types of pin (recursive, direct etc.).
// See the Pin Modes constants for a full list.
type Mode int

var recursiveNRegexp *regexp.Regexp = regexp.MustCompile(fmt.Sprintf("%s([0-9]+)", linkRecursive))

// Pin Modes
const (
	// Recursive pins pin the target cids along with any reachable children.
	Recursive Mode = iota

	// Direct pins pin just the target cid.
	Direct

	// Indirect pins are cids who have some ancestor pinned recursively.
	Indirect

	// Internal pins are cids used to keep the internal state of the pinner.
	Internal

	// NotPinned
	NotPinned

	// Any refers to any pinned cid
	Any

	// RecursiveN are pins pinned up to the Nth level down the tree.
	// RecursiveN == Recursive0. Recursive1 == RecursiveN+1 etc.
	RecursiveN Mode = iota + 100
)

// ModeToString returns a human-readable name for the Mode.
func ModeToString(mode Mode) (string, bool) {
	m := map[Mode]string{
		Recursive: linkRecursive,
		Direct:    linkDirect,
		Indirect:  linkIndirect,
		Internal:  linkInternal,
		NotPinned: linkNotPinned,
		Any:       linkAny,
	}
	s, ok := m[mode]
	if !ok && mode >= RecursiveN {
		s = fmt.Sprintf("%s%d", linkRecursive, ModeToMaxDepth(mode))
		ok = true
	}

	return s, ok
}

// MaxDepthToMode converts a depth limit to the RecursiveN+depth mode.
func MaxDepthToMode(d int) Mode {
	if d < 0 {
		return Recursive
	}
	return RecursiveN + Mode(d)
}

// ModeToMaxDepth converts a mode to the depth limit.
// It is either -1 for recursive or mode - RecursiveN for
// modes >= RecursiveN. For the rest, it's 0.
func ModeToMaxDepth(mode Mode) int {
	switch {
	case mode == Recursive:
		return -1
	case mode >= RecursiveN:
		return int(mode - RecursiveN)
	default:
		return 0
	}
}

// StringToMode parses the result of ModeToString() back to a Mode.
// It returns a boolean which is set to false if the mode is unknown.
func StringToMode(s string) (Mode, bool) {
	// if s is like "recursive33", return RecursiveN+33
	recN := recursiveNRegexp.FindStringSubmatch(s)
	if len(recN) == 2 {
		m, err := strconv.Atoi(recN[1])
		if err != nil {
			return 0, false
		}
		return MaxDepthToMode(m), true
	}

	m := map[string]Mode{
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

// A Pinner provides the necessary methods to keep track of Nodes which are
// to be kept locally, according to a pin mode. In practice, a Pinner is in
// in charge of keeping the list of items from the local storage that should
// not be garbage-collected.
type Pinner interface {
	// IsPinned returns whether or not the given cid is pinned
	// and an explanation of why its pinned
	IsPinned(*cid.Cid) (string, bool, error)

	// IsPinnedWithType returns whether or not the given cid is pinned with the
	// given pin type, as well as returning the type of pin its pinned with.
	IsPinnedWithType(*cid.Cid, Mode) (string, bool, error)

	// Pin the given node, optionally recursively.
	Pin(ctx context.Context, node ipld.Node, recursive bool) error

	// PinMaxDepth pins the given node recursively limiting the DAG depth
	PinMaxDepth(ctx context.Context, node ipld.Node, maxDepth int) error

	// Unpin the given cid. If recursive is true, removes either a recursive or
	// a direct pin. If recursive is false, only removes a direct pin.
	Unpin(ctx context.Context, cid *cid.Cid, recursive bool) error

	// Update updates a recursive pin from one cid to another
	// this is more efficient than simply pinning the new one and unpinning the
	// old one
	Update(ctx context.Context, from, to *cid.Cid, unpin bool) error

	// Check if a set of keys are pinned, more efficient than
	// calling IsPinned for each key
	CheckIfPinned(cids ...*cid.Cid) ([]Pinned, error)

	// PinWithMode is for manually editing the pin structure. Use with
	// care! If used improperly, garbage collection may not be
	// successful.
	PinWithMode(*cid.Cid, Mode)

	// RemovePinWithMode is for manually editing the pin structure.
	// Use with care! If used improperly, garbage collection may not
	// be successful.
	RemovePinWithMode(*cid.Cid, Mode)

	// Flush writes the pin state to the backing datastore
	Flush() error

	// DirectKeys returns all directly pinned cids
	DirectKeys() []*cid.Cid

	// DirectKeys returns all recursively pinned cids and their MaxDepths
	RecursivePins() []*recpinset.RecPin

	// InternalPins returns all cids kept pinned for the internal state of the
	// pinner
	InternalPins() []*cid.Cid
}

// Pinned represents CID which has been pinned with a pinning strategy.
// The Via field allows to identify the pinning parent of this CID, in the
// case that the item is not pinned directly (but rather pinned recursively
// by some ascendant).
type Pinned struct {
	Key  *cid.Cid
	Mode Mode
	Via  *cid.Cid
}

// Pinned returns whether or not the given cid is pinned
func (p Pinned) Pinned() bool {
	return p.Mode != NotPinned
}

// String Returns pin status as string
func (p Pinned) String() string {
	switch p.Mode {
	case NotPinned:
		return "not pinned"
	case Indirect:
		return fmt.Sprintf("pinned via %s", p.Via)
	default:
		modeStr, _ := ModeToString(p.Mode)
		return fmt.Sprintf("pinned: %s", modeStr)
	}
}

// pinner implements the Pinner interface
type pinner struct {
	lock       sync.RWMutex
	recursePin *recpinset.Set
	directPin  *cid.Set

	// Track the keys used for storing the pinning state, so gc does
	// not delete them.
	internalPin *cid.Set
	dserv       ipld.DAGService
	internal    ipld.DAGService // dagservice used to store internal objects
	dstore      ds.Datastore
}

// NewPinner creates a new pinner using the given datastore as a backend
func NewPinner(dstore ds.Datastore, serv, internal ipld.DAGService) Pinner {

	rcset := recpinset.New()
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
func (p *pinner) Pin(ctx context.Context, node ipld.Node, recurse bool) error {
	depth := 0
	if recurse {
		depth = -1
	}
	return p.PinMaxDepth(ctx, node, depth)
}

func (p *pinner) PinMaxDepth(ctx context.Context, node ipld.Node, maxDepth int) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	err := p.dserv.Add(ctx, node)
	if err != nil {
		return err
	}

	c := node.Cid()

	// Pins with maxDepth == 0 are "direct"
	if maxDepth < 0 || maxDepth > 0 {
		curDepth, ok := p.recursePin.MaxDepth(c)

		// only pin is something deeper isn't pinned already
		if ok && !recpinset.IsDeeper(maxDepth, curDepth) {
			return nil
		}

		if p.directPin.Has(c) {
			p.directPin.Remove(c)
		}

		// fetch graph to needed depth
		err := mdag.FetchGraphMaxDepth(ctx, c, maxDepth, p.dserv)
		if err != nil {
			return err
		}

		p.recursePin.Add(c, maxDepth)
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

// ErrNotPinned is returned when trying to unpin items which are not pinned.
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

	if strings.HasPrefix(reason, "recursive") {
		reason = "recursive"
	}

	switch reason {
	case "recursive":
		if recursive {
			p.recursePin.Remove(c)
			return nil
		}
		return fmt.Errorf("%s is pinned recursively", c)
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

// IsPinnedWithType returns whether or not the given cid is pinned with the
// given pin type, as well as returning the type of pin its pinned with.
func (p *pinner) IsPinnedWithType(c *cid.Cid, mode Mode) (string, bool, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.isPinnedWithType(c, mode)
}

// isPinnedWithType is the implementation of IsPinnedWithType that does not lock.
// intended for use by other pinned methods that already take locks
func (p *pinner) isPinnedWithType(c *cid.Cid, mode Mode) (string, bool, error) {
	modeStr, ok := ModeToString(mode)
	if !ok {
		err := fmt.Errorf(
			"invalid Pin Mode '%d', must be one of {%d, %d, %d, RecursiveN, %d, %d}",
			mode, Direct, Indirect, Recursive, Internal, Any)
		return "", false, err
	}

	maxDepth, ok := p.recursePin.MaxDepth(c)
	isRecursive := mode == Recursive || mode > RecursiveN
	if (mode == Any || isRecursive) && ok { // some sort of recursive pin
		modeStr, _ = ModeToString(MaxDepthToMode(maxDepth))
		return modeStr, true, nil
	}

	if isRecursive {
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
	visitedSet := cid.NewSet()

	for _, recPin := range p.recursePin.RecPins() {
		has, err := hasChild(
			p.dserv,
			recPin.Cid, // root
			c,          // child
			recPin.MaxDepth,
			visitedSet.Visit,
		)
		if err != nil {
			return "", false, err
		}
		if has {
			return recPin.Cid.String(), true, nil
		}
	}
	return "", false, nil
}

// CheckIfPinned Checks if a set of keys are pinned, more efficient than
// calling IsPinned for each key, returns the pinned status of cid(s)
func (p *pinner) CheckIfPinned(cids ...*cid.Cid) ([]Pinned, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	pinned := make([]Pinned, 0, len(cids))
	toCheck := cid.NewSet()

	// First check for non-Indirect pins directly
	for _, c := range cids {
		maxDepth, ok := p.recursePin.MaxDepth(c)
		if ok {
			if maxDepth < 0 {
				pinned = append(pinned, Pinned{Key: c, Mode: Recursive})
			} else {
				pinned = append(pinned, Pinned{Key: c, Mode: MaxDepthToMode(maxDepth)})
			}
		} else if p.directPin.Has(c) {
			pinned = append(pinned, Pinned{Key: c, Mode: Direct})
		} else if p.isInternalPin(c) {
			pinned = append(pinned, Pinned{Key: c, Mode: Internal})
		} else {
			toCheck.Add(c)
		}
	}

	// Now walk all recursive pins to check for indirect pins
	var checkChildren func(*cid.Cid, *cid.Cid, int) error
	checkChildren = func(rk, parentKey *cid.Cid, maxDepth int) error {
		if maxDepth == 0 {
			return nil
		}

		if maxDepth > 0 { // ignore depth limit -1
			maxDepth--
		}

		links, err := ipld.GetLinks(context.TODO(), p.dserv, parentKey)
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

			err := checkChildren(rk, c, maxDepth)
			if err != nil {
				return err
			}

			if toCheck.Len() == 0 {
				return nil
			}
		}
		return nil
	}

	for _, recPin := range p.recursePin.RecPins() {
		err := checkChildren(recPin.Cid, recPin.Cid, recPin.MaxDepth)
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

// RemovePinWithMode is for manually editing the pin structure.
// Use with care! If used improperly, garbage collection may not
// be successful.
func (p *pinner) RemovePinWithMode(c *cid.Cid, mode Mode) {
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
func LoadPinner(d ds.Datastore, dserv, internal ipld.DAGService) (Pinner, error) {
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

	p.recursePin = recpinset.New()

	for _, link := range rootpb.Links() {
		mode, ok := StringToMode(link.Name)
		if !ok {
			continue
		}

		switch {
		case mode == Recursive || mode >= RecursiveN:
			depthLimit := -1 // Recursive
			if mode >= RecursiveN {
				depthLimit = ModeToMaxDepth(mode)
			}
			recurseKeys, err := loadSet(ctx, internal, rootpb, link.Name, recordInternal)
			if err != nil {
				return nil, fmt.Errorf("cannot load recursive pins: %v", err)
			}
			for _, c := range recurseKeys {
				p.recursePin.Add(c, depthLimit)
			}
		case mode == Direct:
			directKeys, err := loadSet(ctx, internal, rootpb, linkDirect, recordInternal)
			if err != nil {
				return nil, fmt.Errorf("cannot load direct pins: %v", err)
			}
			p.directPin = cidSetWithValues(directKeys)
		}
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

// RecursivePins returns a slice containing the recursively pinned keys
func (p *pinner) RecursivePins() []*recpinset.RecPin {
	return p.recursePin.RecPins()
}

// RecursiveWithLimitKeys returns a slice containing the recursively
// pinned keys along with their depth limit
func (p *pinner) RecursiveWithLimitKeys() []*recpinset.RecPin {
	return p.recursePin.RecPins()
}

// Update updates a recursive pin from one cid to another
// this is more efficient than simply pinning the new one and unpinning the
// old one
func (p *pinner) Update(ctx context.Context, from, to *cid.Cid, unpin bool) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.recursePin.Has(from) {
		return fmt.Errorf("'from' cid was not recursively pinned already")
	}

	err := dutils.DiffEnumerate(ctx, p.dserv, from, to)
	if err != nil {
		return err
	}

	p.recursePin.Add(to, -1)
	if unpin {
		p.recursePin.Remove(from)
	}
	return nil
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
		depthLimits := make(map[int][]*cid.Cid)
		for _, recPin := range p.recursePin.RecPins() {
			depth := recPin.MaxDepth
			set := depthLimits[depth]
			depthLimits[depth] = append(set, recPin.Cid)
		}

		for depth, set := range depthLimits {
			n, err := storeSet(ctx, p.internal, set, recordInternal)
			if err != nil {
				return err
			}
			linkName := linkRecursive
			if depth >= 0 {
				linkName, _ = ModeToString(MaxDepthToMode(depth))
			}
			if err := root.AddNodeLink(linkName, n); err != nil {
				return err
			}
		}
	}

	// add the empty node, its referenced by the pin sets but never created
	err := p.internal.Add(ctx, new(mdag.ProtoNode))
	if err != nil {
		return err
	}

	err = p.internal.Add(ctx, root)
	if err != nil {
		return err
	}

	k := root.Cid()

	internalset.Add(k)
	if err := p.dstore.Put(pinDatastoreKey, k.Bytes()); err != nil {
		return fmt.Errorf("cannot store pin state: %v", err)
	}
	p.internalPin = internalset
	return nil
}

// InternalPins returns all cids kept pinned for the internal state of the
// pinner
func (p *pinner) InternalPins() []*cid.Cid {
	p.lock.Lock()
	defer p.lock.Unlock()
	var out []*cid.Cid
	out = append(out, p.internalPin.Keys()...)
	return out
}

// PinWithMode allows the user to have fine grained control over pin
// counts
func (p *pinner) PinWithMode(c *cid.Cid, mode Mode) {
	p.lock.Lock()
	defer p.lock.Unlock()
	switch {
	case mode == Recursive:
		p.recursePin.Add(c, -1)
	case mode >= RecursiveN:
		p.recursePin.Add(c, ModeToMaxDepth(mode))
	case mode == Direct:
		p.directPin.Add(c)
	}
}

// hasChild recursively looks for a Cid among the children of a root Cid.
// The visit function can be used to shortcut already-visited branches.
func hasChild(ng ipld.NodeGetter, root *cid.Cid, child *cid.Cid, depthLimit int, visit func(*cid.Cid) bool) (bool, error) {
	if depthLimit == 0 {
		return false, nil
	}

	if depthLimit > 0 { // ignore negative depthLimits
		depthLimit--
	}

	links, err := ipld.GetLinks(context.TODO(), ng, root)
	if err != nil {
		return false, err
	}
	for _, lnk := range links {
		c := lnk.Cid
		if lnk.Cid.Equals(child) {
			return true, nil
		}
		if visit(c) {
			has, err := hasChild(ng, c, child, depthLimit, visit)
			if err != nil {
				return false, err
			}

			if has {
				return has, nil
			}
		}
	}
	return false, nil
}
