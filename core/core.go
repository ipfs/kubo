package core

import (
	"encoding/base64"
	"errors"
	"fmt"

	ds "github.com/jbenet/datastore.go"
	b58 "github.com/jbenet/go-base58"
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
func NewIpfsNode(cfg *config.Config, online bool) (*IpfsNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration required")
	}

	d, err := makeDatastore(cfg.Datastore)
	if err != nil {
		return nil, err
	}

	var swap *bitswap.BitSwap
	if online {
		swap, err = loadBitswap(cfg, d)
		if err != nil {
			return nil, err
		}
		swap.SetStrategy(bitswap.YesManStrategy)
	}

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

func loadBitswap(cfg *config.Config, d ds.Datastore) (*bitswap.BitSwap, error) {
	maddr, err := ma.NewMultiaddr(cfg.Identity.Address)
	if err != nil {
		return nil, err
	}

	skb, err := base64.StdEncoding.DecodeString(cfg.Identity.PrivKey)
	if err != nil {
		return nil, err
	}

	sk, err := ci.UnmarshalPrivateKey(skb)
	if err != nil {
		return nil, err
	}

	local := &peer.Peer{
		ID:        peer.ID(b58.Decode(cfg.Identity.PeerID)),
		Addresses: []*ma.Multiaddr{maddr},
		PrivKey:   sk,
		PubKey:    sk.GetPublic(),
	}

	if len(local.ID) == 0 {
		return nil, errors.New("No peer ID in config! (was ipfs init run?)")
	}

	net := swarm.NewSwarm(local)
	err = net.Listen()
	if err != nil {
		return nil, err
	}

	route := dht.NewDHT(local, net, d)
	route.Start()

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

	return bitswap.NewBitSwap(local, net, d, route), nil
}

func (n *IpfsNode) PinDagNode(nd *merkledag.Node) error {
	u.POut("Pinning node. Currently No-Op\n")
	return nil
}
