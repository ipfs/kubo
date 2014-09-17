package dht

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/peer"
	queue "github.com/jbenet/go-ipfs/peer/queue"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
)

// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
// This is the top level "Store" operation of the DHT
func (dht *IpfsDHT) PutValue(key u.Key, value []byte) error {
	ctx := context.TODO()

	query := &dhtQuery{}
	query.peers = queue.NewXORDistancePQ(key)

	// get the peers we need to announce to
	for _, route := range dht.routingTables {
		peers := route.NearestPeers(kb.ConvertKey(key), KValue)
		for _, p := range peers {
			if p == nil {
				// this shouldn't be happening.
				panic("p should not be nil")
			}

			query.peers.Enqueue(p)
		}
	}

	query.qfunc = func(ctx context.Context, p *peer.Peer) (interface{}, []*peer.Peer, error) {
		dht.putValueToNetwork(ctx, p, string(key), value)
		return nil, nil, nil
	}

	_, err := query.Run(ctx, query.peers.Len())
	return err
}

// GetValue searches for the value corresponding to given Key.
// If the search does not succeed, a multiaddr string of a closer peer is
// returned along with util.ErrSearchIncomplete
func (dht *IpfsDHT) GetValue(key u.Key, timeout time.Duration) ([]byte, error) {
	ll := startNewRPC("GET")
	defer ll.EndAndPrint()

	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	// If we have it local, dont bother doing an RPC!
	// NOTE: this might not be what we want to do...
	val, err := dht.getLocal(key)
	if err == nil {
		ll.Success = true
		u.DOut("Found local, returning.\n")
		return val, nil
	}

	// get closest peers in the routing tables
	routeLevel := 0
	closest := dht.routingTables[routeLevel].NearestPeers(kb.ConvertKey(key), PoolSize)
	if closest == nil || len(closest) == 0 {
		return nil, kb.ErrLookupFailure
	}

	query := &dhtQuery{}
	query.peers = queue.NewXORDistancePQ(key)

	// get the peers we need to announce to
	for _, p := range closest {
		query.peers.Enqueue(p)
	}

	// setup the Query Function
	query.qfunc = func(ctx context.Context, p *peer.Peer) (interface{}, []*peer.Peer, error) {
		return dht.getValueOrPeers(ctx, p, key, routeLevel)
	}

	// run it!
	result, err := query.Run(ctx, query.peers.Len())
	if err != nil {
		return nil, err
	}

	byt, ok := result.([]byte)
	if !ok {
		return nil, fmt.Errorf("received non-byte slice value")
	}
	return byt, nil
}

// Value provider layer of indirection.
// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

// Provide makes this node announce that it can provide a value for the given key
func (dht *IpfsDHT) Provide(key u.Key) error {
	ctx := context.TODO()

	dht.providers.AddProvider(key, dht.self)
	peers := dht.routingTables[0].NearestPeers(kb.ConvertKey(key), PoolSize)
	if len(peers) == 0 {
		return kb.ErrLookupFailure
	}

	for _, p := range peers {
		err := dht.putProvider(ctx, p, string(key))
		if err != nil {
			return err
		}
	}
	return nil
}

// FindProvidersAsync runs FindProviders and sends back results over a channel
func (dht *IpfsDHT) FindProvidersAsync(key u.Key, count int, timeout time.Duration) chan *peer.Peer {
	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	peerOut := make(chan *peer.Peer, count)
	go func() {
		ps := newPeerSet()
		provs := dht.providers.GetProviders(key)
		for _, p := range provs {
			count--
			// NOTE: assuming that this list of peers is unique
			ps.Add(p)
			peerOut <- p
			if count <= 0 {
				return
			}
		}

		peers := dht.routingTables[0].NearestPeers(kb.ConvertKey(key), AlphaValue)
		for _, pp := range peers {
			ppp := pp
			go func() {
				pmes, err := dht.findProvidersSingle(ctx, ppp, key, 0)
				if err != nil {
					u.PErr("%v\n", err)
					return
				}
				dht.addPeerListAsync(key, pmes.GetProviderPeers(), ps, count, peerOut)
			}()
		}

	}()
	return peerOut
}

//TODO: this function could also be done asynchronously
func (dht *IpfsDHT) addPeerListAsync(k u.Key, peers []*Message_Peer, ps *peerSet, count int, out chan *peer.Peer) {
	for _, pbp := range peers {

		// construct new peer
		p, err := dht.ensureConnectedToPeer(pbp)
		if err != nil {
			continue
		}

		dht.providers.AddProvider(k, p)
		if ps.AddIfSmallerThan(p, count) {
			out <- p
		} else if ps.Size() >= count {
			return
		}
	}
}

// FindProviders searches for peers who can provide the value for given key.
func (dht *IpfsDHT) FindProviders(key u.Key, timeout time.Duration) ([]*peer.Peer, error) {
	ll := startNewRPC("FindProviders")
	ll.EndAndPrint()

	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	// get closest peer
	u.DOut("Find providers for: '%s'\n", key)
	p := dht.routingTables[0].NearestPeer(kb.ConvertKey(key))
	if p == nil {
		return nil, kb.ErrLookupFailure
	}

	for level := 0; level < len(dht.routingTables); {

		// attempt retrieving providers
		pmes, err := dht.findProvidersSingle(ctx, p, key, level)
		if err != nil {
			return nil, err
		}

		// handle providers
		provs := pmes.GetProviderPeers()
		if provs != nil {
			u.DOut("Got providers back from findProviders call!\n")
			return dht.addProviders(key, provs), nil
		}

		u.DOut("Didnt get providers, just closer peers.\n")
		closer := pmes.GetCloserPeers()
		if len(closer) == 0 {
			level++
			continue
		}

		np, err := dht.peerFromInfo(closer[0])
		if err != nil {
			u.DOut("no peerFromInfo")
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
	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	// Check if were already connected to them
	p, _ := dht.Find(id)
	if p != nil {
		return p, nil
	}

	// @whyrusleeping why is this here? doesn't the dht.Find above cover it?
	routeLevel := 0
	p = dht.routingTables[routeLevel].NearestPeer(kb.ConvertPeerID(id))
	if p == nil {
		return nil, kb.ErrLookupFailure
	}
	if p.ID.Equal(id) {
		return p, nil
	}

	for routeLevel < len(dht.routingTables) {
		pmes, err := dht.findPeerSingle(ctx, p, id, routeLevel)
		plist := pmes.GetCloserPeers()
		if plist == nil || len(plist) == 0 {
			routeLevel++
			continue
		}
		found := plist[0]

		nxtPeer, err := dht.ensureConnectedToPeer(found)
		if err != nil {
			routeLevel++
			continue
		}

		if nxtPeer.ID.Equal(id) {
			return nxtPeer, nil
		}

		p = nxtPeer
	}
	return nil, u.ErrNotFound
}

func (dht *IpfsDHT) findPeerMultiple(id peer.ID, timeout time.Duration) (*peer.Peer, error) {
	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	// Check if were already connected to them
	p, _ := dht.Find(id)
	if p != nil {
		return p, nil
	}

	query := &dhtQuery{}
	query.peers = queue.NewXORDistancePQ(u.Key(id))

	// get the peers we need to announce to
	routeLevel := 0
	peers := dht.routingTables[routeLevel].NearestPeers(kb.ConvertPeerID(id), AlphaValue)
	if len(peers) == 0 {
		return nil, kb.ErrLookupFailure
	}
	for _, p := range peers {
		query.peers.Enqueue(p)
	}

	// setup query function
	query.qfunc = func(ctx context.Context, p *peer.Peer) (interface{}, []*peer.Peer, error) {
		pmes, err := dht.findPeerSingle(ctx, p, id, routeLevel)
		if err != nil {
			u.DErr("getPeer error: %v\n", err)
			return nil, nil, err
		}

		plist := pmes.GetCloserPeers()
		if len(plist) == 0 {
			routeLevel++
		}

		nxtprs := make([]*peer.Peer, len(plist))
		for i, fp := range plist {
			nxtp, err := dht.peerFromInfo(fp)
			if err != nil {
				u.DErr("findPeer error: %v\n", err)
				continue
			}

			if nxtp.ID.Equal(id) {
				return nxtp, nil, nil
			}

			nxtprs[i] = nxtp
		}

		return nil, nxtprs, nil
	}

	p5, err := query.Run(ctx, query.peers.Len())
	if err != nil {
		return nil, err
	}

	p6, ok := p5.(*peer.Peer)
	if !ok {
		return nil, errors.New("received non peer object")
	}
	return p6, nil
}

// Ping a peer, log the time it took
func (dht *IpfsDHT) Ping(p *peer.Peer, timeout time.Duration) error {
	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	// Thoughts: maybe this should accept an ID and do a peer lookup?
	u.DOut("Enter Ping.\n")

	pmes := newMessage(Message_PING, "", 0)
	_, err := dht.sendRequest(ctx, p, pmes)
	return err
}

func (dht *IpfsDHT) getDiagnostic(timeout time.Duration) ([]*diagInfo, error) {
	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	u.DOut("Begin Diagnostic")
	query := &dhtQuery{}
	query.peers = queue.NewXORDistancePQ(u.Key(dht.self.ID))

	targets := dht.routingTables[0].NearestPeers(kb.ConvertPeerID(dht.self.ID), 10)
	for _, p := range targets {
		query.peers.Enqueue(p)
	}

	var out []*diagInfo

	query.qfunc = func(ctx context.Context, p *peer.Peer) (interface{}, []*peer.Peer, error) {
		pmes := newMessage(Message_DIAGNOSTIC, "", 0)
		rpmes, err := dht.sendRequest(ctx, p, pmes)
		if err != nil {
			return nil, nil, err
		}

		dec := json.NewDecoder(bytes.NewBuffer(rpmes.GetValue()))
		for {
			di := new(diagInfo)
			err := dec.Decode(di)
			if err != nil {
				break
			}

			out = append(out, di)
		}
		return nil, nil, nil
	}

	_, err := query.Run(ctx, query.peers.Len())
	return out, err
}
