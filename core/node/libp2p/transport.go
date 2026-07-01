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

// HTTPProvider HTTP server tuning. The trustless gateway streams large
// block/CAR responses, so it sets only streaming-safe timeouts; Transports
// applies each value.
const (
	// httpProviderMaxConcurrentStreams caps parallel HTTP/2 streams per client
	// connection. Bitswap over HTTP (httpnet) opens one connection per peer and
	// multiplexes block requests as separate streams, so this doubles as the
	// per-peer in-flight block ceiling. Go's HTTP/2 server defaults to 250; 256
	// keeps a power-of-two of headroom so a busy client can saturate parallel
	// fetches while bounding per-connection memory. This is a per-connection
	// transport limit; Gateway.MaxConcurrentRequests caps total in-flight
	// requests across all connections at the application layer (429 once
	// exceeded).
	httpProviderMaxConcurrentStreams = 256

	// httpProviderReadHeaderTimeout guards HTTP/1.1 fallback connections
	// against slow-header clients. Go's HTTP/2 server ignores it.
	httpProviderReadHeaderTimeout = 10 * time.Second

	// httpProviderIdleTimeout caps idle pooled connections. It exceeds
	// httpnet's 30s IdleConnTimeout so the client closes idle connections
	// first.
	httpProviderIdleTimeout = 60 * time.Second

	// httpProviderConnGuardMargin pads Gateway.RetrievalTimeout to derive the
	// HTTP/2 WriteByteTimeout and SendPingTimeout, keeping both guards above
	// the gateway's own timeout so the gateway returns a clean 504 (with
	// diagnostics and a recorded metric) before the connection drops.
	httpProviderConnGuardMargin = 30 * time.Second
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
				// WriteByteTimeout resets on every byte written, so it closes a
				// stalled writer (a client that stopped reading) without
				// truncating a healthy slow download. SendPingTimeout reclaims
				// dead h2 connections, freeing the resource manager's
				// connection budget. WriteTimeout and ReadTimeout stay unset:
				// they cap the whole request and would truncate a large
				// download. See the const block above for the rest.
				connGuard := gatewayRetrievalTimeout + httpProviderConnGuardMargin
				wsOpts = append(wsOpts, websocket.WithHTTPServerConfig(func(s *http.Server) {
					s.ReadHeaderTimeout = httpProviderReadHeaderTimeout
					s.IdleTimeout = httpProviderIdleTimeout
					s.HTTP2 = &http.HTTP2Config{
						MaxConcurrentStreams: httpProviderMaxConcurrentStreams,
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
