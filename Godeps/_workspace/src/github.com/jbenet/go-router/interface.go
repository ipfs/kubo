// Package router is a networking-inspired routing model. It's useful for
// muxing and lots of other things. It enables you to construct your
// abstractions as you would compose computer networks (endpoints, switches,
// routing tables). These could represent processing workers, entire
// subsystems, or even real computers ;).
package router

// Address is our way of knowing where we're headed. Traditionally, addresses
// are things like IP Addresses, or email addresses. But filepaths, or URLs
// can be seen as addresses too. We really leave it up to you. Our routing
// is general, and forces you to pick an addressing scheme, and some logic to
// discriminate addresses that you'll plug into Routers (Address DistanceFunc).
type Address interface{}

// Packet is the unit of moving things in our network. Anything can be routed
// in our network, as long as it has a Destination.
type Packet interface {

	// Destination is the Address of the endpoint this Packet is headed to.
	// They could use the same Addressing throughout (recommended) like the
	// internet, or face the pain of translating addresses at inter-network
	// gateways.
	Destination() Address

	// Payload is here for completeness. This can be the Packet itself, but this
	// function encourages the client to think through their packet design.
	Payload() interface{}
}

// Node is an object which has interfaces to connect to networks. This
// is an "endpoint" object.
type Node interface {

	// Address returns the node's address.
	Address() Address

	// HandlePacket receives a packet sent by another node.
	HandlePacket(Packet, Node) error
}

// Switch is the basic forwarding device. It listens to all its interfaces (it
// is a Node in our network) and Forwards all Packets received by any interface
// out another interface, according to its ForwardingTable.
type Switch interface {
	Node

	// Router returns the Router used to decide where to forward packets.
	Router() Router
}

// Router is an object that decides how a Packet should be Routed. This should
// be as close to a static table lookup as possible, meaning it would be best
// to prepare a forwarding table in parallel, instead of blocking.
//
// Our Router captures the entire Control Plane, meaning that we can implement:
// - Static Routing - forwarding table only
// - Dynamic Routing - routing table computed with an algorithm or protocol
// - "SDN" Routing - Control Plane separated from Data Plane
// And even:
// - URL Routers (like gorilla.Muxer)
// - Protocol Muxers
// entirely within different Router implementations.
//
// Note that this is a break from traditional networking systems. Instead of
// having the abstractions of FIB, RIB, Routing/Forwarding Tables, Routers,
// and Switches, we only have the last two:
// - Router -- the things that "route" (decide where things go)
// - Switch -- connecting Nodes, "switch" Packets according to a Router.
type Router interface {

	// Route decides how to route a Packet.
	// It returns the next hop Node chosen to send the Packet to.
	// Route may return nil, if no route is suitable at all (equivalent of drop).
	Route(Packet) Node
}

// DistanceFunc returns a measure of distance between two Addresses.
// Examples:
// - masked longest prefix match (IP, CIDR)
// - XOR distance (Kademlia)
type DistanceFunc func(a, b Address) int
