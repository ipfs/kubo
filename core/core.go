package core

import (
	"encoding/base64"
	"errors"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/datastore.go"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	bserv "github.com/jbenet/go-ipfs/blockservice"
	config "github.com/jbenet/go-ipfs/config"
	ci "github.com/jbenet/go-ipfs/crypto"
	exchange "github.com/jbenet/go-ipfs/exchange"
	bitswap "github.com/jbenet/go-ipfs/exchange/bitswap"
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
	Peerstore peer.Peerstore

	// the local datastore
	Datastore ds.Datastore

	// the network message stream
	Network inet.Network

	// the routing system. recommend ipfs-dht
	Routing routing.IpfsRouting

	// the block exchange + strategy (bitswap)
	Exchange exchange.Interface

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

	// FIXME(brian): This is a bit dangerous. If any of the vars declared in
	// this block are assigned inside of the "if online" block using the ":="
	// declaration syntax, the compiler permits re-declaration. This is rather
	// undesirable
	var (
		net inet.Network
		// TODO: refactor so we can use IpfsRouting interface instead of being DHT-specific
		route           *dht.IpfsDHT
		exchangeSession exchange.Interface
	)

	if online {
		// add protocol services here.
		ctx := context.TODO() // derive this from a higher context.

		dhtService := netservice.NewService(nil)      // nil handler for now, need to patch it
		exchangeService := netservice.NewService(nil) // nil handler for now, need to patch it

		if err := dhtService.Start(ctx); err != nil {
			return nil, err
		}
		if err := exchangeService.Start(ctx); err != nil {
			return nil, err
		}

		net, err = inet.NewIpfsNetwork(context.TODO(), local, &mux.ProtocolMap{
			mux.ProtocolID_Routing:  dhtService,
			mux.ProtocolID_Exchange: exchangeService,
		})
		if err != nil {
			return nil, err
		}

		route = dht.NewDHT(local, peerstore, net, dhtService, d)
		// TODO(brian): perform this inside NewDHT factory method
		dhtService.Handler = route // wire the handler to the service.

		const alwaysSendToPeer = true // use YesManStrategy
		exchangeSession = bitswap.NetMessageSession(ctx, local, exchangeService, route, d, alwaysSendToPeer)

		// TODO(brian): pass a context to initConnections
		go initConnections(ctx, cfg, peerstore, route)
	}

	// TODO(brian): when offline instantiate the BlockService with a bitswap
	// session that simply doesn't return blocks
	bs, err := bserv.NewBlockService(d, exchangeSession)
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
		Exchange:  exchangeSession,
		Identity:  local,
		Routing:   route,
	}, nil
}

func initIdentity(cfg *config.Config) (*peer.Peer, error) {
	if cfg.Identity.PeerID == "" {
		return nil, errors.New("Identity was not set in config (was ipfs init run?)")
	}

	if len(cfg.Identity.PeerID) == 0 {
		return nil, errors.New("No peer ID in config! (was ipfs init run?)")
	}

	// address is optional
	var addresses []*ma.Multiaddr
	if len(cfg.Addresses.Swarm) > 0 {
		maddr, err := ma.NewMultiaddr(cfg.Addresses.Swarm)
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

func initConnections(ctx context.Context, cfg *config.Config, pstore peer.Peerstore, route *dht.IpfsDHT) {
	for _, p := range cfg.Bootstrap {
		if p.PeerID == "" {
			u.PErr("error: peer does not include PeerID. %v\n", p)
		}

		maddr, err := ma.NewMultiaddr(p.Address)
		if err != nil {
			u.PErr("error: %v\n", err)
			continue
		}

		// setup peer
		npeer := &peer.Peer{ID: peer.DecodePrettyID(p.PeerID)}
		npeer.AddAddress(maddr)

		if err = pstore.Put(npeer); err != nil {
			u.PErr("Bootstrapping error: %v\n", err)
			continue
		}

		if _, err = route.Connect(ctx, npeer); err != nil {
			u.PErr("Bootstrapping error: %v\n", err)
		}
	}
}

// PinDagNode ensures a given node is stored persistently locally.
func (n *IpfsNode) PinDagNode(nd *merkledag.Node) error {
	u.DOut("Pinning node. Currently No-Op\n")
	return nil
}
