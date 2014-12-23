package testutil

import (
	"testing"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
)

type Peer interface {
	Address() ma.Multiaddr
	ID() peer.ID
	PrivateKey() ci.PrivKey
	PublicKey() ci.PubKey
}

func RandPeer() (Peer, error) {
	p, err := RandPeerNetParams()
	if err != nil {
		return nil, err
	}
	return &testpeer{*p}, nil
}

func RandPeerOrFatal(t *testing.T) Peer {
	p, err := RandPeerNetParams()
	if err != nil {
		t.Fatal(err)
	}
	return &testpeer{*p}
}

// peer is a temporary shim to delay binding of PeerNetParams.
type testpeer struct {
	PeerNetParams
}

func (p *testpeer) ID() peer.ID {
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
