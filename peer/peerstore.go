package peer

import (
	"errors"
	"sync"

	ic "github.com/jbenet/go-ipfs/crypto"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

// Peerstore provides a threadsafe store of Peer related
// information.
type Peerstore interface {
	KeyBook
	AddressBook
	Metrics

	// Peers returns a list of all peer.IDs in this Peerstore
	Peers() []ID

	// PeerInfo returns a peer.PeerInfo struct for given peer.ID.
	// This is a small slice of the information Peerstore has on
	// that peer, useful to other services.
	PeerInfo(ID) PeerInfo

	// Get/Put is a simple registry for other peer-related key/value pairs.
	// if we find something we use often, it should become its own set of
	// methods. this is a last resort.
	Get(id ID, key string) (interface{}, error)
	Put(id ID, key string, val interface{}) error
}

// AddressBook tracks the addresses of Peers
type AddressBook interface {
	Addresses(ID) []ma.Multiaddr
	AddAddress(ID, ma.Multiaddr)
	AddAddresses(ID, []ma.Multiaddr)
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

func (ab *addressbook) AddAddresses(p ID, ms []ma.Multiaddr) {
	ab.Lock()
	defer ab.Unlock()

	for _, m := range ms {
		_, found := ab.addrs[p]
		if !found {
			ab.addrs[p] = addressMap{}
		}
		ab.addrs[p][m.String()] = m
	}
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

	if sk == nil {
		return errors.New("sk is nil (PrivKey)")
	}

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
	metrics

	// store other data, like versions
	ds ds.ThreadSafeDatastore
}

// NewPeerstore creates a threadsafe collection of peers.
func NewPeerstore() Peerstore {
	return &peerstore{
		keybook:     *newKeybook(),
		addressbook: *newAddressbook(),
		metrics:     *(NewMetrics()).(*metrics),
		ds:          dssync.MutexWrap(ds.NewMapDatastore()),
	}
}

func (ps *peerstore) Put(p ID, key string, val interface{}) error {
	dsk := ds.NewKey(string(p) + "/" + key)
	return ps.ds.Put(dsk, val)
}

func (ps *peerstore) Get(p ID, key string) (interface{}, error) {
	dsk := ds.NewKey(string(p) + "/" + key)
	return ps.ds.Get(dsk)
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

func (ps *peerstore) PeerInfo(p ID) PeerInfo {
	return PeerInfo{
		ID:    p,
		Addrs: ps.addressbook.Addresses(p),
	}
}

func PeerInfos(ps Peerstore, peers []ID) []PeerInfo {
	pi := make([]PeerInfo, len(peers))
	for i, p := range peers {
		pi[i] = ps.PeerInfo(p)
	}
	return pi
}

func PeerInfoIDs(pis []PeerInfo) []ID {
	ps := make([]ID, len(pis))
	for i, pi := range pis {
		ps[i] = pi.ID
	}
	return ps
}
