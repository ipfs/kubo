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
	"encoding/json"
	"io"
	"time"

	"github.com/ipfs/boxo/filestore"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-datastore"

	bitswap "github.com/ipfs/boxo/bitswap"
	bserv "github.com/ipfs/boxo/blockservice"
	bstore "github.com/ipfs/boxo/blockstore"
	exchange "github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/fetcher"
	mfs "github.com/ipfs/boxo/mfs"
	pathresolver "github.com/ipfs/boxo/path/resolver"
	provider "github.com/ipfs/boxo/provider"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	goprocess "github.com/jbenet/goprocess"
	ddht "github.com/libp2p/go-libp2p-kad-dht/dual"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	psrouter "github.com/libp2p/go-libp2p-pubsub-router"
	record "github.com/libp2p/go-libp2p-record"
	connmgr "github.com/libp2p/go-libp2p/core/connmgr"
	ic "github.com/libp2p/go-libp2p/core/crypto"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	metrics "github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"
	routing "github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/ipfs/boxo/bootstrap"
	"github.com/ipfs/boxo/namesys"
	ipnsrp "github.com/ipfs/boxo/namesys/republisher"
	"github.com/ipfs/boxo/peering"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/p2p"
	"github.com/ipfs/kubo/repo"
	irouting "github.com/ipfs/kubo/routing"
)

var log = logging.Logger("core")

// IpfsNode is IPFS Core module. It represents an IPFS instance.
type IpfsNode struct {
	// Self
	Identity peer.ID // the local node's identity

	Repo repo.Repo

	// Local node
	Pinning         pin.Pinner             // the pinning manager
	Mounts          Mounts                 `optional:"true"` // current mount state, if any.
	PrivateKey      ic.PrivKey             `optional:"true"` // the local node's private Key
	PNetFingerprint libp2p.PNetFingerprint `optional:"true"` // fingerprint of private network

	// Services
	Peerstore                   pstore.Peerstore          `optional:"true"` // storage for other Peer instances
	Blockstore                  bstore.GCBlockstore       // the block store (lower level)
	Filestore                   *filestore.Filestore      `optional:"true"` // the filestore blockstore
	BaseBlocks                  node.BaseBlocks           // the raw blockstore, no filestore wrapping
	GCLocker                    bstore.GCLocker           // the locker used to protect the blockstore during gc
	Blocks                      bserv.BlockService        // the block service, get/add blocks.
	DAG                         ipld.DAGService           // the merkle dag service, get/add objects.
	IPLDFetcherFactory          fetcher.Factory           `name:"ipldFetcher"`          // fetcher that paths over the IPLD data model
	UnixFSFetcherFactory        fetcher.Factory           `name:"unixfsFetcher"`        // fetcher that interprets UnixFS data
	OfflineIPLDFetcherFactory   fetcher.Factory           `name:"offlineIpldFetcher"`   // fetcher that paths over the IPLD data model without fetching new blocks
	OfflineUnixFSFetcherFactory fetcher.Factory           `name:"offlineUnixfsFetcher"` // fetcher that interprets UnixFS data without fetching new blocks
	Reporter                    *metrics.BandwidthCounter `optional:"true"`
	Discovery                   mdns.Service              `optional:"true"`
	FilesRoot                   *mfs.Root
	RecordValidator             record.Validator

	// Online
	PeerHost                  p2phost.Host               `optional:"true"` // the network host (server+client)
	Peering                   *peering.PeeringService    `optional:"true"`
	Filters                   *ma.Filters                `optional:"true"`
	Bootstrapper              io.Closer                  `optional:"true"` // the periodic bootstrapper
	Routing                   irouting.ProvideManyRouter `optional:"true"` // the routing system. recommend ipfs-dht
	DNSResolver               *madns.Resolver            // the DNS resolver
	IPLDPathResolver          pathresolver.Resolver      `name:"ipldPathResolver"`          // The IPLD path resolver
	UnixFSPathResolver        pathresolver.Resolver      `name:"unixFSPathResolver"`        // The UnixFS path resolver
	OfflineIPLDPathResolver   pathresolver.Resolver      `name:"offlineIpldPathResolver"`   // The IPLD path resolver that uses only locally available blocks
	OfflineUnixFSPathResolver pathresolver.Resolver      `name:"offlineUnixFSPathResolver"` // The UnixFS path resolver that uses only locally available blocks
	Exchange                  exchange.Interface         // the block exchange + strategy
	Bitswap                   *bitswap.Bitswap           `optional:"true"` // The Bitswap instance
	Namesys                   namesys.NameSystem         // the name system, resolves paths to hashes
	Provider                  provider.System            // the value provider system
	IpnsRepub                 *ipnsrp.Republisher        `optional:"true"`
	ResourceManager           network.ResourceManager    `optional:"true"`

	PubSub   *pubsub.PubSub             `optional:"true"`
	PSRouter *psrouter.PubsubValueStore `optional:"true"`

	DHT       *ddht.DHT       `optional:"true"`
	DHTClient routing.Routing `name:"dhtc" optional:"true"`

	P2P *p2p.P2P `optional:"true"`

	Process goprocess.Process
	ctx     context.Context

	stop func() error

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
	return n.stop()
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
		cfg.BootstrapPeers = func() []peer.AddrInfo {
			ps, err := n.loadBootstrapPeers()
			if err != nil {
				log.Warn("failed to parse bootstrap peers from config")
				return nil
			}
			return ps
		}
	}
	if load, _ := cfg.BackupPeers(); load == nil {
		save := func(ctx context.Context, peerList []peer.AddrInfo) {
			err := n.saveTempBootstrapPeers(ctx, peerList)
			if err != nil {
				log.Warnf("saveTempBootstrapPeers failed: %s", err)
				return
			}
		}
		load = func(ctx context.Context) []peer.AddrInfo {
			peerList, err := n.loadTempBootstrapPeers(ctx)
			if err != nil {
				log.Warnf("loadTempBootstrapPeers failed: %s", err)
				return nil
			}
			return peerList
		}
		cfg.SetBackupPeers(load, save)
	}

	repoConf, err := n.Repo.Config()
	if err != nil {
		return err
	}
	if repoConf.Internal.BackupBootstrapInterval != nil {
		cfg.BackupBootstrapInterval = repoConf.Internal.BackupBootstrapInterval.WithDefault(time.Hour)
	}

	n.Bootstrapper, err = bootstrap.Bootstrap(n.Identity, n.PeerHost, n.Routing, cfg)
	return err
}

var TempBootstrapPeersKey = datastore.NewKey("/local/temp_bootstrap_peers")

func (n *IpfsNode) loadBootstrapPeers() ([]peer.AddrInfo, error) {
	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	return cfg.BootstrapPeers()
}

func (n *IpfsNode) saveTempBootstrapPeers(ctx context.Context, peerList []peer.AddrInfo) error {
	ds := n.Repo.Datastore()
	bytes, err := json.Marshal(config.BootstrapPeerStrings(peerList))
	if err != nil {
		return err
	}

	if err := ds.Put(ctx, TempBootstrapPeersKey, bytes); err != nil {
		return err
	}
	return ds.Sync(ctx, TempBootstrapPeersKey)
}

func (n *IpfsNode) loadTempBootstrapPeers(ctx context.Context) ([]peer.AddrInfo, error) {
	ds := n.Repo.Datastore()
	bytes, err := ds.Get(ctx, TempBootstrapPeersKey)
	if err != nil {
		return nil, err
	}

	var addrs []string
	if err := json.Unmarshal(bytes, &addrs); err != nil {
		return nil, err
	}
	return config.ParseBootstrapPeers(addrs)
}

type ConstructPeerHostOpts struct {
	AddrsFactory      p2pbhost.AddrsFactory
	DisableNatPortMap bool
	DisableRelay      bool
	EnableRelayHop    bool
	ConnectionManager connmgr.ConnManager
}
