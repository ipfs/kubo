package libp2p

import (
	"fmt"
	"sort"
	"time"

	version "github.com/ipfs/kubo"
	config "github.com/ipfs/kubo/config"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"go.uber.org/fx"
)

var log = logging.Logger("p2pnode")

type Libp2pOpts struct {
	fx.Out

	Opts []libp2p.Option `group:"libp2p"`
}

func ConnectionManager(low, high int, grace time.Duration) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		cm, err := connmgr.NewConnManager(low, high, connmgr.WithGracePeriod(grace))
		if err != nil {
			return opts, err
		}
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

func UserAgent() func() (opts Libp2pOpts, err error) {
	return simpleOpt(libp2p.UserAgent(version.GetUserAgentVersion()))
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
		priority int64 // lower priority values mean higher priority
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
		return enabledOptions[i].priority < enabledOptions[j].priority
	})
	p2pOpts := make([]libp2p.Option, len(enabledOptions))
	for i, opt := range enabledOptions {
		p2pOpts[i] = opt.opt
	}
	return libp2p.ChainOptions(p2pOpts...)
}

func ForceReachability(val *config.OptionalString) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		if val.IsDefault() {
			return
		}
		v := val.WithDefault("unrecognized")
		switch v {
		case "public":
			opts.Opts = append(opts.Opts, libp2p.ForceReachabilityPublic())
		case "private":
			opts.Opts = append(opts.Opts, libp2p.ForceReachabilityPrivate())
		default:
			return opts, fmt.Errorf("unrecognized reachability option: %s", v)
		}
		return
	}
}
