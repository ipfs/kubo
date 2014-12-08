package mockpeer

import (
	"github.com/jbenet/go-ipfs/peer"

	ic "github.com/jbenet/go-ipfs/crypto"
)

func WithKeyPair(sk ic.PrivKey, pk ic.PubKey) (peer.Peer, error) {
	return peer.NewPeerstore().WithKeyPair(sk, pk)
}

func WithID(id peer.ID) peer.Peer {
	return peer.NewPeerstore().WithID(id)
}

func WithIDString(id string) peer.Peer {
	return peer.NewPeerstore().WithIDString(id)
}
