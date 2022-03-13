package config

// Addresses stores the (string) multiaddr addresses for the node.
type Addresses struct {
	Swarm          []string // addresses for the swarm to listen on
	Announce       []string // swarm addresses to announce to the network, if len > 0 replaces auto detected addresses
	AppendAnnounce []string // similar to Announce but doesn't overwrite auto detected addresses, they are just appended
	NoAnnounce     []string // swarm addresses not to announce to the network
	API            Strings  // address for the local API (RPC)
	Gateway        Strings  // address to listen on for IPFS HTTP object gateway
}
