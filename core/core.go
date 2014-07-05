package core

import (
	"fmt"
	ds "github.com/jbenet/datastore.go"
	config "github.com/jbenet/go-ipfs/config"
	peer "github.com/jbenet/go-ipfs/peer"
)

// IPFS Core module. It represents an IPFS instance.

type IpfsNode struct {

	// the node's configuration
	Config *config.Config

	// the local node's identity
	Identity *peer.Peer

	// the book of other nodes (a map of Peer instances)
	PeerBook *peer.PeerBook

	// the local datastore
	Datastore ds.Datastore

	// the network message stream
	// Network *netmux.Netux

	// the routing system. recommend ipfs-dht
	// Routing *routing.Routing

	// the block exchange + strategy (bitswap)
	// BitSwap *bitswap.BitSwap

	// the block service, get/add blocks.
	// Blocks *blocks.BlockService

	// the path resolution system
	// Resolver *resolver.PathResolver

	// the name system, resolves paths to hashes
	// Namesys *namesys.Namesys
}

func NewIpfsNode(cfg *config.Config) (*IpfsNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration required.")
	}

	d, err := makeDatastore(cfg.Datastore)
	if err != nil {
		return nil, err
	}

	n := &IpfsNode{
		Config:    cfg,
		PeerBook:  &peer.PeerBook{},
		Datastore: d,
	}

	return n, nil
}
