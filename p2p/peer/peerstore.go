package peer

import (
	"errors"
	"sync"
	"time"

	ic "github.com/jbenet/go-ipfs/p2p/crypto"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

const (
	// AddressTTL is the expiration time of addresses.
	AddressTTL = time.Hour
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

	// AddPeerInfo absorbs the information listed in given PeerInfo.
	AddPeerInfo(PeerInfo)

	// Get/Put is a simple registry for other peer-related key/value pairs.
	// if we find something we use often, it should become its own set of
	// methods. this is a last resort.
	Get(id ID, key string) (interface{}, error)
	Put(id ID, key string, val interface{}) error
}

// AddressBook tracks the addresses of Peers
type AddressBook interface {
	Addresses(ID) []ma.Multiaddr     // returns addresses for ID
	AddAddress(ID, ma.Multiaddr)     // Adds given addr for ID
	AddAddresses(ID, []ma.Multiaddr) // Adds given addrs for ID
	SetAddresses(ID, []ma.Multiaddr) // Sets given addrs for ID (clears previously stored)
}

type expiringAddr struct {
	Addr ma.Multiaddr
	TTL  time.Time
}

func (e *expiringAddr) Expired() bool {
	return time.Now().After(e.TTL)
}

type addressMap map[string]expiringAddr

type addressbook struct {
	sync.RWMutex // guards all fields

	addrs map[ID]addressMap
	ttl   time.Duration // initial ttl
}

func newAddressbook() *addressbook {
	return &addressbook{
		addrs: map[ID]addressMap{},
		ttl:   AddressTTL,
	}
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
	ab.Lock()
	defer ab.Unlock()

	maddrs, found := ab.addrs[p]
	if !found {
		return nil
	}

	good := make([]ma.Multiaddr, 0, len(maddrs))
	var expired []string
	for s, m := range maddrs {
		if m.Expired() {
			expired = append(expired, s)
		} else {
			good = append(good, m.Addr)
		}
	}

	// clean up the expired ones.
	for _, s := range expired {
		delete(ab.addrs[p], s)
	}
	return good
}

func (ab *addressbook) AddAddress(p ID, m ma.Multiaddr) {
	ab.AddAddresses(p, []ma.Multiaddr{m})
}

func (ab *addressbook) AddAddresses(p ID, ms []ma.Multiaddr) {
	ab.Lock()
	defer ab.Unlock()

	amap, found := ab.addrs[p]
	if !found {
		amap = addressMap{}
		ab.addrs[p] = amap
	}

	ttl := time.Now().Add(ab.ttl)
	for _, m := range ms {
		// re-set all of them for new ttl.
		amap[m.String()] = expiringAddr{
			Addr: m,
			TTL:  ttl,
		}
	}
}

func (ab *addressbook) SetAddresses(p ID, ms []ma.Multiaddr) {
	ab.Lock()
	defer ab.Unlock()

	amap := addressMap{}
	ttl := time.Now().Add(ab.ttl)
	for _, m := range ms {
		amap[m.String()] = expiringAddr{Addr: m, TTL: ttl}
	}
	ab.addrs[p] = amap // clear what was there before
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

func (ps *peerstore) AddPeerInfo(pi PeerInfo) {
	ps.AddAddresses(pi.ID, pi.Addrs)
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
