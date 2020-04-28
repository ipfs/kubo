package libp2p

import (
	"time"

	"github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p"
)

var NatPortMap = simpleOpt(libp2p.NATPortMap())

func AutoNATService(throttle *config.AutoNATThrottleConfig) func() Libp2pOpts {
	return func() (opts Libp2pOpts) {
		opts.Opts = append(opts.Opts, libp2p.EnableNATService())
		if throttle != nil {
			global := throttle.GlobalLimit
			peer := throttle.PeerLimit
			interval := time.Duration(throttle.Interval)
			if interval == 0 {
				interval = time.Minute
			}
			opts.Opts = append(opts.Opts,
				libp2p.AutoNATServiceRateLimit(global, peer, interval),
			)
		}
		return opts
	}
}
