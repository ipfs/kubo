package testutil

import (
	"testing"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ci "github.com/jbenet/go-ipfs/crypto"
	ipfspeer "github.com/jbenet/go-ipfs/peer"
)

type Peer interface {
	Address() ma.Multiaddr
	ID() ipfspeer.ID
	PrivateKey() ci.PrivKey
	PublicKey() ci.PubKey
}

func RandPeer(t *testing.T) Peer {
	p := RandPeerNetParams(t)
	var err error
	p.Addr = RandLocalTCPAddress()
	p.PrivKey, p.PubKey, err = ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	p.ID, err = ipfspeer.IDFromPublicKey(p.PubKey)
	if err != nil {
		t.Fatal(err)
	}

	if err := p.checkKeys(); err != nil {
		t.Fatal(err)
	}
	return &testpeer{p}
}

// peer is a temporary shim to delay binding of PeerNetParams.
type testpeer struct {
	PeerNetParams
}

func (p *testpeer) ID() ipfspeer.ID {
	return p.PeerNetParams.ID
}

func (p *testpeer) Address() ma.Multiaddr {
	return p.Addr
}

func (p *testpeer) PrivateKey() ci.PrivKey {
	return p.PrivKey
}

func (p *testpeer) PublicKey() ci.PubKey {
	return p.PubKey
}
