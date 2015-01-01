package basichost_test

import (
	"bytes"
	"io"
	"testing"

	inet "github.com/jbenet/go-ipfs/p2p/net"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
	testutil "github.com/jbenet/go-ipfs/p2p/test/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func TestHostSimple(t *testing.T) {

	ctx := context.Background()
	h1 := testutil.GenHostSwarm(t, ctx)
	h2 := testutil.GenHostSwarm(t, ctx)
	defer h1.Close()
	defer h2.Close()

	h2pi := h2.Peerstore().PeerInfo(h2.ID())
	if err := h1.Connect(ctx, h2pi); err != nil {
		t.Fatal(err)
	}

	piper, pipew := io.Pipe()
	h2.SetStreamHandler(protocol.TestingID, func(s inet.Stream) {
		defer s.Close()
		w := io.MultiWriter(s, pipew)
		io.Copy(w, s) // mirror everything
	})

	s, err := h1.NewStream(protocol.TestingID, h2pi.ID)
	if err != nil {
		t.Fatal(err)
	}

	// write to the stream
	buf1 := []byte("abcdefghijkl")
	if _, err := s.Write(buf1); err != nil {
		t.Fatal(err)
	}

	// get it from the stream (echoed)
	buf2 := make([]byte, len(buf1))
	if _, err := io.ReadFull(s, buf2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf1, buf2) {
		t.Fatal("buf1 != buf2 -- %x != %x", buf1, buf2)
	}

	// get it from the pipe (tee)
	buf3 := make([]byte, len(buf1))
	if _, err := io.ReadFull(piper, buf3); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf1, buf3) {
		t.Fatal("buf1 != buf3 -- %x != %x", buf1, buf3)
	}
}
