package service

import (
	"bytes"
	"testing"
	"time"

	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
)

// ReverseHandler reverses all Data it receives and sends it back.
type ReverseHandler struct{}

func (t *ReverseHandler) HandleMessage(ctx context.Context, m msg.NetMessage) msg.NetMessage {

	d := m.Data()
	for i, j := 0, len(d)-1; i < j; i, j = i+1, j-1 {
		d[i], d[j] = d[j], d[i]
	}

	return msg.New(m.Peer(), d)
}

func newPeer(t *testing.T, id string) *peer.Peer {
	mh, err := mh.FromHexString(id)
	if err != nil {
		t.Error(err)
		return nil
	}

	return &peer.Peer{ID: peer.ID(mh)}
}

func TestServiceHandler(t *testing.T) {
	ctx := context.Background()
	h := &ReverseHandler{}
	s := NewService(h)
	peer1 := newPeer(t, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275aaaaaa")

	if err := s.Start(ctx); err != nil {
		t.Error(err)
	}

	d, err := wrapData([]byte("beep"), nil)
	if err != nil {
		t.Error(err)
	}

	m1 := msg.New(peer1, d)
	s.GetPipe().Incoming <- m1
	m2 := <-s.GetPipe().Outgoing

	d, rid, err := unwrapData(m2.Data())
	if err != nil {
		t.Error(err)
	}

	if rid != nil {
		t.Error("RequestID should be nil")
	}

	if !bytes.Equal(d, []byte("peeb")) {
		t.Errorf("service handler data incorrect: %v != %v", d, "oof")
	}
}

func TestServiceRequest(t *testing.T) {
	ctx := context.Background()
	s1 := NewService(&ReverseHandler{})
	s2 := NewService(&ReverseHandler{})

	if err := s1.Start(ctx); err != nil {
		t.Error(err)
	}

	if err := s2.Start(ctx); err != nil {
		t.Error(err)
	}

	peer1 := newPeer(t, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275aaaaaa")

	// patch services together
	go func() {
		for {
			select {
			case m := <-s1.GetPipe().Outgoing:
				s2.GetPipe().Incoming <- m
			case m := <-s2.GetPipe().Outgoing:
				s1.GetPipe().Incoming <- m
			case <-ctx.Done():
				return
			}
		}
	}()

	m1 := msg.New(peer1, []byte("beep"))
	m2, err := s1.SendRequest(ctx, m1)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(m2.Data(), []byte("peeb")) {
		t.Errorf("service handler data incorrect: %v != %v", m2.Data(), "oof")
	}
}

func TestServiceRequestTimeout(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Millisecond)
	s1 := NewService(&ReverseHandler{})
	s2 := NewService(&ReverseHandler{})
	peer1 := newPeer(t, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275aaaaaa")

	if err := s1.Start(ctx); err != nil {
		t.Error(err)
	}

	if err := s2.Start(ctx); err != nil {
		t.Error(err)
	}

	// patch services together
	go func() {
		for {
			<-time.After(time.Millisecond)
			select {
			case m := <-s1.GetPipe().Outgoing:
				s2.GetPipe().Incoming <- m
			case m := <-s2.GetPipe().Outgoing:
				s1.GetPipe().Incoming <- m
			case <-ctx.Done():
				return
			}
		}
	}()

	m1 := msg.New(peer1, []byte("beep"))
	m2, err := s1.SendRequest(ctx, m1)
	if err == nil || m2 != nil {
		t.Error("should've timed out")
	}
}
