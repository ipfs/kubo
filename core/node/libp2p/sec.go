package libp2p

import (
	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p"
	noise "github.com/libp2p/go-libp2p-noise"
	tls "github.com/libp2p/go-libp2p-tls"
)

const secioEnabledWarning = `The SECIO security transport was enabled in the config but is no longer supported.

SECIO disabled by default in go-ipfs 0.7 removed in go-ipfs 0.9. Please remove
Swarm.Transports.Security.SECIO from your IPFS config.`

func Security(enabled bool, tptConfig config.Transports) interface{} {
	if !enabled {
		return func() (opts Libp2pOpts) {
			log.Errorf(`Your IPFS node has been configured to run WITHOUT ENCRYPTED CONNECTIONS.
		You will not be able to connect to any nodes configured to use encrypted connections`)
			opts.Opts = append(opts.Opts, libp2p.NoSecurity)
			return opts
		}
	}

	if _, enabled := tptConfig.Security.SECIO.WithDefault(config.Disabled); enabled {
		log.Error(secioEnabledWarning)
	}

	// Using the new config options.
	return func() (opts Libp2pOpts) {
		opts.Opts = append(opts.Opts, prioritizeOptions([]priorityOption{{
			priority:        tptConfig.Security.TLS,
			defaultPriority: 100,
			opt:             libp2p.Security(tls.ID, tls.New),
		}, {
			priority:        tptConfig.Security.Noise,
			defaultPriority: 300,
			opt:             libp2p.Security(noise.ID, noise.New),
		}}))
		return opts
	}
}
