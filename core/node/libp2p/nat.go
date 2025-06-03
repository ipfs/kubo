package libp2p

import (
	"time"

	config "github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
)

var NatPortMap = simpleOpt(libp2p.NATPortMap())

func AutoNATService(throttle *config.AutoNATThrottleConfig, v1only bool) func() Libp2pOpts {
	return func() (opts Libp2pOpts) {
		opts.Opts = append(opts.Opts, libp2p.EnableNATService())
		if throttle != nil {
			opts.Opts = append(opts.Opts,
				libp2p.AutoNATServiceRateLimit(
					throttle.GlobalLimit,
					throttle.PeerLimit,
					throttle.Interval.WithDefault(time.Minute),
				),
			)
		}

		// While V1 still exists and V2 rollout is in progress
		// (https://github.com/ipfs/kubo/issues/10091) we check a flag that
		// allows users to disable V2 and run V1-only mode
		if !v1only {
			opts.Opts = append(opts.Opts, libp2p.EnableAutoNATv2())
		}
		return opts
	}
}
