package namesys

import (
	"context"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	ds "github.com/ipfs/go-datastore"
	path "github.com/ipfs/go-path"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	isd "github.com/jbenet/go-is-domain"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	mh "github.com/multiformats/go-multihash"
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

	cache *lru.Cache
}

// NewNameSystem will construct the IPFS naming system based on Routing
func NewNameSystem(r routing.ValueStore, ds ds.Datastore, cachesize int) NameSystem {
	var cache *lru.Cache
	if cachesize > 0 {
		cache, _ = lru.New(cachesize)
	}

	return &mpns{
		dnsResolver:      NewDNSResolver(),
		proquintResolver: new(ProquintResolver),
		ipnsResolver:     NewIpnsResolver(r),
		ipnsPublisher:    NewIpnsPublisher(r, ds),
		cache:            cache,
	}
}

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
	res := make(chan Result, 1)
	if strings.HasPrefix(name, "/ipfs/") {
		p, err := path.ParsePath(name)
		res <- Result{p, err}
		return res
	}

	if !strings.HasPrefix(name, "/") {
		p, err := path.ParsePath("/ipfs/" + name)
		res <- Result{p, err}
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

	if p, ok := ns.cacheGet(key); ok {
		if len(segments) > 3 {
			var err error
			p, err = path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
			if err != nil {
				emitOnceResult(ctx, out, onceResult{value: p, err: err})
			}
		}

		out <- onceResult{value: p}
		close(out)
		return out
	}

	// Resolver selection:
	// 1. if it is a multihash resolve through "ipns".
	// 2. if it is a domain name, resolve through "dns"
	// 3. otherwise resolve through the "proquint" resolver

	var res resolver
	if _, err := mh.FromB58String(key); err == nil {
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
						ns.cacheSet(key, best.value, best.ttl)
					}
					return
				}
				if res.err == nil {
					best = res
				}
				p := res.value

				// Attach rest of the path
				if len(segments) > 3 {
					var err error
					p, err = path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[3])
					if err != nil {
						emitOnceResult(ctx, out, onceResult{value: p, ttl: res.ttl, err: err})
					}
				}

				emitOnceResult(ctx, out, onceResult{value: p, ttl: res.ttl, err: res.err})
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
		return err
	}
	ttl := DefaultResolverCacheTTL
	if setTTL, ok := checkCtxTTL(ctx); ok {
		ttl = setTTL
	}
	if ttEol := time.Until(eol); ttEol < ttl {
		ttl = ttEol
	}
	ns.cacheSet(peer.Encode(id), value, ttl)
	return nil
}
