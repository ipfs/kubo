package dht

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

	proto "code.google.com/p/goprotobuf/proto"

	ma "github.com/jbenet/go-multiaddr"

	peer "github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
)

// Pool size is the number of nodes used for group find/set RPC calls
var PoolSize = 6

// We put the 'K' in kademlia!
var KValue = 10

// Its in the paper, i swear
var AlphaValue = 3

// GenerateMessageID creates and returns a new message ID
// TODO: determine a way of creating and managing message IDs
func GenerateMessageID() uint64 {
	return (uint64(rand.Uint32()) << 32) | uint64(rand.Uint32())
}

// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
// This is the top level "Store" operation of the DHT
func (dht *IpfsDHT) PutValue(key u.Key, value []byte) {
	complete := make(chan struct{})
	count := 0
	for _, route := range dht.routingTables {
		peers := route.NearestPeers(kb.ConvertKey(key), KValue)
		for _, p := range peers {
			if p == nil {
				dht.network.Error(kb.ErrLookupFailure)
				continue
			}
			count++
			go func(sp *peer.Peer) {
				err := dht.putValueToNetwork(sp, string(key), value)
				if err != nil {
					dht.network.Error(err)
				}
				complete <- struct{}{}
			}(p)
		}
	}
	for i := 0; i < count; i++ {
		<-complete
	}
}

// A counter for incrementing a variable across multiple threads
type counter struct {
	n   int
	mut sync.RWMutex
}

func (c *counter) Increment() {
	c.mut.Lock()
	c.n++
	c.mut.Unlock()
}

func (c *counter) Decrement() {
	c.mut.Lock()
	c.n--
	c.mut.Unlock()
}

func (c *counter) Size() int {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.n
}

type peerSet struct {
	ps map[string]bool
	lk sync.RWMutex
}

func newPeerSet() *peerSet {
	ps := new(peerSet)
	ps.ps = make(map[string]bool)
	return ps
}

func (ps *peerSet) Add(p *peer.Peer) {
	ps.lk.Lock()
	ps.ps[string(p.ID)] = true
	ps.lk.Unlock()
}

func (ps *peerSet) Contains(p *peer.Peer) bool {
	ps.lk.RLock()
	_, ok := ps.ps[string(p.ID)]
	ps.lk.RUnlock()
	return ok
}

func (ps *peerSet) Size() int {
	ps.lk.RLock()
	defer ps.lk.RUnlock()
	return len(ps.ps)
}

// GetValue searches for the value corresponding to given Key.
// If the search does not succeed, a multiaddr string of a closer peer is
// returned along with util.ErrSearchIncomplete
func (dht *IpfsDHT) GetValue(key u.Key, timeout time.Duration) ([]byte, error) {
	ll := startNewRPC("GET")
	defer func() {
		ll.EndLog()
		ll.Print()
	}()

	// If we have it local, dont bother doing an RPC!
	// NOTE: this might not be what we want to do...
	val, err := dht.getLocal(key)
	if err == nil {
		ll.Success = true
		u.DOut("Found local, returning.")
		return val, nil
	}

	routeLevel := 0
	closest := dht.routingTables[routeLevel].NearestPeers(kb.ConvertKey(key), PoolSize)
	if closest == nil || len(closest) == 0 {
		return nil, kb.ErrLookupFailure
	}

	valChan := make(chan []byte)
	npeerChan := make(chan *peer.Peer, 30)
	procPeer := make(chan *peer.Peer, 30)
	errChan := make(chan error)
	after := time.After(timeout)
	pset := newPeerSet()

	for _, p := range closest {
		pset.Add(p)
		npeerChan <- p
	}

	c := counter{}

	count := 0
	go func() {
		for {
			select {
			case p := <-npeerChan:
				count++
				if count >= KValue {
					break
				}
				c.Increment()

				procPeer <- p
			default:
				if c.Size() == 0 {
					errChan <- u.ErrNotFound
				}
			}
		}
	}()

	process := func() {
		for {
			select {
			case p, ok := <-procPeer:
				if !ok || p == nil {
					c.Decrement()
					return
				}
				val, peers, err := dht.getValueOrPeers(p, key, timeout/4, routeLevel)
				if err != nil {
					u.DErr("%v\n", err.Error())
					c.Decrement()
					continue
				}
				if val != nil {
					valChan <- val
					c.Decrement()
					return
				}

				for _, np := range peers {
					// TODO: filter out peers that arent closer
					if !pset.Contains(np) && pset.Size() < KValue {
						pset.Add(np) //This is racey... make a single function to do operation
						npeerChan <- np
					}
				}
				c.Decrement()
			}
		}
	}

	for i := 0; i < AlphaValue; i++ {
		go process()
	}

	select {
	case val := <-valChan:
		return val, nil
	case err := <-errChan:
		return nil, err
	case <-after:
		return nil, u.ErrTimeout
	}
}

// Value provider layer of indirection.
// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

// Provide makes this node announce that it can provide a value for the given key
func (dht *IpfsDHT) Provide(key u.Key) error {
	peers := dht.routingTables[0].NearestPeers(kb.ConvertKey(key), PoolSize)
	if len(peers) == 0 {
		return kb.ErrLookupFailure
	}

	pmes := Message{
		Type: PBDHTMessage_ADD_PROVIDER,
		Key:  string(key),
	}
	pbmes := pmes.ToProtobuf()

	for _, p := range peers {
		mes := swarm.NewMessage(p, pbmes)
		dht.network.Send(mes)
	}
	return nil
}

// FindProviders searches for peers who can provide the value for given key.
func (dht *IpfsDHT) FindProviders(key u.Key, timeout time.Duration) ([]*peer.Peer, error) {
	ll := startNewRPC("FindProviders")
	defer func() {
		ll.EndLog()
		ll.Print()
	}()
	u.DOut("Find providers for: '%s'\n", key)
	p := dht.routingTables[0].NearestPeer(kb.ConvertKey(key))
	if p == nil {
		return nil, kb.ErrLookupFailure
	}

	for level := 0; level < len(dht.routingTables); {
		pmes, err := dht.findProvidersSingle(p, key, level, timeout)
		if err != nil {
			return nil, err
		}
		if pmes.GetSuccess() {
			provs := dht.addPeerList(key, pmes.GetPeers())
			ll.Success = true
			return provs, nil
		}

		closer := pmes.GetPeers()
		if len(closer) == 0 {
			level++
			continue
		}
		if peer.ID(closer[0].GetId()).Equal(dht.self.ID) {
			u.DOut("Got myself back as a closer peer.")
			return nil, u.ErrNotFound
		}
		maddr, err := ma.NewMultiaddr(closer[0].GetAddr())
		if err != nil {
			// ??? Move up route level???
			panic("not yet implemented")
		}

		np, err := dht.network.GetConnection(peer.ID(closer[0].GetId()), maddr)
		if err != nil {
			u.PErr("[%s] Failed to connect to: %s\n", dht.self.ID.Pretty(), closer[0].GetAddr())
			level++
			continue
		}
		p = np
	}
	return nil, u.ErrNotFound
}

// Find specific Peer

// FindPeer searches for a peer with given ID.
func (dht *IpfsDHT) FindPeer(id peer.ID, timeout time.Duration) (*peer.Peer, error) {
	// Check if were already connected to them
	p, _ := dht.Find(id)
	if p != nil {
		return p, nil
	}

	routeLevel := 0
	p = dht.routingTables[routeLevel].NearestPeer(kb.ConvertPeerID(id))
	if p == nil {
		return nil, kb.ErrLookupFailure
	}
	if p.ID.Equal(id) {
		return p, nil
	}

	for routeLevel < len(dht.routingTables) {
		pmes, err := dht.findPeerSingle(p, id, timeout, routeLevel)
		plist := pmes.GetPeers()
		if len(plist) == 0 {
			routeLevel++
		}
		found := plist[0]

		addr, err := ma.NewMultiaddr(found.GetAddr())
		if err != nil {
			return nil, err
		}

		nxtPeer, err := dht.network.GetConnection(peer.ID(found.GetId()), addr)
		if err != nil {
			return nil, err
		}
		if pmes.GetSuccess() {
			if !id.Equal(nxtPeer.ID) {
				return nil, errors.New("got back invalid peer from 'successful' response")
			}
			return nxtPeer, nil
		}
		p = nxtPeer
	}
	return nil, u.ErrNotFound
}

// Ping a peer, log the time it took
func (dht *IpfsDHT) Ping(p *peer.Peer, timeout time.Duration) error {
	// Thoughts: maybe this should accept an ID and do a peer lookup?
	u.DOut("Enter Ping.")

	pmes := Message{ID: GenerateMessageID(), Type: PBDHTMessage_PING}
	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	before := time.Now()
	responseChan := dht.listener.Listen(pmes.ID, 1, time.Minute)
	dht.network.Send(mes)

	tout := time.After(timeout)
	select {
	case <-responseChan:
		roundtrip := time.Since(before)
		p.SetLatency(roundtrip)
		u.DOut("Ping took %s.\n", roundtrip.String())
		return nil
	case <-tout:
		// Timed out, think about removing peer from network
		u.DOut("Ping peer timed out.")
		dht.listener.Unlisten(pmes.ID)
		return u.ErrTimeout
	}
}

func (dht *IpfsDHT) getDiagnostic(timeout time.Duration) ([]*diagInfo, error) {
	u.DOut("Begin Diagnostic")
	//Send to N closest peers
	targets := dht.routingTables[0].NearestPeers(kb.ConvertPeerID(dht.self.ID), 10)

	// TODO: Add timeout to this struct so nodes know when to return
	pmes := Message{
		Type: PBDHTMessage_DIAGNOSTIC,
		ID:   GenerateMessageID(),
	}

	listenChan := dht.listener.Listen(pmes.ID, len(targets), time.Minute*2)

	pbmes := pmes.ToProtobuf()
	for _, p := range targets {
		mes := swarm.NewMessage(p, pbmes)
		dht.network.Send(mes)
	}

	var out []*diagInfo
	after := time.After(timeout)
	for count := len(targets); count > 0; {
		select {
		case <-after:
			u.DOut("Diagnostic request timed out.")
			return out, u.ErrTimeout
		case resp := <-listenChan:
			pmesOut := new(PBDHTMessage)
			err := proto.Unmarshal(resp.Data, pmesOut)
			if err != nil {
				// NOTE: here and elsewhere, need to audit error handling,
				//		some errors should be continued on from
				return out, err
			}

			dec := json.NewDecoder(bytes.NewBuffer(pmesOut.GetValue()))
			for {
				di := new(diagInfo)
				err := dec.Decode(di)
				if err != nil {
					break
				}

				out = append(out, di)
			}
		}
	}

	return nil, nil
}
