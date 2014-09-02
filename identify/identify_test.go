package identify

import (
	"testing"

	"github.com/jbenet/go-ipfs/peer"
)

func TestHandshake(t *testing.T) {
	kpa, err := GenKeypair(512)
	if err != nil {
		t.Fatal(err)
	}
	kpb, err := GenKeypair(512)
	if err != nil {
		t.Fatal(err)
	}

	cha := make(chan []byte, 5)
	chb := make(chan []byte, 5)

	ida, err := kpa.ID()
	if err != nil {
		t.Fatal(err)
	}
	pa := &peer.Peer{
		ID:      ida,
		PubKey:  kpa.Pub,
		PrivKey: kpa.Priv,
	}

	idb, err := kpb.ID()
	if err != nil {
		t.Fatal(err)
	}
	pb := &peer.Peer{
		ID:      idb,
		PubKey:  kpb.Pub,
		PrivKey: kpb.Priv,
	}

	go func() {
		err := Handshake(pa, pb, cha, chb)
		if err != nil {
			t.Fatal(err)
		}
	}()

	err = Handshake(pb, pa, chb, cha)
	if err != nil {
		t.Fatal(err)
	}
}
