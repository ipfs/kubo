package dht

import (
	peer "github.com/jbenet/go-ipfs/peer"
)

// A helper struct to make working with protbuf types easier
type DHTMessage struct {
	Type     PBDHTMessage_MessageType
	Key      string
	Value    []byte
	Response bool
	Id       uint64
	Success  bool
	Peers    []*peer.Peer
}

func peerInfo(p *peer.Peer) *PBDHTMessage_PBPeer {
	pbp := new(PBDHTMessage_PBPeer)
	addr, err := p.Addresses[0].String()
	if err != nil {
		//Temp: what situations could cause this?
		panic(err)
	}
	pbp.Addr = &addr
	pid := string(p.ID)
	pbp.Id = &pid
	return pbp
}

func (m *DHTMessage) ToProtobuf() *PBDHTMessage {
	pmes := new(PBDHTMessage)
	if m.Value != nil {
		pmes.Value = m.Value
	}

	pmes.Type = &m.Type
	pmes.Key = &m.Key
	pmes.Response = &m.Response
	pmes.Id = &m.Id
	pmes.Success = &m.Success
	for _, p := range m.Peers {
		pmes.Peers = append(pmes.Peers, peerInfo(p))
	}

	return pmes
}
