package coreapi_test

import (
	"context"
	"io"
	"io/ioutil"
	"testing"

	coreapi "github.com/ipfs/go-ipfs/core/coreapi"

	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

func TestDhtFindPeer(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 3)
	if err != nil {
		t.Fatal(err)
	}

	out, err := apis[2].Dht().FindPeer(ctx, peer.ID(nds[0].Identity))
	if err != nil {
		t.Fatal(err)
	}

	addr := <-out

	if addr.String() != "/ip4/127.0.0.1/tcp/4001" {
		t.Errorf("got unexpected address from FindPeer: %s", addr.String())
	}

	out, err = apis[1].Dht().FindPeer(ctx, peer.ID(nds[2].Identity))
	if err != nil {
		t.Fatal(err)
	}

	addr = <-out

	if addr.String() != "/ip4/127.0.2.1/tcp/4001" {
		t.Errorf("got unexpected address from FindPeer: %s", addr.String())
	}
}

func TestDhtFindProviders(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 3)
	if err != nil {
		t.Fatal(err)
	}

	p, err := addTestObject(ctx, apis[0])
	if err != nil {
		t.Fatal(err)
	}

	out, err := apis[2].Dht().FindProviders(ctx, p, apis[2].Dht().WithNumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	if provider.String() != nds[0].Identity.String() {
		t.Errorf("got wrong provider: %s != %s", provider.String(), nds[0].Identity.String())
	}
}

func TestDhtProvide(t *testing.T) {
	ctx := context.Background()
	nds, apis, err := makeAPISwarm(ctx, true, 3)
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
	p := coreapi.ParseCid(b.Cid())

	out, err := apis[2].Dht().FindProviders(ctx, p, apis[2].Dht().WithNumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	if provider.String() != "<peer.ID >" {
		t.Errorf("got wrong provider: %s != %s", provider.String(), nds[0].Identity.String())
	}

	err = apis[0].Dht().Provide(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	out, err = apis[2].Dht().FindProviders(ctx, p, apis[2].Dht().WithNumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider = <-out

	if provider.String() != nds[0].Identity.String() {
		t.Errorf("got wrong provider: %s != %s", provider.String(), nds[0].Identity.String())
	}
}
