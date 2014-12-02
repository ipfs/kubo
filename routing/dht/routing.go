package dht

import (
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
)

// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
// This is the top level "Store" operation of the DHT
func (dht *IpfsDHT) PutValue(ctx context.Context, key u.Key, value []byte) error {
	log.Debugf("PutValue %s", key)
	err := dht.putLocal(key, value)
	if err != nil {
		return err
	}

	rec, err := dht.makePutRecord(key, value)
	if err != nil {
		log.Error("Creation of record failed!")
		return err
	}

	var peers []peer.Peer
	for _, route := range dht.routingTables {
		npeers := route.NearestPeers(kb.ConvertKey(key), KValue)
		peers = append(peers, npeers...)
	}

	query := newQuery(key, dht.dialer, func(ctx context.Context, p peer.Peer) (*dhtQueryResult, error) {
		log.Debugf("%s PutValue qry part %v", dht.self, p)
		err := dht.putValueToNetwork(ctx, p, string(key), rec)
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
	log.Debugf("Get Value [%s]", key)

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
		return nil, kb.ErrLookupFailure
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

	log.Debugf("GetValue %v %v", key, result.value)
	if result.value == nil {
		return nil, routing.ErrNotFound
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
	log.Event(ctx, "findProviders", &key)
	peerOut := make(chan peer.Peer, count)
	go func() {
		defer close(peerOut)

		ps := newPeerSet()
		// TODO may want to make this function async to hide latency
		provs := dht.providers.GetProviders(key)
		for _, p := range provs {
			count--
			// NOTE: assuming that this list of peers is unique
			ps.Add(p)
			select {
			case peerOut <- p:
			case <-ctx.Done():
				return
			}
			if count <= 0 {
				return
			}
		}

		var wg sync.WaitGroup
		for _, pp := range dht.routingTables[0].NearestPeers(kb.ConvertKey(key), AlphaValue) {
			wg.Add(1)
			go func(p peer.Peer) {
				defer wg.Done()
				pmes, err := dht.findProvidersSingle(ctx, p, key, 0)
				if err != nil {
					log.Error(err)
					return
				}
				dht.addPeerListAsync(ctx, key, pmes.GetProviderPeers(), ps, count, peerOut)
			}(pp)
		}
		wg.Wait()
	}()
	return peerOut
}

func (dht *IpfsDHT) addPeerListAsync(ctx context.Context, k u.Key, peers []*pb.Message_Peer, ps *peerSet, count int, out chan peer.Peer) {
	var wg sync.WaitGroup
	for _, pbp := range peers {
		wg.Add(1)
		go func(mp *pb.Message_Peer) {
			defer wg.Done()
			// construct new peer
			p, err := dht.ensureConnectedToPeer(ctx, mp)
			if err != nil {
				log.Errorf("%s", err)
				return
			}
			if p == nil {
				log.Error("Got nil peer from ensureConnectedToPeer")
				return
			}

			dht.providers.AddProvider(k, p)
			if ps.AddIfSmallerThan(p, count) {
				select {
				case out <- p:
				case <-ctx.Done():
					return
				}
			} else if ps.Size() >= count {
				return
			}
		}(pbp)
	}
	wg.Wait()
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
	closest := dht.routingTables[routeLevel].NearestPeers(kb.ConvertPeerID(id), AlphaValue)
	if closest == nil || len(closest) == 0 {
		return nil, kb.ErrLookupFailure
	}

	// Sanity...
	for _, p := range closest {
		if p.ID().Equal(id) {
			log.Error("Found target peer in list of closest peers...")
			return p, nil
		}
	}

	// setup the Query
	query := newQuery(u.Key(id), dht.dialer, func(ctx context.Context, p peer.Peer) (*dhtQueryResult, error) {

		pmes, err := dht.findPeerSingle(ctx, p, id, routeLevel)
		if err != nil {
			return nil, err
		}

		closer := pmes.GetCloserPeers()
		var clpeers []peer.Peer
		for _, pbp := range closer {
			np, err := dht.getPeer(peer.ID(pbp.GetId()))
			if err != nil {
				log.Warningf("Received invalid peer from query: %v", err)
				continue
			}
			ma, err := pbp.Address()
			if err != nil {
				log.Warning("Received peer with bad or missing address.")
				continue
			}
			np.AddAddress(ma)
			if pbp.GetId() == string(id) {
				return &dhtQueryResult{
					peer:    np,
					success: true,
				}, nil
			}
			clpeers = append(clpeers, np)
		}

		return &dhtQueryResult{closerPeers: clpeers}, nil
	})

	// run it!
	result, err := query.Run(ctx, closest)
	if err != nil {
		return nil, err
	}

	log.Debugf("FindPeer %v %v", id, result.success)
	if result.peer == nil {
		return nil, routing.ErrNotFound
	}

	return result.peer, nil
}

// Ping a peer, log the time it took
func (dht *IpfsDHT) Ping(ctx context.Context, p peer.Peer) error {
	// Thoughts: maybe this should accept an ID and do a peer lookup?
	log.Infof("ping %s start", p)

	pmes := pb.NewMessage(pb.Message_PING, "", 0)
	_, err := dht.sendRequest(ctx, p, pmes)
	log.Infof("ping %s end (err = %s)", p, err)
	return err
}
