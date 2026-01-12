package node

import (
	"math"
	"net"
	"time"

	"github.com/ipfs/boxo/gateway"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/node/libp2p"
	doh "github.com/libp2p/go-doh-resolver"
	madns "github.com/multiformats/go-multiaddr-dns"
	"go.uber.org/fx"
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

// OverrideDefaultResolver replaces net.DefaultResolver with one that uses
// the provided madns.Resolver. This ensures all Go code in the daemon
// (including third-party libraries like p2p-forge/client) respects the
// DNS.Resolvers configuration.
func OverrideDefaultResolver(resolver *madns.Resolver) {
	net.DefaultResolver = libp2p.NewNetResolverFromMadns(resolver)
}

// maybeOverrideDefaultResolver returns an fx.Option that conditionally
// invokes OverrideDefaultResolver based on the DNS.OverrideSystem config flag.
func maybeOverrideDefaultResolver(enabled bool) fx.Option {
	if enabled {
		return fx.Invoke(OverrideDefaultResolver)
	}
	return fx.Options()
}
