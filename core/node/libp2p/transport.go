package libp2p

import (
	"fmt"
	"os"

	"github.com/ipfs/kubo/config"
	"github.com/ipshipyard/p2p-forge/client"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/metrics"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	webrtc "github.com/libp2p/go-libp2p/p2p/transport/webrtc"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"
	webtransport "github.com/libp2p/go-libp2p/p2p/transport/webtransport"

	"go.uber.org/fx"
)

func Transports(tptConfig config.Transports) interface{} {
	return func(params struct {
		fx.In
		Fprint   PNetFingerprint         `optional:"true"`
		ForgeMgr *client.P2PForgeCertMgr `optional:"true"`
	},
	) (opts Libp2pOpts, err error) {
		privateNetworkEnabled := params.Fprint != nil

		tcpEnabled := tptConfig.Network.TCP.WithDefault(true)
		wsEnabled := tptConfig.Network.Websocket.WithDefault(true)
		if tcpEnabled {
			// TODO(9290): Make WithMetrics configurable
			opts.Opts = append(opts.Opts, libp2p.Transport(tcp.NewTCPTransport, tcp.WithMetrics()))
		}

		if wsEnabled {
			if params.ForgeMgr == nil {
				opts.Opts = append(opts.Opts, libp2p.Transport(websocket.New))
			} else {
				opts.Opts = append(opts.Opts, libp2p.Transport(websocket.New, websocket.WithTLSConfig(params.ForgeMgr.TLSConfig())))
			}
		}

		if tcpEnabled && wsEnabled && os.Getenv("LIBP2P_TCP_MUX") != "false" {
			if privateNetworkEnabled {
				log.Error("libp2p.ShareTCPListener() is not supported in private networks, please disable Swarm.Transports.Network.Websocket or run with LIBP2P_TCP_MUX=false to make this message go away")
			} else {
				opts.Opts = append(opts.Opts, libp2p.ShareTCPListener())
			}
		}

		if tptConfig.Network.QUIC.WithDefault(!privateNetworkEnabled) {
			if privateNetworkEnabled {
				return opts, fmt.Errorf(
					"QUIC transport does not support private networks, please disable Swarm.Transports.Network.QUIC",
				)
			}
			opts.Opts = append(opts.Opts, libp2p.Transport(quic.NewTransport))
		}

		if tptConfig.Network.WebTransport.WithDefault(!privateNetworkEnabled) {
			if privateNetworkEnabled {
				return opts, fmt.Errorf(
					"WebTransport transport does not support private networks, please disable Swarm.Transports.Network.WebTransport",
				)
			}
			opts.Opts = append(opts.Opts, libp2p.Transport(webtransport.New))
		}

		if tptConfig.Network.WebRTCDirect.WithDefault(!privateNetworkEnabled) {
			if privateNetworkEnabled {
				return opts, fmt.Errorf(
					"WebRTC Direct transport does not support private networks, please disable Swarm.Transports.Network.WebRTCDirect",
				)
			}
			opts.Opts = append(opts.Opts, libp2p.Transport(webrtc.New))
		}

		return opts, nil
	}
}

func BandwidthCounter() (opts Libp2pOpts, reporter *metrics.BandwidthCounter) {
	reporter = metrics.NewBandwidthCounter()
	opts.Opts = append(opts.Opts, libp2p.BandwidthReporter(reporter))
	return opts, reporter
}
