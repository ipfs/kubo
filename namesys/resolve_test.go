package namesys

import (
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	bs "github.com/jbenet/go-ipfs/blockservice"
	ci "github.com/jbenet/go-ipfs/crypto"
	mdag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/net/swarm"
	"github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/routing/dht"
	u "github.com/jbenet/go-ipfs/util"
)

func TestRoutingResolve(t *testing.T) {
	local := &peer.Peer{
		ID: []byte("testID"),
	}
	net := swarm.NewSwarm(local)
	lds := ds.NewMapDatastore()
	d := dht.NewDHT(local, net, lds)

	bserv, err := bs.NewBlockService(lds, nil)
	if err != nil {
		t.Fatal(err)
	}

	dag := &mdag.DAGService{Blocks: bserv}

	resolve := NewMasterResolver(d, dag)

	pub := IpnsPublisher{
		dag:     dag,
		routing: d,
	}

	privk, pubk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	err = pub.Publish(privk, u.Key("Hello"))
	if err != nil {
		t.Fatal(err)
	}

	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	pkhash, err := u.Hash(pubkb)
	if err != nil {
		t.Fatal(err)
	}

	res, err := resolve.Resolve(u.Key(pkhash).Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if res != "Hello" {
		t.Fatal("Got back incorrect value.")
	}
}
