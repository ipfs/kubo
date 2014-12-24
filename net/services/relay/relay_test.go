package relay_test

import (
	"io"
	"testing"

	inet "github.com/jbenet/go-ipfs/net"
	netutil "github.com/jbenet/go-ipfs/net/ipfsnet/util"
	mux "github.com/jbenet/go-ipfs/net/services/mux"
	relay "github.com/jbenet/go-ipfs/net/services/relay"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

var log = eventlog.Logger("relay_test")

func TestRelaySimple(t *testing.T) {

	ctx := context.Background()

	// these networks have the relay service wired in already.
	n1 := netutil.GenNetwork(t, ctx)
	n2 := netutil.GenNetwork(t, ctx)
	n3 := netutil.GenNetwork(t, ctx)

	n1p := n1.LocalPeer()
	n2p := n2.LocalPeer()
	n3p := n3.LocalPeer()

	netutil.DivulgeAddresses(n2, n1)
	netutil.DivulgeAddresses(n2, n3)

	if err := n1.DialPeer(ctx, n2p); err != nil {
		t.Fatalf("Failed to dial:", err)
	}
	if err := n3.DialPeer(ctx, n2p); err != nil {
		t.Fatalf("Failed to dial:", err)
	}

	// setup handler on n3 to copy everything over to the pipe.
	piper, pipew := io.Pipe()
	n3.SetHandler(inet.ProtocolTesting, func(s inet.Stream) {
		log.Debug("relay stream opened to n3!")
		log.Debug("piping and echoing everything")
		w := io.MultiWriter(s, pipew)
		io.Copy(w, s)
		log.Debug("closing stream")
		s.Close()
	})

	// ok, now we can try to relay n1--->n2--->n3.
	log.Debug("open relay stream")
	s, err := n1.NewStream(relay.ProtocolRelay, n2p)
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
	if err := mux.WriteProtocolHeader(inet.ProtocolTesting, s); err != nil {
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
