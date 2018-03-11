package republisher_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-ipfs/core"
	mock "github.com/ipfs/go-ipfs/core/mock"
	namesys "github.com/ipfs/go-ipfs/namesys"
	. "github.com/ipfs/go-ipfs/namesys/republisher"
	path "github.com/ipfs/go-ipfs/path"

	mocknet "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/net/mock"
	goprocess "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	floodsub "gx/ipfs/QmSFihvoND3eDaAYRCeLgLPt62yCPgMZs1NSZmKFEtJQQw/go-libp2p-floodsub"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
)

func TestRepublish(t *testing.T) {
	// set cache life to zero for testing low-period repubs

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runTest := func(publisheri int) {
		nodes := createMockNet(ctx, t)
		publisher := nodes[publisheri]

		// have one node publish a record that is valid for 1 second
		p := path.FromString("/ipfs/QmP1DfoUjiWH2ZBo1PBH6FupdBucbDepx3HpWmEY6JMUpY")
		err := publisher.Namesys.PublishWithEOL(ctx, publisher.PrivateKey, p, time.Now().Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}

		name := "/ipns/" + publisher.Identity.Pretty()
		if err := verifyResolution(nodes, name, p); err != nil {
			t.Fatal(err)
		}

		// Now wait a second, the records will be invalid and we should fail to resolve
		time.Sleep(time.Second)
		if err := verifyResolutionFails(nodes, name); err != nil {
			t.Fatal(err)
		}

		// The republishers that are contained within the nodes have their timeout set
		// to 12 hours. Instead of trying to tweak those, we're just going to pretend
		// they dont exist and make our own.
		repub := NewRepublisher(publisher.Routing, publisher.Repo.Datastore(), publisher.PrivateKey, publisher.Repo.Keystore())
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

	// Even nodes have pubsub enabled, odd nodes do not,
	// so run test once with a pubsub node as publisher and
	// once with a regular node as publisher
	runTest(3)
	runTest(4)
}

func TestRepublishOtherPeerKey(t *testing.T) {
	// set cache life to zero for testing low-period repubs

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runTest := func(puba, pubb int) {
		nodes := createMockNet(ctx, t)

		// have node A publish a record to its own key
		publisherA := nodes[puba]
		p := path.FromString("/ipfs/QmP1DfoUjiWH2ZBo1PBH6FupdBucbDepx3HpWmEY6JMUpY")
		err := publisherA.Namesys.PublishWithEOL(ctx, publisherA.PrivateKey, p, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}

		// Wait a moment to make sure record propagates out through pubsub
		time.Sleep(time.Millisecond * 100)

		// verify the record can be resolved by all nodes
		name := "/ipns/" + publisherA.Identity.Pretty()
		if err := verifyResolution(nodes, name, p); err != nil {
			t.Fatal(err)
		}

		// have node B publish an updated record to node A's key
		publisherB := nodes[pubb]
		p2 := path.FromString("/ipfs/QmP1wMAqk6aZYRZirbaAwmrNeqFRgQrwBt3orUtvSa1UYD")
		err = publisherB.Namesys.PublishWithEOL(ctx, publisherA.PrivateKey, p2, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}

		// Wait a moment to make sure record propagates out through pubsub
		time.Sleep(time.Millisecond * 100)

		// verify the record is correctly resolved by all nodes
		if err := verifyResolution(nodes, name, p2); err != nil {
			t.Fatal(err)
		}

		// have node A publish an updated record to its own key again
		p3 := path.FromString("/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr")
		err = publisherA.Namesys.PublishWithEOL(ctx, publisherA.PrivateKey, p3, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}

		// Wait a moment to make sure record propagates out through pubsub
		time.Sleep(time.Millisecond * 1000)

		// verify the record is correctly resolved by all nodes
		if err := verifyResolution(nodes, name, p3); err != nil {
			t.Fatal(err)
		}
	}

	// Even nodes have pubsub enabled, odd nodes do not,
	// so run test with each combination of either pubsub
	// or regular node as publisher A and B respectively
	// Regular / Regular
	runTest(3, 5)
	// Pubsub / Pubsub
	runTest(4, 6)
	// Pubsub / Regular
	runTest(4, 5)
	// Regular / Pubsub
	// runTest(3, 4)
}

func createMockNet(ctx context.Context, t *testing.T) []*core.IpfsNode {
	// create network
	mn := mocknet.New(ctx)

	var nodes []*core.IpfsNode
	for i := 0; i < 10; i++ {
		nd, err := core.NewNode(ctx, &core.BuildCfg{
			Online: true,
			Host:   mock.MockHostOption(mn),
		})
		if err != nil {
			t.Fatal(err)
		}

		nd.Namesys = namesys.NewNameSystem(nd.Routing, nd.Repo.Datastore(), 0)
		// Enable pubsub for even nodes
		if i%2 == 0 {
			service, err := floodsub.NewFloodSub(ctx, nd.PeerHost)
			if err != nil {
				t.Fatal(err)
			}
			nd.Floodsub = service
			err = namesys.AddPubsubNameSystem(ctx, nd.Namesys, nd.PeerHost, nd.Routing, nd.Repo.Datastore(), nd.Floodsub)
			if err != nil {
				t.Fatal(err)
			}
		}
		nodes = append(nodes, nd)
	}

	mn.LinkAll()

	bsinf := core.BootstrapConfigWithPeers(
		[]pstore.PeerInfo{
			nodes[0].Peerstore.PeerInfo(nodes[0].Identity),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.Bootstrap(bsinf); err != nil {
			t.Fatal(err)
		}
	}

	return nodes
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
			return fmt.Errorf("resolved wrong record\nexpected: %s\ngot:      %s", exp, val)
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
