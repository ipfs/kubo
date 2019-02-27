package namesys

import (
	"context"
	"strings"
	"time"

	path "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	ipns "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns"
	pb "gx/ipfs/QmUwMnKKjH3JwGKNVZ3TcP37W93xzqNA4ECFFiMo6sXkkc/go-ipns/pb"
	opts "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options/namesys"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
	dht "gx/ipfs/QmdR6WN3TUEAVQ9KWE2UiFJikWTbUvgBJay6mjB4yUJebq/go-libp2p-kad-dht"
	proto "gx/ipfs/QmddjPSGZb3ieihSseFeCfVRpZzcqczPNsD2DvarSwnjJB/gogo-protobuf/proto"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"
)

var log = logging.Logger("namesys")

// IpnsResolver implements NSResolver for the main IPFS SFS-like naming
type IpnsResolver struct {
	routing routing.ValueStore
}

// NewIpnsResolver constructs a name resolver using the IPFS Routing system
// to implement SFS-like naming on top.
func NewIpnsResolver(route routing.ValueStore) *IpnsResolver {
	if route == nil {
		panic("attempt to create resolver with nil routing system")
	}
	return &IpnsResolver{
		routing: route,
	}
}

// Resolve implements Resolver.
func (r *IpnsResolver) Resolve(ctx context.Context, name string, options ...opts.ResolveOpt) (path.Path, error) {
	return resolve(ctx, r, name, opts.ProcessOpts(options))
}

// ResolveAsync implements Resolver.
func (r *IpnsResolver) ResolveAsync(ctx context.Context, name string, options ...opts.ResolveOpt) <-chan Result {
	return resolveAsync(ctx, r, name, opts.ProcessOpts(options))
}

// resolveOnce implements resolver. Uses the IPFS routing system to
// resolve SFS-like names.
func (r *IpnsResolver) resolveOnceAsync(ctx context.Context, name string, options opts.ResolveOpts) <-chan onceResult {
	out := make(chan onceResult, 1)
	log.Debugf("RoutingResolver resolving %s", name)
	cancel := func() {}

	if options.DhtTimeout != 0 {
		// Resolution must complete within the timeout
		ctx, cancel = context.WithTimeout(ctx, options.DhtTimeout)
	}

	name = strings.TrimPrefix(name, "/ipns/")
	pid, err := peer.IDB58Decode(name)
	if err != nil {
		log.Debugf("RoutingResolver: could not convert public key hash %s to peer ID: %s\n", name, err)
		out <- onceResult{err: err}
		close(out)
		cancel()
		return out
	}

	// Name should be the hash of a public key retrievable from ipfs.
	// We retrieve the public key here to make certain that it's in the peer
	// store before calling GetValue() on the DHT - the DHT will call the
	// ipns validator, which in turn will get the public key from the peer
	// store to verify the record signature
	_, err = routing.GetPublicKey(r.routing, ctx, pid)
	if err != nil {
		log.Debugf("RoutingResolver: could not retrieve public key %s: %s\n", name, err)
		out <- onceResult{err: err}
		close(out)
		cancel()
		return out
	}

	// Use the routing system to get the name.
	// Note that the DHT will call the ipns validator when retrieving
	// the value, which in turn verifies the ipns record signature
	ipnsKey := ipns.RecordKey(pid)

	vals, err := r.routing.SearchValue(ctx, ipnsKey, dht.Quorum(int(options.DhtRecordCount)))
	if err != nil {
		log.Debugf("RoutingResolver: dht get for name %s failed: %s", name, err)
		out <- onceResult{err: err}
		close(out)
		cancel()
		return out
	}

	go func() {
		defer cancel()
		defer close(out)
		for {
			select {
			case val, ok := <-vals:
				if !ok {
					return
				}

				entry := new(pb.IpnsEntry)
				err = proto.Unmarshal(val, entry)
				if err != nil {
					log.Debugf("RoutingResolver: could not unmarshal value for name %s: %s", name, err)
					emitOnceResult(ctx, out, onceResult{err: err})
					return
				}

				var p path.Path
				// check for old style record:
				if valh, err := mh.Cast(entry.GetValue()); err == nil {
					// Its an old style multihash record
					log.Debugf("encountered CIDv0 ipns entry: %s", valh)
					p = path.FromCid(cid.NewCidV0(valh))
				} else {
					// Not a multihash, probably a new style record
					p, err = path.ParsePath(string(entry.GetValue()))
					if err != nil {
						emitOnceResult(ctx, out, onceResult{err: err})
						return
					}
				}

				ttl := DefaultResolverCacheTTL
				if entry.Ttl != nil {
					ttl = time.Duration(*entry.Ttl)
				}
				switch eol, err := ipns.GetEOL(entry); err {
				case ipns.ErrUnrecognizedValidity:
					// No EOL.
				case nil:
					ttEol := eol.Sub(time.Now())
					if ttEol < 0 {
						// It *was* valid when we first resolved it.
						ttl = 0
					} else if ttEol < ttl {
						ttl = ttEol
					}
				default:
					log.Errorf("encountered error when parsing EOL: %s", err)
					emitOnceResult(ctx, out, onceResult{err: err})
					return
				}

				emitOnceResult(ctx, out, onceResult{value: p, ttl: ttl})
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
