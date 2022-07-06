package libp2p

import (
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	disc "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/ipfs/go-ipfs/core/node/helpers"
	"go.uber.org/fx"
)

func TopicDiscovery() interface{} {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, host host.Host, cr BaseIpfsRouting) (service discovery.Discovery, err error) {
		baseDisc := disc.NewRoutingDiscovery(cr)
		minBackoff, maxBackoff := time.Second*60, time.Hour
		rng := rand.New(rand.NewSource(rand.Int63()))
		d, err := backoff.NewBackoffDiscovery(
			baseDisc,
			backoff.NewExponentialBackoff(minBackoff, maxBackoff, backoff.FullJitter, time.Second, 5.0, 0, rng),
		)

		if err != nil {
			return nil, err
		}

		return d, nil
	}
}
