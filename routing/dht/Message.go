package dht

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	peer "github.com/jbenet/go-ipfs/peer"
)

// Message is a a helper struct which makes working with protbuf types easier
type Message struct {
	Type     PBDHTMessage_MessageType
	Key      string
	Value    []byte
	Response bool
	ID       string
	Success  bool
	Peers    []*peer.Peer
}

func peerInfo(p *peer.Peer) *PBDHTMessage_PBPeer {
	pbp := new(PBDHTMessage_PBPeer)
	if len(p.Addresses) == 0 || p.Addresses[0] == nil {
		pbp.Addr = proto.String("")
	} else {
		addr, err := p.Addresses[0].String()
		if err != nil {
			//Temp: what situations could cause this?
			panic(err)
		}
		pbp.Addr = &addr
	}
	pid := string(p.ID)
	pbp.Id = &pid
	return pbp
}

// ToProtobuf takes a Message and produces a protobuf with it.
// TODO: building the protobuf message this way is a little wasteful
//		 Unused fields wont be omitted, find a better way to do this
func (m *Message) ToProtobuf() *PBDHTMessage {
	pmes := new(PBDHTMessage)
	if m.Value != nil {
		pmes.Value = m.Value
	}

	pmes.Type = &m.Type
	pmes.Key = &m.Key
	pmes.Response = &m.Response
	pmes.Id = &m.ID
	pmes.Success = &m.Success
	for _, p := range m.Peers {
		pmes.Peers = append(pmes.Peers, peerInfo(p))
	}

	return pmes
}
