package integrationtest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/bootstrap"
	"github.com/ipfs/kubo/core/coreapi"
	libp2p2 "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"

	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"

	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"

	mock "github.com/ipfs/kubo/core/mock"
	"github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestMessageSeenCacheTTL(t *testing.T) {
	if err := RunMessageSeenCacheTTLTest(t, "10s"); err != nil {
		t.Fatal(err)
	}
}

func mockNode(ctx context.Context, mn mocknet.Mocknet, pubsubEnabled bool, seenMessagesCacheTTL string) (*core.IpfsNode, error) {
	ds := syncds.MutexWrap(datastore.NewMapDatastore())
	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		return nil, err
	}
	count := len(mn.Peers())
	cfg.Addresses.Swarm = []string{
		fmt.Sprintf("/ip4/18.0.%d.%d/tcp/4001", count>>16, count&0xFF),
	}
	cfg.Datastore = config.Datastore{}
	if pubsubEnabled {
		cfg.Pubsub.Enabled = config.True
		var ttl *config.OptionalDuration
		if len(seenMessagesCacheTTL) > 0 {
			ttl = &config.OptionalDuration{}
			if err = ttl.UnmarshalJSON([]byte(seenMessagesCacheTTL)); err != nil {
				return nil, err
			}
		}
		cfg.Pubsub.SeenMessagesTTL = ttl
	}
	return core.NewNode(ctx, &core.BuildCfg{
		Online:  true,
		Routing: libp2p2.DHTServerOption,
		Repo: &repo.Mock{
			C: *cfg,
			D: ds,
		},
		Host: mock.MockHostOption(mn),
		ExtraOpts: map[string]bool{
			"pubsub": pubsubEnabled,
		},
	})
}

func RunMessageSeenCacheTTLTest(t *testing.T, seenMessagesCacheTTL string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var bootstrapNode, consumerNode, producerNode *core.IpfsNode
	var bootstrapPeerID, consumerPeerID, producerPeerID peer.ID
	sendDupMsg := false

	mn := mocknet.New()
	bootstrapNode, err := mockNode(ctx, mn, false, "") // no need for PubSub configuration
	if err != nil {
		t.Fatal(err)
	}
	bootstrapPeerID = bootstrapNode.PeerHost.ID()
	defer bootstrapNode.Close()

	consumerNode, err = mockNode(ctx, mn, true, seenMessagesCacheTTL) // use passed seen cache TTL
	if err != nil {
		t.Fatal(err)
	}
	consumerPeerID = consumerNode.PeerHost.ID()
	defer consumerNode.Close()

	ttl, err := time.ParseDuration(seenMessagesCacheTTL)
	if err != nil {
		t.Fatal(err)
	}

	// Set up the pubsub message ID generation override for the producer
	core.RegisterFXOptionFunc(func(info core.FXNodeInfo) ([]fx.Option, error) {
		var pubsubOptions []pubsub.Option
		pubsubOptions = append(
			pubsubOptions,
			pubsub.WithSeenMessagesTTL(ttl),
			pubsub.WithMessageIdFn(func(pmsg *pubsub_pb.Message) string {
				now := time.Now().Format(time.StampMilli)
				msg := string(pmsg.Data)
				var msgID string
				from, _ := peer.IDFromBytes(pmsg.From)
				if (from == producerPeerID) && sendDupMsg {
					msgID = "DupMsg"
					t.Logf("sending [%s] with duplicate message ID at [%s]", msg, now)
				} else {
					msgID = pubsub.DefaultMsgIdFn(pmsg)
					t.Logf("sending [%s] with unique message ID at [%s]", msg, now)
				}
				return msgID
			}),
		)
		return append(
			info.FXOptions,
			fx.Provide(libp2p2.TopicDiscovery()),
			fx.Decorate(libp2p2.GossipSub(pubsubOptions...)),
		), nil
	})

	producerNode, err = mockNode(ctx, mn, false, "") // PubSub configuration comes from overrides above
	if err != nil {
		t.Fatal(err)
	}
	producerPeerID = producerNode.PeerHost.ID()
	defer producerNode.Close()

	t.Logf("bootstrap peer=%s, consumer peer=%s, producer peer=%s", bootstrapPeerID, consumerPeerID, producerPeerID)

	producerAPI, err := coreapi.NewCoreAPI(producerNode)
	if err != nil {
		t.Fatal(err)
	}
	consumerAPI, err := coreapi.NewCoreAPI(consumerNode)
	if err != nil {
		t.Fatal(err)
	}

	err = mn.LinkAll()
	if err != nil {
		t.Fatal(err)
	}

	bis := bootstrapNode.Peerstore.PeerInfo(bootstrapNode.PeerHost.ID())
	bcfg := bootstrap.BootstrapConfigWithPeers([]peer.AddrInfo{bis})
	if err = producerNode.Bootstrap(bcfg); err != nil {
		t.Fatal(err)
	}
	if err = consumerNode.Bootstrap(bcfg); err != nil {
		t.Fatal(err)
	}

	// Set up the consumer subscription
	const TopicName = "topic"
	consumerSubscription, err := consumerAPI.PubSub().Subscribe(ctx, TopicName)
	if err != nil {
		t.Fatal(err)
	}
	// Utility functions defined inline to include context in closure
	now := func() string {
		return time.Now().Format(time.StampMilli)
	}
	ctr := 0
	msgGen := func() string {
		ctr++
		return fmt.Sprintf("msg_%d", ctr)
	}
	produceMessage := func() string {
		msgTxt := msgGen()
		err = producerAPI.PubSub().Publish(ctx, TopicName, []byte(msgTxt))
		if err != nil {
			t.Fatal(err)
		}
		return msgTxt
	}
	consumeMessage := func(msgTxt string, shouldFind bool) {
		// Set up a separate timed context for receiving messages
		rxCtx, rxCancel := context.WithTimeout(context.Background(), time.Second)
		defer rxCancel()
		msg, err := consumerSubscription.Next(rxCtx)
		if shouldFind {
			if err != nil {
				t.Logf("did not receive [%s] by [%s]", msgTxt, now())
				t.Fatal(err)
			}
			t.Logf("received [%s] at [%s]", string(msg.Data()), now())
			if !bytes.Equal(msg.Data(), []byte(msgTxt)) {
				t.Fatalf("consumed data [%s] does not match published data [%s]", string(msg.Data()), msgTxt)
			}
		} else {
			if err == nil {
				t.Logf("received [%s] at [%s]", string(msg.Data()), now())
				t.Fail()
			}
			t.Logf("did not receive [%s] by [%s]", msgTxt, now())
		}
	}

	// Send message 1 with the message ID we're going to duplicate later
	sendDupMsg = true
	msgTxt := produceMessage()
	consumeMessage(msgTxt, true) // should find message

	// Send message 2 with the same message ID as before
	sendDupMsg = true
	msgTxt = produceMessage()
	consumeMessage(msgTxt, false) // should NOT find message, because it got deduplicated (sent twice within the SeenMessagesTTL window)

	// Wait for seen cache TTL time to let seen cache entries time out
	time.Sleep(ttl)

	// Send message 3 with a new message ID
	//
	// This extra step is necessary for testing the cache TTL because the PubSub code only garbage collects when a
	// message ID was not already present in the cache. This means that message 2's cache entry, even though it has
	// technically timed out, will still cause the message to be considered duplicate. When a message with a different
	// ID passes through, it will be added to the cache and garbage collection will clean up message 2's entry. This is
	// another bug in the pubsub/cache implementation that will be fixed once the code is refactored for this issue:
	// https://github.com/libp2p/go-libp2p-pubsub/issues/502
	sendDupMsg = false
	msgTxt = produceMessage()
	consumeMessage(msgTxt, true) // should find message

	// Send message 4 with the same message ID as before
	sendDupMsg = true
	msgTxt = produceMessage()
	consumeMessage(msgTxt, true) // should find message again (time since the last read > SeenMessagesTTL, so it looks like a new message).

	// Send message 5 with a new message ID
	//
	// This step is not strictly necessary, but has been added for good measure.
	sendDupMsg = false
	msgTxt = produceMessage()
	consumeMessage(msgTxt, true) // should find message
	return nil
}
