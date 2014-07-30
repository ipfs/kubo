package dht

import (
	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	"time"
)

// TODO: determine a way of creating and managing message IDs
func GenerateMessageID() uint64 {
	return 4
}

// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
func (s *IpfsDHT) PutValue(key u.Key, value []byte) error {
	var p *peer.Peer
	p = s.routes.NearestNode(key)

	pmes_type := DHTMessage_PUT_VALUE
	str_key := string(key)
	mes_id := GenerateMessageID()

	pmes := new(DHTMessage)
	pmes.Type = &pmes_type
	pmes.Key = &str_key
	pmes.Value = value
	pmes.Id = &mes_id

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

	str_key := string(key)
	mes_type := DHTMessage_GET_VALUE
	mes_id := GenerateMessageID()
	// protobuf structure
	pmes := new(DHTMessage)
	pmes.Type = &mes_type
	pmes.Key = &str_key
	pmes.Id = &mes_id

	mes := new(swarm.Message)
	mes.Data = []byte(pmes.String())
	mes.Peer = p

	response_chan := s.ListenFor(*pmes.Id)

	// Wait for either the response or a timeout
	timeup := time.After(timeout)
	select {
	case <-timeup:
		// TODO: unregister listener
		return nil, u.ErrTimeout
	case resp := <-response_chan:
		return resp.Data, nil
	}

	// Should never be hit
	return nil, nil
}

// Value provider layer of indirection.
// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

// Announce that this node can provide value for given key
func (s *IpfsDHT) Provide(key u.Key) error {
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
