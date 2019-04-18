/*
Package core implements the IpfsNode object and related methods.

Packages underneath core/ provide a (relatively) stable, low-level API
to carry out most IPFS-related tasks.  For more details on the other
interfaces and how core/... fits into the bigger IPFS picture, see:

  $ godoc github.com/ipfs/go-ipfs
*/
package core

import (
	"context"
	"io"

	"go.uber.org/fx"

	version "github.com/ipfs/go-ipfs"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"
	rp "github.com/ipfs/go-ipfs/exchange/reprovide"
	"github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/fuse/mount"
	"github.com/ipfs/go-ipfs/namesys"
	ipnsrp "github.com/ipfs/go-ipfs/namesys/republisher"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/provider"
	"github.com/ipfs/go-ipfs/repo"

	bserv "github.com/ipfs/go-blockservice"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-path/resolver"
	"github.com/jbenet/goprocess"
	autonat "github.com/libp2p/go-libp2p-autonat-svc"
	ic "github.com/libp2p/go-libp2p-crypto"
	p2phost "github.com/libp2p/go-libp2p-host"
	ifconnmgr "github.com/libp2p/go-libp2p-interface-connmgr"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	metrics "github.com/libp2p/go-libp2p-metrics"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	psrouter "github.com/libp2p/go-libp2p-pubsub-router"
	record "github.com/libp2p/go-libp2p-record"
	routing "github.com/libp2p/go-libp2p-routing"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
)

var log = logging.Logger("core")

func init() {
	identify.ClientVersion = "go-ipfs/" + version.CurrentVersionNumber + "/" + version.CurrentCommit
}

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {

	// Self
	Identity peer.ID // the local node's identity

	Repo repo.Repo

	// Local node
	Pinning         pin.Pinner           // the pinning manager
	Mounts          Mounts               `optional:"true"` // current mount state, if any.
	PrivateKey      ic.PrivKey           // the local node's private Key
	PNetFingerprint node.PNetFingerprint `optional:"true"` // fingerprint of private network

	// Services
	Peerstore       pstore.Peerstore     `optional:"true"` // storage for other Peer instances
	Blockstore      bstore.GCBlockstore  // the block store (lower level)
	Filestore       *filestore.Filestore // the filestore blockstore
	BaseBlocks      node.BaseBlocks      // the raw blockstore, no filestore wrapping
	GCLocker        bstore.GCLocker      // the locker used to protect the blockstore during gc
	Blocks          bserv.BlockService   // the block service, get/add blocks.
	DAG             ipld.DAGService      // the merkle dag service, get/add objects.
	Resolver        *resolver.Resolver   // the path resolution system
	Reporter        metrics.Reporter     `optional:"true"`
	Discovery       discovery.Service    `optional:"true"`
	FilesRoot       *mfs.Root
	RecordValidator record.Validator

	// Online
	PeerHost     p2phost.Host        `optional:"true"` // the network host (server+client)
	Bootstrapper io.Closer           `optional:"true"` // the periodic bootstrapper
	Routing      routing.IpfsRouting `optional:"true"` // the routing system. recommend ipfs-dht
	Exchange     exchange.Interface  // the block exchange + strategy (bitswap)
	Namesys      namesys.NameSystem  // the name system, resolves paths to hashes
	Provider     provider.Provider   // the value provider system
	Reprovider   *rp.Reprovider      `optional:"true"` // the value reprovider system
	IpnsRepub    *ipnsrp.Republisher `optional:"true"`

	AutoNAT  *autonat.AutoNATService    `optional:"true"`
	PubSub   *pubsub.PubSub             `optional:"true"`
	PSRouter *psrouter.PubsubValueStore `optional:"true"`
	DHT      *dht.IpfsDHT               `optional:"true"`
	P2P      *p2p.P2P                   `optional:"true"`

	Process goprocess.Process
	ctx     context.Context

	app *fx.App

	// Flags
	IsOnline bool `optional:"true"` // Online is set when networking is enabled.
	IsDaemon bool `optional:"true"` // Daemon is set when running on a long-running daemon.
}

// Mounts defines what the node's mount state is. This should
// perhaps be moved to the daemon or mount. It's here because
// it needs to be accessible across daemon requests.
type Mounts struct {
	Ipfs mount.Mount
	Ipns mount.Mount
}

// Close calls Close() on the App object
func (n *IpfsNode) Close() error {
	return n.app.Stop(n.ctx)
}

// Context returns the IpfsNode context
func (n *IpfsNode) Context() context.Context {
	if n.ctx == nil {
		n.ctx = context.TODO()
	}
	return n.ctx
}

// Bootstrap will set and call the IpfsNodes bootstrap function.
func (n *IpfsNode) Bootstrap(cfg bootstrap.BootstrapConfig) error {
	// TODO what should return value be when in offlineMode?
	if n.Routing == nil {
		return nil
	}

	if n.Bootstrapper != nil {
		n.Bootstrapper.Close() // stop previous bootstrap process.
	}

	// if the caller did not specify a bootstrap peer function, get the
	// freshest bootstrap peers from config. this responds to live changes.
	if cfg.BootstrapPeers == nil {
		cfg.BootstrapPeers = func() []pstore.PeerInfo {
			ps, err := n.loadBootstrapPeers()
			if err != nil {
				log.Warning("failed to parse bootstrap peers from config")
				return nil
			}
			return ps
		}
	}

	var err error
	n.Bootstrapper, err = bootstrap.Bootstrap(n.Identity, n.PeerHost, n.Routing, cfg)
	return err
}

func (n *IpfsNode) loadBootstrapPeers() ([]pstore.PeerInfo, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	parsed, err := cfg.BootstrapPeers()
	if err != nil {
		return nil, err
	}
	return bootstrap.Peers.ToPeerInfos(parsed), nil
}

type ConstructPeerHostOpts struct {
	AddrsFactory      p2pbhost.AddrsFactory
	DisableNatPortMap bool
	DisableRelay      bool
	EnableRelayHop    bool
	ConnectionManager ifconnmgr.ConnManager
}
