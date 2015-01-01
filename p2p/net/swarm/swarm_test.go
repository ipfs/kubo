package swarm

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func EchoStreamHandler(stream inet.Stream) {
	go func() {
		defer stream.Close()

		// pull out the ipfs conn
		c := stream.Conn()
		log.Debugf("%s ponging to %s", c.LocalPeer(), c.RemotePeer())

		buf := make([]byte, 4)

		for {
			if _, err := stream.Read(buf); err != nil {
				if err != io.EOF {
					log.Error("ping receive error:", err)
				}
				return
			}

			if !bytes.Equal(buf, []byte("ping")) {
				log.Errorf("ping receive error: ping != %s %v", buf, buf)
				return
			}

			if _, err := stream.Write([]byte("pong")); err != nil {
				log.Error("pond send error:", err)
				return
			}
		}
	}()
}

func makeSwarms(ctx context.Context, t *testing.T, num int) ([]*Swarm, []testutil.PeerNetParams) {
	swarms := make([]*Swarm, 0, num)
	peersnp := make([]testutil.PeerNetParams, 0, num)

	for i := 0; i < num; i++ {
		localnp := testutil.RandPeerNetParamsOrFatal(t)
		peersnp = append(peersnp, localnp)

		peerstore := peer.NewPeerstore()
		peerstore.AddAddress(localnp.ID, localnp.Addr)
		peerstore.AddPubKey(localnp.ID, localnp.PubKey)
		peerstore.AddPrivKey(localnp.ID, localnp.PrivKey)

		addrs := peerstore.Addresses(localnp.ID)
		swarm, err := NewSwarm(ctx, addrs, localnp.ID, peerstore)
		if err != nil {
			t.Fatal(err)
		}

		swarm.SetStreamHandler(EchoStreamHandler)
		swarms = append(swarms, swarm)
	}

	return swarms, peersnp
}

func connectSwarms(t *testing.T, ctx context.Context, swarms []*Swarm, peersnp []testutil.PeerNetParams) {

	var wg sync.WaitGroup
	connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
		// TODO: make a DialAddr func.
		s.peers.AddAddress(dst, addr)
		if _, err := s.Dial(ctx, dst); err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}
		wg.Done()
	}

	log.Info("Connecting swarms simultaneously.")
	for _, s := range swarms {
		for _, p := range peersnp {
			if p.ID != s.local { // don't connect to self.
				wg.Add(1)
				connect(s, p.ID, p.Addr)
			}
		}
	}
	wg.Wait()

	for _, s := range swarms {
		log.Infof("%s swarm routing table: %s", s.local, s.Peers())
	}
}

func SubtestSwarm(t *testing.T, SwarmNum int, MsgNum int) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms, peersnp := makeSwarms(ctx, t, SwarmNum)

	// connect everyone
	connectSwarms(t, ctx, swarms, peersnp)

	// ping/pong
	for _, s1 := range swarms {
		log.Debugf("-------------------------------------------------------")
		log.Debugf("%s ping pong round", s1.local)
		log.Debugf("-------------------------------------------------------")

		_, cancel := context.WithCancel(ctx)
		got := map[peer.ID]int{}
		errChan := make(chan error, MsgNum*len(peersnp))
		streamChan := make(chan *Stream, MsgNum)

		// send out "ping" x MsgNum to every peer
		go func() {
			defer close(streamChan)

			var wg sync.WaitGroup
			send := func(p peer.ID) {
				defer wg.Done()

				// first, one stream per peer (nice)
				stream, err := s1.NewStreamWithPeer(p)
				if err != nil {
					errChan <- errors.Wrap(err)
					return
				}

				// send out ping!
				for k := 0; k < MsgNum; k++ { // with k messages
					msg := "ping"
					log.Debugf("%s %s %s (%d)", s1.local, msg, p, k)
					if _, err := stream.Write([]byte(msg)); err != nil {
						errChan <- err
						continue
					}
				}

				// read it later
				streamChan <- stream
			}

			for _, p := range peersnp {
				if p.ID == s1.local {
					continue // dont send to self...
				}

				wg.Add(1)
				go send(p.ID)
			}
			wg.Wait()
		}()

		// receive "pong" x MsgNum from every peer
		go func() {
			defer close(errChan)
			count := 0
			countShouldBe := MsgNum * (len(peersnp) - 1)
			for stream := range streamChan { // one per peer
				defer stream.Close()

				// get peer on the other side
				p := stream.Conn().RemotePeer()

				// receive pings
				msgCount := 0
				msg := make([]byte, 4)
				for k := 0; k < MsgNum; k++ { // with k messages

					// read from the stream
					if _, err := stream.Read(msg); err != nil {
						errChan <- errors.Wrap(err)
						continue
					}

					if string(msg) != "pong" {
						errChan <- errors.Errorf("unexpected message: %s", msg)
						continue
					}

					log.Debugf("%s %s %s (%d)", s1.local, msg, p, k)
					msgCount++
				}

				got[p] = msgCount
				count += msgCount
			}

			if count != countShouldBe {
				errChan <- errors.Errorf("count mismatch: %d != %d", count, countShouldBe)
			}
		}()

		// check any errors (blocks till consumer is done)
		for err := range errChan {
			if err != nil {
				t.Error(err.Error())
			}
		}

		log.Debugf("%s got pongs", s1.local)
		if (len(peersnp) - 1) != len(got) {
			t.Errorf("got (%d) less messages than sent (%d).", len(got), len(peersnp))
		}

		for p, n := range got {
			if n != MsgNum {
				t.Error("peer did not get all msgs", p, n, "/", MsgNum)
			}
		}

		cancel()
		<-time.After(10 * time.Millisecond)
	}

	for _, s := range swarms {
		s.Close()
	}
}

func TestSwarm(t *testing.T) {
	// t.Skip("skipping for another test")

	// msgs := 1000
	msgs := 100
	swarms := 5
	SubtestSwarm(t, swarms, msgs)
}

func TestConnHandler(t *testing.T) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms, peersnp := makeSwarms(ctx, t, 5)

	gotconn := make(chan struct{}, 10)
	swarms[0].SetConnHandler(func(conn *Conn) {
		gotconn <- struct{}{}
	})

	connectSwarms(t, ctx, swarms, peersnp)

	<-time.After(time.Millisecond)
	// should've gotten 5 by now.

	swarms[0].SetConnHandler(nil)

	expect := 4
	for i := 0; i < expect; i++ {
		select {
		case <-time.After(time.Second):
			t.Fatal("failed to get connections")
		case <-gotconn:
		}
	}

	select {
	case <-gotconn:
		t.Fatalf("should have connected to %d swarms", expect)
	default:
	}
}
