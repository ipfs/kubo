package testutil

import (
	crand "crypto/rand"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

func RandPeer() peer.Peer {
	id := make([]byte, 16)
	crand.Read(id)
	mhid := u.Hash(id)
	return NewPeerWithID(peer.ID(mhid))
}
