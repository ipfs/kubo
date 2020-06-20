package libp2p

import (
	"fmt"
	"os"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p"
	smux "github.com/libp2p/go-libp2p-core/mux"
	mplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
)

func yamuxTransport() smux.Multiplexer {
	tpt := *yamux.DefaultTransport
	tpt.AcceptBacklog = 512
	if os.Getenv("YAMUX_DEBUG") != "" {
		tpt.LogOutput = os.Stderr
	}

	return &tpt
}

func makeSmuxTransportOption(tptConfig config.Transports) (libp2p.Option, error) {
	const yamuxID = "/yamux/1.0.0"
	const mplexID = "/mplex/6.7.0"

	ymxtpt := *yamux.DefaultTransport
	ymxtpt.AcceptBacklog = 512

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
			case yamuxID:
				opts = append(opts, libp2p.Muxer(tpt, yamuxTransport))
			case mplexID:
				opts = append(opts, libp2p.Muxer(tpt, mplex.DefaultTransport))
			default:
				return nil, fmt.Errorf("unknown muxer: %s", tpt)
			}
		}
		return libp2p.ChainOptions(opts...), nil
	} else {
		return prioritizeOptions([]priorityOption{{
			priority:        tptConfig.Multiplexers.Yamux,
			defaultPriority: 100,
			opt:             libp2p.Muxer(yamuxID, yamuxTransport),
		}, {
			priority:        tptConfig.Multiplexers.Mplex,
			defaultPriority: 200,
			opt:             libp2p.Muxer(mplexID, mplex.DefaultTransport),
		}}), nil
	}
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
