package mux

import (
	"bytes"
	"testing"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	msg "github.com/jbenet/go-ipfs/net/message"
	pb "github.com/jbenet/go-ipfs/net/mux/internal/pb"
	peer "github.com/jbenet/go-ipfs/peer"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

type TestProtocol struct {
	mux *Muxer
	pid pb.ProtocolID
	msg []*msg.Packet
}

func (t *TestProtocol) ProtocolID() pb.ProtocolID {
	return t.pid
}

func (t *TestProtocol) Address() router.Address {
	return t.pid
}

func (t *TestProtocol) HandlePacket(p router.Packet, from router.Node) error {
	pkt, ok := p.(*msg.Packet)
	if !ok {
		return msg.ErrInvalidPayload
	}

	log.Debugf("TestProtocol %d got: %v", t, p)
	if from == t.mux {
		t.msg = append(t.msg, pkt)
		return nil
	}
	return t.mux.HandlePacket(p, t)
}

func newPeer(t *testing.T, id string) peer.Peer {
	mh, err := mh.FromHexString(id)
	if err != nil {
		t.Error(err)
		return nil
	}

	return testutil.NewPeerWithID(peer.ID(mh))
}

func testMsg(t *testing.T, m *msg.Packet, data []byte) {
	if !bytes.Equal(data, m.Data) {
		t.Errorf("Data does not match: %v != %v", data, m.Data)
	}
}

func testWrappedMsg(t *testing.T, m *msg.Packet, pid pb.ProtocolID, data []byte) {
	data2, pid2, err := unwrapData(m.Data)
	if err != nil {
		t.Error(err)
	}

	if pid != pid2 {
		t.Errorf("ProtocolIDs do not match: %v != %v", pid, pid2)
	}

	if !bytes.Equal(data, data2) {
		t.Errorf("Data does not match: %v != %v", data, data2)
	}
}

func TestSimpleMuxer(t *testing.T) {
	// setup
	peer1 := newPeer(t, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275aaaaaa")
	peer2 := newPeer(t, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275bbbbbb")

	uplink := router.NewQueueNode("queue", make(chan router.Packet, 10))
	mux1 := NewMuxer(string(peer1.ID()), uplink)

	pid1 := pb.ProtocolID_Test
	pid2 := pb.ProtocolID_Routing
	p1 := &TestProtocol{mux1, pid1, nil}
	p2 := &TestProtocol{mux1, pid2, nil}
	mux1.AddProtocol(p1, pid1)
	mux1.AddProtocol(p2, pid2)

	// test outgoing p1
	for _, s := range []string{"foo", "bar", "baz"} {

		pkt := msg.Packet{Src: peer1, Dst: peer2, Data: []byte(s)}
		if err := p1.HandlePacket(&pkt, nil); err != nil {
			t.Fatal(err)
		}
		testWrappedMsg(t, (<-uplink.Queue()).(*msg.Packet), pid1, []byte(s))
	}

	// test incoming p1
	for i, s := range []string{"foo", "bar", "baz"} {
		d, err := wrapData([]byte(s), pid1)
		if err != nil {
			t.Error(err)
		}

		pkt := msg.Packet{Src: peer1, Dst: peer2, Data: d}
		if err := mux1.HandlePacket(&pkt, uplink); err != nil {
			t.Fatal(err)
		}
		testMsg(t, p1.msg[i], []byte(s))
	}

	// test outgoing p2
	for _, s := range []string{"foo", "bar", "baz"} {

		pkt := msg.Packet{Src: peer1, Dst: peer2, Data: []byte(s)}
		if err := p2.HandlePacket(&pkt, nil); err != nil {
			t.Fatal(err)
		}
		testWrappedMsg(t, (<-uplink.Queue()).(*msg.Packet), pid2, []byte(s))
	}

	// test incoming p2
	for i, s := range []string{"foo", "bar", "baz"} {
		d, err := wrapData([]byte(s), pid2)
		if err != nil {
			t.Fatal(err)
		}

		pkt := msg.Packet{Src: peer1, Dst: peer2, Data: d}
		if err := mux1.HandlePacket(&pkt, uplink); err != nil {
			t.Fatal(err)
		}
		testMsg(t, p2.msg[i], []byte(s))
	}
}
