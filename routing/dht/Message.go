package dht

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	peer "github.com/jbenet/go-ipfs/peer"
)

func peerInfo(p *peer.Peer) *Message_Peer {
	pbp := new(Message_Peer)
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

// GetClusterLevel gets and adjusts the cluster level on the message.
// a +/- 1 adjustment is needed to distinguish a valid first level (1) and
// default "no value" protobuf behavior (0)
func (m *Message) GetClusterLevel() int32 {
	return m.GetClusterLevelRaw() - 1
}

// SetClusterLevel adjusts and sets the cluster level on the message.
// a +/- 1 adjustment is needed to distinguish a valid first level (1) and
// default "no value" protobuf behavior (0)
func (m *Message) SetClusterLevel(level int32) {
	m.ClusterLevelRaw = &level
}
