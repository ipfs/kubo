package node

import (
	"math"
	"time"

	"github.com/ipfs/boxo/gateway"
	config "github.com/ipfs/kubo/config"
	doh "github.com/libp2p/go-doh-resolver"
	madns "github.com/multiformats/go-multiaddr-dns"
)

func DNSResolver(cfg *config.Config) (*madns.Resolver, error) {
	var dohOpts []doh.Option
	if !cfg.DNS.MaxCacheTTL.IsDefault() {
		dohOpts = append(dohOpts, doh.WithMaxCacheTTL(cfg.DNS.MaxCacheTTL.WithDefault(time.Duration(math.MaxUint32)*time.Second)))
	}

	// Replace "auto" DNS resolver placeholders with autoconf values
	resolvers := cfg.DNSResolversWithAutoConf()

	return gateway.NewDNSResolver(resolvers, dohOpts...)
}
