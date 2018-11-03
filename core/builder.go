package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"syscall"
	"time"

	filestore "github.com/ipfs/go-ipfs/filestore"
	pin "github.com/ipfs/go-ipfs/pin"
	repo "github.com/ipfs/go-ipfs/repo"
	cidv0v1 "github.com/ipfs/go-ipfs/thirdparty/cidv0v1"
	"github.com/ipfs/go-ipfs/thirdparty/verifbs"
	resolver "gx/ipfs/QmWZr1S1C28AePSC2zBVsU4WKQqe6AxbiFu9SeDtD7Xkdw/go-path/resolver"
	dag "gx/ipfs/QmYu5UyuTYdw9GJCnrLVpm45P3iUsftiBM67UaktYEtVte/go-merkledag"
	uio "gx/ipfs/QmZBYfq7U5FXirpJDvnsm3L8PncRNXnL7yziDR44jWqwUr/go-unixfs/io"
	bserv "gx/ipfs/Qmc5wGHjavEZsAYHBn2jWfu6L7KXCUB1ExwVjV6ptEWxL9/go-blockservice"

	pstore "gx/ipfs/QmNVzTd29YipdnuXB67b2VnFSerudTVS3ZrxyBhNxcdvAg/go-libp2p-peerstore"
	pstoremem "gx/ipfs/QmNVzTd29YipdnuXB67b2VnFSerudTVS3ZrxyBhNxcdvAg/go-libp2p-peerstore/pstoremem"
	cfg "gx/ipfs/QmNqGkQ53mLMJ2Qe7WmYwv2Vo9QjcYTfsM4bcDNvfS6AXV/go-ipfs-config"
	offline "gx/ipfs/QmPpnbwgAuvhUkA9jGooR88ZwZtTUHXXvoQNKdjZC6nYku/go-ipfs-exchange-offline"
	ipns "gx/ipfs/QmQZJ3WAhrhGXrw7DWNJpAvwoEg2JCi8aR2DjYMZHaX1h4/go-ipns"
	goprocessctx "gx/ipfs/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	bstore "gx/ipfs/QmSNLNnL3kq3A1NGdQA9AtgxM9CWKiiSEup3W435jCkRQS/go-ipfs-blockstore"
	p2phost "gx/ipfs/QmUR3nQ8nSP7vUnzTvz2kLxq4ncud6eC2s4BAEC8uiAsde/go-libp2p-host"
	peer "gx/ipfs/QmUz7HmU4ws577TMEVNoTjNghVvq2jXnxnVhbkNXKPKpgK/go-libp2p-peer"
	libp2p "gx/ipfs/QmW1qc6tENFpuvv3VfjSHRz1GiVYGiQa13nnEMJBaJDBqW/go-libp2p"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	retry "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/retrystore"
	dsync "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	ci "gx/ipfs/QmaZ1F8vQdJaQqA8hjAzH8a7Qnqqb2n2ZYeUbKMjMcxAZe/go-libp2p-crypto"
	record "gx/ipfs/QmdpifwYtwHdABGNqZ3jiiqEqX1jNy5c9iPD3uchzpuoZb/go-libp2p-record"
	metrics "gx/ipfs/QmekzFM3hPZjTjUFGTABdQkEnQ3PTiMstY198PwSFr5w1Q/go-metrics-interface"
)

type BuildCfg struct {
	// If online is set, the node will have networking enabled
	Online bool

	// ExtraOpts is a map of extra options used to configure the ipfs nodes creation
	ExtraOpts map[string]bool

	// If permanent then node should run more expensive processes
	// that will improve performance in long run
	Permanent bool

	// DisableEncryptedConnections disables connection encryption *entirely*.
	// DO NOT SET THIS UNLESS YOU'RE TESTING.
	DisableEncryptedConnections bool

	// If NilRepo is set, a repo backed by a nil datastore will be constructed
	NilRepo bool

	Routing RoutingOption
	Host    HostOption
	Repo    repo.Repo
}

func (cfg *BuildCfg) getOpt(key string) bool {
	if cfg.ExtraOpts == nil {
		return false
	}

	return cfg.ExtraOpts[key]
}

func (cfg *BuildCfg) fillDefaults() error {
	if cfg.Repo != nil && cfg.NilRepo {
		return errors.New("cannot set a repo and specify nilrepo at the same time")
	}

	if cfg.Repo == nil {
		var d ds.Datastore
		d = ds.NewMapDatastore()

		if cfg.NilRepo {
			d = ds.NewNullDatastore()
		}
		r, err := defaultRepo(dsync.MutexWrap(d))
		if err != nil {
			return err
		}
		cfg.Repo = r
	}

	if cfg.Routing == nil {
		cfg.Routing = DHTOption
	}

	if cfg.Host == nil {
		cfg.Host = DefaultHostOption
	}

	return nil
}

func defaultRepo(dstore repo.Datastore) (repo.Repo, error) {
	c := cfg.Config{}
	priv, pub, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, rand.Reader)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	privkeyb, err := priv.Bytes()
	if err != nil {
		return nil, err
	}

	c.Bootstrap = cfg.DefaultBootstrapAddresses
	c.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001"}
	c.Identity.PeerID = pid.Pretty()
	c.Identity.PrivKey = base64.StdEncoding.EncodeToString(privkeyb)

	return &repo.Mock{
		D: dstore,
		C: c,
	}, nil
}

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context, cfg *BuildCfg) (*IpfsNode, error) {
	if cfg == nil {
		cfg = new(BuildCfg)
	}

	err := cfg.fillDefaults()
	if err != nil {
		return nil, err
	}

	ctx = metrics.CtxScope(ctx, "ipfs")

	n := &IpfsNode{
		mode:      offlineMode,
		Repo:      cfg.Repo,
		ctx:       ctx,
		Peerstore: pstoremem.NewPeerstore(),
	}

	n.RecordValidator = record.NamespacedValidator{
		"pk":   record.PublicKeyValidator{},
		"ipns": ipns.Validator{KeyBook: n.Peerstore},
	}

	if cfg.Online {
		n.mode = onlineMode
	}

	// TODO: this is a weird circular-ish dependency, rework it
	n.proc = goprocessctx.WithContextAndTeardown(ctx, n.teardown)

	if err := setupNode(ctx, n, cfg); err != nil {
		n.Close()
		return nil, err
	}

	return n, nil
}

func isTooManyFDError(err error) bool {
	perr, ok := err.(*os.PathError)
	if ok && perr.Err == syscall.EMFILE {
		return true
	}

	return false
}

func setupNode(ctx context.Context, n *IpfsNode, cfg *BuildCfg) error {
	// setup local peer ID (private key is loaded in online setup)
	if err := n.loadID(); err != nil {
		return err
	}

	rds := &retry.Datastore{
		Batching:    n.Repo.Datastore(),
		Delay:       time.Millisecond * 200,
		Retries:     6,
		TempErrFunc: isTooManyFDError,
	}

	// hash security
	bs := bstore.NewBlockstore(rds)
	bs = &verifbs.VerifBS{Blockstore: bs}

	opts := bstore.DefaultCacheOpts()
	conf, err := n.Repo.Config()
	if err != nil {
		return err
	}

	// TEMP: setting global sharding switch here
	uio.UseHAMTSharding = conf.Experimental.ShardingEnabled

	opts.HasBloomFilterSize = conf.Datastore.BloomFilterSize
	if !cfg.Permanent {
		opts.HasBloomFilterSize = 0
	}

	if !cfg.NilRepo {
		bs, err = bstore.CachedBlockstore(ctx, bs, opts)
		if err != nil {
			return err
		}
	}

	bs = bstore.NewIdStore(bs)

	bs = cidv0v1.NewBlockstore(bs)

	n.BaseBlocks = bs
	n.GCLocker = bstore.NewGCLocker()
	n.Blockstore = bstore.NewGCBlockstore(bs, n.GCLocker)

	if conf.Experimental.FilestoreEnabled || conf.Experimental.UrlstoreEnabled {
		// hash security
		n.Filestore = filestore.NewFilestore(bs, n.Repo.FileManager())
		n.Blockstore = bstore.NewGCBlockstore(n.Filestore, n.GCLocker)
		n.Blockstore = &verifbs.VerifBSGC{GCBlockstore: n.Blockstore}
	}

	rcfg, err := n.Repo.Config()
	if err != nil {
		return err
	}

	if rcfg.Datastore.HashOnRead {
		bs.HashOnRead(true)
	}

	hostOption := cfg.Host
	if cfg.DisableEncryptedConnections {
		innerHostOption := hostOption
		hostOption = func(ctx context.Context, id peer.ID, ps pstore.Peerstore, options ...libp2p.Option) (p2phost.Host, error) {
			return innerHostOption(ctx, id, ps, append(options, libp2p.NoSecurity)...)
		}
		log.Warningf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
	}

	if cfg.Online {
		do := setupDiscoveryOption(rcfg.Discovery)
		if err := n.startOnlineServices(ctx, cfg.Routing, hostOption, do, cfg.getOpt("pubsub"), cfg.getOpt("ipnsps"), cfg.getOpt("mplex")); err != nil {
			return err
		}
	} else {
		n.Exchange = offline.Exchange(n.Blockstore)
	}

	n.Blocks = bserv.New(n.Blockstore, n.Exchange)
	n.DAG = dag.NewDAGService(n.Blocks)

	internalDag := dag.NewDAGService(bserv.New(n.Blockstore, offline.Exchange(n.Blockstore)))
	n.Pinning, err = pin.LoadPinner(n.Repo.Datastore(), n.DAG, internalDag)
	if err != nil {
		// TODO: we should move towards only running 'NewPinner' explicitly on
		// node init instead of implicitly here as a result of the pinner keys
		// not being found in the datastore.
		// this is kinda sketchy and could cause data loss
		n.Pinning = pin.NewPinner(n.Repo.Datastore(), n.DAG, internalDag)
	}
	n.Resolver = resolver.NewBasicResolver(n.DAG)

	if cfg.Online {
		if err := n.startLateOnlineServices(ctx); err != nil {
			return err
		}
	}

	return n.loadFilesRoot()
}
