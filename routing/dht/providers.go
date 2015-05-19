package dht

import (
	"time"

	ctxgroup "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	u "github.com/ipfs/go-ipfs/util"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type providerInfo struct {
	Creation time.Time
	Value    peer.ID
}

type ProviderManager struct {
	providers map[u.Key][]*providerInfo
	local     map[u.Key]struct{}
	lpeer     peer.ID
	getlocal  chan chan []u.Key
	newprovs  chan *addProv
	getprovs  chan *getProv
	period    time.Duration
	ctxgroup.ContextGroup
}

type addProv struct {
	k   u.Key
	val peer.ID
}

type getProv struct {
	k    u.Key
	resp chan []peer.ID
}

func NewProviderManager(ctx context.Context, local peer.ID) *ProviderManager {
	pm := new(ProviderManager)
	pm.getprovs = make(chan *getProv)
	pm.newprovs = make(chan *addProv)
	pm.providers = make(map[u.Key][]*providerInfo)
	pm.getlocal = make(chan chan []u.Key)
	pm.local = make(map[u.Key]struct{})
	pm.ContextGroup = ctxgroup.WithContext(ctx)

	pm.Children().Add(1)
	go pm.run()

	return pm
}

func (pm *ProviderManager) run() {
	defer pm.Children().Done()

	tick := time.NewTicker(time.Hour)
	for {
		select {
		case np := <-pm.newprovs:
			if np.val == pm.lpeer {
				pm.local[np.k] = struct{}{}
			}
			pi := new(providerInfo)
			pi.Creation = time.Now()
			pi.Value = np.val
			arr := pm.providers[np.k]
			pm.providers[np.k] = append(arr, pi)

		case gp := <-pm.getprovs:
			var parr []peer.ID
			provs := pm.providers[gp.k]
			for _, p := range provs {
				parr = append(parr, p.Value)
			}
			gp.resp <- parr

		case lc := <-pm.getlocal:
			var keys []u.Key
			for k := range pm.local {
				keys = append(keys, k)
			}
			lc <- keys

		case <-tick.C:
			for k, provs := range pm.providers {
				var filtered []*providerInfo
				for _, p := range provs {
					if time.Now().Sub(p.Creation) < time.Hour*24 {
						filtered = append(filtered, p)
					}
				}
				pm.providers[k] = filtered
			}

		case <-pm.Closing():
			return
		}
	}
}

func (pm *ProviderManager) AddProvider(ctx context.Context, k u.Key, val peer.ID) {
	prov := &addProv{
		k:   k,
		val: val,
	}
	select {
	case pm.newprovs <- prov:
	case <-ctx.Done():
	}
}

func (pm *ProviderManager) GetProviders(ctx context.Context, k u.Key) []peer.ID {
	gp := &getProv{
		k:    k,
		resp: make(chan []peer.ID, 1), // buffered to prevent sender from blocking
	}
	select {
	case <-ctx.Done():
		return nil
	case pm.getprovs <- gp:
	}
	select {
	case <-ctx.Done():
		return nil
	case peers := <-gp.resp:
		return peers
	}
}

func (pm *ProviderManager) GetLocal() []u.Key {
	resp := make(chan []u.Key)
	pm.getlocal <- resp
	return <-resp
}
