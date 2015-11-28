package dht

import (
	"sync"

	key "github.com/ipfs/go-ipfs/blocks/key"
	routing "github.com/ipfs/go-ipfs/routing"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	kb "github.com/ipfs/go-ipfs/routing/kbucket"

	pset "github.com/ipfs/go-ipfs/thirdparty/peerset"
	todoctr "github.com/ipfs/go-ipfs/thirdparty/todocounter"
	ctxproc "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess/context"
	peer "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer"
	pqueue "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer/queue"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

// peerFifo implements the peerQueue interface, but provides no additional
// ordering beyond that of insertion order
type peerFifo struct {
	peers []peer.ID
	lk    sync.Mutex
}

func (oq *peerFifo) Enqueue(p peer.ID) {
	oq.lk.Lock()
	defer oq.lk.Unlock()
	oq.peers = append(oq.peers, p)
}

func (oq *peerFifo) Dequeue() peer.ID {
	oq.lk.Lock()
	defer oq.lk.Unlock()
	out := oq.peers[0]
	oq.peers = oq.peers[1:]
	return out
}

func (oq *peerFifo) Len() int {
	oq.lk.Lock()
	defer oq.lk.Unlock()
	return len(oq.peers)
}

type provManyReq struct {
	closest     map[key.Key]pqueue.PeerQueue
	perKeyQuery map[key.Key]pqueue.PeerQueue
	queried     *pset.PeerSet

	keys    []key.Key
	keyStrs []string

	nextTarget int
	gntLock    sync.Mutex
}

func newProvManyReq(ctx context.Context, dht *IpfsDHT, keys []key.Key) *provManyReq {
	closest := make(map[key.Key]pqueue.PeerQueue)
	perKeyQuery := make(map[key.Key]pqueue.PeerQueue)
	var keyStrs []string

	for _, k := range keys {
		keyStrs = append(keyStrs, string(k))
		dht.providers.AddProvider(ctx, k, dht.self)

		closest[k] = pqueue.NewXORDistancePQ(string(k))
		perKeyQuery[k] = pqueue.NewXORDistancePQ(string(k))

		peers := dht.routingTable.NearestPeers(kb.ConvertKey(k), 20)
		for _, p := range peers {
			closest[k].Enqueue(p)
			perKeyQuery[k].Enqueue(p)
		}
	}

	return &provManyReq{
		closest:     closest,
		perKeyQuery: perKeyQuery,
		keys:        keys,
		keyStrs:     keyStrs,
		queried:     pset.New(),
	}
}

func (pmr *provManyReq) getNextTarget() (peer.ID, bool) {
	pmr.gntLock.Lock()
	defer pmr.gntLock.Unlock()
	// iterate through entire list once, starting at last offset
	for i := pmr.nextTarget + 1; i != pmr.nextTarget; i = (i + 1) % len(pmr.keys) {
		k := pmr.keys[i]
		for pmr.perKeyQuery[k].Len() > 0 {
			p := pmr.perKeyQuery[k].Dequeue()
			if pmr.queried.TryAdd(p) {
				pmr.nextTarget = i
				return p, true
			}
		}
	}

	return "", false
}

func (pmr *provManyReq) addCloserPeers(ps []peer.ID) {
	pmr.gntLock.Lock()
	defer pmr.gntLock.Unlock()
	for _, p := range ps {
		if pmr.queried.Contains(p) {
			continue
		}

		for _, k := range pmr.keys {
			pmr.perKeyQuery[k].Enqueue(p)
			pmr.closest[k].Enqueue(p)
		}
	}
}

func (pmr *provManyReq) finalProvideSet() map[peer.ID][]key.Key {
	final := make(map[peer.ID][]key.Key)
	for k, cq := range pmr.closest {
		for i := 0; i < KValue && cq.Len() > 0; i++ {
			p := cq.Dequeue()
			final[p] = append(final[p], k)
		}
	}

	return final
}

func (dht *IpfsDHT) ProvideMany(ctx context.Context, keys []key.Key) error {
	defer log.EventBegin(ctx, "provideMany").Done()

	pmreq := newProvManyReq(ctx, dht, keys)

	t := pb.Message_FIND_NODE
	mes := &pb.Message{
		Type: &t,
		Keys: pmreq.keyStrs,
	}

	query := dht.newQuery("", func(ctx context.Context, p peer.ID) (*dhtQueryResult, error) {
		resp, err := dht.sendRequest(ctx, p, mes)
		if err != nil {
			return nil, err
		}

		peers := pb.PBPeersToPeerInfos(resp.GetCloserPeers())
		var pids []peer.ID
		for _, clpeer := range peers {
			dht.peerstore.AddAddrs(clpeer.ID, clpeer.Addrs, peer.TempAddrTTL)
			pids = append(pids, clpeer.ID)
		}

		pmreq.addCloserPeers(pids)

		result := new(dhtQueryResult)
		next, ok := pmreq.getNextTarget()
		if ok {
			result.closerPeers = []peer.PeerInfo{{ID: next}}
		}

		return result, nil
	})

	dqr := dhtQueryRunner{
		query:          query,
		peersToQuery:   pqueue.NewChanQueue(ctx, new(peerFifo)),
		peersSeen:      pset.New(),
		rateLimit:      make(chan struct{}, query.concurrency),
		peersRemaining: todoctr.NewSyncCounter(),
		proc:           ctxproc.WithContext(ctx),
	}

	var starter []peer.ID
	for i := 0; i < 5; i++ {
		p, ok := pmreq.getNextTarget()
		if ok {
			starter = append(starter, p)
		} else {
			log.Warning("not enough peers to fully start ProvideMany query")
			break
		}
	}

	_, err := dqr.Run(ctx, starter)
	if err != nil && err != routing.ErrNotFound {
		return err
	}

	final := pmreq.finalProvideSet()
	for p, keys := range final {
		// TODO: maybe this in parallel?
		err := dht.putProviders(ctx, p, keys)
		if err != nil {
			log.Errorf("putProviders: %s", err)
			continue
		}
	}

	return nil
}
