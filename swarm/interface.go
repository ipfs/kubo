package swarm

import (
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

type Network interface {
	Find(u.Key) *peer.Peer
	Listen() error
	ConnectNew(*ma.Multiaddr) (*peer.Peer, error)
	GetConnection(id peer.ID, addr *ma.Multiaddr) (*peer.Peer, error)
	Error(error)
	GetErrChan() chan error
	GetChannel(PBWrapper_MessageType) *Chan
	Close()
	CloseConnection(*peer.Peer) error
}
