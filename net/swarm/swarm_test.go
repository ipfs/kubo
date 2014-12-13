package swarm

import (
	"bytes"
	"sync"
	"testing"
	"time"

	ci "github.com/jbenet/go-ipfs/crypto"
	netmsg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

// needed to copy the data. otherwise gets reused... :/
type queueClient struct {
	addr  router.Address
	queue chan *netmsg.Packet
}

func newQueueClient(addr router.Address) *queueClient {
	return &queueClient{addr, make(chan *netmsg.Packet, 20)}
}

func (qc *queueClient) Address() router.Address {
	return qc.addr
}

func (qc *queueClient) HandlePacket(p router.Packet, n router.Node) error {

	pkt1 := p.(*netmsg.Packet)
	pkt2 := netmsg.Packet{}
	pkt2 = *pkt1
	pkt2.Data = make([]byte, len(pkt1.Data))
	copy(pkt2.Data, pkt1.Data)

	qc.queue <- &pkt2
	return nil
}

type pongClient struct {
	peer  peer.Peer
	count int
	queue chan pongPkt
}

type pongPkt struct {
	msg netmsg.Packet
	dst router.Node
}

func newPongClient(ctx context.Context, peer peer.Peer) *pongClient {
	pc := &pongClient{peer: peer, queue: make(chan pongPkt, 10)}
	go pc.echo(ctx)
	return pc
}

func (pc *pongClient) Address() router.Address {
	return pc.peer.ID().Pretty() + "/pong"
}

func (pc *pongClient) HandlePacket(p router.Packet, n router.Node) error {
	pkt1 := p.(*netmsg.Packet)
	if !bytes.Equal(pkt1.Data, []byte("ping")) {
		log.Debugf("%s pong dropped pkt: %s (%s -> %s)", pc.Address(), pkt1.Data, pkt1.Src, pkt1.Dst)
		panic("why")
		return nil // drop
	}

	pc.queue <- pongPkt{pkt1.Response([]byte("pong")), n}
	return nil
}

func (pc *pongClient) echo(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case pkt := <-pc.queue:
			pc.count++
			log.Debugf("%s pong %s (%d)", pkt.msg.Src, pkt.msg.Dst, pc.count)
			if err := pkt.dst.HandlePacket(&pkt.msg, pc); err != nil {
				log.Errorf("pong error sending: %s", err)
			}
		}
	}
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
		pong := newPongClient(ctx, local)
		swarm, err := NewSwarm(ctx, local.Addresses(), local, peerstore, pong)
		if err != nil {
			t.Fatal(err)
		}
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
			if _, err := s.Dial(cp); err != nil {
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
			log.Infof("%s swarm routing table: %s", s.local, s.GetPeerList())
		}
	}

	// ping/pong
	for _, s1 := range swarms {
		log.Debugf("-------------------------------------------------------")
		log.Debugf("%s ping pong round", s1.local)
		log.Debugf("-------------------------------------------------------")

		// for this test, we'll listen on s1.
		queue := newQueueClient(s1.client().Address())
		pong := s1.client() // set it back at the end.
		s1.SetClient(queue)

		ctx, cancel := context.WithCancel(ctx)
		peers, err := s1.peers.All()
		if err != nil {
			t.Fatal(err)
		}

		for k := 0; k < MsgNum; k++ {
			for _, p := range *peers {
				log.Debugf("%s ping %s (%d)", s1.local, p, k)
				pkt := netmsg.Packet{Src: s1.local, Dst: p, Data: []byte("ping"), Context: ctx}
				s1.HandlePacket(&pkt, queue)
			}
		}

		got := map[u.Key]int{}
		for k := 0; k < (MsgNum * len(*peers)); k++ {
			log.Debugf("%s waiting for pong (%d)", s1.local, k)

			msg := <-queue.queue
			if string(msg.Data) != "pong" {
				t.Error("unexpected conn output", string(msg.Data), msg.Data)
			}

			p := msg.Src.(peer.Peer)
			n, _ := got[p.Key()]
			got[p.Key()] = n + 1
		}

		log.Debugf("%s got pongs", s1.local)
		if len(*peers) != len(got) {
			t.Error("got less messages than sent")
		}

		for p, n := range got {
			if n != MsgNum {
				t.Error("peer did not get all msgs", p, n, "/", MsgNum)
			}
		}

		cancel()
		<-time.After(10 * time.Millisecond)
		s1.SetClient(pong)
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
