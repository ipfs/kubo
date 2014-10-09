package dht

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	peer "github.com/jbenet/go-ipfs/peer"
)

func newMessage(typ Message_MessageType, key string, level int) *Message {
	m := &Message{
		Type: &typ,
		Key:  &key,
	}
	m.SetClusterLevel(level)
	return m
}

func peerToPBPeer(p *peer.Peer) *Message_Peer {
	pbp := new(Message_Peer)
	if len(p.Addresses) == 0 || p.Addresses[0] == nil {
		pbp.Addr = proto.String("")
	} else {
		addr := p.Addresses[0].String()
		pbp.Addr = &addr
	}
	pid := string(p.ID)
	pbp.Id = &pid
	return pbp
}

func peersToPBPeers(peers []*peer.Peer) []*Message_Peer {
	pbpeers := make([]*Message_Peer, len(peers))
	for i, p := range peers {
		pbpeers[i] = peerToPBPeer(p)
	}
	return pbpeers
}

// GetClusterLevel gets and adjusts the cluster level on the message.
// a +/- 1 adjustment is needed to distinguish a valid first level (1) and
// default "no value" protobuf behavior (0)
func (m *Message) GetClusterLevel() int {
	level := m.GetClusterLevelRaw() - 1
	if level < 0 {
		log.Debug("GetClusterLevel: no routing level specified, assuming 0")
		level = 0
	}
	return int(level)
}

// SetClusterLevel adjusts and sets the cluster level on the message.
// a +/- 1 adjustment is needed to distinguish a valid first level (1) and
// default "no value" protobuf behavior (0)
func (m *Message) SetClusterLevel(level int) {
	lvl := int32(level)
	m.ClusterLevelRaw = &lvl
}
