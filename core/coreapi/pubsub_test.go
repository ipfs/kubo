package coreapi_test

import (
	"context"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"testing"
	"time"
)

func TestBasicPubSub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nds, apis, err := makeAPISwarm(ctx, true, 2)
	if err != nil {
		t.Fatal(err)
	}

	sub, err := apis[0].PubSub().Subscribe(ctx, "testch")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		tick := time.Tick(100 * time.Millisecond)

		for {
			err = apis[1].PubSub().Publish(ctx, "testch", []byte("hello world"))
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-tick:
			case <-ctx.Done():
				return
			}
		}
	}()

	m, err := sub.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if string(m.Data()) != "hello world" {
		t.Errorf("got invalid data: %s", string(m.Data()))
	}

	if m.From() != nds[1].Identity {
		t.Errorf("m.From didn't match")
	}

	peers, err := apis[1].PubSub().Peers(ctx, options.PubSub.Topic("testch"))
	if err != nil {
		t.Fatal(err)
	}

	if len(peers) != 1 {
		t.Fatalf("got incorrect number of peers: %d", len(peers))
	}

	if peers[0] != nds[0].Identity {
		t.Errorf("peer didn't match")
	}

	peers, err = apis[1].PubSub().Peers(ctx, options.PubSub.Topic("nottestch"))
	if err != nil {
		t.Fatal(err)
	}

	if len(peers) != 0 {
		t.Fatalf("got incorrect number of peers: %d", len(peers))
	}

	topics, err := apis[0].PubSub().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(topics) != 1 {
		t.Fatalf("got incorrect number of topics: %d", len(peers))
	}

	if topics[0] != "testch" {
		t.Errorf("topic didn't match")
	}

	topics, err = apis[1].PubSub().Ls(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(topics) != 0 {
		t.Fatalf("got incorrect number of topics: %d", len(peers))
	}
}
