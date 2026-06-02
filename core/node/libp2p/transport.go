package libp2p

import (
	"fmt"
	"net/http"
	"os"
	"time"

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

func Transports(tptConfig config.Transports, gatewayRetrievalTimeout time.Duration) any {
	return func(params struct {
		fx.In
		Fprint        PNetFingerprint          `optional:"true"`
		ForgeMgr      *client.P2PForgeCertMgr  `optional:"true"`
		HTTPProvider  *HTTPProviderHandler     `optional:"true"`
		SelfSignedTLS *SelfSignedTestTLSConfig `optional:"true"`
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
			var wsOpts []any
			// Test escape hatch wins when set: skip the AutoTLS pipeline
			// and feed the WebSocket transport an in-memory self-signed
			// cert. Production paths use ForgeMgr; both are wired
			// optional so only one provider fires per build.
			switch {
			case params.SelfSignedTLS != nil:
				wsOpts = append(wsOpts, websocket.WithTLSConfig(params.SelfSignedTLS.Config))
			case params.ForgeMgr != nil:
				wsOpts = append(wsOpts, websocket.WithTLSConfig(params.ForgeMgr.TLSConfig()))
			}
			// HTTPProvider: when the master switch is on (and AutoTLS is on),
			// expose the trustless gateway handler on the same TCP port as
			// /tls/ws by routing non-WebSocket requests to a fallback handler.
			// The handler itself is wired post-construction by daemon.go
			// because it needs the fully constructed *core.IpfsNode.
			// See HTTPProviderHandler.
			if params.HTTPProvider != nil {
				wsOpts = append(wsOpts, websocket.WithHTTPHandler(params.HTTPProvider))
				// The trustless gateway streams large block/CAR responses,
				// so set only timeouts that are safe for streaming.
				// ReadHeaderTimeout guards only h1 fallback connections
				// against slow-header clients; Go's HTTP/2 server ignores
				// it. IdleTimeout caps idle pooled connections; 60s sits
				// above httpnet's 30s IdleConnTimeout so the client closes
				// idle connections first.
				// WriteByteTimeout closes a stalled writer (a client that
				// stopped reading) without truncating a healthy slow
				// download, since it resets on every byte written.
				// SendPingTimeout reclaims dead h2 connections so they do
				// not pin the resource manager's connection budget.
				// WriteTimeout/ReadTimeout stay unset, as they cap the whole
				// request and would truncate a large download.
				//
				// Both guards sit above Gateway.RetrievalTimeout so the
				// gateway's own timeout (a clean 504 with diagnostics and a
				// recorded metric) fires before we drop the connection.
				connGuard := gatewayRetrievalTimeout + 30*time.Second
				wsOpts = append(wsOpts, websocket.WithHTTPServerConfig(func(s *http.Server) {
					s.ReadHeaderTimeout = 10 * time.Second
					s.IdleTimeout = 60 * time.Second
					s.HTTP2 = &http.HTTP2Config{
						MaxConcurrentStreams: 256,
						WriteByteTimeout:     connGuard,
						SendPingTimeout:      connGuard,
					}
				}))
			}
			opts.Opts = append(opts.Opts, libp2p.Transport(websocket.New, wsOpts...))
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
