package swarm

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func EchoStreamHandler(stream *Stream) {
	go func() {
		defer stream.Close()

		// pull out the ipfs conn
		c := stream.Conn().RawConn()
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

func setupPeer(t *testing.T, addr string) peer.Peer {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}

	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	p, err := testutil.NewPeerWithKeyPair(sk, pk)
	if err != nil {
		t.Fatal(err)
	}
	p.AddAddress(tcp)
	return p
}

func makeSwarms(ctx context.Context, t *testing.T, addrs []string) ([]*Swarm, []peer.Peer) {
	swarms := []*Swarm{}

	for _, addr := range addrs {
		local := setupPeer(t, addr)
		peerstore := peer.NewPeerstore()
		swarm, err := NewSwarm(ctx, local.Addresses(), local, peerstore)
		if err != nil {
			t.Fatal(err)
		}
		swarm.SetStreamHandler(EchoStreamHandler)
		swarms = append(swarms, swarm)
	}

	peers := make([]peer.Peer, len(swarms))
	for i, s := range swarms {
		peers[i] = s.local
	}

	return swarms, peers
}

func SubtestSwarm(t *testing.T, addrs []string, MsgNum int) {
	// t.Skip("skipping for another test")

	ctx := context.Background()
	swarms, peers := makeSwarms(ctx, t, addrs)

	// connect everyone
	{
		var wg sync.WaitGroup
		connect := func(s *Swarm, dst peer.Peer) {
			// copy for other peer

			cp, err := s.peers.FindOrCreate(dst.ID())
			if err != nil {
				t.Fatal(err)
			}
			cp.AddAddress(dst.Addresses()[0])

			log.Infof("SWARM TEST: %s dialing %s", s.local, dst)
			if _, err := s.Dial(ctx, cp); err != nil {
				t.Fatal("error swarm dialing to peer", err)
			}
			log.Infof("SWARM TEST: %s connected to %s", s.local, dst)
			wg.Done()
		}

		log.Info("Connecting swarms simultaneously.")
		for _, s := range swarms {
			for _, p := range peers {
				if p != s.local { // don't connect to self.
					wg.Add(1)
					connect(s, p)
				}
			}
		}
		wg.Wait()

		for _, s := range swarms {
			log.Infof("%s swarm routing table: %s", s.local, s.Peers())
		}
	}

	// ping/pong
	for _, s1 := range swarms {
		log.Debugf("-------------------------------------------------------")
		log.Debugf("%s ping pong round", s1.local)
		log.Debugf("-------------------------------------------------------")

		_, cancel := context.WithCancel(ctx)
		peers, err := s1.peers.All()
		if err != nil {
			t.Fatal(err)
		}

		got := map[u.Key]int{}
		errChan := make(chan error, MsgNum*len(*peers))
		streamChan := make(chan *Stream, MsgNum)

		// send out "ping" x MsgNum to every peer
		go func() {
			defer close(streamChan)

			var wg sync.WaitGroup
			send := func(p peer.Peer) {
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
					stream.Write([]byte(msg))
				}

				// read it later
				streamChan <- stream
			}

			for _, p := range *peers {
				if p == s1.local {
					continue // dont send to self...
				}

				wg.Add(1)
				go send(p)
			}
			wg.Wait()
		}()

		// receive "pong" x MsgNum from every peer
		go func() {
			defer close(errChan)
			count := 0
			countShouldBe := MsgNum * (len(*peers) - 1)
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

				got[p.Key()] = msgCount
				count += msgCount
			}

			if count != countShouldBe {
				errChan <- errors.Errorf("count mismatch: %d != %d", count, countShouldBe)
			}
		}()

		// check any errors (blocks till consumer is done)
		for err := range errChan {
			if err != nil {
				t.Fatal(err.Error())
			}
		}

		log.Debugf("%s got pongs", s1.local)
		if (len(*peers) - 1) != len(got) {
			t.Error("got less messages than sent")
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

	addrs := []string{
		"/ip4/127.0.0.1/tcp/10234",
		"/ip4/127.0.0.1/tcp/10235",
		"/ip4/127.0.0.1/tcp/10236",
		"/ip4/127.0.0.1/tcp/10237",
		"/ip4/127.0.0.1/tcp/10238",
	}

	// msgs := 1000
	msgs := 100
	SubtestSwarm(t, addrs, msgs)
}
