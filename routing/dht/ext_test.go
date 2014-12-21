package dht

import (
	"math/rand"
	"testing"

	inet "github.com/jbenet/go-ipfs/net"
	mocknet "github.com/jbenet/go-ipfs/net/mock"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	pb "github.com/jbenet/go-ipfs/routing/dht/pb"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	dssync "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"

	"time"
)

func TestGetFailures(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	nets := mn.Nets()
	peers := mn.Peers()

	tsds := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, peers[0], nets[0], tsds)
	d.Update(ctx, peers[1])

	// This one should time out
	// u.POut("Timout Test\n")
	ctx1, _ := context.WithTimeout(context.Background(), time.Second)
	if _, err := d.GetValue(ctx1, u.Key("test")); err != nil {
		if err != context.DeadlineExceeded {
			t.Fatal("Got different error than we expected", err)
		}
	} else {
		t.Fatal("Did not get expected error!")
	}

	msgs := make(chan *pb.Message, 100)

	// u.POut("NotFound Test\n")
	// Reply with failures to every message
	nets[1].SetHandler(inet.ProtocolDHT, func(s inet.Stream) {
		defer s.Close()

		pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
		pbw := ggio.NewDelimitedWriter(s)

		pmes := new(pb.Message)
		if err := pbr.ReadMsg(pmes); err != nil {
			panic(err)
		}

		resp := &pb.Message{
			Type: pmes.Type,
		}
		if err := pbw.WriteMsg(resp); err != nil {
			panic(err)
		}

		msgs <- resp
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

	// Now we test this DHT's handleGetValue failure
	{
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
		s, err := nets[1].NewStream(inet.ProtocolDHT, peers[0])
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()

		pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
		pbw := ggio.NewDelimitedWriter(s)

		if err := pbw.WriteMsg(&req); err != nil {
			t.Fatal(err)
		}

		pmes := new(pb.Message)
		if err := pbr.ReadMsg(pmes); err != nil {
			t.Fatal(err)
		}
		if pmes.GetRecord() != nil {
			t.Fatal("shouldnt have value")
		}
		if pmes.GetProviderPeers() != nil {
			t.Fatal("shouldnt have provider peers")
		}
	}
}

func TestNotFound(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 16)
	if err != nil {
		t.Fatal(err)
	}
	nets := mn.Nets()
	peers := mn.Peers()
	tsds := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, peers[0], nets[0], tsds)

	for _, p := range peers {
		d.Update(ctx, p)
	}

	// Reply with random peers to every message
	for _, neti := range nets {
		neti := neti // shadow loop var
		neti.SetHandler(inet.ProtocolDHT, func(s inet.Stream) {
			defer s.Close()

			pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
			pbw := ggio.NewDelimitedWriter(s)

			pmes := new(pb.Message)
			if err := pbr.ReadMsg(pmes); err != nil {
				panic(err)
			}

			switch pmes.GetType() {
			case pb.Message_GET_VALUE:
				resp := &pb.Message{Type: pmes.Type}

				ps := []peer.PeerInfo{}
				for i := 0; i < 7; i++ {
					p := peers[rand.Intn(len(peers))]
					pi := neti.Peerstore().PeerInfo(p)
					ps = append(ps, pi)
				}

				resp.CloserPeers = pb.PeerInfosToPBPeers(d.network, ps)
				if err := pbw.WriteMsg(resp); err != nil {
					panic(err)
				}

			default:
				panic("Shouldnt recieve this.")
			}
		})
	}

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
	mn, err := mocknet.FullMeshConnected(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	nets := mn.Nets()
	peers := mn.Peers()

	tsds := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, peers[0], nets[0], tsds)

	for i := 1; i < 5; i++ {
		d.Update(ctx, peers[i])
	}

	// Reply with random peers to every message
	for _, neti := range nets {
		neti := neti // shadow loop var
		neti.SetHandler(inet.ProtocolDHT, func(s inet.Stream) {
			defer s.Close()

			pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
			pbw := ggio.NewDelimitedWriter(s)

			pmes := new(pb.Message)
			if err := pbr.ReadMsg(pmes); err != nil {
				panic(err)
			}

			switch pmes.GetType() {
			case pb.Message_GET_VALUE:
				pi := neti.Peerstore().PeerInfo(peers[1])
				resp := &pb.Message{
					Type:        pmes.Type,
					CloserPeers: pb.PeerInfosToPBPeers(d.network, []peer.PeerInfo{pi}),
				}

				if err := pbw.WriteMsg(resp); err != nil {
					panic(err)
				}
			default:
				panic("Shouldnt recieve this.")
			}

		})
	}

	ctx, _ = context.WithTimeout(ctx, time.Second*30)
	if _, err := d.GetValue(ctx, u.Key("hello")); err != nil {
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
