package libp2p

import (
	"sort"
	"time"

	version "github.com/ipfs/go-ipfs"
	config "github.com/ipfs/go-ipfs-config"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"go.uber.org/fx"
)

var log = logging.Logger("p2pnode")

type Libp2pOpts struct {
	fx.Out

	Opts []libp2p.Option `group:"libp2p"`
}

// Misc options

var UserAgent = simpleOpt(libp2p.UserAgent(version.UserAgent))

func ConnectionManager(low, high int, grace time.Duration) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		cm := connmgr.NewConnManager(low, high, grace)
		opts.Opts = append(opts.Opts, libp2p.ConnectionManager(cm))
		return
	}
}

func PstoreAddSelfKeys(id peer.ID, sk crypto.PrivKey, ps peerstore.Peerstore) error {
	if err := ps.AddPubKey(id, sk.GetPublic()); err != nil {
		return err
	}

	return ps.AddPrivKey(id, sk)
}

func simpleOpt(opt libp2p.Option) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		opts.Opts = append(opts.Opts, opt)
		return
	}
}

type priorityOption struct {
	priority, defaultPriority config.Priority
	opt                       libp2p.Option
}

func prioritizeOptions(opts []priorityOption) libp2p.Option {
	type popt struct {
		priority int64
		opt      libp2p.Option
	}
	enabledOptions := make([]popt, 0, len(opts))
	for _, o := range opts {
		if prio, ok := o.priority.WithDefault(o.defaultPriority); ok {
			enabledOptions = append(enabledOptions, popt{
				priority: prio,
				opt:      o.opt,
			})
		}
	}
	sort.Slice(enabledOptions, func(i, j int) bool {
		return enabledOptions[i].priority > enabledOptions[j].priority
	})
	p2pOpts := make([]libp2p.Option, len(enabledOptions))
	for i, opt := range enabledOptions {
		p2pOpts[i] = opt.opt
	}
	return libp2p.ChainOptions(p2pOpts...)
}
