package namesys

import (
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	routing "github.com/jbenet/go-ipfs/routing"
	u "github.com/jbenet/go-ipfs/util"
)

type ipnsPublisher struct {
	dag     *mdag.DAGService
	routing routing.IpfsRouting
}

type Publisher interface {
	Publish(ci.PrivKey, string) error
}

func NewPublisher(dag *mdag.DAGService, route routing.IpfsRouting) Publisher {
	return &ipnsPublisher{
		dag:     dag,
		routing: route,
	}
}

// Publish accepts a keypair and a value,
func (p *ipnsPublisher) Publish(k ci.PrivKey, value string) error {
	log.Debug("namesys: Publish %s", value)
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

func CreateEntryData(pk ci.PrivKey, val string) ([]byte, error) {
	entry := new(IpnsEntry)
	sig, err := pk.Sign([]byte(val))
	if err != nil {
		return nil, err
	}
	entry.Signature = sig
	entry.Value = []byte(val)
	return proto.Marshal(entry)
}
