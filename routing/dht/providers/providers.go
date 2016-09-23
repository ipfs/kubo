package providers

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	goprocess "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess"
	goprocessctx "gx/ipfs/QmQopLATEYMNg7dVqZRNDfeE2S1yKy8zrRh5xnYiuqeZBn/goprocess/context"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	dsq "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/query"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	autobatch "gx/ipfs/QmVvJ27GcLaLSXvcB4auk3Gn3xuWK5ti5ENkZ2pCoJEYW4/autobatch"
	base32 "gx/ipfs/Qmb1DA2A9LS2wR4FFweB4uEDomFsdmnw1VLawLE1yQzudj/base32"

	key "github.com/ipfs/go-ipfs/blocks/key"
	flags "github.com/ipfs/go-ipfs/flags"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

var batchBufferSize = 256

func init() {
	if flags.LowMemMode {
		batchBufferSize = 8
	}
}

var log = logging.Logger("providers")

var lruCacheSize = 256
var ProvideValidity = time.Hour * 24
var defaultCleanupInterval = time.Hour

type ProviderManager struct {
	// all non channel fields are meant to be accessed only within
	// the run method
	providers *lru.Cache
	lpeer     peer.ID
	dstore    ds.Datastore

	newprovs chan *addProv
	getprovs chan *getProv
	period   time.Duration
	proc     goprocess.Process

	cleanupInterval time.Duration
}

type providerSet struct {
	providers []peer.ID
	set       map[peer.ID]time.Time
}

type addProv struct {
	k   key.Key
	val peer.ID
}

type getProv struct {
	k    key.Key
	resp chan []peer.ID
}

func NewProviderManager(ctx context.Context, local peer.ID, dstore ds.Batching) *ProviderManager {
	pm := new(ProviderManager)
	pm.getprovs = make(chan *getProv)
	pm.newprovs = make(chan *addProv)
	pm.dstore = autobatch.NewAutoBatching(dstore, batchBufferSize)
	cache, err := lru.New(lruCacheSize)
	if err != nil {
		panic(err) //only happens if negative value is passed to lru constructor
	}
	pm.providers = cache

	pm.proc = goprocessctx.WithContext(ctx)
	pm.cleanupInterval = defaultCleanupInterval
	pm.proc.Go(func(p goprocess.Process) { pm.run() })

	return pm
}

const providersKeyPrefix = "/providers/"

func mkProvKey(k key.Key) ds.Key {
	return ds.NewKey(providersKeyPrefix + base32.RawStdEncoding.EncodeToString([]byte(k)))
}

func (pm *ProviderManager) Process() goprocess.Process {
	return pm.proc
}

func (pm *ProviderManager) providersForKey(k key.Key) ([]peer.ID, error) {
	pset, err := pm.getProvSet(k)
	if err != nil {
		return nil, err
	}
	return pset.providers, nil
}

func (pm *ProviderManager) getProvSet(k key.Key) (*providerSet, error) {
	cached, ok := pm.providers.Get(k)
	if ok {
		return cached.(*providerSet), nil
	}

	pset, err := loadProvSet(pm.dstore, k)
	if err != nil {
		return nil, err
	}

	if len(pset.providers) > 0 {
		pm.providers.Add(k, pset)
	}

	return pset, nil
}

func loadProvSet(dstore ds.Datastore, k key.Key) (*providerSet, error) {
	res, err := dstore.Query(dsq.Query{Prefix: mkProvKey(k).String()})
	if err != nil {
		return nil, err
	}

	out := newProviderSet()
	for e := range res.Next() {
		if e.Error != nil {
			log.Error("got an error: ", e.Error)
			continue
		}
		parts := strings.Split(e.Key, "/")
		if len(parts) != 4 {
			log.Warning("incorrectly formatted key: ", e.Key)
			continue
		}

		decstr, err := base32.RawStdEncoding.DecodeString(parts[len(parts)-1])
		if err != nil {
			log.Error("base32 decoding error: ", err)
			continue
		}

		pid := peer.ID(decstr)

		t, err := readTimeValue(e.Value)
		if err != nil {
			log.Warning("parsing providers record from disk: ", err)
			continue
		}

		out.setVal(pid, t)
	}

	return out, nil
}

func readTimeValue(i interface{}) (time.Time, error) {
	data, ok := i.([]byte)
	if !ok {
		return time.Time{}, fmt.Errorf("data was not a []byte")
	}

	nsec, _ := binary.Varint(data)

	return time.Unix(0, nsec), nil
}

func (pm *ProviderManager) addProv(k key.Key, p peer.ID) error {
	iprovs, ok := pm.providers.Get(k)
	if !ok {
		iprovs = newProviderSet()
		pm.providers.Add(k, iprovs)
	}
	provs := iprovs.(*providerSet)
	now := time.Now()
	provs.setVal(p, now)

	return writeProviderEntry(pm.dstore, k, p, now)
}

func writeProviderEntry(dstore ds.Datastore, k key.Key, p peer.ID, t time.Time) error {
	dsk := mkProvKey(k).ChildString(base32.RawStdEncoding.EncodeToString([]byte(p)))

	buf := make([]byte, 16)
	n := binary.PutVarint(buf, t.UnixNano())

	return dstore.Put(dsk, buf[:n])
}

func (pm *ProviderManager) deleteProvSet(k key.Key) error {
	pm.providers.Remove(k)

	res, err := pm.dstore.Query(dsq.Query{
		KeysOnly: true,
		Prefix:   mkProvKey(k).String(),
	})

	entries, err := res.Rest()
	if err != nil {
		return err
	}

	for _, e := range entries {
		err := pm.dstore.Delete(ds.NewKey(e.Key))
		if err != nil {
			log.Error("deleting provider set: ", err)
		}
	}
	return nil
}

func (pm *ProviderManager) getAllProvKeys() ([]key.Key, error) {
	res, err := pm.dstore.Query(dsq.Query{
		KeysOnly: true,
		Prefix:   providersKeyPrefix,
	})

	if err != nil {
		return nil, err
	}

	entries, err := res.Rest()
	if err != nil {
		return nil, err
	}

	out := make([]key.Key, 0, len(entries))
	seen := make(map[key.Key]struct{})
	for _, e := range entries {
		parts := strings.Split(e.Key, "/")
		if len(parts) != 4 {
			log.Warning("incorrectly formatted provider entry in datastore")
			continue
		}
		decoded, err := base32.RawStdEncoding.DecodeString(parts[2])
		if err != nil {
			log.Warning("error decoding base32 provider key")
			continue
		}

		k := key.Key(decoded)
		if _, ok := seen[k]; !ok {
			out = append(out, key.Key(decoded))
			seen[k] = struct{}{}
		}
	}

	return out, nil
}

func (pm *ProviderManager) run() {
	tick := time.NewTicker(pm.cleanupInterval)
	for {
		select {
		case np := <-pm.newprovs:
			err := pm.addProv(np.k, np.val)
			if err != nil {
				log.Error("error adding new providers: ", err)
			}
		case gp := <-pm.getprovs:
			provs, err := pm.providersForKey(gp.k)
			if err != nil && err != ds.ErrNotFound {
				log.Error("error reading providers: ", err)
			}

			gp.resp <- provs
		case <-tick.C:
			keys, err := pm.getAllProvKeys()
			if err != nil {
				log.Error("Error loading provider keys: ", err)
				continue
			}
			for _, k := range keys {
				provs, err := pm.getProvSet(k)
				if err != nil {
					log.Error("error loading known provset: ", err)
					continue
				}
				var filtered []peer.ID
				for p, t := range provs.set {
					if time.Now().Sub(t) > ProvideValidity {
						delete(provs.set, p)
					} else {
						filtered = append(filtered, p)
					}
				}

				if len(filtered) > 0 {
					provs.providers = filtered
				} else {
					err := pm.deleteProvSet(k)
					if err != nil {
						log.Error("error deleting provider set: ", err)
					}
				}
			}
		case <-pm.proc.Closing():
			return
		}
	}
}

func (pm *ProviderManager) AddProvider(ctx context.Context, k key.Key, val peer.ID) {
	prov := &addProv{
		k:   k,
		val: val,
	}
	select {
	case pm.newprovs <- prov:
	case <-ctx.Done():
	}
}

func (pm *ProviderManager) GetProviders(ctx context.Context, k key.Key) []peer.ID {
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

func newProviderSet() *providerSet {
	return &providerSet{
		set: make(map[peer.ID]time.Time),
	}
}

func (ps *providerSet) Add(p peer.ID) {
	ps.setVal(p, time.Now())
}

func (ps *providerSet) setVal(p peer.ID, t time.Time) {
	_, found := ps.set[p]
	if !found {
		ps.providers = append(ps.providers, p)
	}

	ps.set[p] = t
}
