package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func RandPeerID() (peer.ID, error) {
	_, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return "", err
	}
	return peer.IDFromPublicKey(pk)
}

var nextPort = 0

// RandLocalTCPAddress returns a random multiaddr. it suppresses errors
// for nice composability-- do check the address isn't nil.
func RandLocalTCPAddress() ma.Multiaddr {
	if nextPort == 0 {
		nextPort = 10000 + SeededRand.Intn(50000)
	}

	// chances are it will work out, but it **might** fail if the port is in use
	// most ports above 10000 aren't in use by long running processes, so yay.
	// (maybe there should be a range of "loopback" ports that are guaranteed
	// to be open for the process, but naturally can only talk to self.)
	nextPort++
	addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", nextPort)
	maddr, _ := ma.NewMultiaddr(addr)
	return maddr
}

// PeerNetParams is a struct to bundle together the four things
// you need to run a connection with a peer: id, 2keys, and addr.
type PeerNetParams struct {
	ID      peer.ID
	PrivKey ci.PrivKey
	PubKey  ci.PubKey
	Addr    ma.Multiaddr
}

func (p *PeerNetParams) checkKeys() error {
	if !p.ID.MatchesPrivateKey(p.PrivKey) {
		return errors.New("p.ID does not match p.PrivKey")
	}

	if !p.ID.MatchesPublicKey(p.PubKey) {
		return errors.New("p.ID does not match p.PubKey")
	}

	var buf bytes.Buffer
	buf.Write([]byte("hello world. this is me, I swear."))
	b := buf.Bytes()

	sig, err := p.PrivKey.Sign(b)
	if err != nil {
		return fmt.Errorf("sig signing failed: %s", err)
	}

	sigok, err := p.PubKey.Verify(b, sig)
	if err != nil {
		return fmt.Errorf("sig verify failed: %s", err)
	}
	if !sigok {
		return fmt.Errorf("sig verify failed: sig invalid!")
	}

	return nil // ok. move along.
}

func RandPeerNetParams(t *testing.T) (p PeerNetParams) {
	var err error
	p.Addr = RandLocalTCPAddress()
	p.PrivKey, p.PubKey, err = ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	p.ID, err = peer.IDFromPublicKey(p.PubKey)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.checkKeys(); err != nil {
		t.Fatal(err)
	}
	return p
}
