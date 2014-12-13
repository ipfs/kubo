// package mux implements a protocol muxer.
package mux

import (
	"errors"
	"fmt"
	"sync"

	msg "github.com/jbenet/go-ipfs/net/message"
	pb "github.com/jbenet/go-ipfs/net/mux/internal/pb"
	u "github.com/jbenet/go-ipfs/util"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	router "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-router"
)

var log = u.Logger("muxer")

// ProtocolIDs used to identify each protocol.
// These should probably be defined elsewhere.
var (
	ProtocolID_Routing    = pb.ProtocolID_Routing
	ProtocolID_Exchange   = pb.ProtocolID_Exchange
	ProtocolID_Diagnostic = pb.ProtocolID_Diagnostic
)

// Protocol objects produce + consume raw data. They are added to the Muxer
// with a ProtocolID, which is added to outgoing payloads. Muxer properly
// encapsulates and decapsulates when interfacing with its Protocols. The
// Protocols do not encounter their ProtocolID.
type Protocol interface {
	ProtocolID() pb.ProtocolID

	// Node is a router.Node, for message connectivity.
	router.Node
}

// ProtocolMap maps ProtocolIDs to Protocols.
type ProtocolMap map[pb.ProtocolID]Protocol

// Muxer is a simple multiplexor that reads + writes to Incoming and Outgoing
// channels. It multiplexes various protocols, wrapping and unwrapping data
// with a ProtocolID.
//
// implements router.Node and router.Route
type Muxer struct {
	local  router.Address
	uplink router.Node

	// Protocols are the multiplexed services.
	Protocols ProtocolMap
	mapLock   sync.Mutex

	bwiLock sync.Mutex
	bwIn    uint64
	msgIn   uint64

	bwoLock sync.Mutex
	bwOut   uint64
	msgOut  uint64
}

// NewMuxer constructs a muxer given a protocol map.
// uplink is a Node to send all outgoing traffic to.
func NewMuxer(local router.Address, uplink router.Node) *Muxer {
	return &Muxer{
		local:     local,
		uplink:    uplink,
		Protocols: ProtocolMap{},
	}
}

// GetMessageCounts return the in/out message count measured over this muxer.
func (m *Muxer) GetMessageCounts() (in uint64, out uint64) {
	m.bwiLock.Lock()
	in = m.msgIn
	m.bwiLock.Unlock()

	m.bwoLock.Lock()
	out = m.msgOut
	m.bwoLock.Unlock()
	return
}

// GetBandwidthTotals return the in/out bandwidth measured over this muxer.
func (m *Muxer) GetBandwidthTotals() (in uint64, out uint64) {
	m.bwiLock.Lock()
	in = m.bwIn
	m.bwiLock.Unlock()

	m.bwoLock.Lock()
	out = m.bwOut
	m.bwoLock.Unlock()
	return
}

// AddProtocol adds a Protocol with given ProtocolID to the Muxer.
func (m *Muxer) AddProtocol(p Protocol, pid pb.ProtocolID) error {
	m.mapLock.Lock()
	defer m.mapLock.Unlock()

	if _, found := m.Protocols[pid]; found {
		return errors.New("Another protocol already using this ProtocolID")
	}

	m.Protocols[pid] = p
	return nil
}

func (m *Muxer) Address() router.Address {
	return m.local
}

func (m *Muxer) HandlePacket(p router.Packet, from router.Node) error {
	pkt, ok := p.(*msg.Packet)
	if !ok {
		return msg.ErrInvalidPayload
	}

	if from == m.uplink {
		return m.handleIncomingPacket(pkt, from)
	} else {
		return m.handleOutgoingPacket(pkt, from)
	}
}

// handleIncomingPacket routes message to the appropriate protocol.
func (m *Muxer) handleIncomingPacket(p *msg.Packet, _ router.Node) error {

	m.bwiLock.Lock()
	// TODO: compensate for overhead
	m.bwIn += uint64(len(p.Data))
	m.msgIn++
	m.bwiLock.Unlock()

	data, pid, err := unwrapData(p.Data)
	if err != nil {
		return fmt.Errorf("muxer de-serializing error: %v", err)
	}

	// TODO: fix this when mpool is fixed.
	// conn.ReleaseBuffer(m1.Data())

	p.Data = data

	m.mapLock.Lock()
	proto, found := m.Protocols[pid]
	m.mapLock.Unlock()

	if !found {
		return fmt.Errorf("muxer: unknown protocol %v", pid)
	}

	log.Debugf("muxer: outgoing packet %d -> %s", proto.ProtocolID(), m.uplink.Address())
	return proto.HandlePacket(p, m)
}

// handleOutgoingMessages sends out messages to the outside world
func (m *Muxer) handleOutgoingPacket(p *msg.Packet, from router.Node) error {

	var pid pb.ProtocolID
	var proto Protocol
	m.mapLock.Lock()
	for pid2, proto2 := range m.Protocols {
		if proto2 == from {
			pid = pid2
			proto = proto2
			break
		}
	}
	m.mapLock.Unlock()

	if proto == nil {
		return errors.New("muxer: packet sent from unknown protocol")
	}

	var err error
	p.Data, err = wrapData(p.Data, pid)
	if err != nil {
		return fmt.Errorf("muxer serializing error: %v", err)
	}

	m.bwoLock.Lock()
	// TODO: compensate for overhead
	// TODO(jbenet): switch this to a goroutine to prevent sync waiting.
	m.bwOut += uint64(len(p.Data))
	m.msgOut++
	m.bwoLock.Unlock()

	// TODO: add multiple uplinks
	log.Debugf("muxer: incoming packet %s -> %d", m.uplink.Address(), proto.ProtocolID())
	return m.uplink.HandlePacket(p, m)
}

func wrapData(data []byte, pid pb.ProtocolID) ([]byte, error) {
	// Marshal
	pbm := new(pb.PBProtocolMessage)
	pbm.ProtocolID = &pid
	pbm.Data = data
	b, err := proto.Marshal(pbm)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func unwrapData(data []byte) ([]byte, pb.ProtocolID, error) {
	// Unmarshal
	pbm := new(pb.PBProtocolMessage)
	err := proto.Unmarshal(data, pbm)
	if err != nil {
		return nil, 0, err
	}

	return pbm.GetData(), pbm.GetProtocolID(), nil
}
