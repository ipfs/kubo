package testutil

import (
	"testing"

	ci "gx/ipfs/QmZMehXD2w81qeVJP6r1mmocxwsD7kqAvuzGm2QWDw1H88/go-libp2p/p2p/crypto"
	peer "gx/ipfs/QmZMehXD2w81qeVJP6r1mmocxwsD7kqAvuzGm2QWDw1H88/go-libp2p/p2p/peer"
	ma "gx/ipfs/QmcobAGsCjYt5DXoq9et9L8yR8er7o7Cu3DTvpaq12jYSz/go-multiaddr"
)

type Identity interface {
	Address() ma.Multiaddr
	ID() peer.ID
	PrivateKey() ci.PrivKey
	PublicKey() ci.PubKey
}

// TODO add a cheaper way to generate identities

func RandIdentity() (Identity, error) {
	p, err := RandPeerNetParams()
	if err != nil {
		return nil, err
	}
	return &identity{*p}, nil
}

func RandIdentityOrFatal(t *testing.T) Identity {
	p, err := RandPeerNetParams()
	if err != nil {
		t.Fatal(err)
	}
	return &identity{*p}
}

// identity is a temporary shim to delay binding of PeerNetParams.
type identity struct {
	PeerNetParams
}

func (p *identity) ID() peer.ID {
	return p.PeerNetParams.ID
}

func (p *identity) Address() ma.Multiaddr {
	return p.Addr
}

func (p *identity) PrivateKey() ci.PrivKey {
	return p.PrivKey
}

func (p *identity) PublicKey() ci.PubKey {
	return p.PubKey
}
