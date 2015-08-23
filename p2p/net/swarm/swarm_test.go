package swarm

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	metrics "github.com/ipfs/go-ipfs/metrics"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	testutil "github.com/ipfs/go-ipfs/util/testutil"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

func EchoStreamHandler(stream inet.Stream) {
	go func() {
		defer stream.Close()

		// pull out the ipfs conn
		c := stream.Conn()
		log.Infof("%s ponging to %s", c.LocalPeer(), c.RemotePeer())

		buf := make([]byte, 4)

		for {
			if _, err := stream.Read(buf); err != nil {
				if err != io.EOF {
					log.Info("ping receive error:", err)
				}
				return
			}

			if !bytes.Equal(buf, []byte("ping")) {
				log.Infof("ping receive error: ping != %s %v", buf, buf)
				return
			}

			if _, err := stream.Write([]byte("pong")); err != nil {
				log.Info("pond send error:", err)
				return
			}
		}
	}()
}

func makeSwarms(ctx context.Context, t *testing.T, num int) []*Swarm {
	swarms := make([]*Swarm, 0, num)

	for i := 0; i < num; i++ {
		localnp := testutil.RandPeerNetParamsOrFatal(t)

		peerstore := peer.NewPeerstore()
		peerstore.AddPubKey(localnp.ID, localnp.PubKey)
		peerstore.AddPrivKey(localnp.ID, localnp.PrivKey)

		addrs := []ma.Multiaddr{localnp.Addr}
		swarm, err := NewSwarm(ctx, addrs, localnp.ID, peerstore, metrics.NewBandwidthCounter())
		if err != nil {
			t.Fatal(err)
		}

		swarm.SetStreamHandler(EchoStreamHandler)
		swarms = append(swarms, swarm)
	}

	return swarms
}

func connectSwarms(t *testing.T, ctx context.Context, swarms []*Swarm) {

	var wg sync.WaitGroup
	connect := func(s *Swarm, dst peer.ID, addr ma.Multiaddr) {
		// TODO: make a DialAddr func.
		s.peers.AddAddr(dst, addr, peer.PermanentAddrTTL)
		if _, err := s.Dial(ctx, dst); err != nil {
			t.Fatal("error swarm dialing to peer", err)
		}
		wg.Done()
	}

	log.Info("Connecting swarms simultaneously.")
	for _, s1 := range swarms {
		for _, s2 := range swarms {
			if s2.local != s1.local { // don't connect to self.
				wg.Add(1)
				connect(s1, s2.LocalPeer(), s2.ListenAddresses()[0]) // try the first.
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
	swarms := makeSwarms(ctx, t, SwarmNum)

	// connect everyone
	connectSwarms(t, ctx, swarms)

	// ping/pong
	for _, s1 := range swarms {
		log.Debugf("-------------------------------------------------------")
		log.Debugf("%s ping pong round", s1.local)
		log.Debugf("-------------------------------------------------------")

		_, cancel := context.WithCancel(ctx)
		got := map[peer.ID]int{}
		errChan := make(chan error, MsgNum*len(swarms))
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
					errChan <- err
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

			for _, s2 := range swarms {
				if s2.local == s1.local {
					continue // dont send to self...
				}

				wg.Add(1)
				go send(s2.local)
			}
			wg.Wait()
		}()

		// receive "pong" x MsgNum from every peer
		go func() {
			defer close(errChan)
			count := 0
			countShouldBe := MsgNum * (len(swarms) - 1)
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
						errChan <- err
						continue
					}

					if string(msg) != "pong" {
						errChan <- fmt.Errorf("unexpected message: %s", msg)
						continue
					}

					log.Debugf("%s %s %s (%d)", s1.local, msg, p, k)
					msgCount++
				}

				got[p] = msgCount
				count += msgCount
			}

			if count != countShouldBe {
				errChan <- fmt.Errorf("count mismatch: %d != %d", count, countShouldBe)
			}
		}()

		// check any errors (blocks till consumer is done)
		for err := range errChan {
			if err != nil {
				t.Error(err.Error())
			}
		}

		log.Debugf("%s got pongs", s1.local)
		if (len(swarms) - 1) != len(got) {
			t.Errorf("got (%d) less messages than sent (%d).", len(got), len(swarms))
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
	t.Parallel()

	// msgs := 1000
	msgs := 100
	swarms := 5
	SubtestSwarm(t, swarms, msgs)
}

func TestConnHandler(t *testing.T) {
	// t.Skip("skipping for another test")
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 5)

	gotconn := make(chan struct{}, 10)
	swarms[0].SetConnHandler(func(conn *Conn) {
		gotconn <- struct{}{}
	})

	connectSwarms(t, ctx, swarms)

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

func TestAddrBlocking(t *testing.T) {
	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	swarms[0].SetConnHandler(func(conn *Conn) {
		t.Fatal("no connections should happen!")
	})

	_, block, err := net.ParseCIDR("127.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}

	swarms[1].Filters.AddDialFilter(block)

	swarms[1].peers.AddAddr(swarms[0].LocalPeer(), swarms[0].ListenAddresses()[0], peer.PermanentAddrTTL)
	_, err = swarms[1].Dial(ctx, swarms[0].LocalPeer())
	if err == nil {
		t.Fatal("dial should have failed")
	}

	swarms[0].peers.AddAddr(swarms[1].LocalPeer(), swarms[1].ListenAddresses()[0], peer.PermanentAddrTTL)
	_, err = swarms[0].Dial(ctx, swarms[1].LocalPeer())
	if err == nil {
		t.Fatal("dial should have failed")
	}
}

func TestFilterBounds(t *testing.T) {
	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 2)

	conns := make(chan struct{}, 8)
	swarms[0].SetConnHandler(func(conn *Conn) {
		conns <- struct{}{}
	})

	// Address that we wont be dialing from
	_, block, err := net.ParseCIDR("192.0.0.1/8")
	if err != nil {
		t.Fatal(err)
	}

	// set filter on both sides, shouldnt matter
	swarms[1].Filters.AddDialFilter(block)
	swarms[0].Filters.AddDialFilter(block)

	connectSwarms(t, ctx, swarms)

	select {
	case <-time.After(time.Second):
		t.Fatal("should have gotten connection")
	case <-conns:
		t.Log("got connect")
	}
}
