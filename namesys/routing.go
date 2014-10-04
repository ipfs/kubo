package namesys

import (
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("namesys")

// RoutingResolver implements NSResolver for the main IPFS SFS-like naming
type RoutingResolver struct {
	routing routing.IpfsRouting
	dag     *mdag.DAGService
}

func NewRoutingResolver(route routing.IpfsRouting, dagservice *mdag.DAGService) *RoutingResolver {
	return &RoutingResolver{
		routing: route,
		dag:     dagservice,
	}
}

func (r *RoutingResolver) Matches(name string) bool {
	_, err := mh.FromB58String(name)
	return err == nil
}

func (r *RoutingResolver) Resolve(name string) (string, error) {
	log.Debug("RoutingResolve: '%s'", name)
	ctx := context.TODO()
	hash, err := mh.FromB58String(name)
	if err != nil {
		log.Warning("RoutingResolve: bad input hash: [%s]\n", name)
		return "", err
	}
	// name should be a multihash. if it isn't, error out here.

	// use the routing system to get the name.
	// /ipns/<name>
	h, err := u.Hash([]byte("/ipns/" + name))
	if err != nil {
		return "", err
	}

	ipnsKey := u.Key(h)
	val, err := r.routing.GetValue(ctx, ipnsKey)
	if err != nil {
		log.Warning("RoutingResolve get failed.")
		return "", err
	}

	entry := new(IpnsEntry)
	err = proto.Unmarshal(val, entry)
	if err != nil {
		return "", err
	}

	// name should be a public key retrievable from ipfs
	// /ipfs/<name>
	key := u.Key(hash)
	pkval, err := r.routing.GetValue(ctx, key)
	if err != nil {
		log.Warning("RoutingResolve PubKey Get failed.")
		return "", err
	}

	// get PublicKey from node.Data
	pk, err := ci.UnmarshalPublicKey(pkval)
	if err != nil {
		return "", err
	}

	// check sig with pk
	if ok, err := pk.Verify(entry.GetValue(), entry.GetSignature()); err != nil || !ok {
		return "", fmt.Errorf("Invalid value. Not signed by PrivateKey corresponding to %v", pk)
	}

	// ok sig checks out. this is a valid name.
	return string(entry.GetValue()), nil
}
