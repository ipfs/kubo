package namesys

import (
	"fmt"
	"time"

	"code.google.com/p/goprotobuf/proto"

	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
)

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

func (r *RoutingResolver) Resolve(name string) (string, error) {
	hash, err := mh.FromB58String(name)
	if err != nil {
		return "", err
	}
	// name should be a multihash. if it isn't, error out here.

	// use the routing system to get the name.
	// /ipns/<name>
	h, err := u.Hash([]byte("ipns:" + name))
	if err != nil {
		return "", err
	}

	inpsKey := u.Key(h)
	val, err := r.routing.GetValue(inpsKey, time.Second*10)
	if err != nil {
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
	node, err := r.dag.Get(key)
	if err != nil {
		return "", err
	}

	// get PublicKey from node.Data
	pk, err := ci.UnmarshalPublicKey(node.Data)
	if err != nil {
		return "", err
	}

	// check sig with pk
	if ok, err := pk.Verify(entry.GetValue(), entry.GetSignature()); err != nil && ok {
		return "", fmt.Errorf("Invalid value. Not signed by PrivateKey corresponding to %v", pk)
	}

	// ok sig checks out. this is a valid name.
	return string(entry.GetValue()), nil
}
