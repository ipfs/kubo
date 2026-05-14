package tests

import (
	"context"
	"testing"
	"time"

	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
)

func (tp *TestSuite) TestPubSub(t *testing.T) {
	tp.hasApi(t, func(api iface.CoreAPI) error {
		if api.PubSub() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestBasicPubSub", tp.TestBasicPubSub)
}

func (tp *TestSuite) TestBasicPubSub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	apis, err := tp.MakeAPISwarm(t, ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	sub, err := apis[0].PubSub().Subscribe(ctx, "testch")
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			err := apis[1].PubSub().Publish(ctx, "testch", []byte("hello world"))
			switch err {
			case nil:
			case context.Canceled:
				return
			default:
				t.Error(err)
				cancel()
				return
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for the sender to finish before we return.
	// Otherwise, we can get random errors as publish fails.
	defer func() {
		cancel()
		<-done
	}()

	m, err := sub.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if string(m.Data()) != "hello world" {
		t.Errorf("got invalid data: %s", string(m.Data()))
	}

	self1, err := apis[1].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if m.From() != self1.ID() {
		t.Errorf("m.From didn't match")
	}

	peers, err := apis[1].PubSub().Peers(ctx, options.PubSub.Topic("testch"))
	if err != nil {
		t.Fatal(err)
	}

	if len(peers) != 1 {
		t.Fatalf("got incorrect number of peers: %d", len(peers))
	}

	self0, err := apis[0].Key().Self(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if peers[0] != self0.ID() {
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
