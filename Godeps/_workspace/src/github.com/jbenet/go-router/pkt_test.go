package router

import (
	"testing"
)

type mockNode struct {
	addr string
	pkts []Packet
}

func (m *mockNode) Address() Address {
	return m.addr
}

func (m *mockNode) HandlePacket(p Packet, n Node) error {
	m.pkts = append(m.pkts, p)
	return nil
}

type mockPacket struct {
	a string
}

func (m *mockPacket) Destination() Address {
	return m.a
}

func (m *mockPacket) Payload() interface{} {
	return m
}

func TestAddrs(t *testing.T) {

	ta := func(a, b Address, expect int) {
		actual := HammingDistance(a, b)
		if actual != expect {
			t.Error("address distance error:", a, b, expect, actual)
		}
	}

	a := "abc"
	b := "abc"
	c := "abd"
	d := "add"
	e := "ddd"

	ta(a, a, 0)
	ta(a, b, 0)
	ta(a, c, 1)
	ta(a, d, 2)
	ta(a, e, 3)
}

func TestNodes(t *testing.T) {

	a := &mockPacket{"abc"}
	b := &mockPacket{"abc"}
	c := &mockPacket{"abd"}
	d := &mockPacket{"add"}
	e := &mockPacket{"ddd"}

	n := &mockNode{addr: "abc"}
	n2 := &mockNode{addr: "ddd"}
	n.HandlePacket(a, n2)
	n.HandlePacket(b, n2)
	n.HandlePacket(c, n2)
	n.HandlePacket(d, n2)
	n.HandlePacket(e, n2)

	pkts := []Packet{a, b, c, d, e}
	for i, p := range pkts {
		if n.pkts[i] != p {
			t.Error("pkts not handled in order.")
		}
	}
}
