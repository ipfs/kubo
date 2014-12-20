package dht_pb

import (
	"errors"
	"fmt"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	inet "github.com/jbenet/go-ipfs/net"
	peer "github.com/jbenet/go-ipfs/peer"
)

// NewMessage constructs a new dht message with given type, key, and level
func NewMessage(typ Message_MessageType, key string, level int) *Message {
	m := &Message{
		Type: &typ,
		Key:  &key,
	}
	m.SetClusterLevel(level)
	return m
}

func peerInfoToPBPeer(p peer.PeerInfo) *Message_Peer {
	pbp := new(Message_Peer)

	pbp.Addrs = make([]string, len(p.Addrs))
	for i, maddr := range p.Addrs {
		pbp.Addrs[i] = maddr.String()
	}
	s := string(p.ID)
	pbp.Id = &s
	return pbp
}

// PBPeerToPeer turns a *Message_Peer into its peer.PeerInfo counterpart
func PBPeerToPeerInfo(pbp *Message_Peer) (peer.PeerInfo, error) {

	maddrs, err := pbp.Addresses()
	if err != nil {
		return peer.PeerInfo{}, fmt.Errorf("Received peer without addresses: %s", pbp.Addrs)
	}

	return peer.PeerInfo{
		ID:    peer.ID(pbp.GetId()),
		Addrs: maddrs,
	}, nil
}

// RawPeerInfosToPBPeers converts a slice of Peers into a slice of *Message_Peers,
// ready to go out on the wire.
func RawPeerInfosToPBPeers(peers []peer.PeerInfo) []*Message_Peer {
	pbpeers := make([]*Message_Peer, len(peers))
	for i, p := range peers {
		pbpeers[i] = peerInfoToPBPeer(p)
	}
	return pbpeers
}

// PeersToPBPeers converts given []peer.Peer into a set of []*Message_Peer,
// which can be written to a message and sent out. the key thing this function
// does (in addition to PeersToPBPeers) is set the ConnectionType with
// information from the given inet.Network.
func PeerInfosToPBPeers(n inet.Network, peers []peer.PeerInfo) []*Message_Peer {
	pbps := RawPeerInfosToPBPeers(peers)
	for i, pbp := range pbps {
		c := ConnectionType(n.Connectedness(peers[i].ID))
		pbp.Connection = &c
	}
	return pbps
}

// PBPeersToPeerInfos converts given []*Message_Peer into []peer.PeerInfo
// Returns two slices, one of peers, and one of errors. The slice of peers
// will ONLY contain successfully converted peers. The slice of errors contains
// whether each input Message_Peer was successfully converted.
func PBPeersToPeerInfos(pbps []*Message_Peer) ([]peer.PeerInfo, []error) {
	errs := make([]error, len(pbps))
	peers := make([]peer.PeerInfo, 0, len(pbps))
	for i, pbp := range pbps {
		p, err := PBPeerToPeerInfo(pbp)
		if err != nil {
			errs[i] = err
		} else {
			peers = append(peers, p)
		}
	}
	return peers, errs
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

// Loggable turns a Message into machine-readable log output
func (m *Message) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"message": map[string]string{
			"type": m.Type.String(),
		},
	}
}

// ConnectionType returns a Message_ConnectionType associated with the
// inet.Connectedness.
func ConnectionType(c inet.Connectedness) Message_ConnectionType {
	switch c {
	default:
		return Message_NOT_CONNECTED
	case inet.NotConnected:
		return Message_NOT_CONNECTED
	case inet.Connected:
		return Message_CONNECTED
	case inet.CanConnect:
		return Message_CAN_CONNECT
	case inet.CannotConnect:
		return Message_CANNOT_CONNECT
	}
}

// Connectedness returns an inet.Connectedness associated with the
// Message_ConnectionType.
func Connectedness(c Message_ConnectionType) inet.Connectedness {
	switch c {
	default:
		return inet.NotConnected
	case Message_NOT_CONNECTED:
		return inet.NotConnected
	case Message_CONNECTED:
		return inet.Connected
	case Message_CAN_CONNECT:
		return inet.CanConnect
	case Message_CANNOT_CONNECT:
		return inet.CannotConnect
	}
}
