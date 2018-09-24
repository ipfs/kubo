package coreapi_test

import (
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	peer "gx/ipfs/QmbNepETomvmXfz1X5pHNFD2QuPqnqi47dTd94QJWSorQ3/go-libp2p-peer"
)

func TestDhtFindPeer(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}

	pi, err := apis[2].Dht().FindPeer(ctx, peer.ID(nds[0].Identity))
	if err != nil {
		t.Fatal(err)
	}

	if pi.Addrs[0].String() != "/ip4/127.0.0.1/tcp/4001" {
		t.Errorf("got unexpected address from FindPeer: %s", pi.Addrs[0].String())
	}

	pi, err = apis[1].Dht().FindPeer(ctx, peer.ID(nds[2].Identity))
	if err != nil {
		t.Fatal(err)
	}

	if pi.Addrs[0].String() != "/ip4/127.0.2.1/tcp/4001" {
		t.Errorf("got unexpected address from FindPeer: %s", pi.Addrs[0].String())
	}
}

func TestDhtFindProviders(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}

	p, err := addTestObject(ctx, apis[0])
	if err != nil {
		t.Fatal(err)
	}

	out, err := apis[2].Dht().FindProviders(ctx, p, options.Dht.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	if provider.ID.String() != nds[0].Identity.String() {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), nds[0].Identity.String())
	}
}

func TestDhtProvide(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}

	// TODO: replace once there is local add on unixfs or somewhere
	data, err := ioutil.ReadAll(&io.LimitedReader{R: rnd, N: 4092})
	if err != nil {
		t.Fatal(err)
	}

	b := blocks.NewBlock(data)
	nds[0].Blockstore.Put(b)
	p := iface.IpfsPath(b.Cid())

	out, err := apis[2].Dht().FindProviders(ctx, p, options.Dht.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	if provider.ID.String() != "<peer.ID >" {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), nds[0].Identity.String())
	}

	err = apis[0].Dht().Provide(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	out, err = apis[2].Dht().FindProviders(ctx, p, options.Dht.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider = <-out

	if provider.ID.String() != nds[0].Identity.String() {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), nds[0].Identity.String())
	}
}
