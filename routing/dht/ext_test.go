package dht

import (
	"testing"

	crand "crypto/rand"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	msg "github.com/jbenet/go-ipfs/net/message"
	mux "github.com/jbenet/go-ipfs/net/mux"
	peer "github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"

	"sync"
	"time"
)

// mesHandleFunc is a function that takes in outgoing messages
// and can respond to them, simulating other peers on the network.
// returning nil will chose not to respond and pass the message onto the
// next registered handler
type mesHandleFunc func(msg.NetMessage) msg.NetMessage

// fauxNet is a standin for a swarm.Network in order to more easily recreate
// different testing scenarios
type fauxSender struct {
	sync.Mutex
	handlers []mesHandleFunc
}

func (f *fauxSender) AddHandler(fn func(msg.NetMessage) msg.NetMessage) {
	f.Lock()
	defer f.Unlock()

	f.handlers = append(f.handlers, fn)
}

func (f *fauxSender) SendRequest(ctx context.Context, m msg.NetMessage) (msg.NetMessage, error) {
	f.Lock()
	handlers := make([]mesHandleFunc, len(f.handlers))
	copy(handlers, f.handlers)
	f.Unlock()

	for _, h := range handlers {
		reply := h(m)
		if reply != nil {
			return reply, nil
		}
	}

	// no reply? ok force a timeout
	select {
	case <-ctx.Done():
	}

	return nil, ctx.Err()
}

func (f *fauxSender) SendMessage(ctx context.Context, m msg.NetMessage) error {
	f.Lock()
	handlers := make([]mesHandleFunc, len(f.handlers))
	copy(handlers, f.handlers)
	f.Unlock()

	for _, h := range handlers {
		reply := h(m)
		if reply != nil {
			return nil
		}
	}
	return nil
}

// fauxNet is a standin for a swarm.Network in order to more easily recreate
// different testing scenarios
type fauxNet struct {
}

// DialPeer attempts to establish a connection to a given peer
func (f *fauxNet) DialPeer(context.Context, peer.Peer) error {
	return nil
}

// ClosePeer connection to peer
func (f *fauxNet) ClosePeer(peer.Peer) error {
	return nil
}

// IsConnected returns whether a connection to given peer exists.
func (f *fauxNet) IsConnected(peer.Peer) (bool, error) {
	return true, nil
}

// GetProtocols returns the protocols registered in the network.
func (f *fauxNet) GetProtocols() *mux.ProtocolMap { return nil }

// SendMessage sends given Message out
func (f *fauxNet) SendMessage(msg.NetMessage) error {
	return nil
}

func (f *fauxNet) GetPeerList() []peer.Peer {
	return nil
}

func (f *fauxNet) GetBandwidthTotals() (uint64, uint64) {
	return 0, 0
}

// Close terminates all network operation
func (f *fauxNet) Close() error { return nil }

func TestGetFailures(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	fn := &fauxNet{}
	fs := &fauxSender{}

	peerstore := peer.NewPeerstore()
	local := makePeer(nil)

	d := NewDHT(ctx, local, peerstore, fn, fs, ds.NewMapDatastore())
	other := makePeer(nil)
	d.Update(other)

	// This one should time out
	// u.POut("Timout Test\n")
	ctx1, _ := context.WithTimeout(context.Background(), time.Second)
	_, err := d.GetValue(ctx1, u.Key("test"))
	if err != nil {
		if err != context.DeadlineExceeded {
			t.Fatal("Got different error than we expected", err)
		}
	} else {
		t.Fatal("Did not get expected error!")
	}

	// u.POut("NotFound Test\n")
	// Reply with failures to every message
	fs.AddHandler(func(mes msg.NetMessage) msg.NetMessage {
		pmes := new(pb.Message)
		err := proto.Unmarshal(mes.Data(), pmes)
		if err != nil {
			t.Fatal(err)
		}

		resp := &pb.Message{
			Type: pmes.Type,
		}
		m, err := msg.FromObject(mes.Peer(), resp)
		return m
	})

	// This one should fail with NotFound
	ctx2, _ := context.WithTimeout(context.Background(), time.Second)
	_, err = d.GetValue(ctx2, u.Key("test"))
	if err != nil {
		if err != routing.ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %s", err)
		}
	} else {
		t.Fatal("expected error, got none.")
	}

	fs.handlers = nil
	// Now we test this DHT's handleGetValue failure
	typ := pb.Message_GET_VALUE
	str := "hello"
	rec, err := d.makePutRecord(u.Key(str), []byte("blah"))
	if err != nil {
		t.Fatal(err)
	}
	req := pb.Message{
		Type:   &typ,
		Key:    &str,
		Record: rec,
	}

	// u.POut("handleGetValue Test\n")
	mes, err := msg.FromObject(other, &req)
	if err != nil {
		t.Error(err)
	}

	mes = d.HandleMessage(ctx, mes)

	pmes := new(pb.Message)
	err = proto.Unmarshal(mes.Data(), pmes)
	if err != nil {
		t.Fatal(err)
	}
	if pmes.GetRecord() != nil {
		t.Fatal("shouldnt have value")
	}
	if pmes.GetProviderPeers() != nil {
		t.Fatal("shouldnt have provider peers")
	}

}

// TODO: Maybe put these in some sort of "ipfs_testutil" package
func _randPeer() peer.Peer {
	id := make(peer.ID, 16)
	crand.Read(id)
	p := peer.WithID(id)
	return p
}

func TestNotFound(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	fn := &fauxNet{}
	fs := &fauxSender{}

	local := makePeer(nil)
	peerstore := peer.NewPeerstore()
	peerstore.Add(local)

	d := NewDHT(ctx, local, peerstore, fn, fs, ds.NewMapDatastore())

	var ps []peer.Peer
	for i := 0; i < 5; i++ {
		ps = append(ps, _randPeer())
		d.Update(ps[i])
	}

	// Reply with random peers to every message
	fs.AddHandler(func(mes msg.NetMessage) msg.NetMessage {
		pmes := new(pb.Message)
		err := proto.Unmarshal(mes.Data(), pmes)
		if err != nil {
			t.Fatal(err)
		}

		switch pmes.GetType() {
		case pb.Message_GET_VALUE:
			resp := &pb.Message{Type: pmes.Type}

			peers := []peer.Peer{}
			for i := 0; i < 7; i++ {
				peers = append(peers, _randPeer())
			}
			resp.CloserPeers = pb.PeersToPBPeers(peers)
			mes, err := msg.FromObject(mes.Peer(), resp)
			if err != nil {
				t.Error(err)
			}
			return mes
		default:
			panic("Shouldnt recieve this.")
		}

	})

	ctx, _ = context.WithTimeout(ctx, time.Second*5)
	v, err := d.GetValue(ctx, u.Key("hello"))
	log.Debugf("get value got %v", v)
	if err != nil {
		switch err {
		case routing.ErrNotFound:
			//Success!
			return
		case u.ErrTimeout:
			t.Fatal("Should not have gotten timeout!")
		default:
			t.Fatalf("Got unexpected error: %s", err)
		}
	}
	t.Fatal("Expected to recieve an error.")
}

// If less than K nodes are in the entire network, it should fail when we make
// a GET rpc and nobody has the value
func TestLessThanKResponses(t *testing.T) {
	// t.Skip("skipping test because it makes a lot of output")

	ctx := context.Background()
	u.Debug = false
	fn := &fauxNet{}
	fs := &fauxSender{}
	local := makePeer(nil)
	peerstore := peer.NewPeerstore()
	peerstore.Add(local)

	d := NewDHT(ctx, local, peerstore, fn, fs, ds.NewMapDatastore())

	var ps []peer.Peer
	for i := 0; i < 5; i++ {
		ps = append(ps, _randPeer())
		d.Update(ps[i])
	}
	other := _randPeer()

	// Reply with random peers to every message
	fs.AddHandler(func(mes msg.NetMessage) msg.NetMessage {
		pmes := new(pb.Message)
		err := proto.Unmarshal(mes.Data(), pmes)
		if err != nil {
			t.Fatal(err)
		}

		switch pmes.GetType() {
		case pb.Message_GET_VALUE:
			resp := &pb.Message{
				Type:        pmes.Type,
				CloserPeers: pb.PeersToPBPeers([]peer.Peer{other}),
			}

			mes, err := msg.FromObject(mes.Peer(), resp)
			if err != nil {
				t.Error(err)
			}
			return mes
		default:
			panic("Shouldnt recieve this.")
		}

	})

	ctx, _ = context.WithTimeout(ctx, time.Second*30)
	_, err := d.GetValue(ctx, u.Key("hello"))
	if err != nil {
		switch err {
		case routing.ErrNotFound:
			//Success!
			return
		case u.ErrTimeout:
			t.Fatal("Should not have gotten timeout!")
		default:
			t.Fatalf("Got unexpected error: %s", err)
		}
	}
	t.Fatal("Expected to recieve an error.")
}
