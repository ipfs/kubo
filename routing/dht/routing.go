package dht

import (
	"math/rand"
	"time"

	proto "code.google.com/p/goprotobuf/proto"

	peer "github.com/jbenet/go-ipfs/peer"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
)

// Pool size is the number of nodes used for group find/set RPC calls
var PoolSize = 6

// TODO: determine a way of creating and managing message IDs
func GenerateMessageID() uint64 {
	return uint64(rand.Uint32()) << 32 & uint64(rand.Uint32())
}

// This file implements the Routing interface for the IpfsDHT struct.

// Basic Put/Get

// PutValue adds value corresponding to given Key.
func (s *IpfsDHT) PutValue(key u.Key, value []byte) error {
	var p *peer.Peer
	p = s.routes.NearestPeer(convertKey(key))
	if p == nil {
		panic("Table returned nil peer!")
	}

	pmes := pDHTMessage{
		Type: DHTMessage_PUT_VALUE,
		Key: string(key),
		Value: value,
		Id: GenerateMessageID(),
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	s.network.Chan.Outgoing <- mes
	return nil
}

// GetValue searches for the value corresponding to given Key.
func (s *IpfsDHT) GetValue(key u.Key, timeout time.Duration) ([]byte, error) {
	var p *peer.Peer
	p = s.routes.NearestPeer(convertKey(key))
	if p == nil {
		panic("Table returned nil peer!")
	}

	pmes := pDHTMessage{
		Type: DHTMessage_GET_VALUE,
		Key: string(key),
		Id: GenerateMessageID(),
	}
	response_chan := s.ListenFor(pmes.Id)

	mes := swarm.NewMessage(p, pmes.ToProtobuf())
	s.network.Chan.Outgoing <- mes

	// Wait for either the response or a timeout
	timeup := time.After(timeout)
	select {
	case <-timeup:
		s.Unlisten(pmes.Id)
		return nil, u.ErrTimeout
	case resp := <-response_chan:
		pmes_out := new(DHTMessage)
		err := proto.Unmarshal(resp.Data, pmes_out)
		if err != nil {
			return nil,err
		}
		return pmes_out.GetValue(), nil
	}
}

// Value provider layer of indirection.
// This is what DSHTs (Coral and MainlineDHT) do to store large values in a DHT.

// Announce that this node can provide value for given key
func (s *IpfsDHT) Provide(key u.Key) error {
	peers := s.routes.NearestPeers(convertKey(key), PoolSize)
	if len(peers) == 0 {
		//return an error
	}

	pmes := pDHTMessage{
		Type: DHTMessage_ADD_PROVIDER,
		Key: string(key),
	}
	pbmes := pmes.ToProtobuf()

	for _,p := range peers {
		mes := swarm.NewMessage(p, pbmes)
		s.network.Chan.Outgoing <-mes
	}
	return nil
}

// FindProviders searches for peers who can provide the value for given key.
func (s *IpfsDHT) FindProviders(key u.Key, timeout time.Duration) ([]*peer.Peer, error) {
	p := s.routes.NearestPeer(convertKey(key))

	pmes := pDHTMessage{
		Type: DHTMessage_GET_PROVIDERS,
		Key: string(key),
		Id: GenerateMessageID(),
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	listen_chan := s.ListenFor(pmes.Id)
	s.network.Chan.Outgoing <-mes
	after := time.After(timeout)
	select {
	case <-after:
		s.Unlisten(pmes.Id)
		return nil, u.ErrTimeout
	case resp := <-listen_chan:
		pmes_out := new(DHTMessage)
		err := proto.Unmarshal(resp.Data, pmes_out)
		if err != nil {
			return nil, err
		}
		panic("Not yet implemented.")
	}
}

// Find specific Peer

// FindPeer searches for a peer with given ID.
func (s *IpfsDHT) FindPeer(id peer.ID, timeout time.Duration) (*peer.Peer, error) {
	p := s.routes.NearestPeer(convertPeerID(id))

	pmes := pDHTMessage{
		Type: DHTMessage_FIND_NODE,
		Key: string(id),
		Id: GenerateMessageID(),
	}

	mes := swarm.NewMessage(p, pmes.ToProtobuf())

	listen_chan := s.ListenFor(pmes.Id)
	s.network.Chan.Outgoing <-mes
	after := time.After(timeout)
	select {
	case <-after:
		s.Unlisten(pmes.Id)
		return nil, u.ErrTimeout
	case resp := <-listen_chan:
		pmes_out := new(DHTMessage)
		err := proto.Unmarshal(resp.Data, pmes_out)
		if err != nil {
			return nil, err
		}
		panic("Not yet implemented.")
	}
}
