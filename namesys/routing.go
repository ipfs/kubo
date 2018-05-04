package namesys

import (
	"context"
	"strings"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"

	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"
	logging "gx/ipfs/QmTG23dvpBCBjqQwyDxV8CQT6jmS4PSftNr1VqHhE3MLy7/go-log"
	routing "gx/ipfs/QmUHRKTeaoASDvDj7cTAXsmjAY7KQ13ErtzkQHZQq6uFUz/go-libp2p-routing"
	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	peer "gx/ipfs/QmcJukH2sAFjY3HdBKq35WDzWoL3UUu2gt9wdfqZTUyM74/go-libp2p-peer"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

var log = logging.Logger("namesys")

// routingResolver implements NSResolver for the main IPFS SFS-like naming
type routingResolver struct {
	routing routing.ValueStore

	cache *lru.Cache
}

func (r *routingResolver) cacheGet(name string) (path.Path, bool) {
	if r.cache == nil {
		return "", false
	}

	ientry, ok := r.cache.Get(name)
	if !ok {
		return "", false
	}

	entry, ok := ientry.(cacheEntry)
	if !ok {
		// should never happen, purely for sanity
		log.Panicf("unexpected type %T in cache for %q.", ientry, name)
	}

	if time.Now().Before(entry.eol) {
		return entry.val, true
	}

	r.cache.Remove(name)

	return "", false
}

func (r *routingResolver) cacheSet(name string, val path.Path, rec *pb.IpnsEntry) {
	if r.cache == nil {
		return
	}

	// if completely unspecified, just use one minute
	ttl := DefaultResolverCacheTTL
	if rec.Ttl != nil {
		recttl := time.Duration(rec.GetTtl())
		if recttl >= 0 {
			ttl = recttl
		}
	}

	cacheTil := time.Now().Add(ttl)
	eol, ok := checkEOL(rec)
	if ok && eol.Before(cacheTil) {
		cacheTil = eol
	}

	r.cache.Add(name, cacheEntry{
		val: val,
		eol: cacheTil,
	})
}

type cacheEntry struct {
	val path.Path
	eol time.Time
}

// NewRoutingResolver constructs a name resolver using the IPFS Routing system
// to implement SFS-like naming on top.
// cachesize is the limit of the number of entries in the lru cache. Setting it
// to '0' will disable caching.
func NewRoutingResolver(route routing.ValueStore, cachesize int) *routingResolver {
	if route == nil {
		panic("attempt to create resolver with nil routing system")
	}

	var cache *lru.Cache
	if cachesize > 0 {
		cache, _ = lru.New(cachesize)
	}

	return &routingResolver{
		routing: route,
		cache:   cache,
	}
}

// Resolve implements Resolver.
func (r *routingResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
}

// resolveOnce implements resolver. Uses the IPFS routing system to
// resolve SFS-like names.
func (r *routingResolver) resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (path.Path, error) {
	log.Debugf("RoutingResolver resolving %s", name)
	cached, ok := r.cacheGet(name)
	if ok {
		return cached, nil
	}

	if options.DhtTimeout != 0 {
		// Resolution must complete within the timeout
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.DhtTimeout)
		defer cancel()
	}

	name = strings.TrimPrefix(name, "/ipns/")
	hash, err := mh.FromB58String(name)
	if err != nil {
		// name should be a multihash. if it isn't, error out here.
		log.Debugf("RoutingResolver: bad input hash: [%s]\n", name)
		return "", err
	}

	// Name should be the hash of a public key retrievable from ipfs.
	// We retrieve the public key here to make certain that it's in the peer
	// store before calling GetValue() on the DHT - the DHT will call the
	// ipns validator, which in turn will get the public key from the peer
	// store to verify the record signature
	_, err = routing.GetPublicKey(r.routing, ctx, hash)
	if err != nil {
		log.Debugf("RoutingResolver: could not retrieve public key %s: %s\n", name, err)
		return "", err
	}

	pid, err := peer.IDFromBytes(hash)
	if err != nil {
		log.Debugf("RoutingResolver: could not convert public key hash %s to peer ID: %s\n", name, err)
		return "", err
	}

	// Use the routing system to get the name.
	// Note that the DHT will call the ipns validator when retrieving
	// the value, which in turn verifies the ipns record signature
	_, ipnsKey := IpnsKeysForID(pid)
	val, err := r.getValue(ctx, ipnsKey, options)
	if err != nil {
		log.Debugf("RoutingResolver: dht get for name %s failed: %s", name, err)
		return "", err
	}

	entry := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, entry)
	if err != nil {
		log.Debugf("RoutingResolver: could not unmarshal value for name %s: %s", name, err)
		return "", err
	}

	// check for old style record:
	valh, err := mh.Cast(entry.GetValue())
	if err != nil {
		// Not a multihash, probably a new record
		p, err := path.ParsePath(string(entry.GetValue()))
		if err != nil {
			return "", err
		}

		r.cacheSet(name, p, entry)
		return p, nil
	} else {
		// Its an old style multihash record
		log.Debugf("encountered CIDv0 ipns entry: %s", valh)
		p := path.FromCid(cid.NewCidV0(valh))
		r.cacheSet(name, p, entry)
		return p, nil
	}
}

func (r *routingResolver) getValue(ctx context.Context, ipnsKey string, options *opts.ResolveOpts) ([]byte, error) {
	// Get specified number of values from the DHT
	vals, err := r.routing.GetValues(ctx, ipnsKey, int(options.DhtRecordCount))
	if err != nil {
		return nil, err
	}

	// Select the best value
	recs := make([][]byte, 0, len(vals))
	for _, v := range vals {
		if v.Val != nil {
			recs = append(recs, v.Val)
		}
	}

	if len(recs) == 0 {
		return nil, routing.ErrNotFound
	}

	i, err := IpnsSelectorFunc(ipnsKey, recs)
	if err != nil {
		return nil, err
	}

	best := recs[i]
	if best == nil {
		log.Errorf("GetValues %s yielded record with nil value", ipnsKey)
		return nil, routing.ErrNotFound
	}

	return best, nil
}

func checkEOL(e *pb.IpnsEntry) (time.Time, bool) {
	if e.GetValidityType() == pb.IpnsEntry_EOL {
		eol, err := u.ParseRFC3339(string(e.GetValidity()))
		if err != nil {
			return time.Time{}, false
		}
		return eol, true
	}
	return time.Time{}, false
}
