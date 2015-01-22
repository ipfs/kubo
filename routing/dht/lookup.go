package dht

import (
	"encoding/json"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	peer "github.com/jbenet/go-ipfs/p2p/peer"
	kb "github.com/jbenet/go-ipfs/routing/kbucket"
	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	pset "github.com/jbenet/go-ipfs/util/peerset"
)

type QueryEventType int

const (
	SendingQuery QueryEventType = iota
	PeerResponse
	FinalPeer
)

type QueryEvent struct {
	ID        peer.ID
	Type      QueryEventType
	Responses []*peer.PeerInfo
}

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
func (dht *IpfsDHT) GetClosestPeers(ctx context.Context, key u.Key, events chan<- *QueryEvent) (<-chan peer.ID, error) {
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
		select {
		case events <- &QueryEvent{
			Type: SendingQuery,
			ID:   p,
		}:
		}

		closer, err := dht.closerPeersSingle(ctx, key, p)
		if err != nil {
			log.Errorf("error getting closer peers: %s", err)
			return nil, err
		}

		var filtered []peer.PeerInfo
		for _, clp := range closer {
			if kb.Closer(clp, dht.self, key) && peerset.TryAdd(clp) {
				select {
				case out <- clp:
					log.Error("Sending out peer: %s", clp.Pretty())
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				filtered = append(filtered, dht.peerstore.PeerInfo(clp))
			}
		}
		log.Errorf("filtered: %v", filtered)

		// For DHT query command
		select {
		case events <- &QueryEvent{
			Type:      PeerResponse,
			ID:        p,
			Responses: pointerizePeerInfos(filtered),
		}:
		}

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
			dht.peerstore.AddAddresses(pid, pbp.Addresses())
			out = append(out, pid)
		}
	}
	return out, nil
}

func (qe *QueryEvent) MarshalJSON() ([]byte, error) {
	out := make(map[string]interface{})
	out["ID"] = peer.IDB58Encode(qe.ID)
	out["Type"] = int(qe.Type)
	out["Responses"] = qe.Responses
	return json.Marshal(out)
}

func (qe *QueryEvent) UnmarshalJSON(b []byte) error {
	temp := struct {
		ID        string
		Type      int
		Responses []*peer.PeerInfo
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}
	pid, err := peer.IDB58Decode(temp.ID)
	if err != nil {
		return err
	}
	qe.ID = pid
	qe.Type = QueryEventType(temp.Type)
	qe.Responses = temp.Responses
	return nil
}
