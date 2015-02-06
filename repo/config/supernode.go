package config

import "github.com/jbenet/go-ipfs/util/ipfsaddr"

// TODO replace with final servers before merge

// TODO rename
type SupernodeClientConfig struct {
	Servers []string
}

var DefaultGCRServers = []string{
	"/ip4/107.170.212.195/tcp/4001/ipfs/QmVy5xh7sYKyQxHG4ZatHj9cCu1H5PR1LySKeTfLdJxp1b",
	"/ip4/107.170.215.87/tcp/4001/ipfs/QmZDYP9GBjkAmZrS5usSzPnLf41haq6jJhJDmWgnSM4zvW",
}

func initSNRConfig() (*SupernodeClientConfig, error) {
	// TODO perform validation
	return &SupernodeClientConfig{
		Servers: DefaultGCRServers,
	}, nil
}

func (gcr *SupernodeClientConfig) ServerIPFSAddrs() ([]ipfsaddr.IPFSAddr, error) {
	var addrs []ipfsaddr.IPFSAddr
	for _, server := range gcr.Servers {
		addr, err := ipfsaddr.ParseString(server)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}
