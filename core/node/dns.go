package node

import (
	config "github.com/ipfs/go-ipfs-config"
	madns "github.com/multiformats/go-multiaddr-dns"
)

func DNSResolver(cfg *config.Config) (*madns.Resolver, error) {
	// TODO custom resolvers from config
	return madns.DefaultResolver, nil
}
