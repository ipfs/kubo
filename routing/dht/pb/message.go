package dht_pb

import (
	"errors"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	peer "github.com/jbenet/go-ipfs/peer"
)

func NewMessage(typ Message_MessageType, key string, level int) *Message {
	m := &Message{
		Type: &typ,
		Key:  &key,
	}
	m.SetClusterLevel(level)
	return m
}

func peerToPBPeer(p peer.Peer) *Message_Peer {
	pbp := new(Message_Peer)

	maddrs := p.Addresses()
	pbp.Addrs = make([]string, len(maddrs))
	for i, maddr := range maddrs {
		pbp.Addrs[i] = maddr.String()
	}
	pid := string(p.ID())
	pbp.Id = &pid
	return pbp
}

// PeersToPBPeers converts a slice of Peers into a slice of *Message_Peers,
// ready to go out on the wire.
func PeersToPBPeers(peers []peer.Peer) []*Message_Peer {
	pbpeers := make([]*Message_Peer, len(peers))
	for i, p := range peers {
		pbpeers[i] = peerToPBPeer(p)
	}
	return pbpeers
}

// Addresses returns a multiaddr associated with the Message_Peer entry
func (m *Message_Peer) Addresses() ([]ma.Multiaddr, error) {
	if m == nil {
		return nil, errors.New("MessagePeer is nil")
	}

	var err error
	maddrs := make([]ma.Multiaddr, len(m.Addrs))
	for i, addr := range m.Addrs {
		maddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
	}
	return maddrs, nil
}

// GetClusterLevel gets and adjusts the cluster level on the message.
// a +/- 1 adjustment is needed to distinguish a valid first level (1) and
// default "no value" protobuf behavior (0)
func (m *Message) GetClusterLevel() int {
	level := m.GetClusterLevelRaw() - 1
	if level < 0 {
		return 0
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

func (m *Message) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"message": map[string]string{
			"type": m.Type.String(),
		},
	}
}
