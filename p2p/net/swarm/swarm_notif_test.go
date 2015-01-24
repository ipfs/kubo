package swarm

import (
	"testing"
	"time"

	inet "github.com/jbenet/go-ipfs/p2p/net"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestNotifications(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	swarms := makeSwarms(ctx, t, 5)
	defer func() {
		for _, s := range swarms {
			s.Close()
		}
	}()

	timeout := 5 * time.Second

	// signup notifs
	notifiees := make([]*netNotifiee, len(swarms))
	for i, swarm := range swarms {
		n := newNetNotifiee()
		swarm.Notify(n)
		notifiees[i] = n
	}

	connectSwarms(t, ctx, swarms)

	<-time.After(time.Millisecond)
	// should've gotten 5 by now.

	// test everyone got the correct connection opened calls
	for i, s := range swarms {
		n := notifiees[i]
		for _, s2 := range swarms {
			if s == s2 {
				continue
			}

			cos := s.ConnectionsToPeer(s2.LocalPeer())
			func() {
				for i := 0; i < len(cos); i++ {
					var c inet.Conn
					select {
					case c = <-n.connected:
					case <-time.After(timeout):
						t.Fatal("timeout")
					}
					for _, c2 := range cos {
						if c == c2 {
							t.Log("got notif for conn", c)
							return
						}
					}
					t.Error("connection not found", c)
				}
			}()
		}
	}

	complement := func(c inet.Conn) (*Swarm, *netNotifiee, *Conn) {
		for i, s := range swarms {
			for _, c2 := range s.Connections() {
				if c.LocalMultiaddr().Equal(c2.RemoteMultiaddr()) &&
					c2.LocalMultiaddr().Equal(c.RemoteMultiaddr()) {
					return s, notifiees[i], c2
				}
			}
		}
		t.Fatal("complementary conn not found", c)
		return nil, nil, nil
	}

	testOCStream := func(n *netNotifiee, s inet.Stream) {
		var s2 inet.Stream
		select {
		case s2 = <-n.openedStream:
			t.Log("got notif for opened stream")
		case <-time.After(timeout):
			t.Fatal("timeout")
		}
		if s != s2 {
			t.Fatal("got incorrect stream", s.Conn(), s2.Conn())
		}

		select {
		case s2 = <-n.closedStream:
			t.Log("got notif for closed stream")
		case <-time.After(timeout):
			t.Fatal("timeout")
		}
		if s != s2 {
			t.Fatal("got incorrect stream", s.Conn(), s2.Conn())
		}
	}

	streams := make(chan inet.Stream)
	for _, s := range swarms {
		s.SetStreamHandler(func(s inet.Stream) {
			streams <- s
			s.Close()
		})
	}

	// open a streams in each conn
	for i, s := range swarms {
		for _, c := range s.Connections() {
			_, n2, _ := complement(c)

			st1, err := c.NewStream()
			if err != nil {
				t.Error(err)
			} else {
				st1.Write([]byte("hello"))
				st1.Close()
				testOCStream(notifiees[i], st1)
				st2 := <-streams
				testOCStream(n2, st2)
			}
		}
	}

	// close conns
	for i, s := range swarms {
		n := notifiees[i]
		for _, c := range s.Connections() {
			_, n2, c2 := complement(c)
			c.Close()
			c2.Close()

			var c3, c4 inet.Conn
			select {
			case c3 = <-n.disconnected:
			case <-time.After(timeout):
				t.Fatal("timeout")
			}
			if c != c3 {
				t.Fatal("got incorrect conn", c, c3)
			}

			select {
			case c4 = <-n2.disconnected:
			case <-time.After(timeout):
				t.Fatal("timeout")
			}
			if c2 != c4 {
				t.Fatal("got incorrect conn", c, c2)
			}
		}
	}
}

type netNotifiee struct {
	connected    chan inet.Conn
	disconnected chan inet.Conn
	openedStream chan inet.Stream
	closedStream chan inet.Stream
}

func newNetNotifiee() *netNotifiee {
	return &netNotifiee{
		connected:    make(chan inet.Conn),
		disconnected: make(chan inet.Conn),
		openedStream: make(chan inet.Stream),
		closedStream: make(chan inet.Stream),
	}
}

func (nn *netNotifiee) Connected(n inet.Network, v inet.Conn) {
	nn.connected <- v
}
func (nn *netNotifiee) Disconnected(n inet.Network, v inet.Conn) {
	nn.disconnected <- v
}
func (nn *netNotifiee) OpenedStream(n inet.Network, v inet.Stream) {
	nn.openedStream <- v
}
func (nn *netNotifiee) ClosedStream(n inet.Network, v inet.Stream) {
	nn.closedStream <- v
}
