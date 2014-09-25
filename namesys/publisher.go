package namesys

import (
	"time"

	"code.google.com/p/go.net/context"
	"code.google.com/p/goprotobuf/proto"

	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

type IpnsPublisher struct {
	dag     *mdag.DAGService
	routing routing.IpfsRouting
}

func NewPublisher(dag *mdag.DAGService, route routing.IpfsRouting) *IpnsPublisher {
	return &IpnsPublisher{
		dag:     dag,
		routing: route,
	}
}

// Publish accepts a keypair and a value,
func (p *IpnsPublisher) Publish(k ci.PrivKey, value u.Key) error {
	log.Debug("namesys: Publish %s", value.Pretty())
	ctx := context.TODO()
	data, err := CreateEntryData(k, value)
	if err != nil {
		return err
	}
	pubkey := k.GetPublic()
	pkbytes, err := pubkey.Bytes()
	if err != nil {
		return nil
	}

	nameb, err := u.Hash(pkbytes)
	if err != nil {
		return nil
	}
	namekey := u.Key(nameb).Pretty()

	ipnskey, err := u.Hash([]byte("/ipns/" + namekey))
	if err != nil {
		return err
	}

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

func CreateEntryData(pk ci.PrivKey, val u.Key) ([]byte, error) {
	entry := new(IpnsEntry)
	sig, err := pk.Sign([]byte(val))
	if err != nil {
		return nil, err
	}
	entry.Signature = sig
	entry.Value = []byte(val)
	return proto.Marshal(entry)
}
