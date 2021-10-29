package namesys

import (
	"context"
	"strings"
	"time"

	proto "github.com/gogo/protobuf/proto"
	cid "github.com/ipfs/go-cid"
	ipns "github.com/ipfs/go-ipns"
	pb "github.com/ipfs/go-ipns/pb"
	logging "github.com/ipfs/go-log"
	path "github.com/ipfs/go-path"
	opts "github.com/ipfs/interface-go-ipfs-core/options/namesys"
	peer "github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	mh "github.com/multiformats/go-multihash"
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

	pid, err := peer.Decode(name)
	if err != nil {
		log.Debugf("RoutingResolver: could not convert public key hash %s to peer ID: %s\n", name, err)
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
					ttEol := time.Until(eol)
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
