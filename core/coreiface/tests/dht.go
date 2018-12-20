package tests

import (
	"context"
	"io"
	"testing"

	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
)

func TestDht(t *testing.T) {
	t.Run("TestDhtFindPeer", TestDhtFindPeer)
	t.Run("TestDhtFindProviders", TestDhtFindProviders)
	t.Run("TestDhtProvide", TestDhtProvide)
}

func TestDhtFindPeer(t *testing.T) {
	ctx := context.Background()
	apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pi, err := apis[2].Dht().FindPeer(ctx, self0.ID())
	if err != nil {
		t.Fatal(err)
	}

	if pi.Addrs[0].String() != "/ip4/127.0.0.1/tcp/4001" {
		t.Errorf("got unexpected address from FindPeer: %s", pi.Addrs[0].String())
	}

	self2, err := apis[2].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pi, err = apis[1].Dht().FindPeer(ctx, self2.ID())
	if err != nil {
		t.Fatal(err)
	}

	if pi.Addrs[0].String() != "/ip4/127.0.2.1/tcp/4001" {
		t.Errorf("got unexpected address from FindPeer: %s", pi.Addrs[0].String())
	}
}

func TestDhtFindProviders(t *testing.T) {
	ctx := context.Background()
	apis, err := makeAPISwarm(ctx, true, 5)
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

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if provider.ID.String() != self0.ID().String() {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), self0.ID().String())
	}
}

func TestDhtProvide(t *testing.T) {
	ctx := context.Background()
	apis, err := makeAPISwarm(ctx, true, 5)
	if err != nil {
		t.Fatal(err)
	}

	off0, err := apis[0].WithOptions(options.Api.Offline(true))
	if err != nil {
		t.Fatal(err)
	}

	s, err := off0.Block().Put(ctx, &io.LimitedReader{R: rnd, N: 4092})
	if err != nil {
		t.Fatal(err)
	}

	p := s.Path()

	out, err := apis[2].Dht().FindProviders(ctx, p, options.Dht.NumProviders(1))
	if err != nil {
		t.Fatal(err)
	}

	provider := <-out

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if provider.ID.String() != "<peer.ID >" {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), self0.ID().String())
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

	if provider.ID.String() != self0.ID().String() {
		t.Errorf("got wrong provider: %s != %s", provider.ID.String(), self0.ID().String())
	}
}
