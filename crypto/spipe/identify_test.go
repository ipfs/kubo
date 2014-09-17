package spipe

import (
	"testing"

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

	go func() {
		_, _, err := Handshake(pa, pb, cha, chb)
		if err != nil {
			t.Fatal(err)
		}
	}()

	_, _, err = Handshake(pb, pa, chb, cha)
	if err != nil {
		t.Fatal(err)
	}
}
