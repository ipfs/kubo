package testutil

import (
	crand "crypto/rand"
	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

func RandPeer() peer.Peer {
	id := make([]byte, 16)
	crand.Read(id)
	mhid := u.Hash(id)
	return NewPeerWithID(peer.ID(mhid))
}

func PeerWithNewKeys() (peer.Peer, error) {
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		return nil, err
	}

	return NewPeerWithKeyPair(sk, pk)
}
