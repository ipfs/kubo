package namesys

import (
	"context"
	"strings"
	"time"

	opts "github.com/ipfs/go-ipfs/namesys/opts"
	path "gx/ipfs/QmdMPBephdLYNESkruDX2hcDTgFYhoCt4LimWhgnomSdV2/go-path"

	ipns "gx/ipfs/QmNqBhXpBKa5jcjoUZHfxDgAFxtqK3rDA5jtW811GBvVob/go-ipns"
	pb "gx/ipfs/QmNqBhXpBKa5jcjoUZHfxDgAFxtqK3rDA5jtW811GBvVob/go-ipns/pb"
	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	routing "gx/ipfs/QmS4niovD1U6pRjUBXivr1zvvLBqiTKbERjFo994JU7oQS/go-libp2p-routing"
	dht "gx/ipfs/QmTRj8mj6X5LtjVochPPSNX6MTbJ6iVojcfakWJKG13re7/go-libp2p-kad-dht"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
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

// resolveOnce implements resolver. Uses the IPFS routing system to
// resolve SFS-like names.
func (r *IpnsResolver) resolveOnce(ctx context.Context, name string, options *opts.ResolveOpts) (path.Path, time.Duration, error) {
	log.Debugf("RoutingResolver resolving %s", name)

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
		return "", 0, err
	}

	pid, err := peer.IDFromBytes(hash)
	if err != nil {
		log.Debugf("RoutingResolver: could not convert public key hash %s to peer ID: %s\n", name, err)
		return "", 0, err
	}

	// Name should be the hash of a public key retrievable from ipfs.
	// We retrieve the public key here to make certain that it's in the peer
	// store before calling GetValue() on the DHT - the DHT will call the
	// ipns validator, which in turn will get the public key from the peer
	// store to verify the record signature
	_, err = routing.GetPublicKey(r.routing, ctx, pid)
	if err != nil {
		log.Debugf("RoutingResolver: could not retrieve public key %s: %s\n", name, err)
		return "", 0, err
	}

	// Use the routing system to get the name.
	// Note that the DHT will call the ipns validator when retrieving
	// the value, which in turn verifies the ipns record signature
	ipnsKey := ipns.RecordKey(pid)
	val, err := r.routing.GetValue(ctx, ipnsKey, dht.Quorum(int(options.DhtRecordCount)))
	if err != nil {
		log.Debugf("RoutingResolver: dht get for name %s failed: %s", name, err)
		return "", 0, err
	}

	entry := new(pb.IpnsEntry)
	err = proto.Unmarshal(val, entry)
	if err != nil {
		log.Debugf("RoutingResolver: could not unmarshal value for name %s: %s", name, err)
		return "", 0, err
	}

	var p path.Path
	// check for old style record:
	if valh, err := mh.Cast(entry.GetValue()); err == nil {
		// Its an old style multihash record
		log.Debugf("encountered CIDv0 ipns entry: %s", valh)
		p = path.FromCid(cid.NewCidV0(valh))
	} else {
		// Not a multihash, probably a new record
		p, err = path.ParsePath(string(entry.GetValue()))
		if err != nil {
			return "", 0, err
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
		return "", 0, err
	}

	return p, ttl, nil
}
