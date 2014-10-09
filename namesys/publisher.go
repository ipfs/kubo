package namesys

import (
	"fmt"
	"time"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	ci "github.com/jbenet/go-ipfs/crypto"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

// ipnsPublisher is capable of publishing and resolving names to the IPFS
// routing system.
type ipnsPublisher struct {
	routing routing.IpfsRouting
}

// NewRoutingPublisher constructs a publisher for the IPFS Routing name system.
func NewRoutingPublisher(route routing.IpfsRouting) Publisher {
	return &ipnsPublisher{routing: route}
}

// Publish implements Publisher. Accepts a keypair and a value,
func (p *ipnsPublisher) Publish(k ci.PrivKey, value string) error {
	log.Debug("namesys: Publish %s", value)

	// validate `value` is a ref (multihash)
	_, err := mh.FromB58String(value)
	if err != nil {
		return fmt.Errorf("publish value must be str multihash. %v", err)
	}

	ctx := context.TODO()
	data, err := createRoutingEntryData(k, value)
	if err != nil {
		return err
	}
	pubkey := k.GetPublic()
	pkbytes, err := pubkey.Bytes()
	if err != nil {
		return nil
	}

	nameb := u.Hash(pkbytes)
	namekey := u.Key(nameb).Pretty()
	ipnskey := u.Hash([]byte("/ipns/" + namekey))

	// Store associated public key
	timectx, _ := context.WithDeadline(ctx, time.Now().Add(time.Second*4))
	err = p.routing.PutValue(timectx, u.Key(nameb), pkbytes)
	if err != nil {
		return err
	}

	// Store ipns entry at h("/ipns/"+b58(h(pubkey)))
	timectx, _ = context.WithDeadline(ctx, time.Now().Add(time.Second*4))
	err = p.routing.PutValue(timectx, u.Key(ipnskey), data)
	if err != nil {
		return err
	}

	return nil
}

func createRoutingEntryData(pk ci.PrivKey, val string) ([]byte, error) {
	entry := new(IpnsEntry)
	sig, err := pk.Sign([]byte(val))
	if err != nil {
		return nil, err
	}
	entry.Signature = sig
	entry.Value = []byte(val)
	return proto.Marshal(entry)
}
