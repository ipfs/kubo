package namesys

import (
	"fmt"
	"time"

	proto "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/gogo/protobuf/proto"
	lru "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/hashicorp/golang-lru"
	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	pb "github.com/ipfs/go-ipfs/namesys/pb"
	path "github.com/ipfs/go-ipfs/path"
	routing "github.com/ipfs/go-ipfs/routing"
	u "github.com/ipfs/go-ipfs/util"
	infd "github.com/ipfs/go-ipfs/util/infduration"
	logging "github.com/ipfs/go-ipfs/vendor/QmQg1J6vikuXF9oDvm4wpdeAUvvkVEKW1EYDw9HhTMnP2b/go-log"
)

var log = logging.Logger("namesys")

// routingResolver implements NSResolver for the main IPFS SFS-like naming
type routingResolver struct {
	routing routing.IpfsRouting

	cache *lru.Cache
}

func (r *routingResolver) cacheGet(name string) (path.Path, time.Duration, bool) {
	if r.cache == nil {
		return "", 0, false
	}

	ientry, ok := r.cache.Get(name)
	if !ok {
		return "", 0, false
	}

	entry, ok := ientry.(cacheEntry)
	if !ok {
		// should never happen, purely for sanity
		log.Panicf("unexpected type %T in cache for %q.", ientry, name)
	}

	now := time.Now()
	if now.Before(entry.eol) {
		return entry.val, entry.eol.Sub(now), true
	}

	r.cache.Remove(name)

	return "", 0, false
}

func (r *routingResolver) cacheSet(name string, val path.Path, eol time.Time, rec *pb.IpnsEntry) {
	if r.cache == nil {
		return
	}

	r.cache.Add(name, cacheEntry{
		val: val,
		eol: eol,
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
	p, _, err := r.ResolveWithTTL(ctx, name)
	return p, err
}

// ResolveN implements Resolver.
func (r *routingResolver) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	p, _, err := r.ResolveNWithTTL(ctx, name, depth)
	return p, err
}

// ResolveWithTTL implements Resolver.
func (r *routingResolver) ResolveWithTTL(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	return r.ResolveNWithTTL(ctx, name, DefaultDepthLimit)
}

// ResolveNWithTTL implements Resolver.
func (r *routingResolver) ResolveNWithTTL(ctx context.Context, name string, depth int) (path.Path, infd.Duration, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver. Uses the IPFS routing system to
// resolve SFS-like names.
func (r *routingResolver) resolveOnce(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	log.Debugf("RoutingResolve: %q", name)
	if cached, ttl, ok := r.cacheGet(name); ok {
		return cached, infd.FiniteDuration(ttl), nil
	}

	hash, err := mh.FromB58String(name)
	if err != nil {
		log.Warningf("RoutingResolve: bad input hash: %q", name)
		return "", infd.InfiniteDuration(), err
	}
	// name should be a multihash. if it isn't, error out here.

	// use the routing system to get the name.
	// /ipns/<name>
	h := []byte("/ipns/" + string(hash))

	ipnsKey := key.Key(h)
	val, err := r.routing.GetValue(ctx, ipnsKey)
	if err != nil {
		log.Warning("RoutingResolve get failed.")
		return "", infd.FiniteDuration(0), err
	}

	entry := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, entry)
	if err != nil {
		return "", infd.FiniteDuration(0), err
	}

	// name should be a public key retrievable from ipfs
	pubkey, err := routing.GetPublicKey(r.routing, ctx, hash)
	if err != nil {
		return "", infd.FiniteDuration(0), err
	}

	hsh, _ := pubkey.Hash()
	log.Debugf("pk hash = %s", key.Key(hsh))

	// check sig with pk
	if ok, err := pubkey.Verify(ipnsEntryDataForSig(entry), entry.GetSignature()); err != nil || !ok {
		return "", infd.FiniteDuration(0), fmt.Errorf("Invalid value. Not signed by PrivateKey corresponding to %v", pubkey)
	}

	// ok sig checks out. this is a valid name.

	p, err := entryPath(entry)
	if err != nil {
		return "", infd.FiniteDuration(0), err
	}

	eol, ttl := entryEOL(name, entry)
	if ttl > 0 {
		r.cacheSet(name, p, eol, entry)
	}

	return p, infd.FiniteDuration(ttl), nil
}

// entryPath computes the path an IPNS entry points to.
func entryPath(e *pb.IpnsEntry) (path.Path, error) {
	// check for old style record:
	valh, err := mh.Cast(e.GetValue())
	if err != nil {
		// Not a multihash, probably a new record
		p, err := path.ParsePath(string(e.GetValue()))
		if err != nil {
			return "", err
		}
		return p, nil
	} else {
		// Its an old style multihash record
		log.Warning("Detected old style multihash record")
		return path.FromKey(key.Key(valh)), nil
	}
}

// entryEOL computes the maximum cache time for an IPNS entry taking the TTL
// and the EOL into account.
func entryEOL(name string, e *pb.IpnsEntry) (time.Time, time.Duration) {
	// if completely unspecified, just use one minute
	ttl := UnknownTTL
	if e.Ttl != nil {
		recttl := time.Duration(e.GetTtl())
		if recttl >= 0 {
			ttl = recttl
		}
	}

	now := time.Now()
	cacheTil := now.Add(ttl)
	eol, ok := checkEOL(e)
	if ok && eol.Before(cacheTil) {
		cacheTil = eol
	}

	if cacheTil.Before(now) {
		// Already EOL, do not cache.  This is unexpected, an expired
		// record should have been caught before this code is called.
		// (But it is possible for the expiry to happen between the two
		// time.Now() calls.)
		log.Warning("%q already expired on %v, will not cache", name, eol)
		return now, 0
	}

	return cacheTil, cacheTil.Sub(now)
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
