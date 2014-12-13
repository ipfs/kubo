package router

import (
	"testing"
)

func TestTable(t *testing.T) {

	na := &mockNode{addr: "abc"}
	nc := &mockNode{addr: "abd"}
	nd := &mockNode{addr: "add"}
	ne := &mockNode{addr: "ddd"}

	pa := &mockPacket{"abc"}
	pb := &mockPacket{"abc"}
	pc := &mockPacket{"abd"}
	pd := &mockPacket{"add"}
	pe := &mockPacket{"ddd"}

	table := &SimpleTable{
		entries: []TableEntry{
			&tableEntry{na.Address(), na},
			&tableEntry{nc.Address(), nc},
			&tableEntry{nd.Address(), nd},
			&tableEntry{ne.Address(), ne},
		},
	}

	s := NewSwitch("sss", table, []Node{na, nc, nd, ne})
	s.HandlePacket(pa, na)
	s.HandlePacket(pb, na)
	s.HandlePacket(pc, na)
	s.HandlePacket(pd, na)
	s.HandlePacket(pe, na)

	tt := func(n *mockNode, pkts []Packet) {
		for i, p := range pkts {
			if len(n.pkts) <= i {
				t.Error("pkts not handled in order.", n, pkts)
				return
			}
			if n.pkts[i] != p {
				t.Error("pkts not handled in order.", n, pkts)
			}
		}
	}

	tt(na, []Packet{pa, pb})
	tt(nc, []Packet{pc})
	tt(nd, []Packet{pd})
	tt(ne, []Packet{pe})
}

func TestTable2(t *testing.T) {

	n1 := NewQueueNode("aaa", make(chan Packet, 1))
	n2 := NewQueueNode("aba", make(chan Packet, 1))
	n3 := NewQueueNode("abc", make(chan Packet, 1))

	var tb SimpleTable
	tb.distance = HammingDistance
	tb.AddNodes(n1, n2)

	p1 := NewPacket("aaa", "hello1")
	p2 := NewPacket("aba", "hello2")
	p3 := NewPacket("abc", "hello3")

	testRoute := func(p Packet, expect Node) {
		if tb.Route(p) != expect {
			t.Error(p, "route should be", expect)
		}
	}

	testRoute(p1, n1)
	testRoute(p2, n2)
	testRoute(p3, n2) // n2 because we don't have n3 and n2 is closet

	tb.AddNode(n3)
	testRoute(p3, n3)
}
