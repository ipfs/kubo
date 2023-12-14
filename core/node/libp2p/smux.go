package libp2p

import (
	"fmt"
	"os"

	"github.com/ipfs/kubo/config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
)

func makeSmuxTransportOption(tptConfig config.Transports) (libp2p.Option, error) {
	if prefs := os.Getenv("LIBP2P_MUX_PREFS"); prefs != "" {
		return nil, fmt.Errorf("configuring muxers with LIBP2P_MUX_PREFS is no longer supported, use Swarm.Transports.Multiplexers")
	}
	if tptConfig.Multiplexers.Mplex != 0 {
		return nil, fmt.Errorf("Swarm.Transports.Multiplexers.Mplex is no longer supported, remove it from your config, see https://github.com/libp2p/specs/issues/553")
	}
	if tptConfig.Multiplexers.Yamux < 0 {
		return nil, fmt.Errorf("running libp2p with Swarm.Transports.Multiplexers.Yamux disabled is not supported")
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
