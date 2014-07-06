package core

import (
	"fmt"
	ds "github.com/jbenet/datastore.go"
	blocks "github.com/jbenet/go-ipfs/blocks"
	config "github.com/jbenet/go-ipfs/config"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
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
	Blocks *blocks.BlockService

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

	bs, err := blocks.NewBlockService(d)
	if err != nil {
		return nil, err
	}

	n := &IpfsNode{
		Config:    cfg,
		PeerBook:  &peer.PeerBook{},
		Datastore: d,
		Blocks:    bs,
	}

	return n, nil
}

func (n *IpfsNode) AddDagNode(nd *merkledag.Node) (u.Key, error) {
	d, err := nd.Encoded(false)
	if err != nil {
		return "", err
	}

	b, err := blocks.NewBlock(d)
	if err != nil {
		return "", err
	}

	return n.Blocks.AddBlock(b)
}

func (n *IpfsNode) GetDagNode(k u.Key) (*merkledag.Node, error) {
	b, err := n.Blocks.GetBlock(k)
	if err != nil {
		return nil, err
	}

	return merkledag.Decoded(b.Data)
}
