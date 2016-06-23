package namesys

import (
	"fmt"
	"strings"
	"time"

	lru "gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	routing "github.com/ipfs/go-ipfs/routing"
	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
	ci "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

var log = logging.Logger("namesys")

// routingResolver implements NSResolver for the main IPFS SFS-like naming
type routingResolver struct {
	routing routing.IpfsRouting

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
func NewRoutingResolver(route routing.IpfsRouting, cachesize int) *routingResolver {
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
func (r *routingResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	return r.ResolveN(ctx, name, DefaultDepthLimit)
}

// ResolveN implements Resolver.
func (r *routingResolver) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver. Uses the IPFS routing system to
// resolve SFS-like names.
func (r *routingResolver) resolveOnce(ctx context.Context, name string) (path.Path, error) {
	log.Debugf("RoutingResolve: '%s'", name)
	cached, ok := r.cacheGet(name)
	if ok {
		return cached, nil
	}

	name = strings.TrimPrefix(name, "/ipns/")
	hash, err := mh.FromB58String(name)
	if err != nil {
		// name should be a multihash. if it isn't, error out here.
		log.Warningf("RoutingResolve: bad input hash: [%s]\n", name)
		return "", err
	}

	// use the routing system to get the name.
	// /ipns/<name>
	h := []byte("/ipns/" + string(hash))

	var entry *pb.IpnsEntry
	var pubkey ci.PubKey

	resp := make(chan error, 2)
	go func() {
		ipnsKey := key.Key(h)
		val, err := r.routing.GetValue(ctx, ipnsKey)
		if err != nil {
			log.Warning("RoutingResolve get failed.")
			resp <- err
		}

		entry = new(pb.IpnsEntry)
		err = proto.Unmarshal(val, entry)
		if err != nil {
			resp <- err
		}
		resp <- nil
	}()

	go func() {
		// name should be a public key retrievable from ipfs
		pubk, err := routing.GetPublicKey(r.routing, ctx, hash)
		if err != nil {
			resp <- err
		}
		pubkey = pubk
		resp <- nil
	}()

	for i := 0; i < 2; i++ {
		err = <-resp
		if err != nil {
			return "", err
		}
	}

	hsh, _ := pubkey.Hash()
	log.Debugf("pk hash = %s", key.Key(hsh))

	// check sig with pk
	if ok, err := pubkey.Verify(ipnsEntryDataForSig(entry), entry.GetSignature()); err != nil || !ok {
		return "", fmt.Errorf("Invalid value. Not signed by PrivateKey corresponding to %v", pubkey)
	}

	// ok sig checks out. this is a valid name.

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
		log.Warning("Detected old style multihash record")
		p := path.FromKey(key.Key(valh))
		r.cacheSet(name, p, entry)
		return p, nil
	}
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
