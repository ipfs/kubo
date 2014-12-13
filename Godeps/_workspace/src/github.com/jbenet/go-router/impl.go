package router

import (
	"errors"
)

type packet struct {
	a Address
	p interface{}
}

// ErrNoRoute signals when there is no Route to a destination
var ErrNoRoute = errors.New("routing error: no route")

// NewPacket constructs a trivial packet linking a destination Address to
// an interface{} payload.
func NewPacket(destination Address, payload interface{}) Packet {
	return &packet{destination, payload}
}

func (p *packet) Destination() Address {
	return p.a
}

func (p *packet) Payload() interface{} {
	return p.p
}

// QueueNode is a trivial node, which accepts packets into a queue
type QueueNode struct {
	a Address
	q chan Packet
}

// NewQueueNode constructs a node with an internal chan Packet queue
func NewQueueNode(addr Address, q chan Packet) *QueueNode {
	return &QueueNode{addr, q}
}

// Queue returns the chan Packet queue
func (n *QueueNode) Queue() <-chan Packet {
	return n.q
}

// Address returns the QueueNode's Address
func (n *QueueNode) Address() Address {
	return n.a
}

// HandlePacket consumes the incomng packet and adds it to the queue.
func (n *QueueNode) HandlePacket(p Packet, s Node) error {
	n.q <- p
	return nil
}

type switchh struct {
	addr   Address
	router Router
	nodes  []Node
}

// NewSwitch constructs a switch with given Router and list of adjacent Nodes.
func NewSwitch(a Address, r Router, adj []Node) Switch {
	return &switchh{a, r, adj}
}

func (s *switchh) Address() Address {
	return s.addr
}

func (s *switchh) Router() Router {
	return s.router
}

func (s *switchh) HandlePacket(p Packet, n Node) error {
	next := s.router.Route(p)
	if next != nil {
		return next.HandlePacket(p, s)
	}
	return ErrNoRoute
}
