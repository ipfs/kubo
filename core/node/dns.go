package node

import (
	"math"
	"time"

	"github.com/ipfs/boxo/gateway"
	config "github.com/ipfs/kubo/config"
	doh "github.com/libp2p/go-doh-resolver"
	madns "github.com/multiformats/go-multiaddr-dns"
)

// Compile-time interface check: *madns.Resolver (returned by gateway.NewDNSResolver
// and madns.NewResolver) must implement madns.BasicResolver for p2pForgeResolver fallback.
var _ madns.BasicResolver = (*madns.Resolver)(nil)

func DNSResolver(cfg *config.Config) (*madns.Resolver, error) {
	var dohOpts []doh.Option
	if !cfg.DNS.MaxCacheTTL.IsDefault() {
		dohOpts = append(dohOpts, doh.WithMaxCacheTTL(cfg.DNS.MaxCacheTTL.WithDefault(time.Duration(math.MaxUint32)*time.Second)))
	}

	// Replace "auto" DNS resolver placeholders with autoconf values
	resolvers := cfg.DNSResolversWithAutoConf()

	// Get base resolver from boxo (handles custom DoH resolvers per eTLD)
	baseResolver, err := gateway.NewDNSResolver(resolvers, dohOpts...)
	if err != nil {
		return nil, err
	}

	// Check if we should skip network DNS lookups for p2p-forge domains
	skipAutoTLSDNS := cfg.AutoTLS.SkipDNSLookup.WithDefault(config.DefaultAutoTLSSkipDNSLookup)
	if !skipAutoTLSDNS {
		// Local resolution disabled, use network DNS for everything
		return baseResolver, nil
	}

	// Build list of p2p-forge domains to resolve locally without network I/O.
	// AutoTLS hostnames encode IP addresses directly (e.g., 1-2-3-4.peerID.libp2p.direct),
	// so DNS lookups are wasteful. We resolve these in-memory when possible.
	forgeDomains := []string{config.DefaultDomainSuffix}
	customDomain := cfg.AutoTLS.DomainSuffix.WithDefault(config.DefaultDomainSuffix)
	if customDomain != config.DefaultDomainSuffix {
		forgeDomains = append(forgeDomains, customDomain)
	}
	forgeResolver := NewP2PForgeResolver(forgeDomains, baseResolver)

	// Register p2p-forge resolver for each domain, fallback to baseResolver for others
	opts := []madns.Option{madns.WithDefaultResolver(baseResolver)}
	for _, domain := range forgeDomains {
		opts = append(opts, madns.WithDomainResolver(domain+".", forgeResolver))
	}

	return madns.NewResolver(opts...)
}
