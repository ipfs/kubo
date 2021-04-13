package node

import (
	"fmt"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	doh "github.com/libp2p/go-doh-resolver"
	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/miekg/dns"
)

const ethResolverURL = "https://eth.link/dns-query"

func newResolver(url string) (madns.BasicResolver, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("invalid resolver url: %s", url)
	}

	return doh.NewResolver(url), nil
}

func DNSResolver(cfg *config.Config) (*madns.Resolver, error) {
	var opts []madns.Option

	hasEth := false
	for domain, url := range cfg.DNS.Resolvers {
		if !dns.IsFqdn(domain) {
			return nil, fmt.Errorf("invalid domain %s; must be FQDN", domain)
		}

		rslv, err := newResolver(url)
		if err != nil {
			return nil, fmt.Errorf("bad resolver for %s: %w", domain, err)
		}

		if domain != "." {
			opts = append(opts, madns.WithDomainResolver(domain, rslv))
		} else {
			opts = append(opts, madns.WithDefaultResolver(rslv))
		}

		if domain == "eth." {
			hasEth = true
		}
	}

	if !hasEth {
		opts = append(opts, madns.WithDomainResolver("eth.", doh.NewResolver(ethResolverURL)))
	}

	return madns.NewResolver(opts...)
}
