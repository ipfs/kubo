package peer

import (
	"errors"
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

// KeyBook tracks the Public keys of Peers.
type KeyBook interface {
	PubKey(ID) ic.PubKey
	AddPubKey(ID, ic.PubKey) error

	PrivKey(ID) ic.PrivKey
	AddPrivKey(ID, ic.PrivKey) error
}

type keybook struct {
	pks map[ID]ic.PubKey
	sks map[ID]ic.PrivKey

	sync.RWMutex // same lock. wont happen a ton.
}

func newKeybook() *keybook {
	return &keybook{
		pks: map[ID]ic.PubKey{},
		sks: map[ID]ic.PrivKey{},
	}
}

func (kb *keybook) Peers() []ID {
	kb.RLock()
	ps := make([]ID, 0, len(kb.pks)+len(kb.sks))
	for p := range kb.pks {
		ps = append(ps, p)
	}
	for p := range kb.sks {
		if _, found := kb.pks[p]; !found {
			ps = append(ps, p)
		}
	}
	kb.RUnlock()
	return ps
}

func (kb *keybook) PubKey(p ID) ic.PubKey {
	kb.RLock()
	pk := kb.pks[p]
	kb.RUnlock()
	return pk
}

func (kb *keybook) AddPubKey(p ID, pk ic.PubKey) error {

	// check it's correct first
	if !p.MatchesPublicKey(pk) {
		return errors.New("ID does not match PublicKey")
	}

	kb.Lock()
	kb.pks[p] = pk
	kb.Unlock()
	return nil
}

func (kb *keybook) PrivKey(p ID) ic.PrivKey {
	kb.RLock()
	sk := kb.sks[p]
	kb.RUnlock()
	return sk
}

func (kb *keybook) AddPrivKey(p ID, sk ic.PrivKey) error {

	// check it's correct first
	if !p.MatchesPrivateKey(sk) {
		return errors.New("ID does not match PrivateKey")
	}

	kb.Lock()
	kb.sks[p] = sk
	kb.Unlock()
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
