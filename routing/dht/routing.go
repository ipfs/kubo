package dht

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
)

// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
// This is the top level "Store" operation of the DHT
func (dht *IpfsDHT) PutValue(ctx context.Context, key u.Key, value []byte) error {
	log.Debug("PutValue %s", key)
	err := dht.putLocal(key, value)
	if err != nil {
		return err
	}

	var peers []peer.Peer
	for _, route := range dht.routingTables {
		npeers := route.NearestPeers(kb.ConvertKey(key), KValue)
		peers = append(peers, npeers...)
	}

	query := newQuery(key, dht.dialer, func(ctx context.Context, p peer.Peer) (*dhtQueryResult, error) {
		log.Debug("%s PutValue qry part %v", dht.self, p)
		err := dht.putValueToNetwork(ctx, p, string(key), value)
		if err != nil {
			return nil, err
		}
		return &dhtQueryResult{success: true}, nil
	})

	_, err = query.Run(ctx, peers)
	return err
}

// GetValue searches for the value corresponding to given Key.
// If the search does not succeed, a multiaddr string of a closer peer is
// returned along with util.ErrSearchIncomplete
func (dht *IpfsDHT) GetValue(ctx context.Context, key u.Key) ([]byte, error) {
	log.Debug("Get Value [%s]", key)

	// If we have it local, dont bother doing an RPC!
	// NOTE: this might not be what we want to do...
	val, err := dht.getLocal(key)
	if err == nil {
		log.Debug("Got value locally!")
		return val, nil
	}

	// get closest peers in the routing tables
	routeLevel := 0
	closest := dht.routingTables[routeLevel].NearestPeers(kb.ConvertKey(key), PoolSize)
	if closest == nil || len(closest) == 0 {
		log.Warning("Got no peers back from routing table!")
		return nil, nil
	}

	// setup the Query
	query := newQuery(key, dht.dialer, func(ctx context.Context, p peer.Peer) (*dhtQueryResult, error) {

		val, peers, err := dht.getValueOrPeers(ctx, p, key, routeLevel)
		if err != nil {
			return nil, err
		}

		res := &dhtQueryResult{value: val, closerPeers: peers}
		if val != nil {
			res.success = true
		}

		return res, nil
	})

	// run it!
	result, err := query.Run(ctx, closest)
	if err != nil {
		return nil, err
	}

	log.Debug("GetValue %v %v", key, result.value)
	if result.value == nil {
		return nil, u.ErrNotFound
	}

	return result.value, nil
}

// Value provider layer of indirection.
// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

// Provide makes this node announce that it can provide a value for the given key
func (dht *IpfsDHT) Provide(ctx context.Context, key u.Key) error {

	dht.providers.AddProvider(key, dht.self)
	peers := dht.routingTables[0].NearestPeers(kb.ConvertKey(key), PoolSize)
	if len(peers) == 0 {
		return nil
	}

	//TODO FIX: this doesn't work! it needs to be sent to the actual nearest peers.
	// `peers` are the closest peers we have, not the ones that should get the value.
	for _, p := range peers {
		err := dht.putProvider(ctx, p, string(key))
		if err != nil {
			return err
		}
	}
	return nil
}

func (dht *IpfsDHT) FindProvidersAsync(ctx context.Context, key u.Key, count int) <-chan peer.Peer {
	peerOut := make(chan peer.Peer, count)
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

		wg := new(sync.WaitGroup)
		peers := dht.routingTables[0].NearestPeers(kb.ConvertKey(key), AlphaValue)
		for _, pp := range peers {
			wg.Add(1)
			go func(p peer.Peer) {
				defer wg.Done()
				pmes, err := dht.findProvidersSingle(ctx, p, key, 0)
				if err != nil {
					log.Error("%s", err)
					return
				}
				dht.addPeerListAsync(key, pmes.GetProviderPeers(), ps, count, peerOut)
			}(pp)
		}
		wg.Wait()
		close(peerOut)
	}()
	return peerOut
}

//TODO: this function could also be done asynchronously
func (dht *IpfsDHT) addPeerListAsync(k u.Key, peers []*Message_Peer, ps *peerSet, count int, out chan peer.Peer) {
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

// Find specific Peer
// FindPeer searches for a peer with given ID.
func (dht *IpfsDHT) FindPeer(ctx context.Context, id peer.ID) (peer.Peer, error) {

	// Check if were already connected to them
	p, _ := dht.FindLocal(id)
	if p != nil {
		return p, nil
	}

	routeLevel := 0
	p = dht.routingTables[routeLevel].NearestPeer(kb.ConvertPeerID(id))
	if p == nil {
		return nil, nil
	}
	if p.ID().Equal(id) {
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

		if nxtPeer.ID().Equal(id) {
			return nxtPeer, nil
		}

		p = nxtPeer
	}
	return nil, u.ErrNotFound
}

func (dht *IpfsDHT) findPeerMultiple(ctx context.Context, id peer.ID) (peer.Peer, error) {

	// Check if were already connected to them
	p, _ := dht.FindLocal(id)
	if p != nil {
		return p, nil
	}

	// get the peers we need to announce to
	routeLevel := 0
	peers := dht.routingTables[routeLevel].NearestPeers(kb.ConvertPeerID(id), AlphaValue)
	if len(peers) == 0 {
		return nil, nil
	}

	// setup query function
	query := newQuery(u.Key(id), dht.dialer, func(ctx context.Context, p peer.Peer) (*dhtQueryResult, error) {
		pmes, err := dht.findPeerSingle(ctx, p, id, routeLevel)
		if err != nil {
			log.Error("%s getPeer error: %v", dht.self, err)
			return nil, err
		}

		plist := pmes.GetCloserPeers()
		if len(plist) == 0 {
			routeLevel++
		}

		nxtprs := make([]peer.Peer, len(plist))
		for i, fp := range plist {
			nxtp, err := dht.peerFromInfo(fp)
			if err != nil {
				log.Error("%s findPeer error: %v", dht.self, err)
				continue
			}

			if nxtp.ID().Equal(id) {
				return &dhtQueryResult{peer: nxtp, success: true}, nil
			}

			nxtprs[i] = nxtp
		}

		return &dhtQueryResult{closerPeers: nxtprs}, nil
	})

	result, err := query.Run(ctx, peers)
	if err != nil {
		return nil, err
	}

	if result.peer == nil {
		return nil, u.ErrNotFound
	}
	return result.peer, nil
}

// Ping a peer, log the time it took
func (dht *IpfsDHT) Ping(ctx context.Context, p peer.Peer) error {
	// Thoughts: maybe this should accept an ID and do a peer lookup?
	log.Info("ping %s start", p)

	pmes := newMessage(Message_PING, "", 0)
	_, err := dht.sendRequest(ctx, p, pmes)
	log.Info("ping %s end (err = %s)", p, err)
	return err
}
