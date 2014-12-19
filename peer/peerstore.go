package peer

import (
	"fmt"
	"sync"

	ic "github.com/jbenet/go-ipfs/crypto"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Peerstore provides a threadsafe store of Peer related
// information.
type Peerstore interface {
	KeyBook
	AddressBook
}

// AddressBook tracks the addresses of Peers
type AddressBook interface {
	Addresses(ID) []ma.Multiaddr
	AddAddress(ID, ma.Multiaddr)
}

// KeyBook tracks the Public keys of Peers.
type KeyBook interface {
	PubKey(ID) ic.PubKey
	AddPubKey(ID, ic.PubKey) error
}

type addressMap map[string]ma.Multiaddr

type addressbook struct {
	addrs map[ID]addressMap
	sync.RWMutex
}

func newAddressbook() *addressbook {
	return &addressbook{addrs: map[ID]addressMap{}}
}

func (ab *addressbook) Peers() []ID {
	ab.RLock()
	ps := make([]ID, 0, len(ab.addrs))
	for p := range ab.addrs {
		ps = append(ps, p)
	}
	ab.RUnlock()
	return ps
}

func (ab *addressbook) Addresses(p ID) []ma.Multiaddr {
	ab.RLock()
	defer ab.RUnlock()

	maddrs, found := ab.addrs[p]
	if !found {
		return nil
	}

	maddrs2 := make([]ma.Multiaddr, 0, len(maddrs))
	for _, m := range maddrs {
		maddrs2 = append(maddrs2, m)
	}
	return maddrs2
}

func (ab *addressbook) AddAddress(p ID, m ma.Multiaddr) {
	ab.Lock()
	defer ab.Unlock()

	_, found := ab.addrs[p]
	if !found {
		ab.addrs[p] = addressMap{}
	}
	ab.addrs[p][m.String()] = m
}

type keybook struct {
	keys map[ID]ic.PubKey
	sync.RWMutex
}

func newKeybook() *keybook {
	return &keybook{keys: map[ID]ic.PubKey{}}
}

func (kb *keybook) Peers() []ID {
	kb.RLock()
	ps := make([]ID, 0, len(kb.keys))
	for p := range kb.keys {
		ps = append(ps, p)
	}
	kb.RUnlock()
	return ps
}

func (kb *keybook) PubKey(p ID) ic.PubKey {
	kb.RLock()
	pk := kb.keys[p]
	kb.RUnlock()
	return pk
}

func (kb *keybook) AddPubKey(p ID, pk ic.PubKey) error {

	// check it's correct first
	if err := VerifyPubKey(p, pk); err != nil {
		return err
	}

	kb.Lock()
	kb.keys[p] = pk
	kb.Unlock()
	return nil
}

// VerifyPubKey checks public key matches given peer Peer
func VerifyPubKey(p ID, pk ic.PubKey) error {
	p2, err := IDFromPubKey(pk)
	if err != nil {
		return fmt.Errorf("Failed to hash public key: %v", err)
	}

	if p != p2 {
		return fmt.Errorf("Public key ID does not match: %s != %s", p, p2)
	}

	return nil
}

type peerstore struct {
	keybook
	addressbook

	// store other data, like versions
	data map[ID]map[string]interface{}
}

// NewPeerstore creates a threadsafe collection of peers.
func NewPeerstore() Peerstore {
	return &peerstore{
		keybook:     *newKeybook(),
		addressbook: *newAddressbook(),
		data:        map[ID]map[string]interface{}{},
	}
}

func (ps *peerstore) Peers() []ID {
	set := map[ID]struct{}{}
	for _, p := range ps.keybook.Peers() {
		set[p] = struct{}{}
	}
	for _, p := range ps.addressbook.Peers() {
		set[p] = struct{}{}
	}

	pps := make([]ID, 0, len(set))
	for p := range set {
		pps = append(pps, p)
	}
	return pps
}
