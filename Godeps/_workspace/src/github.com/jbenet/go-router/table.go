package router

import (
	"sync"
)

// TableEntry is an (Address, Node) pair for a routing Table
type TableEntry interface {
	Address() Address
	NextHop() Node
}

func NewTableEntry(addr Address, nexthop Node) TableEntry {
	return &tableEntry{addr, nexthop}
}

type tableEntry struct {
	addr Address
	next Node
}

func (te *tableEntry) Address() Address {
	return te.addr
}

func (te *tableEntry) NextHop() Node {
	return te.next
}

// Table is a Router (Routing Table, really) based on a distance criterion.
//
// For example:
//
//   n1 := NewQueueNode("aaa", make(chan Packet, 10))
//   n2 := NewQueueNode("aba", make(chan Packet, 10))
//   n3 := NewQueueNode("abc", make(chan Packet, 10))
//
//   var t router.Table
//   t.Distance = router.HammingDistance
//   t.AddNodes(n1, n2)
//
//   p1 := NewPacket("aaa", "hello1")
//   p2 := NewPacket("aba", "hello2")
//   p3 := NewPacket("abc", "hello3")
//
//   t.Route(p1) // n1
//   t.Route(p2) // n2
//   t.Route(p3) // n2, because we don't have n3 and n2 is closet
//
//   t.AddNode(n3)
//   t.Route(p3) // n3
type Table interface {
	Router

	// Entries are the entries in this routing table
	Entries() []TableEntry

	// Distance returns a measure of distance between two Addresses
	Distance() DistanceFunc
}

type SimpleTable struct {
	entries  []TableEntry
	distance DistanceFunc
	sync.RWMutex
}

// Entries are the entries in this routing table
func (t *SimpleTable) Entries() []TableEntry {
	return t.entries
}

// Distance returns a measure of distance between two Addresses.
func (t *SimpleTable) Distance() DistanceFunc {
	return t.distance
}

// AddEntry adds an (Address, NextHop) entry to the Table
func (t *SimpleTable) AddEntry(addr Address, nextHop Node) {
	t.Lock()
	defer t.Unlock()
	t.entries = append(t.entries, NewTableEntry(addr, nextHop))
}

// AddNode calls AddTableEntry for the given Node
func (t *SimpleTable) AddNode(n Node) {
	t.AddEntry(n.Address(), n)
}

// AddNodes calls AddTableEntry for the given Node
func (t *SimpleTable) AddNodes(ns ...Node) {
	t.Lock()
	defer t.Unlock()
	for _, n := range ns {
		t.entries = append(t.entries, NewTableEntry(n.Address(), n))
	}
}

// Route decides how to route a Packet out of a list of Nodes.
// It returns the Node chosen to send the Packet to.
// Route may return nil, if no route is suitable at all (equivalent of drop).
func (t *SimpleTable) Route(p Packet) Node {
	if t.entries == nil {
		return nil
	}

	t.RLock()
	defer t.RUnlock()

	dist := t.Distance()
	if dist == nil {
		dist = equalDistance
	}

	var best Node
	var bestDist int
	var addr = p.Destination()

	for _, e := range t.entries {
		d := dist(e.Address(), addr)
		if d < 0 {
			continue
		}
		if best == nil || d < bestDist {
			bestDist = d
			best = e.NextHop()
		}
	}
	return best
}

func equalDistance(a, b Address) int {
	if a == b {
		return 0
	}
	return -1
}
