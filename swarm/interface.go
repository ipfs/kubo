package swarm

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-multiaddr"
)

type Network interface {
	Send(*Message)
	Error(error)
	Find(u.Key) *peer.Peer
	Listen() error
	Connect(*ma.Multiaddr) (*peer.Peer, error)
	GetChan() *Chan
	Close()
	Drop(*peer.Peer) error
}
