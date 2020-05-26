package libp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p"
	metrics "github.com/libp2p/go-libp2p-core/metrics"
	noise "github.com/libp2p/go-libp2p-noise"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	secio "github.com/libp2p/go-libp2p-secio"
	tls "github.com/libp2p/go-libp2p-tls"

	"go.uber.org/fx"
)

// default security transports for libp2p
var defaultSecurityTransports = []string{"tls", "secio", "noise"}

func Transports(pnet struct {
	fx.In
	Fprint PNetFingerprint `optional:"true"`
}) (opts Libp2pOpts) {
	opts.Opts = append(opts.Opts, libp2p.DefaultTransports)
	if pnet.Fprint == nil {
		opts.Opts = append(opts.Opts, libp2p.Transport(libp2pquic.NewTransport))
	}
	return opts
}

func Security(enabled bool, securityTransportOverride []string) interface{} {
	if !enabled {
		return func() (opts Libp2pOpts) {
			// TODO: shouldn't this be Errorf to guarantee visibility?
			log.Warnf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
			opts.Opts = append(opts.Opts, libp2p.NoSecurity)
			return opts
		}
	}

	securityTransports := defaultSecurityTransports
	if len(securityTransportOverride) > 0 {
		securityTransports = securityTransportOverride
	}

	var libp2pOpts []libp2p.Option
	for _, tpt := range securityTransports {
		switch tpt {
		case "tls":
			libp2pOpts = append(libp2pOpts, libp2p.Security(tls.ID, tls.New))
		case "secio":
			libp2pOpts = append(libp2pOpts, libp2p.Security(secio.ID, secio.New))
		case "noise":
			libp2pOpts = append(libp2pOpts, libp2p.Security(noise.ID, noise.New))
		default:
			return fx.Error(fmt.Errorf("invalid security transport specified in config: %s", tpt))
		}
	}

	return func() (opts Libp2pOpts) {
		opts.Opts = append(opts.Opts, libp2p.ChainOptions(libp2pOpts...))
		return opts
	}
}

func BandwidthCounter() (opts Libp2pOpts, reporter *metrics.BandwidthCounter) {
	reporter = metrics.NewBandwidthCounter()
	opts.Opts = append(opts.Opts, libp2p.BandwidthReporter(reporter))
	return opts, reporter
}
