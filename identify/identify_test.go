package identify

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	ci "github.com/jbenet/go-ipfs/crypto"
	"github.com/jbenet/go-ipfs/peer"
)

func TestHandshake(t *testing.T) {
	ska, pka, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}
	skb, pkb, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	cha := make(chan []byte, 5)
	chb := make(chan []byte, 5)

	ida, err := IDFromPubKey(pka)
	if err != nil {
		t.Fatal(err)
	}
	pa := &peer.Peer{
		ID:      ida,
		PubKey:  pka,
		PrivKey: ska,
	}

	idb, err := IDFromPubKey(pkb)
	if err != nil {
		t.Fatal(err)
	}
	pb := &peer.Peer{
		ID:      idb,
		PubKey:  pkb,
		PrivKey: skb,
	}

	ctx := context.Background()
	go func() {
		_, _, err := Handshake(ctx, pa, pb, cha, chb)
		if err != nil {
			t.Fatal(err)
		}
	}()

	_, _, err = Handshake(ctx, pb, pa, chb, cha)
	if err != nil {
		t.Fatal(err)
	}
}
