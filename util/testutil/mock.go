package testutil

import (
	ic "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
)

func NewPeerWithKeyPair(sk ic.PrivKey, pk ic.PubKey) (peer.Peer, error) {
	return peer.NewPeerstore().WithKeyPair(sk, pk)
}

func NewPeerWithID(id peer.ID) peer.Peer {
	return peer.NewPeerstore().WithID(id)
}

func NewPeerWithIDString(id string) peer.Peer {
	return peer.NewPeerstore().WithIDString(id)
}
