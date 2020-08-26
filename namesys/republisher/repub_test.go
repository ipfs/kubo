package republisher_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"

	goprocess "github.com/jbenet/goprocess"
	peer "github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipns"
	"github.com/ipfs/go-ipns/pb"
	path "github.com/ipfs/go-path"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	mock "github.com/ipfs/go-ipfs/core/mock"
	namesys "github.com/ipfs/go-ipfs/namesys"
	. "github.com/ipfs/go-ipfs/namesys/republisher"
)

func TestRepublish(t *testing.T) {
	// set cache life to zero for testing low-period repubs

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create network
	mn := mocknet.New(ctx)

	var nodes []*core.IpfsNode
	for i := 0; i < 10; i++ {
		nd, err := mock.MockPublicNode(ctx, mn)
		if err != nil {
			t.Fatal(err)
		}

		nd.Namesys = namesys.NewNameSystem(nd.Routing, nd.Repo.Datastore(), 0)

		nodes = append(nodes, nd)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	bsinf := bootstrap.BootstrapConfigWithPeers(
		[]peer.AddrInfo{
			nodes[0].Peerstore.PeerInfo(nodes[0].Identity),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.Bootstrap(bsinf); err != nil {
			t.Fatal(err)
		}
	}

	// have one node publish a record that is valid for 1 second
	publisher := nodes[3]
	p := path.FromString("/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn") // does not need to be valid
	rp := namesys.NewIpnsPublisher(publisher.Routing, publisher.Repo.Datastore())
	name := "/ipns/" + publisher.Identity.Pretty()

	// Retry in case the record expires before we can fetch it. This can
	// happen when running the test on a slow machine.
	var expiration time.Time
	timeout := time.Second
	for {
		expiration = time.Now().Add(time.Second)
		err := rp.PublishWithEOL(ctx, publisher.PrivateKey, p, expiration)
		if err != nil {
			t.Fatal(err)
		}

		err = verifyResolution(nodes, name, p)
		if err == nil {
			break
		}

		if time.Now().After(expiration) {
			timeout *= 2
			continue
		}
		t.Fatal(err)
	}

	// Now wait a second, the records will be invalid and we should fail to resolve
	time.Sleep(timeout)
	if err := verifyResolutionFails(nodes, name); err != nil {
		t.Fatal(err)
	}

	// The republishers that are contained within the nodes have their timeout set
	// to 12 hours. Instead of trying to tweak those, we're just going to pretend
	// they don't exist and make our own.
	repub := NewRepublisher(rp, publisher.Repo.Datastore(), publisher.PrivateKey, publisher.Repo.Keystore())
	repub.Interval = time.Second
	repub.RecordLifetime = time.Second * 5

	proc := goprocess.Go(repub.Run)
	defer proc.Close()

	// now wait a couple seconds for it to fire
	time.Sleep(time.Second * 2)

	// we should be able to resolve them now
	if err := verifyResolution(nodes, name, p); err != nil {
		t.Fatal(err)
	}
}

func TestLongEOLRepublish(t *testing.T) {
	// set cache life to zero for testing low-period repubs

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create network
	mn := mocknet.New(ctx)

	var nodes []*core.IpfsNode
	for i := 0; i < 10; i++ {
		nd, err := mock.MockPublicNode(ctx, mn)
		if err != nil {
			t.Fatal(err)
		}

		nd.Namesys = namesys.NewNameSystem(nd.Routing, nd.Repo.Datastore(), 0)

		nodes = append(nodes, nd)
	}

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}

	bsinf := bootstrap.BootstrapConfigWithPeers(
		[]peer.AddrInfo{
			nodes[0].Peerstore.PeerInfo(nodes[0].Identity),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.Bootstrap(bsinf); err != nil {
			t.Fatal(err)
		}
	}

	// have one node publish a record that is valid for 1 second
	publisher := nodes[3]
	p := path.FromString("/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn") // does not need to be valid
	rp := namesys.NewIpnsPublisher(publisher.Routing, publisher.Repo.Datastore())
	name := "/ipns/" + publisher.Identity.Pretty()

	expiration := time.Now().Add(time.Hour)
	err := rp.PublishWithEOL(ctx, publisher.PrivateKey, p, expiration)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyResolution(nodes, name, p)
	if err != nil {
		t.Fatal(err)
	}

	// The republishers that are contained within the nodes have their timeout set
	// to 12 hours. Instead of trying to tweak those, we're just going to pretend
	// they don't exist and make our own.
	repub := NewRepublisher(rp, publisher.Repo.Datastore(), publisher.PrivateKey, publisher.Repo.Keystore())
	repub.Interval = time.Millisecond * 500
	repub.RecordLifetime = time.Second

	proc := goprocess.Go(repub.Run)
	defer proc.Close()

	// now wait a couple seconds for it to fire a few times
	time.Sleep(time.Second * 2)

	err = verifyResolution(nodes, name, p)
	if err != nil {
		t.Fatal(err)
	}

	entry, err := getLastIPNSEntry(publisher.Repo.Datastore(), publisher.Identity)
	if err != nil {
		t.Fatal(err)
	}

	finalEol, err := ipns.GetEOL(entry)
	if err != nil {
		t.Fatal(err)
	}

	if !finalEol.Equal(expiration) {
		t.Fatal("expiration time modified")
	}
}

func getLastIPNSEntry(dstore ds.Datastore, id peer.ID) (*ipns_pb.IpnsEntry, error) {
	// Look for it locally only
	val, err := dstore.Get(namesys.IpnsDsKey(id))
	if err != nil {
		return nil, err
	}

	e := new(ipns_pb.IpnsEntry)
	if err := proto.Unmarshal(val, e); err != nil {
		return nil, err
	}
	return e, nil
}

func verifyResolution(nodes []*core.IpfsNode, key string, exp path.Path) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, n := range nodes {
		val, err := n.Namesys.Resolve(ctx, key)
		if err != nil {
			return err
		}

		if val != exp {
			return errors.New("resolved wrong record")
		}
	}
	return nil
}

func verifyResolutionFails(nodes []*core.IpfsNode, key string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, n := range nodes {
		_, err := n.Namesys.Resolve(ctx, key)
		if err == nil {
			return errors.New("expected resolution to fail")
		}
	}
	return nil
}
