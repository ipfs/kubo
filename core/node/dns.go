package node

import (
	"fmt"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	doh "github.com/libp2p/go-doh-resolver"
	madns "github.com/multiformats/go-multiaddr-dns"
)

func DNSResolver(cfg *config.Config) (*madns.Resolver, error) {
	var opts []madns.Option
	if url := cfg.DNS.DefaultResolver; url != "" {
		if !strings.HasPrefix(url, "https://") {
			return nil, fmt.Errorf("invalid default resolver url: %s", url)
		}
		opts = append(opts, madns.WithDefaultResolver(doh.NewResolver(url)))
	}
	for domain, url := range cfg.DNS.CustomResolvers {
		if !strings.HasPrefix(url, "https://") {
			return nil, fmt.Errorf("invalid domain resolver url for %s: %s", domain, url)
		}
		opts = append(opts, madns.WithDomainResolver(domain, doh.NewResolver(url)))
	}
	return madns.NewResolver(opts...)
}
