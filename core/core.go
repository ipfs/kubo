package core

import (
	"encoding/base64"
	"errors"
	"fmt"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	"github.com/jbenet/go-ipfs/bitswap"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	ci "github.com/jbenet/go-ipfs/crypto"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	swarm "github.com/jbenet/go-ipfs/swarm"
	u "github.com/jbenet/go-ipfs/util"
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
func NewIpfsNode(cfg *config.Config, online bool) (*IpfsNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration required")
	}

	d, err := makeDatastore(cfg.Datastore)
	if err != nil {
		return nil, err
	}

	local, err := initIdentity(cfg)
	if err != nil {
		return nil, err
	}

	var (
		net *swarm.Swarm
		// TODO: refactor so we can use IpfsRouting interface instead of being DHT-specific
		route* dht.IpfsDHT
		swap *bitswap.BitSwap
	)

	if online {
		net = swarm.NewSwarm(local)
		err = net.Listen()
		if err != nil {
			return nil, err
		}

		route = dht.NewDHT(local, net, d)
		route.Start()

		swap = bitswap.NewBitSwap(local, net, d, route)
		swap.SetStrategy(bitswap.YesManStrategy)

		go initConnections(cfg, route)
	}

	bs, err := bserv.NewBlockService(d, swap)
	if err != nil {
		return nil, err
	}

	dag := &merkledag.DAGService{Blocks: bs}

	return &IpfsNode{
		Config:    cfg,
		PeerMap:   &peer.Map{},
		Datastore: d,
		Blocks:    bs,
		DAG:       dag,
		Resolver:  &path.Resolver{DAG: dag},
		BitSwap:   swap,
		Identity:  local,
		Routing:   route,
	}, nil
}

func initIdentity(cfg *config.Config) (*peer.Peer, error) {
	if len(cfg.Identity.PeerID) == 0 {
		return nil, errors.New("No peer ID in config! (was ipfs init run?)")
	}

	// address is optional
	var addresses []*ma.Multiaddr
	if len(cfg.Identity.Address) > 0 {
		maddr, err := ma.NewMultiaddr(cfg.Identity.Address)
		if err != nil {
			return nil, err
		}

		addresses = []*ma.Multiaddr{ maddr }
	}

	skb, err := base64.StdEncoding.DecodeString(cfg.Identity.PrivKey)
	if err != nil {
		return nil, err
	}

	sk, err := ci.UnmarshalPrivateKey(skb)
	if err != nil {
		return nil, err
	}

	return &peer.Peer{
		ID:        peer.ID(b58.Decode(cfg.Identity.PeerID)),
		Addresses: addresses,
		PrivKey:   sk,
		PubKey:    sk.GetPublic(),
	}, nil
}

func initConnections(cfg *config.Config, route *dht.IpfsDHT) {
	for _, p := range cfg.Peers {
		maddr, err := ma.NewMultiaddr(p.Address)
		if err != nil {
			u.PErr("error: %v\n", err)
			continue
		}

		_, err = route.Connect(maddr)
		if err != nil {
			u.PErr("Bootstrapping error: %v\n", err)
		}
	}
}

func (n *IpfsNode) PinDagNode(nd *merkledag.Node) error {
	u.POut("Pinning node. Currently No-Op\n")
	return nil
}
