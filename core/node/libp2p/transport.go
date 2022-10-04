package libp2p

import (
	"fmt"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	webtransport "github.com/libp2p/go-libp2p/p2p/transport/webtransport"

	"go.uber.org/fx"
)

func Transports(tptConfig config.Transports) interface{} {
	return func(pnet struct {
		fx.In
		Fprint PNetFingerprint `optional:"true"`
	}) (opts Libp2pOpts, err error) {
		privateNetworkEnabled := pnet.Fprint != nil

		if tptConfig.Network.TCP.WithDefault(true) {
			// TODO(9290): Make WithMetrics configurable
			opts.Opts = append(opts.Opts, libp2p.Transport(tcp.NewTCPTransport, tcp.WithMetrics()))
		}

		if tptConfig.Network.Websocket.WithDefault(true) {
			opts.Opts = append(opts.Opts, libp2p.Transport(websocket.New))
		}

		if tptConfig.Network.QUIC.WithDefault(!privateNetworkEnabled) {
			if privateNetworkEnabled {
				return opts, fmt.Errorf(
					"The QUIC transport does not support private networks. " +
						"Please disable Swarm.Transports.Network.QUIC.",
				)
			}
			// TODO(9290): Make WithMetrics configurable
			opts.Opts = append(opts.Opts, libp2p.Transport(quic.NewTransport, quic.WithMetrics()))
		}

		// TODO(9292): Remove the false && to allows it enabled by default
		if tptConfig.Network.WebTransport.WithDefault(false && !privateNetworkEnabled) {
			if privateNetworkEnabled {
				return opts, fmt.Errorf(
					"The WebTransport transport does not support private networks. " +
						"Please disable Swarm.Transports.Network.WebTransport.",
				)
			}
			opts.Opts = append(opts.Opts, libp2p.Transport(webtransport.New))
		}

		return opts, nil
	}
}

func BandwidthCounter() (opts Libp2pOpts, reporter *metrics.BandwidthCounter) {
	reporter = metrics.NewBandwidthCounter()
	opts.Opts = append(opts.Opts, libp2p.BandwidthReporter(reporter))
	return opts, reporter
}
