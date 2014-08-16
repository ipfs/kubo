package dht

import (
	"testing"

	"code.google.com/p/goprotobuf/proto"

	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"

	"time"
)

// fauxNet is a standin for a swarm.Network in order to more easily recreate
// different testing scenarios
type fauxNet struct {
	Chan     *swarm.Chan
	handlers []mesHandleFunc

	swarm.Network
}

// mesHandleFunc is a function that takes in outgoing messages
// and can respond to them, simulating other peers on the network.
// returning nil will chose not to respond and pass the message onto the
// next registered handler
type mesHandleFunc func(*swarm.Message) *swarm.Message

func newFauxNet() *fauxNet {
	fn := new(fauxNet)
	fn.Chan = swarm.NewChan(8)

	return fn
}

// Instead of 'Listening' Start up a goroutine that will check
// all outgoing messages against registered message handlers,
// and reply if needed
func (f *fauxNet) Listen() error {
	go func() {
		for {
			select {
			case in := <-f.Chan.Outgoing:
				for _, h := range f.handlers {
					reply := h(in)
					if reply != nil {
						f.Chan.Incoming <- reply
						break
					}
				}
			}
		}
	}()
	return nil
}

func (f *fauxNet) AddHandler(fn func(*swarm.Message) *swarm.Message) {
	f.handlers = append(f.handlers, fn)
}

func (f *fauxNet) Send(mes *swarm.Message) {
	f.Chan.Outgoing <- mes
}

func (f *fauxNet) GetChan() *swarm.Chan {
	return f.Chan
}

func (f *fauxNet) Connect(addr *ma.Multiaddr) (*peer.Peer, error) {
	return nil, nil
}

func TestGetFailures(t *testing.T) {
	fn := newFauxNet()
	fn.Listen()

	local := new(peer.Peer)
	local.ID = peer.ID("test_peer")

	d := NewDHT(local, fn)

	other := &peer.Peer{ID: peer.ID("other_peer")}

	d.Start()

	d.Update(other)

	// This one should time out
	_, err := d.GetValue(u.Key("test"), time.Millisecond*10)
	if err != nil {
		if err != u.ErrTimeout {
			t.Fatal("Got different error than we expected.")
		}
	} else {
		t.Fatal("Did not get expected error!")
	}

	// Reply with failures to every message
	fn.AddHandler(func(mes *swarm.Message) *swarm.Message {
		pmes := new(PBDHTMessage)
		err := proto.Unmarshal(mes.Data, pmes)
		if err != nil {
			t.Fatal(err)
		}

		resp := DHTMessage{
			Type:     pmes.GetType(),
			Id:       pmes.GetId(),
			Response: true,
			Success:  false,
		}
		return swarm.NewMessage(mes.Peer, resp.ToProtobuf())
	})

	// This one should fail with NotFound
	_, err = d.GetValue(u.Key("test"), time.Millisecond*1000)
	if err != nil {
		if err != u.ErrNotFound {
			t.Fatalf("Expected ErrNotFound, got: %s", err)
		}
	} else {
		t.Fatal("expected error, got none.")
	}

	success := make(chan struct{})
	fn.handlers = nil
	fn.AddHandler(func(mes *swarm.Message) *swarm.Message {
		resp := new(PBDHTMessage)
		err := proto.Unmarshal(mes.Data, resp)
		if err != nil {
			t.Fatal(err)
		}
		if resp.GetSuccess() {
			t.Fatal("Get returned success when it shouldnt have.")
		}
		success <- struct{}{}
		return nil
	})

	// Now we test this DHT's handleGetValue failure
	req := DHTMessage{
		Type:  PBDHTMessage_GET_VALUE,
		Key:   "hello",
		Id:    GenerateMessageID(),
		Value: []byte{0},
	}
	fn.Chan.Incoming <- swarm.NewMessage(other, req.ToProtobuf())

	<-success
}
