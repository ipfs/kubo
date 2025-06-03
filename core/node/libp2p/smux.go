package libp2p

import (
	"errors"
	"os"

	"github.com/ipfs/kubo/config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
)

func makeSmuxTransportOption(tptConfig config.Transports) (libp2p.Option, error) {
	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		return nil, errors.New("configuring muxers with LIBP2P_MUX_PREFS is no longer supported, use Swarm.Transports.Multiplexers")
	}
	if tptConfig.Multiplexers.Yamux < 0 {
		return nil, errors.New("running libp2p with Swarm.Transports.Multiplexers.Yamux disabled is not supported")
	}

	return libp2p.Muxer(yamux.ID, yamux.DefaultTransport), nil
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
