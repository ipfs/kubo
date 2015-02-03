package dht

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	notif "github.com/jbenet/go-ipfs/notifications"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	pset "github.com/jbenet/go-ipfs/util/peerset"
)

// Required in order for proper JSON marshaling
func pointerizePeerInfos(pis []peer.PeerInfo) []*peer.PeerInfo {
	out := make([]*peer.PeerInfo, len(pis))
	for i, p := range pis {
		np := p
		out[i] = &np
	}
	return out
}

// Kademlia 'node lookup' operation. Returns a channel of the K closest peers
// to the given key
func (dht *IpfsDHT) GetClosestPeers(ctx context.Context, key u.Key) (<-chan peer.ID, error) {
	e := log.EventBegin(ctx, "getClosestPeers", &key)
	tablepeers := dht.routingTable.NearestPeers(kb.ConvertKey(key), AlphaValue)
	if len(tablepeers) == 0 {
		return nil, errors.Wrap(kb.ErrLookupFailure)
	}

	out := make(chan peer.ID, KValue)
	peerset := pset.NewLimited(KValue)

	for _, p := range tablepeers {
		select {
		case out <- p:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		peerset.Add(p)
	}

	query := dht.newQuery(key, func(ctx context.Context, p peer.ID) (*dhtQueryResult, error) {
		// For DHT query command
		notif.PublishQueryEvent(ctx, &notif.QueryEvent{
			Type: notif.SendingQuery,
			ID:   p,
		})

		closer, err := dht.closerPeersSingle(ctx, key, p)
		if err != nil {
			log.Debugf("error getting closer peers: %s", err)
			return nil, err
		}

		var filtered []peer.PeerInfo
		for _, clp := range closer {
			if kb.Closer(clp, dht.self, key) && peerset.TryAdd(clp) {
				select {
				case out <- clp:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				filtered = append(filtered, dht.peerstore.PeerInfo(clp))
			}
		}

		// For DHT query command
		notif.PublishQueryEvent(ctx, &notif.QueryEvent{
			Type:      notif.PeerResponse,
			ID:        p,
			Responses: pointerizePeerInfos(filtered),
		})

		return &dhtQueryResult{closerPeers: filtered}, nil
	})

	go func() {
		defer close(out)
		defer e.Done()
		// run it!
		_, err := query.Run(ctx, tablepeers)
		if err != nil {
			log.Debugf("closestPeers query run error: %s", err)
		}
	}()

	return out, nil
}

func (dht *IpfsDHT) closerPeersSingle(ctx context.Context, key u.Key, p peer.ID) ([]peer.ID, error) {
	pmes, err := dht.findPeerSingle(ctx, p, peer.ID(key))
	if err != nil {
		return nil, err
	}

	var out []peer.ID
	for _, pbp := range pmes.GetCloserPeers() {
		pid := peer.ID(pbp.GetId())
		if pid != dht.self { // dont add self
			dht.peerstore.AddAddrs(pid, pbp.Addresses(), peer.TempAddrTTL)
			out = append(out, pid)
		}
	}
	return out, nil
}
