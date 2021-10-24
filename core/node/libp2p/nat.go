package libp2p

import (
	"time"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p"
)

var NatPortMap = simpleOpt(libp2p.NATPortMap())

func AutoNATService(throttle *config.AutoNATThrottleConfig) func() Libp2pOpts {
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
		return opts
	}
}
