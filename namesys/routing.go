package namesys

import (
	"context"
	"strings"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmdrpbDgeYH3VxkCciQCJY5LkDYdXtig6unDzQmMxFtWEw/go-path"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	routing "gx/ipfs/QmPmFeQ5oY5G6M7aBWggi5phxEPXwsQntE1DFcUzETULdp/go-libp2p-routing"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	dht "gx/ipfs/QmSteomMgXnSQxLEY5UpxmkYAd8QF9JuLLeLYBokTHxFru/go-libp2p-kad-dht"
	ipns "gx/ipfs/QmX72XT6sSQRkNHKcAFLM2VqB3B4bWPetgWnHY8LgsUVeT/go-ipns"
	pb "gx/ipfs/QmX72XT6sSQRkNHKcAFLM2VqB3B4bWPetgWnHY8LgsUVeT/go-ipns/pb"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
	proto "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"
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
	return resolve(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
}

// ResolveAsync implements Resolver.
func (r *IpnsResolver) ResolveAsync(ctx context.Context, name string, options ...opts.ResolveOpt) <-chan Result {
	return resolveAsync(ctx, r, name, opts.ProcessOpts(options), "/ipns/")
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
					select {
					case out <- onceResult{err: err}:
					case <-ctx.Done():
					}
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
						select {
						case out <- onceResult{err: err}:
						case <-ctx.Done():
						}
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
					select {
					case out <- onceResult{err: err}:
					case <-ctx.Done():
					}
					return
				}

				select {
				case out <- onceResult{value: p, ttl: ttl}:
				case <-ctx.Done():
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
