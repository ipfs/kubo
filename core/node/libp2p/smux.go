package libp2p

import (
	"fmt"
	"os"
	"strings"

	"github.com/ipfs/kubo/config"

	"github.com/ipfs/kubo/core/node/libp2p/internal/mplex"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
)

func makeSmuxTransportOption(tptConfig config.Transports) (libp2p.Option, error) {
	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		// Using legacy LIBP2P_MUX_PREFS variable.
		log.Error("LIBP2P_MUX_PREFS is now deprecated.")
		log.Error("Use the `Swarm.Transports.Multiplexers' config field.")
		muxers := strings.Fields(prefs)
		enabled := make(map[string]bool, len(muxers))

		var opts []libp2p.Option
		for _, tpt := range muxers {
			if enabled[tpt] {
				return nil, fmt.Errorf(
					"duplicate muxer found in LIBP2P_MUX_PREFS: %s",
					tpt,
				)
			}
			switch tpt {
			case yamux.ID:
				opts = append(opts, libp2p.Muxer(tpt, yamux.DefaultTransport))
			case mplex.ID:
				opts = append(opts, libp2p.Muxer(tpt, mplex.DefaultTransport))
			default:
				return nil, fmt.Errorf("unknown muxer: %s", tpt)
			}
		}
		return libp2p.ChainOptions(opts...), nil
	}
	return prioritizeOptions([]priorityOption{{
		priority:        tptConfig.Multiplexers.Yamux,
		defaultPriority: 100,
		opt:             libp2p.Muxer(yamux.ID, yamux.DefaultTransport),
	}, {
		priority:        tptConfig.Multiplexers.Mplex,
		defaultPriority: config.Disabled,
		opt:             libp2p.Muxer(mplex.ID, mplex.DefaultTransport),
	}}), nil
}

func SmuxTransport(tptConfig config.Transports) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		res, err := makeSmuxTransportOption(tptConfig)
		if err != nil {
			return opts, err
		}
		opts.Opts = append(opts.Opts, res)
		return opts, nil
	}
}
