package relay_test

import (
	"io"
	"testing"

	inet "github.com/jbenet/go-ipfs/p2p/net"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
	relay "github.com/jbenet/go-ipfs/p2p/protocol/relay"
	testutil "github.com/jbenet/go-ipfs/p2p/test/util"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

var log = eventlog.Logger("relay_test")

func TestRelaySimple(t *testing.T) {

	ctx := context.Background()

	// these networks have the relay service wired in already.
	n1 := testutil.GenHostSwarm(t, ctx)
	n2 := testutil.GenHostSwarm(t, ctx)
	n3 := testutil.GenHostSwarm(t, ctx)

	n1p := n1.ID()
	n2p := n2.ID()
	n3p := n3.ID()

	n2pi := n2.Peerstore().PeerInfo(n2p)
	if err := n1.Connect(ctx, n2pi); err != nil {
		t.Fatal("Failed to connect:", err)
	}
	if err := n3.Connect(ctx, n2pi); err != nil {
		t.Fatal("Failed to connect:", err)
	}

	// setup handler on n3 to copy everything over to the pipe.
	piper, pipew := io.Pipe()
	n3.SetStreamHandler(protocol.TestingID, func(s inet.Stream) {
		log.Debug("relay stream opened to n3!")
		log.Debug("piping and echoing everything")
		w := io.MultiWriter(s, pipew)
		io.Copy(w, s)
		log.Debug("closing stream")
		s.Close()
	})

	// ok, now we can try to relay n1--->n2--->n3.
	log.Debug("open relay stream")
	s, err := n1.NewStream(relay.ID, n2p)
	if err != nil {
		t.Fatal(err)
	}

	// ok first thing we write the relay header n1->n3
	log.Debug("write relay header")
	if err := relay.WriteHeader(s, n1p, n3p); err != nil {
		t.Fatal(err)
	}

	// ok now the header's there, we can write the next protocol header.
	log.Debug("write testing header")
	if err := protocol.WriteHeader(s, protocol.TestingID); err != nil {
		t.Fatal(err)
	}

	// okay, now we should be able to write text, and read it out.
	buf1 := []byte("abcdefghij")
	buf2 := make([]byte, 10)
	buf3 := make([]byte, 10)
	log.Debug("write in some text.")
	if _, err := s.Write(buf1); err != nil {
		t.Fatal(err)
	}

	// read it out from the pipe.
	log.Debug("read it out from the pipe.")
	if _, err := io.ReadFull(piper, buf2); err != nil {
		t.Fatal(err)
	}
	if string(buf1) != string(buf2) {
		t.Fatal("should've gotten that text out of the pipe")
	}

	// read it out from the stream (echoed)
	log.Debug("read it out from the stream (echoed).")
	if _, err := io.ReadFull(s, buf3); err != nil {
		t.Fatal(err)
	}
	if string(buf1) != string(buf3) {
		t.Fatal("should've gotten that text out of the stream")
	}

	// sweet. relay works.
	log.Debug("sweet, relay works.")
	s.Close()
}

func TestRelayAcrossFour(t *testing.T) {

	ctx := context.Background()

	// these networks have the relay service wired in already.
	n1 := testutil.GenHostSwarm(t, ctx)
	n2 := testutil.GenHostSwarm(t, ctx)
	n3 := testutil.GenHostSwarm(t, ctx)
	n4 := testutil.GenHostSwarm(t, ctx)
	n5 := testutil.GenHostSwarm(t, ctx)

	n1p := n1.ID()
	n2p := n2.ID()
	n3p := n3.ID()
	n4p := n4.ID()
	n5p := n5.ID()

	n2pi := n2.Peerstore().PeerInfo(n2p)
	n4pi := n4.Peerstore().PeerInfo(n4p)

	if err := n1.Connect(ctx, n2pi); err != nil {
		t.Fatalf("Failed to dial:", err)
	}
	if err := n3.Connect(ctx, n2pi); err != nil {
		t.Fatalf("Failed to dial:", err)
	}
	if err := n3.Connect(ctx, n4pi); err != nil {
		t.Fatalf("Failed to dial:", err)
	}
	if err := n5.Connect(ctx, n4pi); err != nil {
		t.Fatalf("Failed to dial:", err)
	}

	// setup handler on n5 to copy everything over to the pipe.
	piper, pipew := io.Pipe()
	n5.SetStreamHandler(protocol.TestingID, func(s inet.Stream) {
		log.Debug("relay stream opened to n5!")
		log.Debug("piping and echoing everything")
		w := io.MultiWriter(s, pipew)
		io.Copy(w, s)
		log.Debug("closing stream")
		s.Close()
	})

	// ok, now we can try to relay n1--->n2--->n3--->n4--->n5
	log.Debug("open relay stream")
	s, err := n1.NewStream(relay.ID, n2p)
	if err != nil {
		t.Fatal(err)
	}

	log.Debugf("write relay header n1->n3 (%s -> %s)", n1p, n3p)
	if err := relay.WriteHeader(s, n1p, n3p); err != nil {
		t.Fatal(err)
	}

	log.Debugf("write relay header n1->n4 (%s -> %s)", n1p, n4p)
	if err := protocol.WriteHeader(s, relay.ID); err != nil {
		t.Fatal(err)
	}
	if err := relay.WriteHeader(s, n1p, n4p); err != nil {
		t.Fatal(err)
	}

	log.Debugf("write relay header n1->n5 (%s -> %s)", n1p, n5p)
	if err := protocol.WriteHeader(s, relay.ID); err != nil {
		t.Fatal(err)
	}
	if err := relay.WriteHeader(s, n1p, n5p); err != nil {
		t.Fatal(err)
	}

	// ok now the header's there, we can write the next protocol header.
	log.Debug("write testing header")
	if err := protocol.WriteHeader(s, protocol.TestingID); err != nil {
		t.Fatal(err)
	}

	// okay, now we should be able to write text, and read it out.
	buf1 := []byte("abcdefghij")
	buf2 := make([]byte, 10)
	buf3 := make([]byte, 10)
	log.Debug("write in some text.")
	if _, err := s.Write(buf1); err != nil {
		t.Fatal(err)
	}

	// read it out from the pipe.
	log.Debug("read it out from the pipe.")
	if _, err := io.ReadFull(piper, buf2); err != nil {
		t.Fatal(err)
	}
	if string(buf1) != string(buf2) {
		t.Fatal("should've gotten that text out of the pipe")
	}

	// read it out from the stream (echoed)
	log.Debug("read it out from the stream (echoed).")
	if _, err := io.ReadFull(s, buf3); err != nil {
		t.Fatal(err)
	}
	if string(buf1) != string(buf3) {
		t.Fatal("should've gotten that text out of the stream")
	}

	// sweet. relay works.
	log.Debug("sweet, relaying across 4 works.")
	s.Close()
}

func TestRelayStress(t *testing.T) {
	buflen := 1 << 18
	iterations := 10

	ctx := context.Background()

	// these networks have the relay service wired in already.
	n1 := testutil.GenHostSwarm(t, ctx)
	n2 := testutil.GenHostSwarm(t, ctx)
	n3 := testutil.GenHostSwarm(t, ctx)

	n1p := n1.ID()
	n2p := n2.ID()
	n3p := n3.ID()

	n2pi := n2.Peerstore().PeerInfo(n2p)
	if err := n1.Connect(ctx, n2pi); err != nil {
		t.Fatalf("Failed to dial:", err)
	}
	if err := n3.Connect(ctx, n2pi); err != nil {
		t.Fatalf("Failed to dial:", err)
	}

	// setup handler on n3 to copy everything over to the pipe.
	piper, pipew := io.Pipe()
	n3.SetStreamHandler(protocol.TestingID, func(s inet.Stream) {
		log.Debug("relay stream opened to n3!")
		log.Debug("piping and echoing everything")
		w := io.MultiWriter(s, pipew)
		io.Copy(w, s)
		log.Debug("closing stream")
		s.Close()
	})

	// ok, now we can try to relay n1--->n2--->n3.
	log.Debug("open relay stream")
	s, err := n1.NewStream(relay.ID, n2p)
	if err != nil {
		t.Fatal(err)
	}

	// ok first thing we write the relay header n1->n3
	log.Debug("write relay header")
	if err := relay.WriteHeader(s, n1p, n3p); err != nil {
		t.Fatal(err)
	}

	// ok now the header's there, we can write the next protocol header.
	log.Debug("write testing header")
	if err := protocol.WriteHeader(s, protocol.TestingID); err != nil {
		t.Fatal(err)
	}

	// okay, now write lots of text and read it back out from both
	// the pipe and the stream.
	buf1 := make([]byte, buflen)
	buf2 := make([]byte, len(buf1))
	buf3 := make([]byte, len(buf1))

	fillbuf := func(buf []byte, b byte) {
		for i := range buf {
			buf[i] = b
		}
	}

	for i := 0; i < iterations; i++ {
		fillbuf(buf1, byte(int('a')+i))
		log.Debugf("writing %d bytes (%d/%d)", len(buf1), i, iterations)
		if _, err := s.Write(buf1); err != nil {
			t.Fatal(err)
		}

		log.Debug("read it out from the pipe.")
		if _, err := io.ReadFull(piper, buf2); err != nil {
			t.Fatal(err)
		}
		if string(buf1) != string(buf2) {
			t.Fatal("should've gotten that text out of the pipe")
		}

		// read it out from the stream (echoed)
		log.Debug("read it out from the stream (echoed).")
		if _, err := io.ReadFull(s, buf3); err != nil {
			t.Fatal(err)
		}
		if string(buf1) != string(buf3) {
			t.Fatal("should've gotten that text out of the stream")
		}
	}

	log.Debug("sweet, relay works under stress.")
	s.Close()
}
