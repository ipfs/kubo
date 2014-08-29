package core

import (
	"fmt"

	ds "github.com/jbenet/datastore.go"
	"github.com/jbenet/go-ipfs/bitswap"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
	ma "github.com/jbenet/go-multiaddr"
)

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// the node's configuration
	Config *config.Config

	// the local node's identity
	Identity *peer.Peer

	// the map of other nodes (Peer instances)
	PeerMap *peer.Map

	// the local datastore
	Datastore ds.Datastore

	// the network message stream
	Swarm *swarm.Swarm

	// the routing system. recommend ipfs-dht
	Routing routing.IpfsRouting

	// the block exchange + strategy (bitswap)
	BitSwap *bitswap.BitSwap

	// the block service, get/add blocks.
	Blocks *bserv.BlockService

	// the merkle dag service, get/add objects.
	DAG *merkledag.DAGService

	// the path resolution system
	Resolver *path.Resolver

	// the name system, resolves paths to hashes
	// Namesys *namesys.Namesys
}

// NewIpfsNode constructs a new IpfsNode based on the given config.
func NewIpfsNode(cfg *config.Config) (*IpfsNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration required")
	}

	d, err := makeDatastore(cfg.Datastore)
	if err != nil {
		return nil, err
	}

	maddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	if err != nil {
		return nil, err
	}

	local := &peer.Peer{
		ID:        peer.ID(cfg.Identity.PeerID),
		Addresses: []*ma.Multiaddr{maddr},
	}

	if len(local.ID) == 0 {
		mh, err := u.Hash([]byte("blah blah blah ID"))
		if err != nil {
			return nil, err
		}
		local.ID = peer.ID(mh)
	}

	net := swarm.NewSwarm(local)
	err = net.Listen()
	if err != nil {
		return nil, err
	}

	route := dht.NewDHT(local, net, d)
	route.Start()

	swap := bitswap.NewBitSwap(local, net, d, route)

	bs, err := bserv.NewBlockService(d, swap)
	if err != nil {
		return nil, err
	}

	dag := &merkledag.DAGService{Blocks: bs}

	n := &IpfsNode{
		Config:    cfg,
		PeerMap:   &peer.Map{},
		Datastore: d,
		Blocks:    bs,
		DAG:       dag,
		Resolver:  &path.Resolver{DAG: dag},
	}

	return n, nil
}
