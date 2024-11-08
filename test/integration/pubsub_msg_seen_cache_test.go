package integrationtest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	libp2p2 "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"

	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p-pubsub/timecache"
	"github.com/libp2p/go-libp2p/core/peer"

	mock "github.com/ipfs/kubo/core/mock"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

func TestMessageSeenCacheTTL(t *testing.T) {
	t.Skip("skipping PubSub seen cache TTL test due to flakiness")
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

	// Used for logging the timeline
	startTime := time.Time{}

	// Used for overriding the message ID
	sendMsgID := ""

	// Set up the pubsub message ID generation override for the producer
	core.RegisterFXOptionFunc(func(info core.FXNodeInfo) ([]fx.Option, error) {
		var pubsubOptions []pubsub.Option
		pubsubOptions = append(
			pubsubOptions,
			pubsub.WithSeenMessagesTTL(ttl),
			pubsub.WithMessageIdFn(func(pmsg *pubsub_pb.Message) string {
				now := time.Now()
				if startTime.Second() == 0 {
					startTime = now
				}
				timeElapsed := now.Sub(startTime).Seconds()
				msg := string(pmsg.Data)
				from, _ := peer.IDFromBytes(pmsg.From)
				var msgID string
				if from == producerPeerID {
					msgID = sendMsgID
					t.Logf("sending [%s] with message ID [%s] at T%fs", msg, msgID, timeElapsed)
				} else {
					msgID = pubsub.DefaultMsgIdFn(pmsg)
				}
				return msgID
			}),
			pubsub.WithSeenMessagesStrategy(timecache.Strategy_LastSeen),
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
	now := func() float64 {
		return time.Since(startTime).Seconds()
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
				t.Logf("expected but did not receive [%s] at T%fs", msgTxt, now())
				t.Fatal(err)
			}
			t.Logf("received [%s] at T%fs", string(msg.Data()), now())
			if !bytes.Equal(msg.Data(), []byte(msgTxt)) {
				t.Fatalf("consumed data [%s] does not match published data [%s]", string(msg.Data()), msgTxt)
			}
		} else {
			if err == nil {
				t.Logf("not expected but received [%s] at T%fs", string(msg.Data()), now())
				t.Fail()
			}
			t.Logf("did not receive [%s] at T%fs", msgTxt, now())
		}
	}

	const MsgID1 = "MsgID1"
	const MsgID2 = "MsgID2"
	const MsgID3 = "MsgID3"

	// Send message 1 with the message ID we're going to duplicate
	sentMsg1 := time.Now()
	sendMsgID = MsgID1
	msgTxt := produceMessage()
	// Should find the message because it's new
	consumeMessage(msgTxt, true)

	// Send message 2 with a duplicate message ID
	sendMsgID = MsgID1
	msgTxt = produceMessage()
	// Should NOT find message because it got deduplicated (sent 2 times within the SeenMessagesTTL window).
	consumeMessage(msgTxt, false)

	// Send message 3 with a new message ID
	sendMsgID = MsgID2
	msgTxt = produceMessage()
	// Should find the message because it's new
	consumeMessage(msgTxt, true)

	// Wait till just before the SeenMessagesTTL window has passed since message 1 was sent
	time.Sleep(time.Until(sentMsg1.Add(ttl - 100*time.Millisecond)))

	// Send message 4 with a duplicate message ID
	sendMsgID = MsgID1
	msgTxt = produceMessage()
	// Should NOT find the message because it got deduplicated (sent 3 times within the SeenMessagesTTL window). This
	// time, however, the expiration for the message should also get pushed out for a whole SeenMessagesTTL window since
	// the default time cache now implements a sliding window algorithm.
	consumeMessage(msgTxt, false)

	// Send message 5 with a duplicate message ID. This will be a second after the last attempt above since NOT finding
	// a message takes a second to determine. That would put this attempt at ~1 second after the SeenMessagesTTL window
	// starting at message 1 has expired.
	sentMsg5 := time.Now()
	sendMsgID = MsgID1
	msgTxt = produceMessage()
	// Should NOT find the message, because it got deduplicated (sent 2 times since the updated SeenMessagesTTL window
	// started). This time again, the expiration should get pushed out for another SeenMessagesTTL window.
	consumeMessage(msgTxt, false)

	// Send message 6 with a message ID that hasn't been seen within a SeenMessagesTTL window
	sendMsgID = MsgID2
	msgTxt = produceMessage()
	// Should find the message since last read > SeenMessagesTTL, so it looks like a new message.
	consumeMessage(msgTxt, true)

	// Sleep for a full SeenMessagesTTL window to let cache entries time out
	time.Sleep(time.Until(sentMsg5.Add(ttl + 100*time.Millisecond)))

	// Send message 7 with a duplicate message ID
	sendMsgID = MsgID1
	msgTxt = produceMessage()
	// Should find the message this time since last read > SeenMessagesTTL, so it looks like a new message.
	consumeMessage(msgTxt, true)

	// Send message 8 with a brand new message ID
	//
	// This step is not strictly necessary, but has been added for good measure.
	sendMsgID = MsgID3
	msgTxt = produceMessage()
	// Should find the message because it's new
	consumeMessage(msgTxt, true)
	return nil
}
