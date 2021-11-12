package node

import (
	"fmt"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	doh "github.com/libp2p/go-doh-resolver"
	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/miekg/dns"
)

var defaultResolvers = map[string]string{
	"eth.":    "https://resolver.cloudflare-eth.com/dns-query",
	"crypto.": "https://resolver.cloudflare-eth.com/dns-query",
}

func newResolver(url string) (madns.BasicResolver, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("invalid resolver url: %s", url)
	}

	return doh.NewResolver(url), nil
}

func DNSResolver(cfg *config.Config) (*madns.Resolver, error) {
	var opts []madns.Option
	var err error

	domains := make(map[string]struct{})           // to track overridden default resolvers
	rslvrs := make(map[string]madns.BasicResolver) // to reuse resolvers for the same URL

	for domain, url := range cfg.DNS.Resolvers {
		if domain != "." && !dns.IsFqdn(domain) {
			return nil, fmt.Errorf("invalid domain %s; must be FQDN", domain)
		}

		domains[domain] = struct{}{}
		if url == "" {
			// allow overriding of implicit defaults with the default resolver
			continue
		}

		rslv, ok := rslvrs[url]
		if !ok {
			rslv, err = newResolver(url)
			if err != nil {
				return nil, fmt.Errorf("bad resolver for %s: %w", domain, err)
			}
			rslvrs[url] = rslv
		}

		if domain != "." {
			opts = append(opts, madns.WithDomainResolver(domain, rslv))
		} else {
			opts = append(opts, madns.WithDefaultResolver(rslv))
		}
	}

	// fill in defaults if not overridden by the user
	for domain, url := range defaultResolvers {
		_, ok := domains[domain]
		if ok {
			continue
		}

		rslv, ok := rslvrs[url]
		if !ok {
			rslv, err = newResolver(url)
			if err != nil {
				return nil, fmt.Errorf("bad resolver for %s: %w", domain, err)
			}
			rslvrs[url] = rslv
		}

		opts = append(opts, madns.WithDomainResolver(domain, rslv))
	}

	return madns.NewResolver(opts...)
}
