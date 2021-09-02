package libp2p

import (
	"github.com/libp2p/go-libp2p"
	madns "github.com/multiformats/go-multiaddr-dns"
)

func MultiaddrResolver(rslv *madns.Resolver) (opts Libp2pOpts, err error) {
	opts.Opts = append(opts.Opts, libp2p.MultiaddrResolver(rslv))
	return opts, nil
}
