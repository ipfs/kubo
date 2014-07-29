package dht

import (
  "time"
  peer "github.com/jbenet/go-ipfs/peer"
  u "github.com/jbenet/go-ipfs/util"
  swarm "github.com/jbenet/go-ipfs/swarm"
)


// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
func (s *IpfsDHT) PutValue(key u.Key, value []byte) (error) {
	var p *peer.Peer
	p = s.routes.NearestNode(key)

	pmes := new(PutValue)
	pmes.Key = &key
	pmes.Value = value

	mes := new(swarm.Message)
	mes.Data = []byte(pmes.String())
	mes.Peer = p

	s.network.Chan.Outgoing <- mes
	return nil
}

// GetValue searches for the value corresponding to given Key.
func (s *IpfsDHT) GetValue(key u.Key, timeout time.Duration) ([]byte, error) {
	var p *peer.Peer
	p = s.routes.NearestNode(key)

	// protobuf structure
	pmes := new(GetValue)
	pmes.Key = &key
	pmes.Id = GenerateMessageID()

	mes := new(swarm.Message)
	mes.Data = []byte(pmes.String())
	mes.Peer = p

	response_chan := s.network.ListenFor(pmes.Id)

	timeup := time.After(timeout)
	select {
		case <-timeup:
			return nil, timeoutError
		case resp := <-response_chan:
	}
}


// Value provider layer of indirection.
// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

// Announce that this node can provide value for given key
func (s *IpfsDHT) Provide(key u.Key) (error) {
  return u.ErrNotImplemented
}

// FindProviders searches for peers who can provide the value for given key.
func (s *IpfsDHT) FindProviders(key u.Key, timeout time.Duration) (*peer.Peer, error) {
  return nil, u.ErrNotImplemented
}


// Find specific Peer

// FindPeer searches for a peer with given ID.
func (s *IpfsDHT) FindPeer(id peer.ID, timeout time.Duration) (*peer.Peer, error) {
  return nil, u.ErrNotImplemented
}
