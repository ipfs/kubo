package core

import (
	"encoding/base64"
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	bitswap "github.com/jbenet/go-ipfs/bitswap"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	ci "github.com/jbenet/go-ipfs/crypto"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	inet "github.com/jbenet/go-ipfs/net"
	mux "github.com/jbenet/go-ipfs/net/mux"
	netservice "github.com/jbenet/go-ipfs/net/service"
	path "github.com/jbenet/go-ipfs/path"
	peer "github.com/jbenet/go-ipfs/peer"
	routing "github.com/jbenet/go-ipfs/routing"
	dht "github.com/jbenet/go-ipfs/routing/dht"
	u "github.com/jbenet/go-ipfs/util"
)

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// the node's configuration
	Config *config.Config

	// the local node's identity
	Identity *peer.Peer

	// storage for other Peer instances
	Peerstore *peer.Peerstore

	// the local datastore
	Datastore ds.Datastore

	// the network message stream
	Network inet.Network

	// the routing system. recommend ipfs-dht
	Routing routing.IpfsRouting

	// the block exchange + strategy (bitswap)
	BitSwap bitswap.BitSwap

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

	peerstore := peer.NewPeerstore()

	var (
		net *inet.Network
		// TODO: refactor so we can use IpfsRouting interface instead of being DHT-specific
		route *dht.IpfsDHT
	)

	if online {
		// add protocol services here.
		ctx := context.TODO() // derive this from a higher context.

		dhts := netservice.Service(nil) // nil handler for now, need to patch it
		if err := dhts.Start(ctx); err != nil {
			return nil, err
		}

		net, err := inet.NewIpfsNetwork(context.TODO(), local, &mux.ProtocolMap{
			netservice.ProtocolID_Routing: dhtService,
			// netservice.ProtocolID_Bitswap: bitswapService,
		})
		if err != nil {
			return nil, err
		}

		route = dht.NewDHT(local, net, dhts, d)
		dhts.Handler = route // wire the handler to the service.

		// TODO(brian): pass a context to DHT for its async operations
		route.Start()

		// TODO(brian): pass a context to bs for its async operations
		bitswapSession := bitswap.NewSession(context.TODO(), local, d, route)

		// TODO(brian): pass a context to initConnections
		go initConnections(cfg, route)
	}

	// TODO(brian): when offline instantiate the BlockService with a bitswap
	// session that simply doesn't return blocks
	bs, err := bserv.NewBlockService(d, bitswapSession)
	if err != nil {
		return nil, err
	}

	dag := &merkledag.DAGService{Blocks: bs}

	return &IpfsNode{
		Config:    cfg,
		Peerstore: peerstore,
		Datastore: d,
		Blocks:    bs,
		DAG:       dag,
		Resolver:  &path.Resolver{DAG: dag},
		BitSwap:   bitswapSession,
		Identity:  local,
		Routing:   route,
	}, nil
}

func initIdentity(cfg *config.Config) (*peer.Peer, error) {
	if cfg.Identity == nil {
		return nil, errors.New("Identity was not set in config (was ipfs init run?)")
	}

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

		addresses = []*ma.Multiaddr{maddr}
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
