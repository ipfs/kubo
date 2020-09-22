package namesys

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	path "github.com/ipfs/go-path"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	isd "github.com/jbenet/go-is-domain"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
)

// mpns (a multi-protocol NameSystem) implements generic IPFS naming.
//
// Uses several Resolvers:
// (a) IPFS routing naming: SFS-like PKI names.
// (b) dns domains: resolves using links in DNS TXT records
// (c) proquints: interprets string as the raw byte data.
//
// It can only publish to: (a) IPFS routing naming.
//
type mpns struct {
	dnsResolver, proquintResolver, ipnsResolver resolver
	ipnsPublisher                               Publisher

	staticMap map[string]path.Path
	cache     *lru.Cache
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.ValueStore, ds ds.Datastore, cachesize int) NameSystem {
	var (
		cache     *lru.Cache
		staticMap map[string]path.Path
	)
	if cachesize > 0 {
		cache, _ = lru.New(cachesize)
	}

	// Prewarm namesys cache with static records for deterministic tests and debugging.
	// Useful for testing things like DNSLink without real DNS lookup.
	// Example:
	// IPFS_NS_MAP="dnslink-test.example.com:/ipfs/bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am"
	if list := os.Getenv("IPFS_NS_MAP"); list != "" {
		staticMap = make(map[string]path.Path)
		for _, pair := range strings.Split(list, ",") {
			mapping := strings.SplitN(pair, ":", 2)
			key := mapping[0]
			value := path.FromString(mapping[1])
			staticMap[key] = value
		}
	}

	return &mpns{
		dnsResolver:      NewDNSResolver(),
		proquintResolver: new(ProquintResolver),
		ipnsResolver:     NewIpnsResolver(r),
		ipnsPublisher:    NewIpnsPublisher(r, ds),
		staticMap:        staticMap,
		cache:            cache,
	}
}

// DefaultResolverCacheTTL defines max ttl of a record placed in namesys cache.
const DefaultResolverCacheTTL = time.Minute

// Resolve implements Resolver.
func (ns *mpns) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	if strings.HasPrefix(name, "/ipfs/") {
		return path.ParsePath(name)
	}

	if !strings.HasPrefix(name, "/") {
		return path.ParsePath("/ipfs/" + name)
	}

	return resolve(ctx, ns, name, opts.ProcessOpts(options))
}

func (ns *mpns) ResolveAsync(ctx context.Context, name string, options ...opts.ResolveOpt) <-chan Result {
	if strings.HasPrefix(name, "/ipfs/") {
		p, err := path.ParsePath(name)
		res := make(chan Result, 1)
		res <- Result{p, err}
		close(res)
		return res
	}

	if !strings.HasPrefix(name, "/") {
		p, err := path.ParsePath("/ipfs/" + name)
		res := make(chan Result, 1)
		res <- Result{p, err}
		close(res)
		return res
	}

	return resolveAsync(ctx, ns, name, opts.ProcessOpts(options))
}

// resolveOnce implements resolver.
func (ns *mpns) resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult {
	out := make(chan onceResult, 1)

	if !strings.HasPrefix(name, ipnsPrefix) {
		name = ipnsPrefix + name
	}
	segments := strings.SplitN(name, "/", 4)
	if len(segments) < 3 || segments[0] != "" {
		log.Debugf("invalid name syntax for %s", name)
		out <- onceResult{err: ErrResolveFailed}
		close(out)
		return out
	}

	key := segments[2]

	// Resolver selection:
	// 1. if it is a PeerID/CID/multihash resolve through "ipns".
	// 2. if it is a domain name, resolve through "dns"
	// 3. otherwise resolve through the "proquint" resolver

	var res resolver
	ipnsKey, err := peer.Decode(key)

	// CIDs in IPNS are expected to have libp2p-key multicodec
	// We ease the transition by returning a more meaningful error with a valid CID
	if err != nil && err.Error() == "can't convert CID of type protobuf to a peer ID" {
		ipnsCid, cidErr := cid.Decode(key)
		if cidErr == nil && ipnsCid.Version() == 1 && ipnsCid.Type() != cid.Libp2pKey {
			fixedCid := cid.NewCidV1(cid.Libp2pKey, ipnsCid.Hash()).String()
			codecErr := fmt.Errorf("peer ID represented as CIDv1 require libp2p-key multicodec: retry with /ipns/%s", fixedCid)
			log.Debugf("RoutingResolver: could not convert public key hash %s to peer ID: %s\n", key, codecErr)
			out <- onceResult{err: codecErr}
			close(out)
			return out
		}
	}

	cacheKey := key
	if err == nil {
		cacheKey = string(ipnsKey)
	}

	if p, ok := ns.cacheGet(cacheKey); ok {
		var err error
		if len(segments) > 3 {
			p, err = path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
		}

		out <- onceResult{value: p, err: err}
		close(out)
		return out
	}

	if err == nil {
		res = ns.ipnsResolver
	} else if isd.IsDomain(key) {
		res = ns.dnsResolver
	} else {
		res = ns.proquintResolver
	}

	resCh := res.resolveOnceAsync(ctx, key, options)
	var best onceResult
	go func() {
		defer close(out)
		for {
			select {
			case res, ok := <-resCh:
				if !ok {
					if best != (onceResult{}) {
						ns.cacheSet(cacheKey, best.value, best.ttl)
					}
					return
				}
				if res.err == nil {
					best = res
				}
				p := res.value
				err := res.err
				ttl := res.ttl

				// Attach rest of the path
				if len(segments) > 3 {
					p, err = path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
				}

				emitOnceResult(ctx, out, onceResult{value: p, ttl: ttl, err: err})
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

func emitOnceResult(ctx context.Context, outCh chan<- onceResult, r onceResult) {
	select {
	case outCh <- r:
	case <-ctx.Done():
	}
}

// Publish implements Publisher
func (ns *mpns) Publish(ctx context.Context, name ci.PrivKey, value path.Path) error {
	return ns.PublishWithEOL(ctx, name, value, time.Now().Add(DefaultRecordEOL))
}

func (ns *mpns) PublishWithEOL(ctx context.Context, name ci.PrivKey, value path.Path, eol time.Time) error {
	id, err := peer.IDFromPrivateKey(name)
	if err != nil {
		return err
	}
	if err := ns.ipnsPublisher.PublishWithEOL(ctx, name, value, eol); err != nil {
		// Invalidate the cache. Publishing may _partially_ succeed but
		// still return an error.
		ns.cacheInvalidate(string(id))
		return err
	}
	ttl := DefaultResolverCacheTTL
	if setTTL, ok := checkCtxTTL(ctx); ok {
		ttl = setTTL
	}
	if ttEol := time.Until(eol); ttEol < ttl {
		ttl = ttEol
	}
	ns.cacheSet(string(id), value, ttl)
	return nil
}
