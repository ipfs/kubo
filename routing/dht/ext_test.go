package dht

import (
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore/sync"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	routing "github.com/ipfs/go-ipfs/routing"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	record "github.com/ipfs/go-ipfs/routing/record"
	inet "gx/ipfs/QmRW2xiYTpDLWTHb822ZYbPBoh3dGLJwaXLGS9tnPyWZpq/go-libp2p/p2p/net"
	mocknet "gx/ipfs/QmRW2xiYTpDLWTHb822ZYbPBoh3dGLJwaXLGS9tnPyWZpq/go-libp2p/p2p/net/mock"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
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
	hosts := mn.Hosts()

	tsds := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, hosts[0], tsds)
	d.Update(ctx, hosts[1].ID())

	// Reply with failures to every message
	hosts[1].SetStreamHandler(ProtocolDHT, func(s inet.Stream) {
		defer s.Close()
		io.Copy(ioutil.Discard, s)
	})

	// This one should time out
	ctx1, _ := context.WithTimeout(context.Background(), 200*time.Millisecond)
	if _, err := d.GetValue(ctx1, key.Key("test")); err != nil {
		if merr, ok := err.(u.MultiErr); ok && len(merr) > 0 {
			err = merr[0]
		}

		if err.Error() != "process closing" {
			t.Fatal("Got different error than we expected", err)
		}
	} else {
		t.Fatal("Did not get expected error!")
	}

	t.Log("Timeout test passed.")

	// Reply with failures to every message
	hosts[1].SetStreamHandler(ProtocolDHT, func(s inet.Stream) {
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
	})

	// This one should fail with NotFound.
	// long context timeout to ensure we dont end too early.
	// the dht should be exhausting its query and returning not found.
	// (was 3 seconds before which should be _plenty_ of time, but maybe
	// travis machines really have a hard time...)
	ctx2, _ := context.WithTimeout(context.Background(), 20*time.Second)
	_, err = d.GetValue(ctx2, key.Key("test"))
	if err != nil {
		if merr, ok := err.(u.MultiErr); ok && len(merr) > 0 {
			err = merr[0]
		}
		if err != routing.ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %s", err)
		}
	} else {
		t.Fatal("expected error, got none.")
	}

	t.Log("ErrNotFound check passed!")

	// Now we test this DHT's handleGetValue failure
	{
		typ := pb.Message_GET_VALUE
		str := "hello"

		sk, err := d.getOwnPrivateKey()
		if err != nil {
			t.Fatal(err)
		}

		rec, err := record.MakePutRecord(sk, key.Key(str), []byte("blah"), true)
		if err != nil {
			t.Fatal(err)
		}
		req := pb.Message{
			Type:   &typ,
			Key:    &str,
			Record: rec,
		}

		s, err := hosts[1].NewStream(context.Background(), ProtocolDHT, hosts[0].ID())
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
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 16)
	if err != nil {
		t.Fatal(err)
	}
	hosts := mn.Hosts()
	tsds := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, hosts[0], tsds)

	for _, p := range hosts {
		d.Update(ctx, p.ID())
	}

	// Reply with random peers to every message
	for _, host := range hosts {
		host := host // shadow loop var
		host.SetStreamHandler(ProtocolDHT, func(s inet.Stream) {
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
					p := hosts[rand.Intn(len(hosts))].ID()
					pi := host.Peerstore().PeerInfo(p)
					ps = append(ps, pi)
				}

				resp.CloserPeers = pb.PeerInfosToPBPeers(d.host.Network(), ps)
				if err := pbw.WriteMsg(resp); err != nil {
					panic(err)
				}

			default:
				panic("Shouldnt recieve this.")
			}
		})
	}

	// long timeout to ensure timing is not at play.
	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()
	v, err := d.GetValue(ctx, key.Key("hello"))
	log.Debugf("get value got %v", v)
	if err != nil {
		if merr, ok := err.(u.MultiErr); ok && len(merr) > 0 {
			err = merr[0]
		}
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
	// t.Skip("skipping test to debug another")
	// t.Skip("skipping test because it makes a lot of output")

	ctx := context.Background()
	mn, err := mocknet.FullMeshConnected(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	hosts := mn.Hosts()

	tsds := dssync.MutexWrap(ds.NewMapDatastore())
	d := NewDHT(ctx, hosts[0], tsds)

	for i := 1; i < 5; i++ {
		d.Update(ctx, hosts[i].ID())
	}

	// Reply with random peers to every message
	for _, host := range hosts {
		host := host // shadow loop var
		host.SetStreamHandler(ProtocolDHT, func(s inet.Stream) {
			defer s.Close()

			pbr := ggio.NewDelimitedReader(s, inet.MessageSizeMax)
			pbw := ggio.NewDelimitedWriter(s)

			pmes := new(pb.Message)
			if err := pbr.ReadMsg(pmes); err != nil {
				panic(err)
			}

			switch pmes.GetType() {
			case pb.Message_GET_VALUE:
				pi := host.Peerstore().PeerInfo(hosts[1].ID())
				resp := &pb.Message{
					Type:        pmes.Type,
					CloserPeers: pb.PeerInfosToPBPeers(d.host.Network(), []peer.PeerInfo{pi}),
				}

				if err := pbw.WriteMsg(resp); err != nil {
					panic(err)
				}
			default:
				panic("Shouldnt recieve this.")
			}

		})
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	if _, err := d.GetValue(ctx, key.Key("hello")); err != nil {
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
