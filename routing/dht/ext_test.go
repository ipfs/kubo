package dht

import (
	"testing"

	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	swarm "github.com/jbenet/go-ipfs/swarm"
	//ma "github.com/jbenet/go-multiaddr"

	"fmt"
	"time"
)

// fauxNet is a standin for a swarm.Network in order to more easily recreate
// different testing scenarios
type fauxNet struct {
	Chan *swarm.Chan

	swarm.Network

	handlers []mesHandleFunc
}

type mesHandleFunc func(*swarm.Message) *swarm.Message

func newFauxNet() *fauxNet {
	fn := new(fauxNet)
	fn.Chan = swarm.NewChan(8)

	return fn
}

func (f *fauxNet) Listen() error {
	go func() {
		for {
			select {
			case in := <-f.Chan.Outgoing:
				for _,h := range f.handlers {
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

}

func TestGetFailure(t *testing.T) {
	fn := newFauxNet()
	fn.Listen()

	local := new(peer.Peer)
	local.ID = peer.ID([]byte("test_peer"))

	d := NewDHT(local, fn)

	d.Start()

	b, err := d.GetValue(u.Key("test"), time.Second)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(b)
}
