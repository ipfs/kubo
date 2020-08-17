package libp2p

import (
	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p"
	noise "github.com/libp2p/go-libp2p-noise"
	secio "github.com/libp2p/go-libp2p-secio"
	tls "github.com/libp2p/go-libp2p-tls"
)

func Security(enabled bool, tptConfig config.Transports) interface{} {
	if !enabled {
		return func() (opts Libp2pOpts) {
			log.Errorf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
			opts.Opts = append(opts.Opts, libp2p.NoSecurity)
			return opts
		}
	}

	// Using the new config options.
	return func() (opts Libp2pOpts) {
		opts.Opts = append(opts.Opts, prioritizeOptions([]priorityOption{{
			priority:        tptConfig.Security.TLS,
			defaultPriority: 100,
			opt:             libp2p.Security(tls.ID, tls.New),
		}, {
			priority:        tptConfig.Security.SECIO,
			defaultPriority: config.Disabled,
			opt:             libp2p.Security(secio.ID, secio.New),
		}, {
			priority:        tptConfig.Security.Noise,
			defaultPriority: 300,
			opt:             libp2p.Security(noise.ID, noise.New),
		}}))
		return opts
	}
}
