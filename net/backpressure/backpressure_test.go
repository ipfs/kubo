package backpressure_tests

import (
	"testing"
	"time"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

var log = eventlog.Logger("backpressure")

func GenNetwork(ctx context.Context) (inet.Network, error) {
	p, err := testutil.PeerWithKeysAndAddress(testutil.RandLocalTCPAddress())
	if err != nil {
		return nil, err
	}

	listen := p.Addresses()
	ps := peer.NewPeerstore()
	return inet.NewNetwork(ctx, listen, p, ps)
}

// TestBackpressureStreamHandler tests whether mux handler
// ratelimiting works. Meaning, since the handler is sequential
// it should block senders.
//
// Important note: spdystream (which peerstream uses) has a set
// of n workers (n=spdsystream.FRAME_WORKERS) which handle new
// frames, including those starting new streams. So all of them
// can be in the handler at one time. Also, the sending side
// does not rate limit unless we call stream.Wait()
//
//
// Note: right now, this happens muxer-wide. the muxer should
// learn to flow control, so handlers cant block each other.
func TestBackpressureStreamHandler(t *testing.T) {
	t.Skip(`Sadly, as cool as this test is, it doesn't work
Because spdystream doesnt handle stream open backpressure
well IMO. I'll see about rewriting that part when it becomes
a problem.
`)

	// a number of concurrent request handlers
	limit := 10

	// our way to signal that we're done with 1 request
	requestHandled := make(chan struct{})

	// handler rate limiting
	receiverRatelimit := make(chan struct{}, limit)
	for i := 0; i < limit; i++ {
		receiverRatelimit <- struct{}{}
	}

	// sender counter of successfully opened streams
	senderOpened := make(chan struct{}, limit*100)

	// sender signals it's done (errored out)
	senderDone := make(chan struct{})

	// the receiver handles requests with some rate limiting
	receiver := func(s inet.Stream) {
		log.Debug("receiver received a stream")

		<-receiverRatelimit // acquire
		go func() {
			// our request handler. can do stuff here. we
			// simulate something taking time by waiting
			// on requestHandled
			log.Error("request worker handling...")
			<-requestHandled
			log.Error("request worker done!")
			receiverRatelimit <- struct{}{} // release
		}()
	}

	// the sender opens streams as fast as possible
	sender := func(net inet.Network, remote peer.Peer) {
		var s inet.Stream
		var err error
		defer func() {
			t.Error(err)
			log.Debug("sender error. exiting.")
			senderDone <- struct{}{}
		}()

		for {
			s, err = net.NewStream(inet.ProtocolTesting, remote)
			if err != nil {
				return
			}

			_ = s
			// if err = s.SwarmStream().Stream().Wait(); err != nil {
			// 	return
			// }

			// "count" another successfully opened stream
			// (large buffer so shouldn't block in normal operation)
			log.Debug("sender opened another stream!")
			senderOpened <- struct{}{}
		}
	}

	// count our senderOpened events
	countStreamsOpenedBySender := func(min int) int {
		opened := 0
		for opened < min {
			log.Debugf("countStreamsOpenedBySender got %d (min %d)", opened, min)
			select {
			case <-senderOpened:
				opened++
			case <-time.After(10 * time.Millisecond):
			}
		}
		return opened
	}

	// count our received events
	// waitForNReceivedStreams := func(n int) {
	// 	for n > 0 {
	// 		log.Debugf("waiting for %d received streams...", n)
	// 		select {
	// 		case <-receiverRatelimit:
	// 			n--
	// 		}
	// 	}
	// }

	testStreamsOpened := func(expected int) {
		log.Debugf("testing rate limited to %d streams", expected)
		if n := countStreamsOpenedBySender(expected); n != expected {
			t.Fatalf("rate limiting did not work :( -- %d != %d", expected, n)
		}
	}

	// ok that's enough setup. let's do it!

	ctx := context.Background()
	n1, err := GenNetwork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := GenNetwork(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// setup receiver handler
	n1.SetHandler(inet.ProtocolTesting, receiver)

	log.Debugf("dialing %s", n2.ListenAddresses())
	if err := n1.DialPeer(ctx, n2.LocalPeer()); err != nil {
		t.Fatalf("Failed to dial:", err)
	}

	// launch sender!
	go sender(n2, n1.LocalPeer())

	// ok, what do we expect to happen? the receiver should
	// receive 10 requests and stop receiving, blocking the sender.
	// we can test this by counting 10x senderOpened requests

	<-senderOpened // wait for the sender to successfully open some.
	testStreamsOpened(limit - 1)

	// let's "handle" 3 requests.
	<-requestHandled
	<-requestHandled
	<-requestHandled
	// the sender should've now been able to open exactly 3 more.

	testStreamsOpened(3)

	// shouldn't have opened anything more
	testStreamsOpened(0)

	// let's "handle" 100 requests in batches of 5
	for i := 0; i < 20; i++ {
		<-requestHandled
		<-requestHandled
		<-requestHandled
		<-requestHandled
		<-requestHandled
		testStreamsOpened(5)
	}

	// success!

	// now for the sugar on top: let's tear down the receiver. it should
	// exit the sender.
	n1.Close()

	// shouldn't have opened anything more
	testStreamsOpened(0)

	select {
	case <-time.After(100 * time.Millisecond):
		t.Error("receiver shutdown failed to exit sender")
	case <-senderDone:
		log.Info("handler backpressure works!")
	}
}
